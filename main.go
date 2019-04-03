package main

import (
	. "cluster-monitor-poc/cluster-monitor"
	"cluster-monitor-poc/entities"
	kubernetes2 "cluster-monitor-poc/kubernetes"
	"cluster-monitor-poc/logger"
	"cluster-monitor-poc/provider/azure"
	"gopkg.in/alecthomas/kingpin.v2"
	v1 "k8s.io/api/core/v1"
	"time"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	kubeconfig := kingpin.Flag("kubeconfig", "Path to kubernetes config, in cluster initialization will be used if missing").Default("").Short('c').String()

	soft_age := kingpin.Flag("soft-reboot-age", "How long pod should be in not Ready state before we initiate soft reboot").Short('s').Default("1").Int()
	hard_age := kingpin.Flag("hard-reboot-age", "How long pod should be in not Ready state before we initiate hard reboot").Short('h').Default("2").Int()

	loopDelay := kingpin.Flag("loopDelay", "Sleep time in minutes between iterations").Default("2").Int()
	collections := kingpin.Flag("collections", "Number of get pods collections to identify rogue pods").Default("3").Int()
	collectionDelay := kingpin.Flag("collection-delay", "Sleep time between rogue pods collections").Default("1").Int()

	namespace := kingpin.Flag("namespace", "namespace to annotate").String()
	currentNode := kingpin.Flag("currentNode", "node name were current instance is running no to kill ourselves").Short('n').String()

	dry_run := kingpin.Flag("dry-run", "Do not set any annotation and do no do hard restart if specified, only add log statement about action").Default("false").Bool()

	azureServicePrincipalConfig := kingpin.Flag("azure-principal", "azure service principal config file location").Short('f').Default("/etc/kubernetes/azure.json").String()

	kingpin.Parse()

	var kube kubernetes2.Kubernetes

	if *kubeconfig == "" {
		kube = kubernetes2.GetKubeClient()
	} else {
		kube = kubernetes2.GetKubeClientFromConfig(*kubeconfig)
	}
	creds := azure.FromConfigFile(*azureServicePrincipalConfig)
	az := azure.InitProvider(creds)

	soft_perdicate := GetAgePredicate(*soft_age)
	hard_predicate := GetAgePredicate(*hard_age)

	logger.Printfln("finally started...")

	go PodsMonitor(collections, kube, soft_perdicate, hard_predicate, collectionDelay, dry_run, namespace, az, currentNode, loopDelay)
	go NodesMonitor(kube, *dry_run, az, *currentNode, *loopDelay, *namespace)
	select {}
}

func NodesMonitor(kube kubernetes2.Kubernetes, dryRun bool, az azure.Azure, currentNode string, loopDelay int, namespace string) {
	logger.Printfln("starting nodes monitor")
	for {
		logger.Printfln("scanning through all node for not ready status")

		nodeList, e := kube.Kubeclient.CoreV1().Nodes().List(meta.ListOptions{})

		if e != nil {
			logger.Printfln("%v", e.Error())
			return
		}
		for _, n := range nodeList.Items {
			for _, condition := range n.Status.Conditions {
				if condition.Type == v1.NodeReady && condition.Status != v1.ConditionTrue {

					hourAgo := time.Now().Add(-time.Duration(1) * time.Hour)
					minutesAgo := time.Now().Add(-time.Duration(15) * time.Minute)

					if condition.LastTransitionTime.Time.Before(hourAgo) {
						logger.Printfln("node %s is not ready, run hard kill", n.Name)
						kube.HardRestartNode(az, dryRun, n.Name, currentNode)

					} else if condition.LastTransitionTime.Time.Before(minutesAgo) {
						// this is most likely is a bug because once restart os it will reset time counter
						logger.Printfln("node %s is not ready, run soft kill", n.Name)
						kube.SetSoftRebootNodeAnnotation(dryRun, namespace, n.Name)
					}

				}
			}
		}

		logger.Printfln("nodes monitor is sleeping for %v minutes...", loopDelay)
		time.Sleep(time.Duration(loopDelay) * time.Minute)
	}
}

func PodsMonitor(collections *int, kube kubernetes2.Kubernetes, soft_perdicate func(pod entities.Pod) bool, hard_predicate func(pod entities.Pod) bool, collectionDelay *int, dry_run *bool, namespace *string, az azure.Azure, currentNode *string, loopDelay *int) {
	for {
		logger.Printfln("starting pods monitor")
		var convertedPods []entities.Pod
		softKillNodes := make(map[string]int)
		hardKillNodes := make(map[string]int)

		for i := 0; i < *collections; i++ {
			pods := kube.GetAllPods()

			for _, pod := range pods {
				convertedPods = append(convertedPods, kubernetes2.ConvertPod(pod))
			}

			softKillNodes = GetNodesToKill(convertedPods, softKillNodes, soft_perdicate)

			if len(softKillNodes) == 0 {
				break
			}
			hardKillNodes = GetNodesToKill(convertedPods, hardKillNodes, hard_predicate)

			time.Sleep(time.Duration(*collectionDelay) * time.Second)
		}

		CleanupFlakyNodes(softKillNodes, *collections)
		CleanupFlakyNodes(hardKillNodes, *collections)

		CleanSoftNodesFromHardNodes(hardKillNodes, softKillNodes)
		logger.Printfln("soft candidates: ")
		for e := range softKillNodes {
			logger.Printfln(e)
		}

		logger.Printfln("hard candidates:")

		for e := range hardKillNodes {
			print(e)
		}

		kube.SetSoftRebootAnnotation(*dry_run, *namespace, softKillNodes)
		kube.HardRestart(az, *dry_run, hardKillNodes, *currentNode)
		logger.Printfln("pods monitor is sleeping for %v minutes...", *loopDelay)
		time.Sleep(time.Duration(*loopDelay) * time.Minute)
	}
}

func CleanupFlakyNodes(survivedNodes map[string]int, collections int) {
	for k, v := range survivedNodes {
		if v < collections {
			delete(survivedNodes, k)
		}
	}
}
