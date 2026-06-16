package sdk

import (
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"google.golang.org/grpc"
)

// Serve runs a plugin as a gRPC server listening on a unix socket at
// socketPath, blocking until the process is signaled (SIGINT/SIGTERM). A
// plugin's main() typically calls this and nothing else.
func Serve(impl Step, socketPath string) error {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return err
	}
	// Remove any stale socket left by a previous run.
	_ = os.Remove(socketPath)

	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	// Make the socket connectable by other containers in the pod, which may run
	// as a different user than this plugin (connecting to a unix socket requires
	// write permission on the socket file). Pod isolation makes this safe.
	if err = os.Chmod(socketPath, 0o777); err != nil {
		return err
	}

	srv := grpc.NewServer()
	srv.RegisterService(&stepServiceDesc, impl)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		srv.GracefulStop()
	}()

	err = srv.Serve(lis)
	_ = os.Remove(socketPath)
	return err
}
