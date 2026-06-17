// Module github.com/shamsalmon/superkargo/sdk is the superkargo plugin
// SDK: the gRPC contract between the controller and promotion-step plugins. It
// deliberately has a tiny dependency footprint (just gRPC) so plugin modules can
// import it without pulling in Kargo or the controller.
module github.com/shamsalmon/superkargo/sdk

go 1.26.0

require google.golang.org/grpc v1.79.3

require (
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)
