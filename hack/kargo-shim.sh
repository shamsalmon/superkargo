#!/bin/sh
# Entry-point shim for the combined image. The Kargo Helm chart runs every
# component from a single image as `/usr/local/bin/kargo <subcommand>`. This shim
# replaces ONLY the "controller" subcommand with the kargo-plugin-ext controller
# (which adds CustomPromotionStep / go-plugin dispatch); every other subcommand
# (api, management-controller, webhooks, garbage-collector, …) falls through to
# the upstream Kargo binary, so the rest of the control plane is unchanged.
if [ "$1" = "controller" ]; then
	shift
	exec /usr/local/bin/kargo-plugin-ext-controller "$@"
fi
exec /usr/local/bin/kargo-upstream "$@"
