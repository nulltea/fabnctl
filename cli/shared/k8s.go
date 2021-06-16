package shared

import (
	"flag"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// K8s defines shared client interface for Kubernetes cli.
var K8s *kubernetes.Clientset

func initK8s() {
	var (
		err error
		kubeconfig *string
	)

	if home := homedir.HomeDir(); len(home) != 0 {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	if K8s, err = kubernetes.NewForConfig(config); err != nil {
		panic(err.Error())
	}
}
