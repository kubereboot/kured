#!/usr/bin/env bash
DEBUG="${DEBUG:-false}"
CONTAINER_NAME_FORMAT=${CONTAINER_NAME_FORMAT:-"chart-testing-*"}

kubectl_flags=( )
[[ "$1" != "" ]] && kubectl_flags=("${kubectl_flags[@]}" --context "$1")
REBOOTCOUNT=${2:-1} # Number of nodes expected to reboot, defaulting to 1.

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
        for id in $(docker ps -a -q); do
            echo "############################################################"
            echo "docker logs for container $id:"
            docker logs "$id"
        done

    fi
}
trap gather_logs_and_cleanup EXIT

declare -A was_unschedulable
declare -A has_recovered
max_attempts="200"
sleep_time=5
attempt_num=1

# Get docker info of each of those kind containers. If one has crashed, restart it.

set +o errexit
echo "There are $REBOOTCOUNT nodes total needing reboot in the cluster"
until [ ${#was_unschedulable[@]} == "$REBOOTCOUNT" ] && [ ${#has_recovered[@]} == "$REBOOTCOUNT" ]
do
    echo "${#was_unschedulable[@]} nodes were removed from pool once:" "${!was_unschedulable[@]}"
    echo "${#has_recovered[@]} nodes removed from the pool are now back:" "${!has_recovered[@]}"


    ${KUBECTL_CMD:-kubectl} "${kubectl_flags[@]}" get nodes -o custom-columns=NAME:.metadata.name,SCHEDULABLE:.spec.unschedulable --no-headers | grep -v control-plane > "$tmp_dir"/node_output
    if [[ "$DEBUG" == "true" ]]; then
        # This is useful to see if a node gets stuck after drain, and doesn't
        # come back up.
        echo "Result of command kubectl unschedulable nodes:"
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

        # If the container has crashed, restart it.
        node_name=$(echo "$node" | cut -f 1 -d ' ')
        stopped_container_id=$(docker container ls --filter=name="$node_name" --filter=status=exited -q)
        if [ -n "$stopped_container_id" ]; then echo "Node $stopped_container_id needs restart"; docker start "$stopped_container_id"; echo "Container started."; fi

    done < "$tmp_dir"/node_output

    if [[ "${#has_recovered[@]}" == "$REBOOTCOUNT" ]]; then
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
