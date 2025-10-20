package strategy_test

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/apache/yunikorn-core/pkg/common/configs"
	"github.com/apache/yunikorn-core/pkg/common/resources"
	"github.com/apache/yunikorn-core/pkg/common/security"
	"github.com/apache/yunikorn-core/pkg/metrics"
	"github.com/apache/yunikorn-core/pkg/scheduler"
	"github.com/apache/yunikorn-core/pkg/scheduler/objects"
	"github.com/apache/yunikorn-core/pkg/scheduler/strategy"
	"github.com/apache/yunikorn-scheduler-interface/lib/go/si"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// --- 測試輔助 ---

const saConfig = `
partitions:
  - name: default
    strategy: "annealing"
    annealing_params:
      initTemp: 10
      coolRate: 0.9
      iterations: 20
      weights: [1, 0.5, 10]
    queues:
      - name: root
        submitacl: '*'
        queues:
          - name: default
`

func newTestNode(nodeID string, cpu, mem int64) *objects.Node {
	resMap := map[string]*si.Quantity{"vcore": {Value: cpu}, "memory": {Value: mem}}
	return objects.NewNode(&si.NodeInfo{
		NodeID:              nodeID,
		SchedulableResource: &si.Resource{Resources: resMap},
	})
}

func newTestAsk(key, appID string, cpu, mem int64) *objects.Allocation {
	return objects.NewAllocationFromSI(&si.Allocation{
		AllocationKey:    key,
		ApplicationID:    appID,
		PartitionName:    "default",
		ResourcePerAlloc: resources.NewResourceFromMap(map[string]resources.Quantity{"vcore": resources.Quantity(cpu), "memory": resources.Quantity(mem)}).ToProto(),
	})
}

func newApplication(t *testing.T, appID, partition, queueName string) *objects.Application {
	user := security.UserGroup{User: "testuser", Groups: []string{"testgroup"}}
	siApp := &si.AddApplicationRequest{
		ApplicationID: appID,
		QueueName:     queueName,
		PartitionName: partition,
	}
	app := objects.NewApplication(siApp, user, nil, "rmID")

	// 創建根隊列和默認隊列
	rootQueue, err := objects.NewConfiguredQueue(configs.QueueConfig{
		Name:   "root",
		Parent: true,
	}, nil)
	assert.NilError(t, err)

	defaultQueue, err := objects.NewConfiguredQueue(configs.QueueConfig{
		Name:   "default",
		Parent: false,
	}, rootQueue)
	assert.NilError(t, err)

	// 設置隊列關係
	app.SetQueue(defaultQueue)

	return app
}

// MockPartitionView 用於在單元測試中模擬 PartitionContext 的行為
type MockPartitionView struct {
	apps  []*objects.Application
	nodes []*objects.Node
}

func (mpv *MockPartitionView) GetName() string           { return "mock_partition" }
func (mpv *MockPartitionView) GetNodes() []*objects.Node { return mpv.nodes }
func (mpv *MockPartitionView) GetApplication(appID string) *objects.Application {
	for _, app := range mpv.apps {
		if app.ApplicationID == appID {
			return app
		}
	}
	return nil
}
func (mpv *MockPartitionView) GetApplications() []*objects.Application { return mpv.apps }
func (mpv *MockPartitionView) RLock()                                  {}
func (mpv *MockPartitionView) RUnlock()                                {}

// 添加缺少的方法
func (mpv *MockPartitionView) Allocate(result *objects.AllocationResult) *objects.AllocationResult {
	return result
}

func (mpv *MockPartitionView) LoadSAResults(results []*objects.AllocationResult) {
	// 在測試中不需要實際實現
}

// --- 保留的測試：配置加載 ---

func Test_YAML_Triggers_SAInstance(t *testing.T) {
	_, err := configs.LoadSchedulerConfigFromByteArray([]byte(saConfig))
	assert.NilError(t, err)

	cc, err := scheduler.NewClusterContext("rmUT", "policyUT", []byte(saConfig))
	assert.NilError(t, err)

	part := cc.GetPartition("[rmUT]default")
	assert.Assert(t, part != nil, "partition not created")
	assert.Assert(t, part.SAInstance != nil, "SAInstance should be initialised when strategy==annealing")
}

// --- 新的單元測試：取代所有集成測試 ---

