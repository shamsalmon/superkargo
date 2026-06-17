package pluginstep

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/pkg/promotion"

	"github.com/shamsalmon/superkargo/sdk"
)

type fakeClient struct {
	gotReq *sdk.Request
	resp   *sdk.Response
	err    error
	closed bool
}

func (f *fakeClient) Run(_ context.Context, req *sdk.Request) (*sdk.Response, error) {
	f.gotReq = req
	return f.resp, f.err
}

func (f *fakeClient) Close() error {
	f.closed = true
	return nil
}

func dialFake(c *fakeClient, err error) dialer {
	return func(string) (stepClient, error) {
		if err != nil {
			return nil, err
		}
		return c, nil
	}
}

func stepCtx() *promotion.StepContext {
	return &promotion.StepContext{
		Project:   "proj",
		Stage:     "stg",
		Promotion: "promo-1",
		Alias:     "step-0",
		WorkDir:   "/work",
		Config:    promotion.Config{"hello": "foo", "world": "bar"},
	}
}

func TestPluginRunnerRun(t *testing.T) {
	ctx := context.Background()

	t.Run("success decodes output and shares folder", func(t *testing.T) {
		fc := &fakeClient{resp: &sdk.Response{
			Output:  json.RawMessage(`{"message":"foo-bar"}`),
			Message: "done",
		}}
		r := &pluginRunner{pluginName: "p", shareFolder: true, dial: dialFake(fc, nil)}

		res, err := r.Run(ctx, stepCtx())
		require.NoError(t, err)
		require.Equal(t, kargoapi.PromotionStepStatusSucceeded, res.Status)
		require.Equal(t, map[string]any{"message": "foo-bar"}, res.Output)
		require.True(t, fc.closed, "connection must be closed")

		require.JSONEq(t, `{"hello":"foo","world":"bar"}`, string(fc.gotReq.Config))
		require.Equal(t, "/work", fc.gotReq.WorkDir)
		require.Equal(t, "promo-1", fc.gotReq.Promotion)
	})

	t.Run("workdir withheld when not shared", func(t *testing.T) {
		fc := &fakeClient{resp: &sdk.Response{Output: json.RawMessage(`{}`)}}
		r := &pluginRunner{pluginName: "p", shareFolder: false, dial: dialFake(fc, nil)}
		_, err := r.Run(ctx, stepCtx())
		require.NoError(t, err)
		require.Empty(t, fc.gotReq.WorkDir)
	})

	t.Run("plugin-signaled failure is terminal", func(t *testing.T) {
		fc := &fakeClient{resp: &sdk.Response{Failed: true, Message: "bad config"}}
		r := &pluginRunner{pluginName: "p", dial: dialFake(fc, nil)}
		res, err := r.Run(ctx, stepCtx())
		require.Error(t, err)
		require.True(t, promotion.IsTerminal(err))
		require.Equal(t, kargoapi.PromotionStepStatusFailed, res.Status)
		require.Equal(t, "bad config", res.Message)
	})

	t.Run("transport error is non-terminal", func(t *testing.T) {
		fc := &fakeClient{err: errors.New("rpc broke")}
		r := &pluginRunner{pluginName: "p", dial: dialFake(fc, nil)}
		res, err := r.Run(ctx, stepCtx())
		require.Error(t, err)
		require.False(t, promotion.IsTerminal(err))
		require.Equal(t, kargoapi.PromotionStepStatusErrored, res.Status)
	})

	t.Run("dial failure is non-terminal", func(t *testing.T) {
		r := &pluginRunner{pluginName: "missing", dial: dialFake(nil, errors.New("no socket"))}
		res, err := r.Run(ctx, stepCtx())
		require.Error(t, err)
		require.False(t, promotion.IsTerminal(err))
		require.Equal(t, kargoapi.PromotionStepStatusErrored, res.Status)
	})
}
