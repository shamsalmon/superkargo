// Command hello-world-plugin is an example kargo-plugin-ext promotion-step
// plugin. It runs as a sidecar serving the plugin SDK's gRPC contract on a unix
// socket. It reads "hello" and "world" from the step config and returns a
// "message" output of "<hello>-<world>"; when the promotion folder is shared it
// also writes that message to a file to demonstrate filesystem access.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shamsalmon/kargo-plugin-ext/sdk"
)

func run(_ context.Context, req *sdk.Request) (*sdk.Response, error) {
	var cfg struct {
		Hello string `json:"hello"`
		World string `json:"world"`
	}
	if len(req.Config) > 0 {
		if err := json.Unmarshal(req.Config, &cfg); err != nil {
			return &sdk.Response{Failed: true, Message: fmt.Sprintf("invalid config: %v", err)}, nil
		}
	}

	message := fmt.Sprintf("%s-%s", cfg.Hello, cfg.World)

	if req.WorkDir != "" {
		if err := os.WriteFile(
			filepath.Join(req.WorkDir, "hello-world.txt"),
			[]byte(message),
			0o644,
		); err != nil {
			return &sdk.Response{Failed: true, Message: err.Error()}, nil
		}
	}

	out, err := json.Marshal(map[string]any{"message": message})
	if err != nil {
		return &sdk.Response{Failed: true, Message: err.Error()}, nil
	}
	return &sdk.Response{Output: out, Message: "greeting computed"}, nil
}

func main() {
	socket := os.Getenv("PLUGIN_SOCKET")
	if socket == "" {
		socket = "/run/plugins/hello-world.sock"
	}
	if err := sdk.Serve(sdk.StepFunc(run), socket); err != nil {
		fmt.Fprintln(os.Stderr, "hello-world-plugin:", err)
		os.Exit(1)
	}
}