func Test_SA_Schedule_Returns_Valid_Allocation_Candidates(t *testing.T) {
	// 步驟 1: 設置測試環境 (手動創建物件)
	node1 := newTestNode("node-1", 10, 1000)
	node2 := newTestNode("node-2", 10, 1000)
	app1 := newApplication(t, "app-1", "mock_partition", "root.default")
	ask1 := newTestAsk("ask-1-1", "app-1", 1, 100)
	ask2 := newTestAsk("ask-1-2", "app-1", 2, 200)
	_ = app1.AddAllocationAsk(ask1)
	_ = app1.AddAllocationAsk(ask2)

	// 步驟 2: 創建模擬的 PartitionView
	mockPartition := &MockPartitionView{
		apps:  []*objects.Application{app1},
		nodes: []*objects.Node{node1, node2},
	}

	// 步驟 3: 創建被測試的 SA 實例
	saScheduler := strategy.NewSimulatedAnnealingScheduler(configs.AnnealingParams{
		InitTemp:   10,
		CoolRate:   0.9,
		Iterations: 50,
		Weights:    []float64{1.0, 0.5, 10.0},
	})

	// 步驟 4: 直接調用被測試的方法
	results := saScheduler.Schedule(mockPartition)

	// 步驟 5: 驗證輸出結果
	assert.Equal(t, 2, len(results), "SA 應該為 2 個請求都產生分配建議")

	allocatedAsks := make(map[string]bool)
	for _, res := range results {
		assert.Equal(t, objects.Allocated, res.ResultType, "結果類型應為 Allocated")
		nodeIsValid := res.NodeID == "node-1" || res.NodeID == "node-2"
		assert.Assert(t, nodeIsValid, "分配的節點 ID '%s' 無效", res.NodeID)
		allocatedAsks[res.Request.GetAllocationKey()] = true
	}

	assert.Assert(t, allocatedAsks["ask-1-1"], "ask-1-1 應該被分配")
	assert.Assert(t, allocatedAsks["ask-1-2"], "ask-1-2 應該被分配")
}

// --- 保留並優化的測試：核心演算法單元測試 ---

func TestSAInitialization(t *testing.T) {
	params := configs.AnnealingParams{
		InitTemp:   10.0,
		CoolRate:   0.95,
		Iterations: 100,
		Weights:    []float64{1.0, 0.5, 10.0},
	}
	sa := strategy.NewSimulatedAnnealingScheduler(params)
	assert.Equal(t, 10.0, sa.InitTemp)
	assert.Equal(t, 0.95, sa.CoolRate)
	assert.Equal(t, 100, sa.Iterations)
	assert.Equal(t, 3, len(sa.Weights))
}

func TestSAScore_With_Penalty(t *testing.T) {
	sa := strategy.NewSimulatedAnnealingScheduler(configs.AnnealingParams{Weights: []float64{1.0, 0.5, 10.0}})
	nodes := []*objects.Node{newTestNode("node-1", 10, 100)}

	// 預先在節點上放入現有負載，模擬實際叢集狀態
	existing := newTestAsk("existing", "app-existing", 6, 60)
	existing.SetNodeID(nodes[0].NodeID)
	nodes[0].AddAllocation(existing)

	// Case 1: 合法的解，沒有懲罰
	askFit := newTestAsk("ask-fit", "app-1", 2, 20)
	solFit := strategy.Solution{Assign: map[*objects.Allocation]*objects.Node{askFit: nodes[0]}}
	scoreFit := sa.Score(solFit, nodes)

	// Case 2: 不合法的解（超分），應該有懲罰
	askOver := newTestAsk("ask-over", "app-1", 5, 50) // 資源需求超過節點剩餘容量
	solOver := strategy.Solution{Assign: map[*objects.Allocation]*objects.Node{askOver: nodes[0]}}
	scoreOver := sa.Score(solOver, nodes)

	// 驗證有懲罰的分數遠低於沒有懲罰的分數
	assert.Assert(t, scoreFit > scoreOver, "Score with penalty should be lower than score without penalty")
}

func TestSARandomNeighbor_Guaranteed_Change(t *testing.T) {
	sa := strategy.NewSimulatedAnnealingScheduler(configs.AnnealingParams{})
	nodes := []*objects.Node{
		newTestNode("node1", 10, 10),
		newTestNode("node2", 8, 8),
	}
	app := newApplication(t, "app1", "mock_partition", "root.default")
	ask := newTestAsk("ask1", "app1", 2, 2)
	_ = app.AddAllocationAsk(ask)
	initSol := strategy.Solution{Assign: map[*objects.Allocation]*objects.Node{ask: nodes[0]}}

	// 在有多個節點可選的情況下，重複執行多次，一定會遇到不同的鄰居
	foundDifferent := false
	for i := 0; i < 30; i++ { // 增加嘗試次數以應對隨機性
		neighborSol := sa.RandomNeighbor(initSol, nodes)
		if neighborSol.Assign[ask] != initSol.Assign[ask] {
			foundDifferent = true
			break
		}
	}
	assert.Assert(t, foundDifferent, "RandomNeighbor failed to produce a different solution when alternatives exist")
}

// --- 保留並優化的測試：指標單元測試 ---

func TestSAMetrics_Observe(t *testing.T) {
	// 設置一個獨立的 metrics 實例以避免測試間干擾
	metrics.Reset()
	met := metrics.GetSchedulerMetrics()
	partitionName := "default"

	// 執行觀測
	met.ObserveSA(partitionName, 100*time.Millisecond, 42.0)
	met.ObserveSA(partitionName, 200*time.Millisecond, 45.0) // 最佳分數
	met.ObserveSA(partitionName, 50*time.Millisecond, 30.0)  // 較差的分數

	// 驗證最佳分數
	bestScore := testutil.ToFloat64(met.GetSAScore(partitionName))
	assert.Equal(t, 45.0, bestScore, "Best score was not updated correctly")
}
