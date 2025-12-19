#!/usr/bin/env bash

kubectl_flags=( )
[[ "$1" != "" ]] && kubectl_flags=("${kubectl_flags[@]}" --context "$1")

cordon() {
  kubectl "${kubectl_flags[@]}" cordon "${precordonned_node}"
}

create_sentinel() {
  docker exec "${precordonned_node}" touch "${SENTINEL_FILE:-/var/run/reboot-required}"
  docker exec "${notcordonned_node}" touch "${SENTINEL_FILE:-/var/run/reboot-required}"
}

check_reboot_required() {
  while true;
  do
    docker exec "${precordonned_node}" stat /var/run/reboot-required > /dev/null && echo "Reboot still required" || return 0
    sleep 3
  done
}

check_node_back_online_as_cordonned() {
  sleep 5 # For safety, wait for 5 seconds, so that the kubectl command succeeds.
  # This test might be giving us false positive until we work on reliability of the
  # test.
  while true;
  do
    result=$(kubectl "${kubectl_flags[@]}" get node "${precordonned_node}" --no-headers | awk '{print $2;}')
    test "${result}" != "Ready,SchedulingDisabled" && echo "Node ${precordonned_node} in state ${result}" || return 0
    sleep 3
  done
}

check_node_back_online_as_uncordonned() {
  while true;
  do
    result=$(kubectl "${kubectl_flags[@]}" get node "${notcordonned_node}" --no-headers | awk '{print $2;}')
    test "${result}" != "Ready" && echo "Node ${notcordonned_node} in state ${result}" || return 0
    sleep 3
  done
}
### Start main

worker_nodes=$(${KUBECTL_CMD:-kubectl} "${kubectl_flags[@]}" get nodes -o custom-columns=name:metadata.name --no-headers | grep worker)
precordonned_node=$(echo "$worker_nodes" | head -n 1)
notcordonned_node=$(echo "$worker_nodes" | tail -n 1)

# Wait for kured to install correctly
sleep 15
cordon
create_sentinel
check_reboot_required
echo "Node has rebooted, but may take time to come back ready"
check_node_back_online_as_cordonned
check_node_back_online_as_uncordonned
echo "Showing final node state"
${KUBECTL_CMD:-kubectl} "${kubectl_flags[@]}" get nodes
echo "Test successful"