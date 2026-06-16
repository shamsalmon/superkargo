# Tilt dev environment for kargo-plugin-ext, modeled on Kargo's own Tiltfile.
#
# It deploys a full Kargo control plane via the Kargo Helm chart, but with the
# controller swapped for kargo-plugin-ext and the kcl-build plugin running as a
# sidecar. Editing Go source recompiles the affected binary, rebuilds its image,
# and Tilt redeploys.
#
# Prerequisites: a local cluster (docker-desktop/orbstack/kind), cert-manager
# installed, and a local checkout of the Kargo chart (KARGO_CHART, default
# ../kargo/charts/kargo).
#
# Run:   tilt up        UI: http://localhost:30081  (admin / admin)

KARGO_CHART = os.environ.get('KARGO_CHART', '../kargo/charts/kargo')

# --- compile natively (fast); the binaries are copied into thin images ---

local_resource(
  'compile-controller',
  'CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -o dist/linux/kargo-plugin-ext-controller ./cmd/controller',
  deps = ['cmd/', 'internal/', 'pkg/', 'api/', 'go.mod', 'go.sum'],
  # Don't let the compiled output re-trigger the build.
  ignore = ['dist/'],
  labels = ['compile'],
)

local_resource(
  'compile-kcl-plugin',
  'cd examples/kcl-plugin && CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -o dist/kcl-build .',
  deps = ['examples/kcl-plugin/', 'sdk/'],
  # The output (examples/kcl-plugin/dist/) is inside the watched dep dir, so
  # ignore it or the build re-triggers itself forever.
  ignore = ['examples/kcl-plugin/dist/'],
  labels = ['compile'],
)

# --- images: the controller (Kargo image + our shim) and the plugin sidecar ---

docker_build(
  'kargo-plugin-ext-chart',
  '.',
  dockerfile = 'Dockerfile',
  only = ['dist/linux', 'hack/kargo-shim.sh'],
)
docker_build(
  'kcl-build-plugin',
  'examples/kcl-plugin',
  dockerfile = 'examples/kcl-plugin/Dockerfile',
  only = ['dist/kcl-build'],
)

# --- deploy the full Kargo control plane (chart) with our overrides ---

k8s_yaml(
  helm(
    KARGO_CHART,
    name = 'kargo',
    namespace = 'kargo',
    values = [
      'hack/tilt/values.dev.yaml',
      'config/helm/values-plugin-ext.yaml',
    ],
  )
)

# Our CRD + the RBAC the chart doesn't manage + the kcl-build CustomPromotionStep.
k8s_yaml([
  'config/crd/plugin.kargo.akuity.io_custompromotionsteps.yaml',
  'config/helm/customsteps-rbac.yaml',
  'config/samples/kcl/20-custompromotionstep.yaml',
])

k8s_resource(
  workload = 'kargo-api',
  new_name = 'api',
  port_forwards = ['30081:8080'],
  labels = ['kargo'],
)
k8s_resource(
  workload = 'kargo-controller',
  new_name = 'controller',
  resource_deps = ['compile-controller', 'compile-kcl-plugin'],
  labels = ['kargo'],
)
