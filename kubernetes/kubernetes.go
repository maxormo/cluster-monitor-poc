package kubernetes

import (
	"cluster-monitor-poc/entities"
	"cluster-monitor-poc/logger"
	"cluster-monitor-poc/provider"
	"fmt"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

//TODO: need to extract interface to have proper dry run and incapsulation
type Kubernetes struct {
	Kubeclient    kubernetes.Interface
	metricsClient entities.MetricsClient
	log           logger.Logger
	provider      provider.Provider
	currentNode   string
}

type KubeAction = string

type NodeAction = func(node string) error

const Restart KubeAction = "Restart"
const Drain KubeAction = "Drain"

const DisableScaleDownKey = "cluster-autoscaler.kubernetes.io/scale-down-disabled"

type Condition struct {
	// node condition that we want to react on like: Ready=False
	// were Ready will be condition type and False is a value
	ConditionType  v1.NodeConditionType
	ConditionValue v1.ConditionStatus
	//How long that condition is in effect before we act on it
	TimeInEffect time.Duration
	// function that will be supplied with node name that match that condition
	Action KubeAction
}

func ParseConditions(actions []string) []Condition {
	parsed := make([]Condition, len(actions))

	for i := 0; i < len(actions); i++ {
		parsed[i] = ParseCondition(actions[i])
	}
	return parsed
}

// parse structure like Ready=False,10s,Drain
func ParseCondition(value string) Condition {
	var empty Condition

	ts := strings.SplitN(value, "=", 2)

	if len(ts) != 2 {
		return empty
	}

	sm := strings.SplitN(ts[1], ",", 3)
	m, err := time.ParseDuration(sm[1])

	if err != nil {
		return empty
	}

	return Condition{ConditionType: v1.NodeConditionType(ts[0]), ConditionValue: v1.ConditionStatus(sm[0]), TimeInEffect: m, Action: sm[2]}
}

func GetKubeClient(kubeconfig string, client entities.MetricsClient, log logger.Logger, provider provider.Provider, currentNode string) Kubernetes {

	if kubeconfig == "" {
		return getInClusterKubeClient(client, log, provider, currentNode)
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)

	handlePanicError(err)

	return getKubeclient(config, client, log, provider, currentNode)
}

func getInClusterKubeClient(client entities.MetricsClient, log logger.Logger, provider provider.Provider, currentNode string) Kubernetes {

	config, err := rest.InClusterConfig()

	handlePanicError(err)

	return getKubeclient(config, client, log, provider, currentNode)
}

func getKubeclient(config *rest.Config, client entities.MetricsClient, log logger.Logger, provider provider.Provider, currentNode string) Kubernetes {
	clientset, err := kubernetes.NewForConfig(config)

	handlePanicError(err)

	return Kubernetes{
		Kubeclient:    clientset,
		metricsClient: client,
		log:           log,
		provider:      provider,
		currentNode:   currentNode,
	}
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

	kube.log.Printfln("execute get all pods command")
	pods, err := kube.Kubeclient.CoreV1().Pods(meta.NamespaceAll).List(meta.ListOptions{})

	handlePanicError(err)

	result := pods.Items

	kube.log.Printfln("found %v pods", len(result))
	return result
}

func (kube Kubernetes) addNodeAnnotation(nodeName, key, value string) {
	node, e := kube.Kubeclient.CoreV1().Nodes().Get(nodeName, meta.GetOptions{})
	handlePanicError(e)

	annotations := node.GetAnnotations()

	if annotations == nil {
		return
	}

	annotations[key] = value

	node.SetAnnotations(annotations)
	_, e = kube.Kubeclient.CoreV1().Nodes().Update(node)
	handlePanicError(e)
}

func (kube Kubernetes) RemoveNodeAnnotation(nodeName, key string) {
	node, e := kube.Kubeclient.CoreV1().Nodes().Get(nodeName, meta.GetOptions{})

	handlePanicError(e)

	annotations := node.GetAnnotations()

	if annotations == nil {
		return
	}

	delete(annotations, key)

	node.SetAnnotations(annotations)
	_, e = kube.Kubeclient.CoreV1().Nodes().Update(node)
	handlePanicError(e)
}

func (kube Kubernetes) addNamespaceAnnotation(nodeName, namespaceName, key, value string) {

	namespace, err := kube.Kubeclient.CoreV1().Namespaces().Get(namespaceName, meta.GetOptions{})
	handlePanicError(err)

	current := namespace.GetAnnotations()

	current[key] = value

	namespace.SetAnnotations(current)

	kube.log.Printfln("annotating namespace %s to reboot node %s with %s=%s", namespaceName, nodeName, key, value)

	_, err = kube.Kubeclient.CoreV1().Namespaces().Update(namespace)
	if err != nil {
		kube.log.Printfln("cannot set annotation %s=%s on the namespace %s for node  %s\n", key, value, namespaceName, nodeName)
	}
}

func (kube Kubernetes) SetSoftRebootAnnotation(dryRun bool, namespace string, nodes map[string]int) {
	for node := range nodes {
		kube.setSoftRebootNodeAnnotation(dryRun, namespace, node) // no rolling reboot :(
	}
}

func (kube Kubernetes) setSoftRebootNodeAnnotation(dryRun bool, namespace string, node string) {
	if dryRun {
		kube.log.Printfln("running reboot on node %s", node)
		return
	}

	kube.addNamespaceAnnotation(node, namespace, "Rebooter.Node."+node, "Zombie-Killer.Soft-Kill")
	kube.metricsClient.IncSoftRestart()
}

/// poor man's leader election
/// most likely should be replaced with leaderelection/LeaderElector
func (kube Kubernetes) setHardKillLock(node string) bool {

	now := time.Now()
	leaseDuration := time.Minute * 10

	for i := 0; i < 10; i++ {

		// 1. get current value
		kNode, err := kube.Kubeclient.CoreV1().Nodes().Get(node, meta.GetOptions{})

		handlePanicError(err)

		currentAnnotations := kNode.GetAnnotations()

		if value, exists := currentAnnotations["ZombieKiller.HardKill"]; exists {

			existingTimeStamp, err := strconv.ParseInt(value, 10, 64)

			// no error can proceed
			if err == nil {
				// this is our annotation means we have the lock
				if existingTimeStamp == now.Unix() {

					leaseTimeStamp := now.Add(leaseDuration).Unix()
					currentAnnotations["ZombieKiller.HardKill"] = strconv.FormatInt(leaseTimeStamp, 10)
					kNode.SetAnnotations(currentAnnotations)
					_, err = kube.Kubeclient.CoreV1().Nodes().Update(kNode)
					handlePanicError(err)
					return true
				}

				if existingTimeStamp > now.Unix() {
					// someone else lease is current, wont do anything
					return false
				}
			}
		}

		// update now value since it is different retry
		now = time.Now()

		// if we are here then annotation is old or garbage or missing so we overwrite it
		currentAnnotations["ZombieKiller.HardKill"] = strconv.FormatInt(now.Unix(), 10)

		kNode.SetAnnotations(currentAnnotations)
		_, err = kube.Kubeclient.CoreV1().Nodes().Update(kNode)
		handlePanicError(err)
	}
	// fallback since we unable to get the lock
	return false
}

func (kube Kubernetes) CordonNode(nodeName string) {
	kube.log.Printfln("attempting to cordon node %s", nodeName)
	kube.cordonUncordonNode(nodeName, true)
	kube.log.Printfln("node %s condoned", nodeName)
}

func (kube Kubernetes) UncordonNode(nodeName string) {
	kube.log.Printfln("attempting to uncordon node %s", nodeName)

	kube.cordonUncordonNode(nodeName, false)
	kube.log.Printfln("node %s uncordoned", nodeName)

}

func (kube Kubernetes) cordonUncordonNode(nodeName string, isCordone bool) {
	node, err := kube.Kubeclient.CoreV1().Nodes().Get(nodeName, meta.GetOptions{})

	handlePanicError(err)

	node.Spec.Unschedulable = isCordone
	_, err = kube.Kubeclient.CoreV1().Nodes().Update(node)

	handlePanicError(err)
}

func (kube Kubernetes) getPodsForNode(nodeName string) []v1.Pod {
	options := meta.ListOptions{FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName}).String()}
	pods, err := kube.Kubeclient.CoreV1().Pods(meta.NamespaceAll).List(options)
	handlePanicError(err)
	return pods.Items
}

