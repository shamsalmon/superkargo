package sdk

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

const (
	serviceName = "kargoext.plugin.Step"
	runMethod   = "/kargoext.plugin.Step/Run"
	codecName   = "kargoext-json"
)

// jsonCodec is a gRPC codec that marshals messages as JSON. Using it lets the
// contract be defined with plain Go structs and no protobuf code generation,
// while still communicating over real gRPC (HTTP/2).
type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string                       { return codecName }

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

// stepServiceDesc is the hand-written gRPC service descriptor for the single
// Step.Run unary method.
var stepServiceDesc = grpc.ServiceDesc{
	ServiceName: serviceName,
	HandlerType: (*Step)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Run",
			Handler:    runHandler,
		},
	},
	Streams: []grpc.StreamDesc{},
}

func runHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	req := new(Request)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(Step).Run(ctx, req)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: runMethod}
	handler := func(ctx context.Context, r any) (any, error) {
		return srv.(Step).Run(ctx, r.(*Request))
	}
	return interceptor(ctx, req, info, handler)
}
