package strategy

import (
	"github.com/apache/yunikorn-core/pkg/scheduler/objects"
)

type PartitionView interface {
	GetApplications() []*objects.Application
	GetNodes() []*objects.Node
	GetApplication(id string) *objects.Application
	GetName() string
	RLock()
	RUnlock()
}

type SchedulerStrategy interface {
	Schedule(p PartitionView) []*objects.AllocationResult
}
