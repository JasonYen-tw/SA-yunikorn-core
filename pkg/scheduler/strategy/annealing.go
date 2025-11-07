package strategy

import (
	"math"
	"math/rand"
	"time"

	"go.uber.org/zap"

	"github.com/apache/yunikorn-core/pkg/common/configs"
	"github.com/apache/yunikorn-core/pkg/common/resources"
	"github.com/apache/yunikorn-core/pkg/log"
	"github.com/apache/yunikorn-core/pkg/metrics"
	"github.com/apache/yunikorn-core/pkg/scheduler/objects"
)

type OnlyNodeIterator struct {
	node *objects.Node
	done bool
}

func (it *OnlyNodeIterator) Next() *objects.Node {
	if it.done {
		return nil
	}
	it.done = true
	return it.node
}

func (it *OnlyNodeIterator) ForEachNode(fn func(*objects.Node) bool) {
	if it.done {
		return
	}
	it.done = true
	fn(it.node)
}

func NewOnlyNodeIterator(node *objects.Node) objects.NodeIterator {
	return &OnlyNodeIterator{node: node}
}

// === SA型別定義 ===
type SimulatedAnnealingScheduler struct {
	InitTemp       float64
	CoolRate       float64
	Iterations     int
	Weights        []float64
	MaxPendingAsks int
	rand           *rand.Rand
}

//	func NewSimulatedAnnealingScheduler() *SimulatedAnnealingScheduler {
//		return &SimulatedAnnealingScheduler{
//			InitTemp:   10.0,
//			CoolRate:   0.95,
//			Iterations: 10,w
//			Weights:    []float64{1, 1, 1, 1},
//			rand:       rand.New(rand.NewSource(time.Now().UnixNano())),
//		}
//	}

