package cluster_monitor

import (
	. "cluster-monitor-poc/entities"
	"cluster-monitor-poc/kubernetes"
	"github.com/stretchr/testify/assert"
	core "k8s.io/api/core/v1"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func getPodsList() (pods []Pod) {
	pods = make([]Pod, 2)
	pods[0] = Pod{Node: "node1", LastTransitionTime: time.Now().Add(-time.Second * 100)}
	pods[1] = Pod{Node: "node2", LastTransitionTime: time.Now()}
	return pods
}

func TestGetAgePredicate(t *testing.T) {
	pred := GetAgePredicate(10)

	pod := Pod{Node: "test", LastTransitionTime: time.Now().Add(-time.Second * 601)}
	assert.True(t, pred(pod))

	pod = Pod{Node: "test", LastTransitionTime: time.Now().Add(-time.Second * 599)}
	assert.False(t, pred(pod))

}

func TestGetSeconds(t *testing.T) {
	println(strconv.FormatInt(time.Now().Unix(), 10))
}
func TestGetSoftKillNodes(t *testing.T) {
	pods := getPodsList()
	nodes := GetNodesToKill(pods, make(map[string]int), func(pod Pod) bool {
		return pod.LastTransitionTime.Before(time.Now().Add(-time.Second * 10))
	})
	assert.ObjectsAreEqual(1, len(nodes))
}

func TestParseConditions(t *testing.T) {
	cases := []struct {
		name       string
		conditions []string
		expect     []kubernetes.Condition
	}{
		{
			name:       "OldFormat=Ready",
			conditions: []string{"Ready=True,1s,Drain"},
			expect:     []kubernetes.Condition{{ConditionType: "Ready", ConditionValue: "True", TimeInEffect: time.Second * 1, Action: "Drain"}},
		},
		{
			name:       "Mixed",
			conditions: []string{"Ready=False,30m,Drain", "OutOfDisk=True,10m,Restart"},
			expect: []kubernetes.Condition{
				{ConditionType: core.NodeConditionType("Ready"), ConditionValue: core.ConditionStatus("False"), TimeInEffect: 30 * time.Minute, Action: "Drain"},
				{ConditionType: core.NodeConditionType("OutOfDisk"), ConditionValue: core.ConditionStatus("True"), TimeInEffect: 10 * time.Minute, Action: "Restart"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed := kubernetes.ParseConditions(tc.conditions)
			if !reflect.DeepEqual(tc.expect, parsed) {
				t.Errorf("expect %v, got: %v", tc.expect, parsed)
			}
		})
	}
}
