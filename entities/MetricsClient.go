package entities

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

type MetricsClient interface {
	IncSoftRestart()
	IncHardRestart()
}

type metricsClient struct {
	nodeHardRestarts prometheus.Counter
	nodeSoftRestarts prometheus.Counter
}

func InitMetrics() MetricsClient {
	mc := metricsClient{
		nodeSoftRestarts: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cluster_monitor_hard_restart",
			Help: "The total number of node hard restarts",
		}),
		nodeHardRestarts: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cluster_monitor_soft_restart",
			Help: "The total number of os reboot",
		}),
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(mc.nodeHardRestarts, mc.nodeSoftRestarts)

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	return mc
}

func (m metricsClient) IncSoftRestart() {
	m.nodeSoftRestarts.Inc()
}

func (m metricsClient) IncHardRestart() {
	m.nodeHardRestarts.Inc()
}
