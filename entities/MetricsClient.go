package entities

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

type MetricsClient interface {
	IncSoftRestart(node string)
	IncHardRestart(node string)
	InitNodeLabel(node string)
}

type metricsClient struct {
	nodeHardRestarts prometheus.CounterVec
	nodeSoftRestarts prometheus.CounterVec
}

func InitMetrics() MetricsClient {
	mc := metricsClient{
		nodeSoftRestarts: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "cluster_monitor_soft_restart",
			Help: "The total number of node hard restarts",
		}, []string{"node"}),

		nodeHardRestarts: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "cluster_monitor_hard_restart",
			Help: "The total number of os reboot",
		}, []string{"node"}),
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(mc.nodeHardRestarts, mc.nodeSoftRestarts)

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	return mc
}

func (m metricsClient) IncSoftRestart(node string) {
	m.nodeSoftRestarts.WithLabelValues(node).Inc()
}

func (m metricsClient) IncHardRestart(node string) {
	m.nodeHardRestarts.WithLabelValues(node).Inc()
}

func (m metricsClient) InitNodeLabel(node string) {
	m.nodeSoftRestarts.WithLabelValues(node).Add(0)

	m.nodeHardRestarts.WithLabelValues(node).Add(0)
}
