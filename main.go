package main

import (
	. "cluster-monitor-poc/cluster-monitor"
	"cluster-monitor-poc/kubernetes"
	"cluster-monitor-poc/logger"
	"cluster-monitor-poc/provider/azure"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	"os"
)

func main() {
	kubeconfig := kingpin.Flag("kubeconfig", "Path to kubernetes config, in cluster initialization will be used if missing").Default("").Short('c').String()

	softAge := kingpin.Flag("soft-reboot-age", "How long pod should be in not Ready state before we initiate soft reboot").Short('s').Default("1").Int()
	hardAge := kingpin.Flag("hard-reboot-age", "How long pod should be in not Ready state before we initiate hard reboot").Short('h').Default("2").Int()
	restartThreshold := kingpin.Flag("node-restart-threshold", "How long in minutes node should be in not Ready state before we initiate hard reboot").Short('k').Default("30").Int()

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

	var kube kubernetes.Kubernetes

	if *kubeconfig == "" {
		kube = kubernetes.GetKubeClient()
	} else {
		kube = kubernetes.GetKubeClientFromConfig(*kubeconfig)
	}

	creds := azure.FromConfigFile(*azureServicePrincipalConfig)
	az := azure.InitProvider(creds)

	softPerdicate := GetAgePredicate(*softAge)
	hardPredicate := GetAgePredicate(*hardAge)

	podsLogger := logger.Logger{Component: "PodsMonitor"}
	nodesLogger := logger.Logger{Component: "NodesMonitor"}

	settings := PodsMonitorSettings{
		Kube:                kube,
		LoopDelay:           *loopDelay,
		DryRun:              *dryRun,
		Provider:            az,
		Collections:         *collections,
		CollectionDelay:     *collectionDelay,
		CurrentNode:         currentNode,
		HardRebootPredicate: hardPredicate,
		Namespace:           *namespace,
		SoftRebootPredicate: softPerdicate,
		Log:                 podsLogger,
	}
	nodesMonitor := NodeMonitorSettings{
		CurrentNode: currentNode,
		Provider:    az,
		DryRun:      *dryRun,
		LoopDelay:   *loopDelay,
		Kube:        kube,
		Log:         nodesLogger,
		Threshold:   *restartThreshold,
	}

	registerMetrics(kube)
	registerHealth()

	go settings.PodsMonitor()
	go nodesMonitor.NodesMonitor()
	_ = http.ListenAndServe(":8080", nil)
}

func registerMetrics(kube kubernetes.Kubernetes) {
	kube.InitMetrics()
	reg := prometheus.NewRegistry()

	metrics := kube.GetMetrics()
	reg.MustRegister(metrics...)

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

}

func registerHealth() {
	noopHandler := func(w http.ResponseWriter, r *http.Request) { _ = r.Body.Close() }

	http.Handle("/healthz", http.HandlerFunc(noopHandler))

}
