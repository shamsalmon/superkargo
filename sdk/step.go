// Package sdk defines the contract between the kargo-plugin-ext controller and
// promotion-step plugins. Plugins run as their own containers (sidecars) and
// serve this contract over gRPC on a unix socket; the controller dials them.
//
// A plugin implements Step and calls Serve. The controller dials with Dial.
package sdk

import "context"

// Request is the input handed to a plugin for a single step execution.
type Request struct {
	// Config is the referencing Promotion step's already-evaluated config,
	// encoded as JSON.
	Config []byte `json:"config,omitempty"`
	// Project, Stage, Promotion, and Alias identify the promotion context.
	Project   string `json:"project,omitempty"`
	Stage     string `json:"stage,omitempty"`
	Promotion string `json:"promotion,omitempty"`
	Alias     string `json:"alias,omitempty"`
	// WorkDir is the promotion's working directory. It is populated only when the
	// CustomPromotionStep sets sharePromotionFolder: true. The plugin sidecar
	// shares this directory with the controller via a shared volume.
	WorkDir string `json:"workDir,omitempty"`
}

// Response is a plugin's result for a single step execution.
type Response struct {
	// Output is the step output, encoded as a JSON object.
	Output []byte `json:"output,omitempty"`
	// Message is an optional human-readable message.
	Message string `json:"message,omitempty"`
	// Failed indicates the step failed; the controller treats this as a terminal
	// failure of the step.
	Failed bool `json:"failed,omitempty"`
}

// Step is the interface a promotion-step plugin implements.
type Step interface {
	Run(context.Context, *Request) (*Response, error)
}

// StepFunc adapts a function to the Step interface.
type StepFunc func(context.Context, *Request) (*Response, error)

// Run implements Step.
func (f StepFunc) Run(ctx context.Context, req *Request) (*Response, error) {
	return f(ctx, req)
}
