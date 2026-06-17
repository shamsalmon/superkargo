package main

import (
	"os"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/akuity/kargo/pkg/logging"

	"github.com/shamsalmon/superkargo/internal/bootstrap"
)

func main() {
	ctx := signals.SetupSignalHandler()
	if err := bootstrap.Run(ctx); err != nil {
		logging.LoggerFromContext(ctx).Error(err, "")
		os.Exit(1)
	}
}
