# Image for the kcl-build plugin sidecar. Built from a binary cross-compiled on
# the host (see the repo Makefile's kcl-plugin-image target). debian-slim
# provides the glibc that the embedded KCL toolchain (kcl-lang.io/lib) needs.
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY dist/kcl-build /usr/local/bin/kcl-build

# Default socket + a writable HOME for the KCL module cache. These are typically
# overridden by the sidecar's env / shared volumes.
ENV PLUGIN_SOCKET=/run/plugins/kcl-build.sock \
    HOME=/tmp

ENTRYPOINT ["/usr/local/bin/kcl-build"]
