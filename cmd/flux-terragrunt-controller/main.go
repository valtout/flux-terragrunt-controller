package main

import (
	"flag"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"flux-terragrunt-controller/pkg/controller"
)

func main() {
	var metricsAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to")
	flag.Parse()

	_ = os.Setenv("METRICS_BIND_ADDRESS", metricsAddr)

	// Placeholder: in a full controller this would create a controller-runtime manager and register reconciler(s).
	// This scaffold focuses on repo structure + CI/tests scaffolding.
	mgr := manager.New(manager.GetConfigOrDie(), manager.Options{})
	_ = mgr

	_ = controller.Register(mgr)

	if err := signals.SetupSignalHandler(); err != nil {
		// Ignore placeholder errors
	}
}

