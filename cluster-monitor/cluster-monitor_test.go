package cluster_monitor

import (
	. "cluster-monitor-poc/entities"
	"github.com/stretchr/testify/assert"
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
