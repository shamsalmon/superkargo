package pluginstep

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuity/kargo/pkg/promotion"

	kargoext "github.com/shamsalmon/kargo-plugin-ext/api/v1alpha1"
)

// DynamicRegistry is a promotion.StepRunnerRegistry that augments a delegate
// (typically promotion.DefaultStepRunnerRegistry) with steps defined at runtime
// via CustomPromotionStep resources. On a miss in the delegate, Get resolves a
// cluster-scoped CustomPromotionStep by name and returns a registration whose
// runner dispatches to the named plugin sidecar over gRPC.
//
// Lookups are read-only (no mutation of any shared map), so the registry is safe
// for concurrent use without additional locking.
type DynamicRegistry struct {
	delegate promotion.StepRunnerRegistry
	client   client.Client
	dial     dialer
}

// NewDynamicRegistry returns a DynamicRegistry wrapping the given delegate.
// Plugins are reached as gRPC unix sockets ("<plugin>.sock") within socketDir.
func NewDynamicRegistry(
	delegate promotion.StepRunnerRegistry,
	kargoClient client.Client,
	socketDir string,
) *DynamicRegistry {
	return &DynamicRegistry{
		delegate: delegate,
		client:   kargoClient,
		dial:     socketDialer(socketDir),
	}
}

// Register implements promotion.StepRunnerRegistry by delegating.
func (r *DynamicRegistry) Register(reg promotion.StepRunnerRegistration) error {
	return r.delegate.Register(reg)
}

// MustRegister implements promotion.StepRunnerRegistry by delegating.
func (r *DynamicRegistry) MustRegister(reg promotion.StepRunnerRegistration) {
	r.delegate.MustRegister(reg)
}

// Get implements promotion.StepRunnerRegistry. It prefers a built-in runner and
// falls back to a CustomPromotionStep of the same name.
//
// The StepRunnerRegistry interface's Get carries no context.Context, so the
// lookup uses context.Background(); the client is cache-backed and Get is only
// invoked during reconciliation, after the cache has synced.
func (r *DynamicRegistry) Get(kind string) (promotion.StepRunnerRegistration, error) {
	if reg, err := r.delegate.Get(kind); err == nil {
		return reg, nil
	}

	var cps kargoext.CustomPromotionStep
	if err := r.client.Get(context.Background(), client.ObjectKey{Name: kind}, &cps); err != nil {
		if apierrors.IsNotFound(err) {
			// Preserve the delegate's not-found error for consistent behavior.
			return r.delegate.Get(kind)
		}
		return promotion.StepRunnerRegistration{},
			fmt.Errorf("error looking up CustomPromotionStep %q: %w", kind, err)
	}

	return r.registrationFor(&cps), nil
}

// registrationFor builds a step runner registration from a CustomPromotionStep.
func (r *DynamicRegistry) registrationFor(
	cps *kargoext.CustomPromotionStep,
) promotion.StepRunnerRegistration {
	pluginName := cps.Spec.Plugin
	if pluginName == "" {
		pluginName = cps.Name
	}
	shareFolder := cps.Spec.SharePromotionFolder

	meta := promotion.StepRunnerMetadata{
		// The plugin runner only forks a subprocess; it needs no Kargo
		// capabilities (no Kubernetes/Argo CD/credentials access).
		DefaultErrorThreshold: cps.Spec.DefaultErrorThreshold,
	}
	if meta.DefaultErrorThreshold == 0 {
		meta.DefaultErrorThreshold = 1
	}
	if cps.Spec.DefaultTimeout != nil {
		meta.DefaultTimeout = cps.Spec.DefaultTimeout.Duration
	}

	return promotion.StepRunnerRegistration{
		Name:     cps.Name,
		Metadata: meta,
		Value: func(promotion.StepRunnerCapabilities) promotion.StepRunner {
			return &pluginRunner{
				pluginName:  pluginName,
				shareFolder: shareFolder,
				dial:        r.dial,
			}
		},
	}
}
