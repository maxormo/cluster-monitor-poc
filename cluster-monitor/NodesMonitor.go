package cluster_monitor

import (
	"cluster-monitor-poc/kubernetes"
	"cluster-monitor-poc/logger"
	"cluster-monitor-poc/provider"
	v1 "k8s.io/api/core/v1"
	"time"
)

type NodeMonitorSettings struct {
	Kube        kubernetes.Kubernetes
	DryRun      bool
	Provider    provider.Provider
	CurrentNode string
	LoopDelay   int
}

func (s NodeMonitorSettings) NodesMonitor() {
	logger.Printfln("starting nodes monitor")

	for {
		logger.Printfln("scanning through all node for not ready status")

		nodeList := s.Kube.GetNodeList()

		for _, n := range nodeList {
			for _, condition := range n.Status.Conditions {
				if condition.Type == v1.NodeReady && condition.Status != v1.ConditionTrue {

					minutesAgo := time.Now().Add(-time.Duration(30) * time.Minute)

					if condition.LastTransitionTime.Time.Before(minutesAgo) {
						logger.Printfln("node %s is not ready, run hard kill", n.Name)
						s.Kube.HardRestartNode(s.Provider, s.DryRun, n.Name, s.CurrentNode)

					}

				}
			}
		}

		logger.Printfln("nodes monitor is sleeping for %v minutes...", s.LoopDelay)
		time.Sleep(time.Duration(s.LoopDelay) * time.Minute)
	}
}
