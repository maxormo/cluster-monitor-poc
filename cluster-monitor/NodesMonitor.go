package cluster_monitor

import (
	"cluster-monitor-poc/kubernetes"
	"cluster-monitor-poc/logger"
	"cluster-monitor-poc/provider"
	"time"
)

type NodeMonitorSettings struct {
	Kube        kubernetes.Kubernetes
	DryRun      bool
	Provider    provider.Provider
	CurrentNode string
	LoopDelay   int
	Log         logger.Logger
	Threshold   int
}

func (s NodeMonitorSettings) NodesMonitor() {
	s.Log.Printfln("starting nodes monitor")

	for {
		s.Log.Printfln("scanning through all node for not ready status")

		nodeList := s.Kube.GetNodeList()

		for _, n := range nodeList {
			if condition, isReady := s.Kube.IsReadyNode(n); isReady {
				minutesAgo := time.Now().Add(-time.Duration(s.Threshold) * time.Minute)

				if condition.LastTransitionTime.Time.Before(minutesAgo) {
					s.Log.Printfln("node %s is not ready, run hard kill", n.Name)
					s.Kube.HardRestartNode(s.Provider, s.DryRun, n.Name, s.CurrentNode)
				}
			}
		}
		s.Log.Printfln("done for now sleeping for %v minutes...", s.LoopDelay)
		time.Sleep(time.Duration(s.LoopDelay) * time.Minute)
	}
}
