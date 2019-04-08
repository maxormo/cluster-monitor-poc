package kubernetes

import (
	"cluster-monitor-poc/logger"
	"cluster-monitor-poc/provider"

	"cluster-monitor-poc/entities"
	"fmt"
	v1 "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"strconv"
	"time"
)

type Kubernetes struct {
	Kubeclient kubernetes.Interface
}

func GetKubeClient() Kubernetes {
	config, err := rest.InClusterConfig()

	handlePanicError(err)

	return getKubeclient(config)
}

func GetKubeClientFromConfig(kubeconfig string) Kubernetes {

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)

	handlePanicError(err)

	return getKubeclient(config)
}

func getKubeclient(config *rest.Config) Kubernetes {
	clientset, err := kubernetes.NewForConfig(config)

	handlePanicError(err)

	return Kubernetes{Kubeclient: clientset}
}

func ConvertPod(pod v1.Pod) entities.Pod {
	var transitionTime = time.Now() // to avoid getting failed pods get into the output result
	var conditionStatus = "False"
	var conditionReason = ""

	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady {
			transitionTime = condition.LastTransitionTime.Time
			conditionStatus = string(condition.Status)
			conditionReason = condition.Reason
		}
	}
	i := entities.Pod{Name: pod.Name, Node: pod.Spec.NodeName, LastTransitionTime: transitionTime, ReadyCondition: conditionStatus, Reason: conditionReason}

	return i
}

func (kube Kubernetes) GetAllPods() []v1.Pod {

	logger.Printfln("execute get all pods command")
	pods, err := kube.Kubeclient.CoreV1().Pods(meta.NamespaceAll).List(meta.ListOptions{})

	handlePanicError(err)

	result := pods.Items

	logger.Printfln("found %v pods", len(result))
	return result
}

func (kube Kubernetes) addAnnotation(nodeName, namespaceName, key, value string) {

	namespace, err := kube.Kubeclient.CoreV1().Namespaces().Get(namespaceName, meta.GetOptions{})
	handlePanicError(err)

	current := namespace.GetAnnotations()

	current[key] = value

	namespace.SetAnnotations(current)

	logger.Printfln("annotating namespace %s to reboot node %s with %s=%s", namespaceName, nodeName, key, value)

	_, err = kube.Kubeclient.CoreV1().Namespaces().Update(namespace)
	if err != nil {
		logger.Printfln("cannot set annotation %s=%s on the namespace %s for node  %s\n", key, value, namespaceName, nodeName)
	}
}

func (kube Kubernetes) SetSoftRebootAnnotation(dryRun bool, namespace string, nodes map[string]int) {
	for node := range nodes {
		kube.SetSoftRebootNodeAnnotation(dryRun, namespace, node) // no rolling reboot :(
	}
}

func (kube Kubernetes) SetSoftRebootNodeAnnotation(dryRun bool, namespace string, node string) {
	if dryRun {
		logger.Printfln("running hard reset on node %s", node)
		return
	}

	kube.addAnnotation(node, namespace, "Rebooter.Node."+node, "Zombie-Killer.Soft-Kill") // no rolling reboot :(
}

/// poor man's leader election for who will restart particular node we are or not
/// most likely should be replaced with leaderelection/LeaderElector
func (kube Kubernetes) SetHardKillLock(node string) bool {

	kNode, err := kube.Kubeclient.CoreV1().Nodes().Get(node, meta.GetOptions{})
	handlePanicError(err)
	currentAnnotations := kNode.GetAnnotations()

	now := time.Now().Unix()

	for i := 0; i < 10; i++ {

		// 1. get current value
		if value, exists := currentAnnotations["ZombieKiller.HardKill"]; exists {

			existingTimeStamp, err := strconv.ParseInt(value, 10, 64)

			// no error can proceed
			if err == nil {
				// this is our annotation means we have the lock
				if existingTimeStamp == now {
					return true
				}
				if existingTimeStamp > now {
					// someone else lease is current, wont do anything
					return false
				}
			}

		}

		// update now value since it is different retry
		now := time.Now().Unix()

		// if we are here then annotation is old or garbage or missing so we overwrite it
		currentAnnotations["ZombieKiller.HardKill"] = strconv.FormatInt(now, 10)
		kNode.SetAnnotations(currentAnnotations)
		_, err = kube.Kubeclient.CoreV1().Nodes().Update(kNode)
		handlePanicError(err)
	}
	// fallback since we unable to get the lock
	return false
}

