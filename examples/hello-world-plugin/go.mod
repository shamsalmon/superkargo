// The hello-world plugin is its own module, depending only on the plugin SDK.
module github.com/shamsalmon/superkargo/examples/hello-world-plugin

go 1.26.0

require github.com/shamsalmon/superkargo/sdk v0.0.0-00010101000000-000000000000

require (
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

replace github.com/shamsalmon/superkargo/sdk => ../../sdk
