#!/usr/bin/env bash

kubectl_flags=( )
[[ "$1" != "" ]] && kubectl_flags=("${kubectl_flags[@]}" --context "$1")

# To speed up the system, let's not kill the control plane.
for nodename in $(${KUBECTL_CMD:-kubectl} "${kubectl_flags[@]}" get nodes -o name | grep -v control-plane); do
    echo "Creating reboot sentinel on $nodename"
    docker exec "${nodename/node\//}" hostname
    docker exec "${nodename/node\//}" touch "${SENTINEL_FILE:-/run/reboot-required}"
done
