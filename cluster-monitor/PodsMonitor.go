package cluster_monitor

import (
	"cluster-monitor-poc/entities"
	"cluster-monitor-poc/kubernetes"
	"cluster-monitor-poc/logger"
	"cluster-monitor-poc/provider"
	"time"
)

type PodsMonitorSettings struct {
	Kube        kubernetes.Kubernetes
	DryRun      bool
	Provider    provider.Provider
	CurrentNode string
	LoopDelay   int

	Collections     int
	CollectionDelay int

	SoftRebootPredicate PodPredicate
	HardRebootPredicate PodPredicate

	Namespace string
	Log       logger.Logger
}

func (s PodsMonitorSettings) PodsMonitor() {
	s.Log.Printfln("starting pods monitor")

	for {
		var convertedPods []entities.Pod
		softKillNodes := make(map[string]int)
		hardKillNodes := make(map[string]int)

		for i := 0; i < s.Collections; i++ {
			pods := s.Kube.GetAllPods()

			for _, pod := range pods {
				convertedPods = append(convertedPods, kubernetes.ConvertPod(pod))
			}

			softKillNodes = GetNodesToKill(convertedPods, softKillNodes, s.SoftRebootPredicate)

			if len(softKillNodes) == 0 {
				break
			}
			hardKillNodes = GetNodesToKill(convertedPods, hardKillNodes, s.HardRebootPredicate)

			time.Sleep(time.Duration(s.CollectionDelay) * time.Second)
		}

		CleanupFlakyNodes(softKillNodes, s.Collections)
		CleanupFlakyNodes(hardKillNodes, s.Collections)

		CleanSoftNodesFromHardNodes(hardKillNodes, softKillNodes)
		s.Log.Printfln("soft candidates: ")
		for e := range softKillNodes {
			s.Log.Printfln(e)
		}

		s.Log.Printfln("hard candidates:")

		for e := range hardKillNodes {
			print(e)
		}

		s.Kube.SetSoftRebootAnnotation(s.DryRun, s.Namespace, softKillNodes)
		s.Kube.HardRestart(s.Provider, s.DryRun, hardKillNodes, s.CurrentNode)
		s.Log.Printfln("done for now sleeping for %v minutes...", s.LoopDelay)
		time.Sleep(time.Duration(s.LoopDelay) * time.Minute)
	}
}

func CleanupFlakyNodes(survivedNodes map[string]int, collections int) {
	for k, v := range survivedNodes {
		if v < collections {
			delete(survivedNodes, k)
		}
	}
}
