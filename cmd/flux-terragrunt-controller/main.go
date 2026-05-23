package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	fluxv1 "flux-terragrunt-controller/pkg/apis/flux/v1"
	terragruntv1alpha1 "flux-terragrunt-controller/pkg/apis/terragrunt/v1alpha1"
	"flux-terragrunt-controller/pkg/controller"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(terragruntv1alpha1.AddToScheme(scheme))
	utilruntime.Must(fluxv1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager")

	flag.Parse()

	config, err := rest.InClusterConfig()
	if err != nil {
		config = &rest.Config{
			Host:  "localhost:6443",
			QPS:   100,
			Burst: 100,
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		os.Exit(1)
	}
	_ = clientset

	mgr, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Scheme:         scheme,
		LeaderElection: enableLeaderElection,
	})
	if err != nil {
		os.Exit(1)
	}

	if err := controller.Register(mgr); err != nil {
		os.Exit(1)
	}

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		os.Exit(1)
	}
}
