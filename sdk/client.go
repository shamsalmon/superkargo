package sdk

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a connection to a plugin's gRPC unix socket.
type Client struct {
	conn *grpc.ClientConn
}

// Dial connects to a plugin serving on the given unix socket path. The
// connection is lazy; it is established on the first call.
func Dial(socketPath string) (*Client, error) {
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
	)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

// Run invokes the plugin's Run method.
func (c *Client) Run(ctx context.Context, req *Request) (*Response, error) {
	resp := new(Response)
	if err := c.conn.Invoke(ctx, runMethod, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Close closes the connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
