package kube

import (
	"flag"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client defines shared client interface for Kubernetes cli.
var (
	Client *kubernetes.Clientset
	Config *rest.Config
)

func init() {
	var (
		err        error
		kubeconfig *string
	)

	if home := homedir.HomeDir(); len(home) != 0 {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	Config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	if Client, err = kubernetes.NewForConfig(Config); err != nil {
		panic(err.Error())
	}
}
