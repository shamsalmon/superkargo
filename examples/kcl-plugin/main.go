// Command kcl-plugin is a superkargo promotion-step plugin that builds KCL
// into rendered manifests using the KCL Go SDK (kcl-lang.io/kcl-go). It runs as
// a sidecar container and serves the plugin SDK's gRPC contract on a unix
// socket; the controller dials it. The KCL toolchain is embedded, so no external
// `kcl` binary is required.
//
// Config (the step's `config:` block):
//
//	path:        KCL source — a file or directory relative to the promotion
//	             folder. Defaults to ".".
//	format:      "yaml" (default) or "json".
//	outputFile:  Optional path (relative to the promotion folder) to write the
//	             rendered manifests to. Requires sharePromotionFolder: true.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	kcl "kcl-lang.io/kcl-go"

	"github.com/shamsalmon/superkargo/sdk"
)

type config struct {
	Path       string `json:"path"`
	Format     string `json:"format"`
	OutputFile string `json:"outputFile"`
}

func run(_ context.Context, req *sdk.Request) (*sdk.Response, error) {
	var cfg config
	if len(req.Config) > 0 {
		if err := json.Unmarshal(req.Config, &cfg); err != nil {
			return failf("invalid config: %v", err), nil
		}
	}

	path := cfg.Path
	if path == "" {
		path = "."
	}
	// Resolve relative paths inside the shared promotion folder.
	if req.WorkDir != "" && !filepath.IsAbs(path) {
		path = filepath.Join(req.WorkDir, path)
	}

	result, err := kcl.Run(path)
	if err != nil {
		return failf("kcl build failed: %v", err), nil
	}

	format := strings.ToLower(cfg.Format)
	var manifests string
	switch format {
	case "json":
		manifests = result.GetRawJsonResult()
		format = "json"
	default:
		manifests = result.GetRawYamlResult()
		format = "yaml"
	}

	output := map[string]any{"format": format, "path": cfg.Path}

	if cfg.OutputFile != "" {
		if req.WorkDir == "" {
			return failf("outputFile requires sharePromotionFolder: true"), nil
		}
		dest := filepath.Join(req.WorkDir, cfg.OutputFile)
		if err = os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return failf("creating output directory: %v", err), nil
		}
		if err = os.WriteFile(dest, []byte(manifests), 0o644); err != nil {
			return failf("writing output file: %v", err), nil
		}
		output["outputFile"] = cfg.OutputFile
		output["bytes"] = len(manifests)
	} else {
		output["manifests"] = manifests
	}

	out, err := json.Marshal(output)
	if err != nil {
		return failf("encoding output: %v", err), nil
	}
	return &sdk.Response{Output: out, Message: fmt.Sprintf("kcl built %s", cfg.Path)}, nil
}

func failf(format string, a ...any) *sdk.Response {
	return &sdk.Response{Failed: true, Message: fmt.Sprintf(format, a...)}
}

func main() {
	socket := os.Getenv("PLUGIN_SOCKET")
	if socket == "" {
		socket = "/run/plugins/kcl-build.sock"
	}
	if err := sdk.Serve(sdk.StepFunc(run), socket); err != nil {
		fmt.Fprintln(os.Stderr, "kcl-plugin:", err)
		os.Exit(1)
	}
}
