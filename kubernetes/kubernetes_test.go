package kubernetes

import (
	"cluster-monitor-poc/entities"
	"cluster-monitor-poc/logger"
	"cluster-monitor-poc/provider"
	"k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

var (
	mertr = entities.InitMetrics()
)

func getFake(objects ...runtime.Object) Kubernetes {
	node := &v1.Node{ObjectMeta: v12.ObjectMeta{Name: "NodeToRestart", Annotations: map[string]string{}}}

	objects = append(objects, node)

	return Kubernetes{

		Kubeclient:    fake.NewSimpleClientset(objects...),
		metricsClient: mertr,
		log:           logger.GetLogger("test"),
	}
}

func TestGetNodeList(t *testing.T) {

	kubernetes := getFake()

	list := kubernetes.GetNodeList()
	if len(list) == 0 {
		t.Fail()
	}
}
func TestCordoneNode(t *testing.T) {

	kubernetes := getFake(&v1.Node{ObjectMeta: v12.ObjectMeta{Name: "one"}})

	kubernetes.CordonNode("one")
	list := kubernetes.GetNodeList()

	if list[0].Spec.Unschedulable != true {
		t.Errorf("node was not cordoned")
	}

}

func TestUnCordoneNode(t *testing.T) {

	kubernetes := getFake(&v1.Node{ObjectMeta: v12.ObjectMeta{Name: "one"}})

	kubernetes.UncordonNode("one")
	list := kubernetes.GetNodeList()

	if list[0].Spec.Unschedulable == true {
		t.Errorf("node was not uncordoned")
	}

}
func TestHardRestartNode(t *testing.T) {

	kubernetes := getFake()

	kubernetes.HardRestartNode(provider.NoopProvider(), false, "NodeToRestart", "currentNode")

	// verify that we at least are not failing
	//TODO: add provider mock and verify execution of the restart method
}

func TestHardRestartNodeForSelf(t *testing.T) {

	kubernetes := getFake()

	kubernetes.HardRestartNode(provider.NoopProvider(), false, "NodeToRestart", "NodeToRestart")

	// verify that we at least are not failing
	//TODO: add provider mock and verify that we are not restarting itself
}

func TestDrainNode(t *testing.T) {

	kubernetes := getFake(&v1.Node{ObjectMeta: v12.ObjectMeta{Name: "NodeToDrain"}})

	kubernetes.DrainNode("NodeToDrain")

	// verify that we at least are not failing
	//TODO: add assertions
}

func TestEvictPods(t *testing.T) {

	kubernetes := getFake(&v1.Node{ObjectMeta: v12.ObjectMeta{Name: "NodeToEvictPods"}})

	kubernetes.EvictPods("NodeToEvictPods")

	// verify that we at least are not failing
	//TODO: add assertions
}

func TestNodeIsNotReady(t *testing.T) {

	kubernetes := getFake()

	_, b := kubernetes.IsNotReadyNode(v1.Node{ObjectMeta: v12.ObjectMeta{Name: "one"}})

	if b {
		t.Fail()
	}
}

func TestNodeIsReady(t *testing.T) {

	kubernetes := getFake()
	var conditions []v1.NodeCondition
	conditions = append(conditions, v1.NodeCondition{Type: v1.NodeReady, Status: v1.ConditionTrue})

	_, b := kubernetes.IsNotReadyNode(v1.Node{ObjectMeta: v12.ObjectMeta{Name: "one"}, Status: v1.NodeStatus{Conditions: conditions}})

	if b {
		t.Errorf("node is expected to be ready")
	}
}

// just an example of negative test with panic
//func TestGetNodeListShouldFail(t *testing.T) {
//	defer func() {
//		if r := recover(); r == nil {
//			t.Errorf("The code did not panic")
//		} else if r.(error).Error() != "hello" {
//			t.Errorf("expecting correct error propagation")
//		}
//	}()
//	node := &v1.Node{ObjectMeta: v12.ObjectMeta{Name: "one"}}
//
//	kubernetes := getFake(node)
//
//	kubernetes.Kubeclient.(*fake.Clientset).Fake.AddReactor("list", "*", func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
//		println("code is running")
//		return true, nil, errors.New("hello")
//	})
//
//	kubernetes.GetNodeList()
//
//}