func NewSimulatedAnnealingScheduler(param configs.AnnealingParams) *SimulatedAnnealingScheduler {
	return &SimulatedAnnealingScheduler{
		InitTemp:       param.InitTemp,
		CoolRate:       param.CoolRate,
		Iterations:     param.Iterations,
		Weights:        param.Weights,
		MaxPendingAsks: param.MaxPendingAsks,
		rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// 封裝一個"解"
type Solution struct {
	Assign map[*objects.Allocation]*objects.Node
	Score  float64
}

// // ask 是否能放進 node（目前僅驗 CPU/Mem + requiredNode）
// func canFit(ask *objects.Allocation, node *objects.Node) bool {
// 	if rn := ask.GetRequiredNode(); rn != "" && rn != node.NodeID {
// 		return false
// 	}
// 	free := resources.Sub(node.GetCapacity(), node.GetAllocatedResource())
// 	return free.FitIn(ask.GetAllocatedResource())
// }

// 取得所有可用asks與nodes
// func (sa *SimulatedAnnealingScheduler) getAsksAndNodes(p PartitionView) ([]*objects.Allocation, []*objects.Node) {
// 	asks := []*objects.Allocation{}
// 	nodes := []*objects.Node{}

// 	for _, app := range p.GetApplications() {
// 		for _, ask := range app.GetPendingAsks() {
// 			asks = append(asks, ask)
// 		}
// 	}
// 	for _, n := range p.GetNodes() {
// 		nodes = append(nodes, n)
// 	}

// 	return asks, nodes
// }

// ask 是否能放進 node（僅判斷理論容量和指定節點要求）
func canFit(ask *objects.Allocation, node *objects.Node) bool {
	// 檢查是否有指定節點要求
	if requiredNode := ask.GetRequiredNode(); requiredNode != "" && requiredNode != node.NodeID {
		return false
	}
	// 考慮節點當前已分配的資源，确保新增請求仍能放進剩餘容量
	currentUsage := node.GetAllocatedResource()
	expectedUsage := resources.Add(currentUsage, ask.GetAllocatedResource())
	return node.GetCapacity().FitIn(expectedUsage)
}

// 取得所有可用asks與nodes (如果 PartitionView 介面有 RLock/RUnlock)
func (sa *SimulatedAnnealingScheduler) getAsksAndNodes(p PartitionView) ([]*objects.Allocation, []*objects.Node) {
	p.RLock()
	defer p.RUnlock()

	asks := []*objects.Allocation{}
	nodes := []*objects.Node{}

	for _, app := range p.GetApplications() {
		// 注意：app.GetPendingAsks() 內部也需要鎖來保證安全
		// 從 YuniKorn 原始碼看，Application 物件有自己的鎖，所以這裡應該是安全的。
		for _, ask := range app.GetPendingAsks() {
			asks = append(asks, ask)
		}
	}
	for _, n := range p.GetNodes() {
		nodes = append(nodes, n)
	}

	if sa.MaxPendingAsks > 0 && len(asks) > sa.MaxPendingAsks {
		log.Log(log.Scheduler).Debug("annealing ask limit applied",
			zap.String("partition", p.GetName()),
			zap.Int("pendingAsks", len(asks)),
			zap.Int("limit", sa.MaxPendingAsks))
		asks = asks[:sa.MaxPendingAsks]
	}

	return asks, nodes
}

func (sa *SimulatedAnnealingScheduler) RandomInit(asks []*objects.Allocation, nodes []*objects.Node) Solution {
	assign := make(map[*objects.Allocation]*objects.Node, len(asks))
	for _, ask := range asks {
		valid := make([]*objects.Node, 0, len(nodes))
		for _, n := range nodes {
			if canFit(ask, n) {
				valid = append(valid, n)
			}
		}
		// 若沒有合法節點仍先硬塞一個，讓成本函式自己扣分
		if len(valid) == 0 {
			valid = nodes
		}
		assign[ask] = valid[sa.rand.Intn(len(valid))]
	}
	return Solution{Assign: assign}
}

// func (sa *SimulatedAnnealingScheduler) RandomNeighbor(sol Solution, nodes []*objects.Node) Solution {
// 	fmt.Println("[DEBUG] RandomNeighbor started")
// 	assign := make(map[*objects.Allocation]*objects.Node, len(sol.Assign))
// 	for k, v := range sol.Assign {
// 		assign[k] = v
// 	}
// 	for ask := range assign { // 只換一個 ask
// 		valid := make([]*objects.Node, 0, len(nodes))
// 		for _, n := range nodes {
// 			if canFit(ask, n) {
// 				valid = append(valid, n)
// 			}
// 		}
// 		if len(valid) == 0 {
// 			valid = nodes
// 		}
// 		assign[ask] = valid[sa.rand.Intn(len(valid))]
// 		break
// 	}
// 	return Solution{Assign: assign}
// }

func (sa *SimulatedAnnealingScheduler) RandomNeighbor(sol Solution, nodes []*objects.Node) Solution {
	assign := make(map[*objects.Allocation]*objects.Node, len(sol.Assign))
	for k, v := range sol.Assign {
		assign[k] = v
	}

	if len(assign) == 0 {
		return Solution{Assign: assign}
	}

	// 將 map keys 轉為 slice 以便隨機選取
	asks := make([]*objects.Allocation, 0, len(assign))
	for ask := range assign {
		asks = append(asks, ask)
	}

	// 隨機打亂 asks 順序，嘗試找到一個可以移動的 ask
	sa.rand.Shuffle(len(asks), func(i, j int) { asks[i], asks[j] = asks[j], asks[i] })

	for _, askToChange := range asks {
		originalNode := assign[askToChange]

		// 找到所有此 ask 合法的節點
		validNodes := make([]*objects.Node, 0, len(nodes))
		for _, n := range nodes {
			if canFit(askToChange, n) {
				validNodes = append(validNodes, n)
			}
		}

		// 收集所有與原節點不同的合法節點
		otherNodes := make([]*objects.Node, 0, len(validNodes))
		for _, n := range validNodes {
			if n != originalNode {
				otherNodes = append(otherNodes, n)
			}
		}

		// 如果有其他選擇，就隨機選一個並回傳新解
		if len(otherNodes) > 0 {
			assign[askToChange] = otherNodes[sa.rand.Intn(len(otherNodes))]
			return Solution{Assign: assign}
		}
	}

	// 如果遍歷完所有 asks 都無法找到一個可以移動到不同節點的，則回傳原解的拷貝
	return Solution{Assign: assign}
}

// /*  -------- score() : 完整成本函式 -------- */
// func (sa *SimulatedAnnealingScheduler) Score(sol Solution) float64 {
// 	fmt.Println("[DEBUG] score() started")
// 	if len(sol.Assign) == 0 {
// 		return 0
// 	}

// 	penaltyCnt := 0
// 	for ask, n := range sol.Assign {
// 		if !canFit(ask, n) {
// 			penaltyCnt++ // 單純計次，可改成計資源差額
// 		}
// 	}

// 	// ── 1. 聚集各節點已用資源 ────────────────────
// 	nodeUsed := make(map[*objects.Node]*resources.Resource)
// 	for ask, n := range sol.Assign {
// 		if nodeUsed[n] == nil {
// 			nodeUsed[n] = resources.NewResource()
// 		}
// 		nodeUsed[n] = resources.Add(nodeUsed[n], ask.GetAllocatedResource())
// 	}

// 	// ── 2. 計算每節點使用率、整體利用率、stdDev ──
// 	var (
// 		totalUsedCPU, totalCapCPU float64
// 		totalUsedMem, totalCapMem float64
// 		meanUtil, sumSq           float64
// 		nodeCnt                   int
// 	)

// 	for n, used := range nodeUsed {
// 		cap := n.GetCapacity()
// 		// 跳過 cap==0 的節點
// 		if cap.GetCPU() == 0 || cap.GetMemory() == 0 {
// 			continue
// 		}

// 		// 個別使用率
// 		utilCPU := float64(used.GetCPU()) / float64(cap.GetCPU())
// 		utilMem := float64(used.GetMemory()) / float64(cap.GetMemory())

// 		util := (utilCPU + utilMem) / 2.0

// 		// 統計
// 		meanUtil += util
// 		sumSq += util * util
// 		nodeCnt++

// 		// 整體
// 		totalUsedCPU += float64(used.GetCPU())
// 		totalCapCPU += float64(cap.GetCPU())
// 		totalUsedMem += float64(used.GetMemory())
// 		totalCapMem += float64(cap.GetMemory())
// 	}

// 	if nodeCnt == 0 || totalCapCPU+totalCapMem == 0 {
// 		return 0
// 	}

// 	meanUtil /= float64(nodeCnt)
// 	stdDev := math.Sqrt(sumSq/float64(nodeCnt) - meanUtil*meanUtil)

// 	clusterUtil := (totalUsedCPU + totalUsedMem) / (totalCapCPU + totalCapMem)

// 	alpha, beta, gamma := 1.0, 0.5, 10.0
// 	if len(sa.Weights) >= 3 {
// 		alpha, beta, gamma = sa.Weights[0], sa.Weights[1], sa.Weights[2]
// 	}
// 	return alpha*clusterUtil - beta*stdDev - gamma*float64(penaltyCnt)
// }

/*  -------- score() : 完整成本函式 -------- */
// [FIXED] 修正函數簽名，並改進懲罰和利用率的計算邏輯
func (sa *SimulatedAnnealingScheduler) Score(sol Solution, allNodes []*objects.Node) float64 {
	if len(sol.Assign) == 0 {
		return 0
	}

	// ── 1. 聚集各節點的預計新增使用資源 ────────────────────
	nodeDemand := make(map[*objects.Node]*resources.Resource)
	for ask, node := range sol.Assign {
		nodeDemand[node] = resources.Add(nodeDemand[node], ask.GetAllocatedResource())
	}

	// ── 2. 計算懲罰項、使用率、標準差 ──────────────────
	var (
		penalty                    float64
		totalUsedCPU, totalUsedMem float64
		totalCapCPU, totalCapMem   float64
		meanUtil, sumSq            float64
		utilNodeCount              int
	)

	for _, node := range allNodes {
		capacity := node.GetCapacity()
		if capacity == nil {
			continue
		}

		demand := nodeDemand[node]
		currentUsage := node.GetAllocatedResource()
		combinedUsage := resources.Add(currentUsage, demand)

		// 如果節點總使用量超過其總容量，計算懲罰
		if !capacity.FitIn(combinedUsage) {
			excess := resources.Sub(combinedUsage, capacity)
			for name, qty := range excess.Resources {
				if qty <= 0 {
					continue
				}
				capQty := capacity.Resources[name]
				if capQty > 0 {
					penalty += float64(qty) / float64(capQty)
				} else {
					penalty += float64(qty)
				}
			}
		}

		usedCPU := float64(combinedUsage.GetCPU())
		usedMem := float64(combinedUsage.GetMemory())
		capCPU := float64(capacity.GetCPU())
		capMem := float64(capacity.GetMemory())

		if capCPU > 0 || capMem > 0 {
			utilNodeCount++
			utilCPU := 0.0
			if capCPU > 0 {
				utilCPU = usedCPU / capCPU
			}
			utilMem := 0.0
			if capMem > 0 {
				utilMem = usedMem / capMem
			}
			avgUtil := (utilCPU + utilMem) / 2.0
			meanUtil += avgUtil
			sumSq += avgUtil * avgUtil
		}

		totalUsedCPU += usedCPU
		totalUsedMem += usedMem
		totalCapCPU += capCPU
		totalCapMem += capMem
	}

	if utilNodeCount == 0 {
		return -10.0 * penalty
	}

	// 計算標準差和集群利用率
	meanUtil /= float64(utilNodeCount)
	variance := sumSq/float64(utilNodeCount) - meanUtil*meanUtil
	if variance < 0 {
		variance = 0
	}
	stdDev := math.Sqrt(variance)

	clusterUtil := 0.0
	if totalCapCPU+totalCapMem > 0 {
		clusterUtil = (totalUsedCPU + totalUsedMem) / (totalCapCPU + totalCapMem)
	}

	// ── 4. 根據權重計算最終分數 ──────────────────────
	alpha, beta, gamma := 1.0, 0.5, 10.0 // 預設權重
	if len(sa.Weights) >= 3 {
		alpha, beta, gamma = sa.Weights[0], sa.Weights[1], sa.Weights[2]
	}

	// 目標：最大化集群利用率，最小化負載標準差和懲罰
	return alpha*clusterUtil - beta*stdDev - gamma*penalty
}

// // === 主 SA 行為 ===
// func (sa *SimulatedAnnealingScheduler) Schedule(p PartitionView) []*objects.AllocationResult {
// 	fmt.Println("[DEBUG] SimulatedAnnealingScheduler called")
// 	start := time.Now()

// 	// 取得所有可用asks與nodes
// 	asks, nodes := sa.getAsksAndNodes(p)
// 	if len(asks) == 0 || len(nodes) == 0 {
// 		fmt.Println("[DEBUG] No asks or nodes found")
// 		return nil
// 	}
// 	// 產生初始解
// 	curr := sa.RandomInit(asks, nodes)
// 	curr.Score = sa.Score(curr)
// 	best := curr
// 	T := sa.InitTemp
// 	fail := 0

// 	// SA主迴圈
// 	for T > 0.1 {
// 		for i := 0; i < sa.Iterations; i++ {
// 			neighbor := sa.RandomNeighbor(curr, nodes)
// 			neighbor.Score = sa.Score(neighbor)
// 			delta := neighbor.Score - curr.Score
// 			if delta > 0 || sa.rand.Float64() < math.Exp(delta/T) {
// 				curr = neighbor
// 				if curr.Score > best.Score {
// 					best = curr
// 				}
// 			}
// 		}
// 		T *= sa.CoolRate
// 	}

// 	// ==== 關鍵步驟：真的讓這些 ask 被 YuniKorn 綁到 node ====
// 	var results []*objects.AllocationResult

// 	for ask, node := range best.Assign {
// 		// 找到 app
// 		app := p.GetApplication(ask.GetApplicationID())
// 		if app == nil {
// 			continue
// 		}
// 		// 封裝單一 node 的 iterator
// 		nodeIter := func() objects.NodeIterator { return NewOnlyNodeIterator(node) }
// 		// 這裡 headRoom 直接給 nil（或你想計算的 headRoom）
// 		allocResult := app.TryAllocate(
// 			nil,      // headRoom, 你可以用 p.GetTotalNodeResource().Clone()，或 nil
// 			false,    // 不開 preemption
// 			0,        // 不等 preemption
// 			nil,      // preemptAttemptsRemaining
// 			nodeIter, // 只給你 SA 決定的 node
// 			nodeIter, // fullNodeIterator
// 			func(id string) *objects.Node { return node }, // getNodeFn
// 		)
// 		if allocResult != nil && allocResult.ResultType == objects.Allocated {
// 			// 呼叫 partition 分配
// 			result := p.Allocate(allocResult)
// 			if result != nil {
// 				results = append(results, result)
// 			} else {
// 				fail++
// 			}
// 		} else {
// 			fail++
// 		}
// 	}

// 	fmt.Printf("[DEBUG] SA returns %d allocations\n", len(results))

// 	// === 主 SA 行為結尾 ===
// 	if len(results) > 0 {
// 		// 把整批結果存進 partition 緩衝池
// 		p.LoadSAResults(results)
// 	}

// 	metrics.GetSchedulerMetrics().ObserveSA(p.GetName(), time.Since(start), best.Score)
// 	if fail > 0 {
// 		metrics.GetSchedulerMetrics().AddSAFailures(p.GetName(), float64(fail))
// 	}
// 	return nil
// }

// === 主 SA 行為 ===
func (sa *SimulatedAnnealingScheduler) Schedule(p PartitionView) []*objects.AllocationResult {
	partitionName := p.GetName()
	start := time.Now()

	// 步驟 1: 獲取當前調度週期的 asks 和 nodes 快照
	asks, nodes := sa.getAsksAndNodes(p)
	log.Log(log.Scheduler).Info("annealing initialization",
		zap.String("partition", partitionName),
		zap.Int("pendingAsks", len(asks)),
		zap.Int("availableNodes", len(nodes)),
		zap.Float64("initTemp", sa.InitTemp),
		zap.Float64("coolRate", sa.CoolRate),
		zap.Int("iterationsPerStep", sa.Iterations))
	if len(asks) == 0 || len(nodes) == 0 {
		log.Log(log.Scheduler).Debug("annealing skipped - insufficient workload",
			zap.String("partition", partitionName),
			zap.Int("pendingAsks", len(asks)),
			zap.Int("availableNodes", len(nodes)))
		return nil
	}

	// 步驟 2: SA 主循環，找到最佳解 'best'
	curr := sa.RandomInit(asks, nodes)
	// [FIXED] 傳入 allNodes
	curr.Score = sa.Score(curr, nodes)
	best := curr
	T := sa.InitTemp
	temperatureStep := 0

	for T > 0.1 {
		log.Log(log.Scheduler).Debug("annealing temperature iteration",
			zap.String("partition", partitionName),
			zap.Float64("temperature", T),
			zap.Int("step", temperatureStep))
		for i := 0; i < sa.Iterations; i++ {
			neighbor := sa.RandomNeighbor(curr, nodes)
			// [FIXED] 傳入 allNodes
			neighbor.Score = sa.Score(neighbor, nodes)
			delta := neighbor.Score - curr.Score
			if delta > 0 || sa.rand.Float64() < math.Exp(delta/T) {
				curr = neighbor
				if curr.Score > best.Score {
					best = curr
					log.Log(log.Scheduler).Debug("annealing best candidate updated",
						zap.String("partition", partitionName),
						zap.Float64("score", best.Score),
						zap.Int("assignments", len(best.Assign)),
						zap.Int("innerIteration", i),
						zap.Int("temperatureStep", temperatureStep))
				}
			}
		}
		T *= sa.CoolRate
		temperatureStep++
	}

	// 步驟 3: [FIXED] 將最佳解打包成 AllocationResult，作為「建議」返回
	// 不再執行任何分配，只產生決策。
	var results []*objects.AllocationResult
	for ask, node := range best.Assign {
		results = append(results, &objects.AllocationResult{
			ResultType: objects.Allocated, // 總是標記為 Allocated，由主循環驗證
			Request:    ask,
			NodeID:     node.NodeID,
		})
	}

	duration := time.Since(start)
	log.Log(log.Scheduler).Info("annealing search complete",
		zap.String("partition", partitionName),
		zap.Duration("duration", duration),
		zap.Float64("bestScore", best.Score),
		zap.Int("candidates", len(results)))
	metrics.GetSchedulerMetrics().ObserveSA(partitionName, duration, best.Score)
	if len(results) > 0 {
		log.Log(log.Scheduler).Info("annealing allocation candidates ready",
			zap.String("partition", partitionName),
			zap.Int("candidates", len(results)))
	}

	return results
}
