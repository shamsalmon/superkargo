SHELL := /bin/bash

# controller-gen is fetched on demand from the module proxy (no local checkout
# needed). Override to a prebuilt binary if you prefer.
CONTROLLER_GEN ?= go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0
# Target architecture for in-cluster images (match your nodes; docker-desktop on
# Apple Silicon is arm64).
GOARCH ?= arm64

CONTROLLER_IMAGE ?= superkargo-chart:dev
KCL_PLUGIN_IMAGE ?= kcl-build-plugin:dev

KARGO_CHART ?= oci://ghcr.io/akuity/kargo-charts/kargo
KARGO_CHART_VERSION ?= 1.10.7

# --- build / test (across all modules: root, sdk, and each plugin) ---

.PHONY: build
build:
	go build -o bin/superkargo-controller ./cmd/controller

.PHONY: test
test:
	go test -race ./...
	cd sdk && go test -race ./...
	cd examples/kcl-plugin && go test -race ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: tidy
tidy:
	go mod tidy
	cd sdk && go mod tidy
	cd examples/kcl-plugin && go mod tidy
	cd examples/hello-world-plugin && go mod tidy

.PHONY: codegen
codegen: generate manifests

.PHONY: generate
generate:
	$(CONTROLLER_GEN) object paths=./api/v1alpha1/...

.PHONY: manifests
manifests:
	$(CONTROLLER_GEN) crd paths=./api/v1alpha1/... output:crd:dir=config/crd

# --- images ---

# Controller image: the official Kargo image with the controller subcommand
# replaced by ours, so the whole Helm release runs from one image.
.PHONY: image
image:
	GOOS=linux GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o dist/linux/superkargo-controller ./cmd/controller
	docker build --provenance=false -f Dockerfile -t $(CONTROLLER_IMAGE) .

# Plugin sidecar image(s) — each plugin is its own module/build/image.
.PHONY: plugin-images
plugin-images:
	cd examples/kcl-plugin && \
		GOOS=linux GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o dist/kcl-build . && \
		docker build --provenance=false -t $(KCL_PLUGIN_IMAGE) .

.PHONY: images
images: image plugin-images

# --- deploy via the default Kargo Helm chart ---

# Upgrade an existing Kargo release to run the superkargo controller plus
# the plugin sidecars. Pass extra args via HELM_ARGS (e.g. to preserve admin
# values: HELM_ARGS="-f my-values.yaml").
.PHONY: helm-deploy
helm-deploy:
	kubectl apply -f config/crd
	kubectl apply -f config/helm/customsteps-rbac.yaml
	helm upgrade --install kargo $(KARGO_CHART) --version $(KARGO_CHART_VERSION) -n kargo \
		-f config/helm/values-superkargo.yaml $(HELM_ARGS)
