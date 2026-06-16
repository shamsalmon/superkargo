package pluginstep

import (
	"context"
	"fmt"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/pkg/credentials"
	"github.com/akuity/kargo/pkg/promotion"
)

// Engine is a promotion.Engine that executes steps with a custom registry. It is
// the thin seam that lets a DynamicRegistry be supplied where Kargo's
// promotion.LocalEngine hardcodes the default registry. All real work is
// delegated to Kargo's LocalOrchestrator.
type Engine struct {
	orchestrator *promotion.LocalOrchestrator
}

// NewEngine returns an Engine backed by a Kargo LocalOrchestrator using the
// provided registry.
func NewEngine(
	registry promotion.StepRunnerRegistry,
	kargoClient client.Client,
	argoCDClient client.Client,
	credsDB credentials.Database,
	gitUserResolver promotion.GitUserResolver,
	cacheFn promotion.ExprDataCacheFn,
) *Engine {
	return &Engine{
		orchestrator: promotion.NewLocalOrchestrator(
			registry,
			kargoClient,
			argoCDClient,
			credsDB,
			gitUserResolver,
			cacheFn,
		),
	}
}

// Promote implements promotion.Engine.
func (e *Engine) Promote(
	ctx context.Context,
	promoCtx promotion.Context,
	steps []promotion.Step,
) (promotion.Result, error) {
	if promoCtx.WorkDir == "" {
		workDir, err := os.MkdirTemp("", "run-")
		if err != nil {
			return promotion.Result{Status: kargoapi.PromotionPhaseErrored},
				fmt.Errorf("temporary working directory creation failed: %w", err)
		}
		promoCtx.WorkDir = workDir
		defer os.RemoveAll(workDir)
	}
	return e.orchestrator.ExecuteSteps(ctx, promoCtx, steps)
}
