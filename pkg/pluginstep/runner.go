package pluginstep

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/pkg/logging"
	"github.com/akuity/kargo/pkg/promotion"

	"github.com/shamsalmon/kargo-plugin-ext/sdk"
)

// stepClient is the subset of sdk.Client the runner uses. It is an interface so
// tests can substitute an in-process fake instead of a real gRPC connection.
type stepClient interface {
	Run(context.Context, *sdk.Request) (*sdk.Response, error)
	Close() error
}

// dialer connects to the named plugin's gRPC socket.
type dialer func(pluginName string) (stepClient, error)

// pluginRunner is a promotion.StepRunner that delegates execution to a plugin
// sidecar over gRPC. Execution is synchronous: the plugin runs to completion
// within the RPC, so no Job-style Running/RetryAfter lifecycle is needed.
type pluginRunner struct {
	pluginName  string
	shareFolder bool
	dial        dialer
}

// Run implements promotion.StepRunner.
func (r *pluginRunner) Run(
	ctx context.Context,
	stepCtx *promotion.StepContext,
) (promotion.StepResult, error) {
	logger := logging.LoggerFromContext(ctx).WithValues(
		"plugin", r.pluginName,
		"promotion", stepCtx.Promotion,
		"alias", stepCtx.Alias,
	)

	client, err := r.dial(r.pluginName)
	if err != nil {
		return errored, fmt.Errorf("error connecting to plugin %q: %w", r.pluginName, err)
	}
	defer client.Close()

	cfg, err := json.Marshal(map[string]any(stepCtx.Config))
	if err != nil {
		return errored, fmt.Errorf("error encoding config for plugin %q: %w", r.pluginName, err)
	}

	req := &sdk.Request{
		Config:    cfg,
		Project:   stepCtx.Project,
		Stage:     stepCtx.Stage,
		Promotion: stepCtx.Promotion,
		Alias:     stepCtx.Alias,
	}
	if r.shareFolder {
		req.WorkDir = stepCtx.WorkDir
	}

	logger.Debug("invoking plugin sidecar")
	resp, err := client.Run(ctx, req)
	if err != nil {
		return errored, fmt.Errorf("plugin %q returned an error: %w", r.pluginName, err)
	}

	if resp.Failed {
		msg := resp.Message
		if msg == "" {
			msg = "plugin reported failure"
		}
		// Plugin-signaled failure is a business decision; do not retry.
		return promotion.StepResult{
				Status:  kargoapi.PromotionStepStatusFailed,
				Message: msg,
			},
			&promotion.TerminalError{Err: errors.New(msg)}
	}

	output := map[string]any{}
	if len(resp.Output) > 0 {
		if err = json.Unmarshal(resp.Output, &output); err != nil {
			return errored, fmt.Errorf("error decoding output from plugin %q: %w", r.pluginName, err)
		}
	}

	return promotion.StepResult{
		Status:  kargoapi.PromotionStepStatusSucceeded,
		Message: resp.Message,
		Output:  output,
	}, nil
}

// errored is returned alongside a (non-terminal) technical error.
var errored = promotion.StepResult{Status: kargoapi.PromotionStepStatusErrored}

// socketDialer returns a dialer that connects to plugin sidecars over unix
// sockets in socketDir (one socket per plugin, named "<plugin>.sock").
func socketDialer(socketDir string) dialer {
	return func(pluginName string) (stepClient, error) {
		path := filepath.Join(socketDir, pluginName+".sock")
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("plugin socket %q not available: %w", path, err)
		}
		return sdk.Dial(path)
	}
}
