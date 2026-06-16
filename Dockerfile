# Controller image for deploying via the DEFAULT Kargo Helm chart. It is the
# official Kargo image with the "controller" subcommand replaced by the
# kargo-plugin-ext controller (see hack/kargo-shim.sh). Because the chart uses a
# single image for every component, basing on the official image lets api,
# webhooks, management-controller, etc. keep running the upstream binary while
# only the controller is swapped.
#
# Plugins are NOT in this image — each runs as its own sidecar container with its
# own image (e.g. examples/kcl-plugin/Dockerfile).
#
# Build with the controller binary cross-compiled on the host (make image),
# matching the KARGO_IMAGE base architecture.
ARG KARGO_IMAGE=ghcr.io/akuity/kargo:v1.10.7
FROM ${KARGO_IMAGE}

USER root
RUN mv /usr/local/bin/kargo /usr/local/bin/kargo-upstream

COPY dist/linux/kargo-plugin-ext-controller /usr/local/bin/kargo-plugin-ext-controller
COPY hack/kargo-shim.sh /usr/local/bin/kargo
RUN chmod 0755 /usr/local/bin/kargo /usr/local/bin/kargo-plugin-ext-controller

# Plugins are no longer baked in — they run as sidecar containers and are reached
# over gRPC unix sockets (see PLUGIN_SOCKET_DIR).

# Restore the upstream image's non-root user.
USER 65532
