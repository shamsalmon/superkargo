package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shamsalmon/kargo-plugin-ext/sdk"
)

func TestKCLBuildToOutput(t *testing.T) {
	resp, err := run(context.Background(), &sdk.Request{
		Config: []byte(`{"path":"testdata/main.k","format":"yaml"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Failed {
		t.Fatalf("failed: %s", resp.Message)
	}

	var out struct {
		Manifests string `json:"manifests"`
		Format    string `json:"format"`
	}
	if err = json.Unmarshal(resp.Output, &out); err != nil {
		t.Fatal(err)
	}
	if out.Format != "yaml" {
		t.Fatalf("format = %q", out.Format)
	}
	for _, want := range []string{"name: demo", "replicas: 3", "image: nginx:1.27"} {
		if !strings.Contains(out.Manifests, want) {
			t.Fatalf("manifests missing %q:\n%s", want, out.Manifests)
		}
	}
}

func TestKCLBuildToFile(t *testing.T) {
	workDir := t.TempDir()
	src, err := os.ReadFile("testdata/main.k")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(workDir, "main.k"), src, 0o644); err != nil {
		t.Fatal(err)
	}

	resp, err := run(context.Background(), &sdk.Request{
		Config:  []byte(`{"path":"main.k","outputFile":"rendered/manifests.yaml"}`),
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Failed {
		t.Fatalf("failed: %s", resp.Message)
	}

	written, err := os.ReadFile(filepath.Join(workDir, "rendered", "manifests.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "name: demo") {
		t.Fatalf("rendered file missing content:\n%s", written)
	}
}

func TestKCLBuildFailure(t *testing.T) {
	resp, err := run(context.Background(), &sdk.Request{
		Config: []byte(`{"path":"testdata/does-not-exist.k"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Failed || !strings.Contains(resp.Message, "kcl build failed") {
		t.Fatalf("expected build failure, got failed=%v msg=%q", resp.Failed, resp.Message)
	}
}
