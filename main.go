package main

import (
	. "cluster-monitor-poc/cluster-monitor"
	kubernetes2 "cluster-monitor-poc/kubernetes"
	"cluster-monitor-poc/logger"
	"cluster-monitor-poc/provider/azure"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
)

func main() {
	kubeconfig := kingpin.Flag("kubeconfig", "Path to kubernetes config, in cluster initialization will be used if missing").Default("").Short('c').String()

	soft_age := kingpin.Flag("soft-reboot-age", "How long pod should be in not Ready state before we initiate soft reboot").Short('s').Default("1").Int()
	hard_age := kingpin.Flag("hard-reboot-age", "How long pod should be in not Ready state before we initiate hard reboot").Short('h').Default("2").Int()

	loopDelay := kingpin.Flag("loopDelay", "Sleep time in minutes between iterations").Default("2").Int()
	collections := kingpin.Flag("collections", "Number of get pods collections to identify rogue pods").Default("3").Int()
	collectionDelay := kingpin.Flag("collection-delay", "Sleep time between rogue pods collections").Default("1").Int()

	namespace := kingpin.Flag("namespace", "namespace to annotate").String()
	currentNode, present := os.LookupEnv("CURRENT_NODE")
	if !present {
		panic("environment variable CURRENT_NODE should be present")
	}

	dryRun := kingpin.Flag("dry-run", "Do not set any annotation and do no do hard restart if specified, only add log statement about action").Default("false").Bool()

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

	softPerdicate := GetAgePredicate(*soft_age)
	hardPredicate := GetAgePredicate(*hard_age)

	logger.Printfln("finally started...")

	settings := PodsMonitorSettings{Kube: kube, LoopDelay: *loopDelay, DryRun: *dryRun, Provider: az, Collections: collections, CollectionDelay: collectionDelay, CurrentNode: currentNode, HardRebootPredicate: hardPredicate, Namespace: namespace, SoftRebootPredicate: softPerdicate}
	nodesMonitor := NodeMonitorSettings{CurrentNode: currentNode, Provider: az, DryRun: *dryRun, LoopDelay: *loopDelay, Kube: kube}

	go settings.PodsMonitor()
	go nodesMonitor.NodesMonitor()
	select {}
}
