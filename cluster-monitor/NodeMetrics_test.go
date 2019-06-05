package cluster_monitor

import (
	"cluster-monitor-poc/entities"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"testing"
)

type FakeMetrics struct {
	callTimes int
}

func (FakeMetrics) IncSoftRestart(node string) {
}

func (FakeMetrics) IncHardRestart(node string) {
}

func (FakeMetrics) InitNodeLabel(node string) {
}

func (m *FakeMetrics) UpdateNodeCondition(node, condition string, value entities.NodeConditionStatus) {
	m.callTimes++
}

func GetFakeMetrics() FakeMetrics {
	return FakeMetrics{}
}

func TestNodesMetrics_NodesMetrics(t *testing.T) {
	testConditions := []v1.NodeCondition{
		{Type: v1.NodeReady, Status: v1.ConditionTrue},
		{Type: v1.NodeMemoryPressure, Status: v1.ConditionTrue},
		{Type: "CustomCondition", Status: v1.ConditionTrue},
	}

	testNode := []v1.Node{
		{
			ObjectMeta: v12.ObjectMeta{Name: "testNodeName", Annotations: map[string]string{}},
			Status:     v1.NodeStatus{Conditions: testConditions},
		},
	}

	metrics := GetFakeMetrics()
	UpdateMetricsForNodes(testNode, &metrics)
	if metrics.callTimes != len(testConditions) {
		t.Errorf("missing update metrics call for conditions")
	}

}