func (kube Kubernetes) CordonNode(nodeName string) {
	logger.Printfln("attempting to cordon node %s", nodeName)
	kube.cordonUncordonNode(nodeName, true)
	logger.Printfln("node %s condoned", nodeName)
}

func (kube Kubernetes) UncordonNode(nodeName string) {
	logger.Printfln("attempting to uncordon node %s", nodeName)

	kube.cordonUncordonNode(nodeName, false)
	logger.Printfln("node %s uncordoned", nodeName)

}

func (kube Kubernetes) cordonUncordonNode(nodeName string, isCordone bool) {
	node, err := kube.Kubeclient.CoreV1().Nodes().Get(nodeName, meta.GetOptions{})

	handlePanicError(err)

	node.Spec.Unschedulable = isCordone
	_, err = kube.Kubeclient.CoreV1().Nodes().Update(node)

	handlePanicError(err)
}

func (kube Kubernetes) GetPodsForNode(nodeName string) []v1.Pod {
	pods, err := kube.Kubeclient.CoreV1().Pods(meta.NamespaceAll).List(meta.ListOptions{FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName}).String()})
	handlePanicError(err)
	return pods.Items
}

func (kube Kubernetes) EvictPods(nodeName string) {
	pods := kube.GetPodsForNode(nodeName)
	logger.Printfln("will evict %v pods", len(pods))
	for _, pod := range pods {
		kube.evictPod(pod)
	}
}

func (kube Kubernetes) evictPod(pod v1.Pod) {

	gp := int64(1)

	evictionPolicy := &policy.Eviction{

		ObjectMeta:    meta.ObjectMeta{Namespace: pod.Namespace, Name: pod.Name},
		DeleteOptions: &meta.DeleteOptions{GracePeriodSeconds: &gp},
	}
	logger.Printfln("evicting pod %s in namespace %s", pod.Name, pod.Namespace)

	err := kube.Kubeclient.CoreV1().Pods(pod.Namespace).Evict(evictionPolicy)

	switch {
	case apierrors.IsTooManyRequests(err):
		fmt.Printf("too many requests to evict pod %s in namespace %s, sleeping 5 seconds\n", pod.Name, pod.Namespace)
		time.Sleep(5 * time.Second)

	case apierrors.IsNotFound(err):
		fmt.Printf("cannot evict pod %s in %s namespace\n", pod.Name, pod.Namespace)

	case err != nil:
		fmt.Printf("cannot evict pod %s in %s namespace due to %s\n", pod.Name, pod.Namespace, err.Error())
	}
}

func (kube Kubernetes) DrainNode(nodeName string) {
	kube.CordonNode(nodeName)
	kube.EvictPods(nodeName)
}

func (kube Kubernetes) HardRestart(provider provider.Provider, dryRun bool, nodes map[string]int, currentNode string) {
	for node := range nodes {
		kube.HardRestartNode(provider, dryRun, node, currentNode)
	}
}

func (kube Kubernetes) HardRestartNode(provider provider.Provider, dryRun bool, node string, currentNode string) {
	if dryRun {
		logger.Printfln("running hard reset on node %s", node)
		return
	}

	if node == currentNode {
		logger.Printfln("skipping hard restart of node we are running on %s", node)
		// we should not restart ourselves
		return
	}

	// check lock
	if kube.SetHardKillLock(node) {
		// cordon node and drain/evict all pods
		logger.Printfln("hard kill lock aquired for node %s", node)
		kube.DrainNode(node)
		logger.Printfln("node %s drained", node)

		err := provider.RestartNode(node)
		logger.Printfln("restart node %s command executed", node)
		if err != nil {
			logger.Printfln("and return error %s", err.Error())
		}

	}
	// do uncordone in any case
	kube.UncordonNode(node) // uncordone the node
}

func (kube Kubernetes) GetNodeList() []v1.Node {

	nodeList, e := kube.Kubeclient.CoreV1().Nodes().List(meta.ListOptions{})

	if e != nil {
		logger.Printfln("%v", e.Error())
		panic(e)

	}
	return nodeList.Items
}

func handlePanicError(err error) {
	if err != nil {
		panic(err)
	}
}
