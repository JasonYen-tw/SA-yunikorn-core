package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/apache/yunikorn-core/pkg/log"
)

func registerCollector(metric prometheus.Collector) {
	if err := prometheus.Register(metric); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return
		}
		log.Log(log.Metrics).Warn("failed to register metrics collector", zap.Error(err))
	}
}
