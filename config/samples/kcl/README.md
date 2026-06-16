# KCL GitOps example

An end-to-end "render manifests and open a PR" flow: a Warehouse watches a Git
repo containing KCL source, and a Promotion **renders** the KCL with the
`kcl-build` plugin sidecar, **proposes** the result as a pull request, and stays
**Running until that PR is merged**.

The `kcl-build` plugin embeds the KCL toolchain via
[`kcl-lang.io/kcl-go`](https://github.com/kcl-lang/kcl-go) (no external `kcl`
binary) and runs as a sidecar container that the controller dials over gRPC.

## Pipeline (the Stage's promotion steps)

1. **`git-clone`** — checks out the Freight's source commit into `./src` and the
   (auto-created) `rendered` branch into `./out`.
2. **`kcl-build`** (this plugin, `sharePromotionFolder: true`) — compiles
   `src/kcl` and writes `out/manifests.yaml`.
3. **`git-commit`** + **`git-push`** (`generateTargetBranch: true`) — commit and
   push to a generated branch.
4. **`git-open-pr`** — open a pull request into the `rendered` branch.
5. **`git-wait-for-pr`** — keep the Promotion Running until the PR is merged.

## Source repo layout

The watched repo just needs KCL under `kcl/` on its `main` branch, e.g.:

```
kcl/
  kcl.mod
  main.k        # renders your Kubernetes manifests
```

## Apply

```bash
# Edit the repoURL/username in the manifests, and put a real token in
# 15-credentials.yaml (a fine-grained PAT or deploy key is recommended).
kubectl apply -f config/crd/                  # the CustomPromotionStep CRD
kubectl apply -f config/samples/kcl/

# Once the Warehouse produces Freight, promote it to the `render` stage (Kargo
# UI / CLI, or a Promotion with the same steps inline). A PR with the rendered
# manifests is opened against the `rendered` branch; the Promotion completes
# when you merge it.
```

> Credentials: the controller clones/pushes and opens/inspects PRs using the Git
> credentials Secret (`kargo.akuity.io/cred-type=git`) in the project namespace,
> matched by repoURL. The token needs `repo` scope (PR read/write).