func (kube Kubernetes) EvictPods(nodeName string) {
	pods := kube.getPodsForNode(nodeName)
	kube.log.Printfln("will evict %v pods", len(pods))
	for _, pod := range pods {
		kube.evictPod(pod)
	}
}

func (kube Kubernetes) evictPod(pod v1.Pod) {

	gracePeriod := int64(1)

	evictionPolicy := &policy.Eviction{

		ObjectMeta:    meta.ObjectMeta{Namespace: pod.Namespace, Name: pod.Name},
		DeleteOptions: &meta.DeleteOptions{GracePeriodSeconds: &gracePeriod},
	}

	kube.log.Printfln("evicting pod %s in namespace %s", pod.Name, pod.Namespace)

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

func (kube Kubernetes) DrainNode(nodeName string) error {
	kube.CordonNode(nodeName)
	kube.EvictPods(nodeName)
	return nil //TODO: need to propagate an errors if any
}

func (kube Kubernetes) HardRestartNodes(nodes map[string]int) {
	for node := range nodes {
		kube.HardRestartNode(node)
	}
}

func (kube Kubernetes) HardRestartNode(node string) error {

	if node == kube.currentNode {
		kube.log.Printfln("skipping hard restart of node we are running on %s", node)
		// we should not restart ourselves
		return nil
	}

	//TODO: need to set cluster autoscaler annotation to prevent scaledown
	kube.addNodeAnnotation(node, DisableScaleDownKey, "true")

	if kube.setHardKillLock(node) {
		// cordon node and drain/evict all pods
		kube.log.Printfln("hard kill lock aquired for node %s", node)
		err := kube.DrainNode(node)

		kube.log.Printfln("node %s drained", node)

		kube.log.PrintErr(err)

		err = kube.provider.RestartNode(node)

		kube.log.Printfln("restart node %s command executed", node)
		kube.metricsClient.IncHardRestart()
		if err != nil {
			kube.log.Printfln("and return error %s", err.Error())
		}
	}
	// uncordon
	kube.UncordonNode(node)
	//enable cluster autoscaler
	kube.RemoveNodeAnnotation(node, DisableScaleDownKey)
	return nil
}

func (kube Kubernetes) GetNodeList() []v1.Node {

	nodeList, e := kube.Kubeclient.CoreV1().Nodes().List(meta.ListOptions{})

	if e != nil {
		kube.log.Printfln("%v", e.Error())
		panic(e)

	}
	return nodeList.Items
}

func (kube Kubernetes) IsNotReadyNode(node v1.Node) (v1.NodeCondition, bool) {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady && condition.Status != v1.ConditionTrue {
			return condition, true
		}
	}
	var empty v1.NodeCondition
	return empty, false
}

func (kube Kubernetes) GetAction(condition Condition) NodeAction {

	if strings.EqualFold(condition.Action, Drain) {
		return kube.DrainNode
	}

	if strings.EqualFold(condition.Action, Restart) {
		return kube.HardRestartNode
	}

	return func(node string) error {
		kube.log.Printfln("supplied action is not valid %s", condition.Action)
		return nil
	}
}

func handlePanicError(err error) {
	if err != nil {
		panic(err)
	}
}
