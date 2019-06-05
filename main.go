package main

import (
	. "cluster-monitor-poc/cluster-monitor"
	"cluster-monitor-poc/entities"
	"cluster-monitor-poc/kubernetes"
	"cluster-monitor-poc/logger"
	"cluster-monitor-poc/provider/azure"
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	"os"
	"time"
)

const DefaultMetricsCollectionDuration = time.Duration(20) * time.Second

func main() {
	kubeconfig := kingpin.Flag("kubeconfig", "Path to kubernetes config, in cluster initialization will be used if missing").Default("").Short('c').String()

	softAge := kingpin.Flag("soft-reboot-age", "How long pod should be in not Ready state before we initiate soft reboot").Short('s').Default("1").Int()
	hardAge := kingpin.Flag("hard-reboot-age", "How long pod should be in not Ready state before we initiate hard reboot").Short('h').Default("2").Int()
	restartThreshold := kingpin.Flag("node-restart-threshold", "How long in minutes node should be in not Ready state before we initiate hard reboot").Short('k').Default("30").Int()

	loopDelay := kingpin.Flag("loopDelay", "Sleep time in minutes between iterations").Default("2").Int()
	collections := kingpin.Flag("collections", "Number of get pods collections to identify rogue pods").Default("3").Int()
	collectionDelay := kingpin.Flag("collection-delay", "Sleep time between rogue pods collections").Default("1").Int()
	metricsCollectionDelay := kingpin.Flag("metrics-delay", "Sleep time between node metrics collections").Default("20s").String()

	namespace := kingpin.Flag("namespace", "namespace to annotate").String()

	currentNode, present := os.LookupEnv("CURRENT_NODE")
	if !present {
		panic("environment variable CURRENT_NODE should be present")
	}

	dryRun := kingpin.Flag("dry-run", "Do not set any annotation and do no do hard restart if specified, only add log statement about action").Default("false").Bool()

	azureServicePrincipalConfig := kingpin.Flag("azure-principal", "azure service principal config file location").Short('f').Default("/etc/kubernetes/azure.json").String()

	conditions := kingpin.Arg("node-conditions", "Nodes for which any of these conditions are true will be cordoned and drained."+
		"Example:"+
		"	Ready=False,30m,Drain \n"+
		"	MemoryLeak=False,10m,Reboot").Required().Strings()

	kingpin.Parse()

	creds := azure.FromConfigFile(*azureServicePrincipalConfig)
	az := azure.InitProvider(creds)

	softPredicate := GetAgePredicate(*softAge)
	hardPredicate := GetAgePredicate(*hardAge)

	podsLogger := logger.GetLogger("PodsMonitor")
	nodesLogger := logger.GetLogger("NodesMonitor")
	nodeMetricsLogger := logger.GetLogger("NodeMetricsMonitor")

	metricsClient := entities.InitMetrics()

	podsMonitor := PodsMonitor{
		Kube:                kubernetes.GetKubeClient(*kubeconfig, metricsClient, podsLogger, az, currentNode),
		LoopDelay:           *loopDelay,
		DryRun:              *dryRun,
		Provider:            az,
		Collections:         *collections,
		CollectionDelay:     *collectionDelay,
		CurrentNode:         currentNode,
		HardRebootPredicate: hardPredicate,
		Namespace:           *namespace,
		SoftRebootPredicate: softPredicate,
		Log:                 podsLogger,
	}

	nodeMonitorKubeClient := kubernetes.GetKubeClient(*kubeconfig, metricsClient, nodesLogger, az, currentNode)

	nodesMonitor := NodeMonitor{
		CurrentNode: currentNode,
		Provider:    az,
		DryRun:      *dryRun,
		LoopDelay:   *loopDelay,
		Kube:        nodeMonitorKubeClient,
		Log:         nodesLogger,
		Threshold:   *restartThreshold,
		Conditions:  kubernetes.ParseConditions(*conditions),
	}

	registerHealth()

	metricsCollectionTimeoutDuration, e := time.ParseDuration(*metricsCollectionDelay)

	if e != nil {
		metricsCollectionTimeoutDuration = DefaultMetricsCollectionDuration
	}
	nodeMetrics := NodesMetrics{
		Kube:                     kubernetes.GetKubeClient(*kubeconfig, metricsClient, nodeMetricsLogger, az, currentNode),
		MetricsClient:            metricsClient,
		MetricsCollectionTimeout: metricsCollectionTimeoutDuration,
	}

	go nodeMetrics.NodesMetrics()

	go podsMonitor.PodsMonitor()

	go nodesMonitor.NodesMonitor()

	// start web service for metrics and health
	// all handlers should be register by now
	_ = http.ListenAndServe(":8080", nil)
}

func registerHealth() {
	noopHandler := func(w http.ResponseWriter, r *http.Request) { _ = r.Body.Close() }

	http.Handle("/healthz", http.HandlerFunc(noopHandler))

}
