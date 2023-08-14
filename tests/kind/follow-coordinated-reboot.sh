#!/usr/bin/env bash

NODECOUNT=${NODECOUNT:-5}
KUBECTL_CMD="${KUBECTL_CMD:-kubectl}"
DEBUG="${DEBUG:-false}"
CONTAINER_NAME_FORMAT=${CONTAINER_NAME_FORMAT:-"chart-testing-*"}

tmp_dir=$(mktemp -d -t kured-XXXX)
function gather_logs_and_cleanup {
    if [[ -f "$tmp_dir"/node_output ]]; then
        rm "$tmp_dir"/node_output
    fi
    rmdir "$tmp_dir"

    # The next commands are useful regardless of success or failures.
    if [[ "$DEBUG" == "true" ]]; then
        echo "############################################################"
        # This is useful to see if containers have crashed.
        echo "docker ps -a:"
        docker ps -a
	echo "docker journal logs"
	journalctl -u docker --no-pager

        # This is useful to see if the nodes have _properly_ rebooted.
        # It should show the reboot/two container starts per node.
        for name in $(docker ps -a -f "name=${CONTAINER_NAME_FORMAT}" -q); do
            echo "############################################################"
            echo "docker logs for container $name:"
            docker logs "$name"
        done

    fi
}
trap gather_logs_and_cleanup EXIT

declare -A was_unschedulable
declare -A has_recovered
max_attempts="60"
sleep_time=60
attempt_num=1

set +o errexit
echo "There are $NODECOUNT nodes in the cluster"
until [ ${#was_unschedulable[@]} == "$NODECOUNT" ] && [ ${#has_recovered[@]} == "$NODECOUNT" ]
do
    echo "${#was_unschedulable[@]} nodes were removed from pool once:" "${!was_unschedulable[@]}"
    echo "${#has_recovered[@]} nodes removed from the pool are now back:" "${!has_recovered[@]}"

    #"$KUBECTL_CMD" logs -n kube-system -l name=kured --ignore-errors > "$tmp_dir"/node_output
    #if [[ "$DEBUG" == "true" ]]; then
    #    echo "Kured pod logs:"
    #    cat "$tmp_dir"/node_output
    #fi

    "$KUBECTL_CMD" get nodes -o custom-columns=NAME:.metadata.name,SCHEDULABLE:.spec.unschedulable --no-headers > "$tmp_dir"/node_output
    if [[ "$DEBUG" == "true" ]]; then
        # This is useful to see if a node gets stuck after drain, and doesn't
        # come back up.
        echo "Result of command $KUBECTL_CMD get nodes ... showing unschedulable nodes:"
        cat "$tmp_dir"/node_output
    fi
    while read -r node; do
        unschedulable=$(echo "$node" | grep true | cut -f 1 -d ' ')
        if [ -n "$unschedulable" ] && [ -z ${was_unschedulable["$unschedulable"]+x} ] ; then
            echo "$unschedulable is now unschedulable!"
            was_unschedulable["$unschedulable"]=1
        fi
        schedulable=$(echo "$node" | grep '<none>' | cut -f 1 -d ' ')
        if [ -n "$schedulable" ] && [ ${was_unschedulable["$schedulable"]+x} ] && [ -z ${has_recovered["$schedulable"]+x} ]; then
            echo "$schedulable has recovered!"
            has_recovered["$schedulable"]=1
        fi
    done < "$tmp_dir"/node_output

    if [[ "${#has_recovered[@]}" == "$NODECOUNT" ]]; then
        echo "All nodes recovered."
        break
    else
        if (( attempt_num == max_attempts ))
        then
            echo "Attempt $attempt_num failed and there are no more attempts left!"
            exit 1
        else
            echo "Attempt $attempt_num failed! Trying again in $sleep_time seconds..."
            sleep "$sleep_time"
        fi
    fi
    (( attempt_num++ ))
done

echo "Test successful"
