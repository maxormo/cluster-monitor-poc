package cluster_monitor

import (
	"cluster-monitor-poc/entities"
	"fmt"
	"time"
)

func GetAgePredicate(age int) func(pod entities.Pod) bool {
	return func(pod entities.Pod) bool {
		return pod.LastTransitionTime.Before(time.Now().Add(-time.Minute * time.Duration(age)))
	}
}

func CleanSoftNodesFromHardNodes(hardKillNodes map[string]int, softKillNodes map[string]int) {
	for node := range hardKillNodes {
		if _, exists := softKillNodes[node]; exists {
			delete(softKillNodes, node)
		}
	}
}

func GetNodesToKill(pods []entities.Pod, prevNode map[string]int, isOldEnough func(pod entities.Pod) bool) map[string]int {

	NotZombieReasons := map[string]bool{"PodCompleted": true, "ContainersNotReady": true}

	nodes := make(map[string]int)
	for _, pod := range pods {
		if pod.ReadyCondition != "True" && isOldEnough(pod) {
			if _, exists := NotZombieReasons[pod.Reason]; !exists {
				nodes[pod.Node] = prevNode[pod.Node] + 1
				fmt.Println(pod)
			}
		}
	}

	return nodes
}
