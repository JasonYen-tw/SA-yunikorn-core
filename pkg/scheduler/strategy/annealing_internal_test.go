/*
 Licensed to the Apache Software Foundation (ASF) under one
 or more contributor license agreements.  See the NOTICE file
 distributed with this work for additional information
 regarding copyright ownership.  The ASF licenses this file
 to you under the Apache License, Version 2.0 (the
 "License"); you may not use this file except in compliance
 with the License.  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package strategy

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/apache/yunikorn-core/pkg/common/resources"
	"github.com/apache/yunikorn-core/pkg/scheduler/objects"
	"github.com/apache/yunikorn-scheduler-interface/lib/go/si"
)

func TestCanFitRespectsAllocatedResources(t *testing.T) {
	node := objects.NewNode(&si.NodeInfo{
		NodeID: "node-1",
		SchedulableResource: (&si.Resource{
			Resources: map[string]*si.Quantity{
				"vcore":  {Value: 10},
				"memory": {Value: 100},
			},
		}),
	})

	existing := objects.NewAllocationFromSI(&si.Allocation{
		AllocationKey:    "existing",
		ApplicationID:    "app-existing",
		NodeID:           node.NodeID,
		ResourcePerAlloc: resources.NewResourceFromMap(map[string]resources.Quantity{"vcore": 6, "memory": 60}).ToProto(),
	})
	existing.SetNodeID(node.NodeID)
	node.AddAllocation(existing)

	askFits := objects.NewAllocationFromSI(&si.Allocation{
		AllocationKey:    "ask-fit",
		ApplicationID:    "app-1",
		ResourcePerAlloc: resources.NewResourceFromMap(map[string]resources.Quantity{"vcore": 2, "memory": 20}).ToProto(),
	})
	assert.Assert(t, canFit(askFits, node), "ask should fit when respecting remaining resources")

	askTooBig := objects.NewAllocationFromSI(&si.Allocation{
		AllocationKey:    "ask-over",
		ApplicationID:    "app-2",
		ResourcePerAlloc: resources.NewResourceFromMap(map[string]resources.Quantity{"vcore": 5, "memory": 50}).ToProto(),
	})
	assert.Assert(t, !canFit(askTooBig, node), "ask should not fit when exceeding remaining resources")
}
