// The kcl-build plugin is its OWN module: it depends only on the plugin SDK and
// the KCL toolchain, never on Kargo or the controller. It builds to its own
// container image and runs as a sidecar.
module github.com/shamsalmon/superkargo/examples/kcl-plugin

go 1.26.0

require (
	github.com/shamsalmon/superkargo/sdk v0.0.0-00010101000000-000000000000
	kcl-lang.io/kcl-go v0.12.3
)

require (
	github.com/chai2010/jsonv v1.1.3 // indirect
	github.com/chai2010/protorpc v1.1.4 // indirect
	github.com/ebitengine/purego v0.9.1 // indirect
	github.com/goccy/go-yaml v1.19.0 // indirect
	github.com/gofrs/flock v0.12.1 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/rogpeppe/go-internal v1.15.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	kcl-lang.io/lib v0.12.3 // indirect
)

replace github.com/shamsalmon/superkargo/sdk => ../../sdk
