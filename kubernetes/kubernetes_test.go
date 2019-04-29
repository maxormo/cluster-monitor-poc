package kubernetes

import (
	"k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func getFake(objects ...runtime.Object) Kubernetes {
	return Kubernetes{
		Kubeclient: fake.NewSimpleClientset(objects...),
	}
}

func TestGetNodeList(t *testing.T) {
	node := &v1.Node{ObjectMeta: v12.ObjectMeta{Name: "one"}}

	kubernetes := getFake(node)

	list := kubernetes.GetNodeList()
	if len(list) == 0 {
		t.Fail()
	}
}
func TestCordoneNode(t *testing.T) {
	node := &v1.Node{ObjectMeta: v12.ObjectMeta{Name: "one"}}

	kubernetes := getFake(node)

	kubernetes.CordonNode("one")
	list := kubernetes.GetNodeList()

	if list[0].Spec.Unschedulable != true {
		t.Errorf("node was not cordoned")
	}

}

func TestUnCordoneNode(t *testing.T) {
	node := &v1.Node{ObjectMeta: v12.ObjectMeta{Name: "one"}}

	kubernetes := getFake(node)

	kubernetes.UncordonNode("one")
	list := kubernetes.GetNodeList()

	if list[0].Spec.Unschedulable == true {
		t.Errorf("node was not uncordoned")
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
