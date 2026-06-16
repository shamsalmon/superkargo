package pluginstep

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/pkg/promotion"

	"github.com/shamsalmon/kargo-plugin-ext/sdk"
)

// TestSocketDialerEndToEnd runs a real SDK gRPC server on a unix socket and
// drives a pluginRunner against it via socketDialer, exercising the full
// controller<->sidecar path in-process.
func TestSocketDialerEndToEnd(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "echo.sock")

	impl := sdk.StepFunc(func(_ context.Context, req *sdk.Request) (*sdk.Response, error) {
		out, _ := json.Marshal(map[string]any{
			"config":  string(req.Config),
			"workdir": req.WorkDir,
		})
		return &sdk.Response{Output: out, Message: "ok"}, nil
	})
	go func() { _ = sdk.Serve(impl, sock) }()

	for i := 0; i < 100; i++ {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	r := &pluginRunner{pluginName: "echo", shareFolder: true, dial: socketDialer(dir)}
	res, err := r.Run(context.Background(), &promotion.StepContext{
		Promotion: "p1",
		WorkDir:   "/work",
		Config:    promotion.Config{"hello": "world"},
	})
	require.NoError(t, err)
	require.Equal(t, kargoapi.PromotionStepStatusSucceeded, res.Status)
	require.Equal(t, "/work", res.Output["workdir"])
	require.JSONEq(t, `{"hello":"world"}`, res.Output["config"].(string))
}
