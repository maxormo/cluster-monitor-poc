package cluster_monitor

import (
	"cluster-monitor-poc/kubernetes"
	"cluster-monitor-poc/logger"
	"cluster-monitor-poc/provider"
	"k8s.io/api/core/v1"
	"strings"
	"time"
)

//TODO: extract interface and encapsulate initialization of the struct in constructor like object to avoid partial init
type NodeMonitorSettings struct {
	Kube        kubernetes.Kubernetes
	DryRun      bool
	Provider    provider.Provider
	CurrentNode string
	LoopDelay   int
	Log         logger.Logger
	Threshold   int
	Conditions  []kubernetes.Condition
}

func (s NodeMonitorSettings) NodesMonitor() {
	s.Log.Printfln("starting nodes monitor")
	for {
		s.Log.Printfln("scanning through all node for not ready status")

		if len(s.Conditions) == 0 {
			s.Log.Printfln("warning no conditions supplied to check against, wont do anything")
		}

		nodeList := s.Kube.GetNodeList()

		for _, node := range nodeList {
			for _, nodeCondition := range node.Status.Conditions {
				for _, suppliedCondition := range s.Conditions {
					if isMatch(nodeCondition, suppliedCondition) {
						s.Log.Printfln("Node is under %v will do %s", suppliedCondition, suppliedCondition.Action)
						action := s.Kube.GetAction(suppliedCondition)

						e := action(node.Name)

						if e != nil {
							s.Log.Printfln(e.Error())
						}
					}
				}
			}
		}
		s.Log.Printfln("done for now sleeping for %v minutes...", s.LoopDelay)
		time.Sleep(time.Duration(s.LoopDelay) * time.Minute)
	}
}

func isMatch(condition v1.NodeCondition, c kubernetes.Condition) bool {
	return strings.EqualFold(string(condition.Type), string(c.ConditionType)) &&
		strings.EqualFold(string(condition.Status), string(c.ConditionValue)) &&
		time.Now().Add(-c.TimeInEffect).Before(condition.LastTransitionTime.Time)
}
