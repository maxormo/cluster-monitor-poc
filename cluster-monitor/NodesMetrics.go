package cluster_monitor

import (
	"cluster-monitor-poc/entities"
	"cluster-monitor-poc/kubernetes"
	"k8s.io/api/core/v1"
	"time"
)

type NodesMetrics struct {
	Kube                     kubernetes.Kubernetes
	MetricsClient            entities.MetricsClient
	MetricsCollectionTimeout time.Duration
}

func (m NodesMetrics) NodesMetrics() {
	for {
		list := m.Kube.GetNodeList()
		UpdateMetricsForNodes(list, m.MetricsClient)
		time.Sleep(m.MetricsCollectionTimeout)
	}
}

func UpdateMetricsForNodes(nodes []v1.Node, metricsClient entities.MetricsClient) {
	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {
			metricsClient.UpdateNodeCondition(node.Name, string(condition.Type), kubernetes.GetNodeConditionStatus(condition.Status))
		}
	}
}
