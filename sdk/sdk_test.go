package sdk

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServeAndDial(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "p.sock")

	impl := StepFunc(func(_ context.Context, req *Request) (*Response, error) {
		return &Response{Output: req.Config, Message: "ok:" + req.Promotion}, nil
	})
	go func() { _ = Serve(impl, sock) }()

	// Wait for the socket to appear.
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	c, err := Dial(sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.Run(ctx, &Request{Config: []byte(`{"a":1}`), Promotion: "p1"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Message != "ok:p1" {
		t.Fatalf("message = %q, want ok:p1", resp.Message)
	}
	if string(resp.Output) != `{"a":1}` {
		t.Fatalf("output = %q, want {\"a\":1}", string(resp.Output))
	}
	if resp.Failed {
		t.Fatal("unexpected Failed=true")
	}
}
