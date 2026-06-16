# kargo-plugin-ext

> ⚠️ **Not affiliated with Akuity or the Kargo project.** This is an independent,
> community experiment — not endorsed by, supported by, or part of Kargo. It is
> unofficial and provided as-is. Use at your own risk.

An extension layer for [Kargo](https://github.com/akuity/kargo) that adds
**custom promotion steps** via plugins which run as **sidecar containers** —
each with its own Go module, build, dependencies, and image — communicating with
the controller over gRPC on a unix socket.

It demonstrates *one* way to run arbitrary custom logic as a promotion step on
the open-source Kargo controller.

> 💙 **Please support Kargo.** Robust, supported promotion capabilities are best
> served by **Kargo Enterprise** and the **Akuity Platform**. If Kargo is useful
> to you, buy it and check out the platform: <https://akuity.io/akuity-platform>.
> Consider this project a hobbyist alternative for the OSS controller, not a
> replacement for the supported product.

## What it does

A `CustomPromotionStep` resource binds a step name to a plugin:

```yaml
apiVersion: plugin.kargo.akuity.io/v1alpha1
kind: CustomPromotionStep
metadata:
  name: kcl-build
spec:
  plugin: kcl-build           # the sidecar that serves this step
  sharePromotionFolder: true  # share the promotion working directory
```

A `Promotion` then references it by name; the controller hands the step's
evaluated config (and, optionally, the promotion's working directory) to the
plugin and uses the plugin's response as the step output:

```yaml
steps:
- uses: kcl-build
  as: build
  config:
    path: src/kcl
    outputFile: out/manifests.yaml
```

## Architecture

```
Promotion step (uses: kcl-build)
   │
   ▼  orchestrator.registry.Get("kcl-build")
DynamicRegistry ──miss in Kargo's built-in registry──► GET CustomPromotionStep "kcl-build"
   │
   ▼  pluginRunner dials  unix:///run/plugins/kcl-build.sock  (gRPC, JSON codec)
┌─────────────────────────── controller Pod ───────────────────────────┐
│  controller container            kcl-build sidecar (its own image)    │
│  - Kargo controller + this ext   - own module: SDK + kcl-go           │
│  - dials the plugin socket       - serves sdk.Step over gRPC          │
│        │                                  ▲                            │
│        └── /run/plugins (emptyDir) socket ┘                           │
│        └── /tmp (emptyDir) shared promotion workdir ──────────────────│
└───────────────────────────────────────────────────────────────────────┘
```

- **Plugins are sidecars.** Each plugin is its own Go module and container image.
  Because it is a separate build, it can use any dependencies it likes (the
  `kcl-build` plugin embeds the heavy KCL toolchain) **without affecting the
  controller's dependency graph**. The controller never imports a plugin.
- **gRPC over a unix socket.** The contract lives in a tiny SDK module
  ([`sdk/`](sdk)) with a minimal dependency footprint. It uses gRPC (HTTP/2) with
  a JSON codec, so there is no protobuf code generation. Plugins implement one
  method, `Run`, and call `sdk.Serve`.
- **`sharePromotionFolder`** works because the controller and the sidecars share
  the promotion working directory via a `/tmp` `emptyDir` volume. The plugin
  reads source / writes rendered output in the same files the controller's
  built-in `git-clone` / `git-commit` steps use.
- **`promotion.StepRunnerRegistry` is an interface.** `pkg/pluginstep.DynamicRegistry`
  wraps Kargo's built-in registry and, on a miss, resolves a `CustomPromotionStep`
  of the same name — the single dispatch point.
- **Synchronous execution.** The plugin runs to completion within one RPC, so
  there is no Job-style Running/RetryAfter lifecycle in the runner.

### Writing a plugin

A plugin is a small `main` in its own module that depends only on the SDK (plus
whatever it needs):

```go
func run(_ context.Context, req *sdk.Request) (*sdk.Response, error) {
    out, _ := json.Marshal(map[string]any{"message": "hi"})
    return &sdk.Response{Output: out}, nil
}

func main() { sdk.Serve(sdk.StepFunc(run), os.Getenv("PLUGIN_SOCKET")) }
```

See [`examples/hello-world-plugin`](examples/hello-world-plugin) (SDK only) and
[`examples/kcl-plugin`](examples/kcl-plugin) (SDK + `kcl-lang.io/kcl-go`).

## Example: render KCL and open a PR

`config/samples/kcl/` is an end-to-end GitOps flow. A Warehouse watches a Git
repo of KCL source; a Promotion renders it with the `kcl-build` sidecar and
proposes the result as a pull request:

1. **`git-clone`** — check out the Freight's source commit and a `rendered`
   branch.
2. **`kcl-build`** (sidecar) — render `src/kcl` into `out/manifests.yaml`.
3. **`git-commit`** + **`git-push`** (`generateTargetBranch: true`) — commit and
   push to a generated branch.
4. **`git-open-pr`** — open a pull request into the `rendered` branch.
5. **`git-wait-for-pr`** — the Promotion stays *Running* until the PR is merged,
   then succeeds.

All of `git-*` are Kargo's own built-in steps; only `kcl-build` is the plugin.

## Modules

This repo is a small multi-module workspace so plugins stay isolated from the
controller and from each other:

| Module | Path | Depends on |
|--------|------|------------|
| Controller + extension | `.` (root) | Kargo, the SDK |
| Plugin SDK (gRPC contract) | `sdk/` | gRPC only |
| `kcl-build` plugin | `examples/kcl-plugin/` | SDK, `kcl-go` |
| `hello-world` plugin | `examples/hello-world-plugin/` | SDK |

Root and plugins consume the SDK via a local `replace`; the controller never
imports `kcl-go`.

## Deploy (via the default Kargo Helm chart)

The controller image is the **official Kargo image with the `controller`
subcommand replaced by ours** (see [`Dockerfile`](Dockerfile) +
[`hack/kargo-shim.sh`](hack/kargo-shim.sh)). The chart uses one image for every
component, and our shim passes all non-`controller` subcommands through to the
upstream binary — so api/webhooks/etc. are unchanged. Plugins are added as
**native sidecars** (init containers with `restartPolicy: Always`) via
`controller.initContainers`, sharing the sockets and `/tmp` volumes.

```bash
make images          # build the controller image + plugin sidecar image(s)
make helm-deploy     # helm upgrade with config/helm/values-plugin-ext.yaml
                     #   (preserve existing release values via HELM_ARGS="-f ...")
```

[`config/helm/values-plugin-ext.yaml`](config/helm/values-plugin-ext.yaml) is the
only overlay you need; it overrides just the image, the controller env, and the
plugin sidecars/volumes. On docker-desktop a locally-built image is served to the
cluster via its registry mirror (plain tag + `pullPolicy: Always`); on a real
cluster, push the images to a registry the nodes can reach.

## Dependency on Kargo

The controller depends on Kargo as a Go library at **v1.10.7**, resolved from the
module proxy — **no local Kargo checkout is required to build**. Two wrinkles,
both handled in `go.mod`:

- Kargo doesn't publish/tag its nested `api` module, and the published `kargo`
  module requires it at a tag that doesn't exist. It's pinned to the v1.10.7
  commit via a proxy pseudo-version (bump it when bumping Kargo).
- Kargo's kustomize version pins are repeated here, since `replace` directives
  don't propagate to downstream modules.

(The Makefile's `controller-gen` and the optional Tiltfile's chart path can still
point at a local Kargo checkout for convenience, but both are overridable and not
needed for `go build` / `make image`.)

The controller bootstrap mirrors Kargo's own `cmd/controlplane/controller.go`
(including the optional Argo CD / Argo Rollouts integrations, which self-disable
when those CRDs are absent). The one genuinely custom piece is a ~40-line engine
seam that supplies the `DynamicRegistry` where Kargo's `promotion.NewLocalEngine`
hardcodes the default registry.

## Build & test

```bash
make build           # build the controller
make test            # test root + sdk + plugin modules
make images          # controller image + plugin sidecar image(s)
make codegen         # regenerate CRD + deepcopy (controller-gen)
```

## Develop with Tilt

A [`Tiltfile`](Tiltfile) brings up a full local Kargo (via the chart) with the
controller and the `kcl-build` sidecar, with live-reload: edit Go → recompile →
rebuild image → redeploy. It needs a local cluster (docker-desktop/orbstack/kind),
cert-manager, and a local Kargo chart (`KARGO_CHART`, default
`../kargo/charts/kargo`).

```bash
tilt up      # Tilt UI: http://localhost:10350   Kargo UI: http://localhost:30081 (admin/admin)
```

## Status

A working proof of concept. It is unofficial and unsupported — for anything you
depend on, use Kargo and the [Akuity Platform](https://akuity.io/akuity-platform).
