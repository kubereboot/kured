#!/usr/bin/env bash

# USE KUBECTL_CMD to pass context and/or namespaces.
KUBECTL_CMD="${KUBECTL_CMD:-kubectl}"
SENTINEL_FILE="${SENTINEL_FILE:-/var/run/reboot-required}"

echo "Creating reboot sentinel on worker nodes"

# To speed up the system, let's not kill the control plane.
for nodename in $("$KUBECTL_CMD" get nodes -o name | grep -v control-plane); do
    docker exec "${nodename/node\//}" hostname
    docker exec "${nodename/node\//}" touch "${SENTINEL_FILE}"
done
