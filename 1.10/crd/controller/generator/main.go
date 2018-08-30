package main

import (
	"flag"
	"time"

	glog "github.com/golang/glog"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	kubernetes "k8s.io/client-go/kubernetes"
	kubeinformers "k8s.io/client-go/informers"
	signals "k8s.io/sample-controller/pkg/signals"

	clientset "lsalab.nthu.edu.tw/ericyeh/tensorflow-controller/pkg/client/clientset/versioned"
	informers "lsalab.nthu.edu.tw/ericyeh/tensorflow-controller/pkg/client/informers/externalversions"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	flag.Parse()
	kubeconfig = "/home/kube/.kube/config"

	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	tensorflowClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building example clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	tensorflowInformerFactory := informers.NewSharedInformerFactory(tensorflowClient, time.Second*30)

	controller := NewController(kubeClient, tensorflowClient, kubeInformerFactory.Core().V1().Pods(), tensorflowInformerFactory.Lsalab().V1().Tensorflows())

	go kubeInformerFactory.Start(stopCh)
	go tensorflowInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
