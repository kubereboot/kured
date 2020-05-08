#!/bin/sh

set -o errexit
###############
# User config #
###############

# How to use this?
# Define env vars KIND_IMAGE, KIND_CLUSTER_NAME (or let them be the default),
# and the KURED_IMAGE_DEST (the public repo which will contain the dev version
# of the image to test).
# The process will build kured (using make image), tag the image with
# KURED_IMAGE_DEST, publish it somewhere, spin up a kind cluster, install
# kured in it, create a reboot sentinel file, track if reboot is happening
# by watching the kubernetes api.
# The resources files/kind cluster are destroyed at the end of the process
# unless DO_NOT_TEARDOWN is not empty
# For CI, you should probably set -x before running this script.

#Define the image used in kind, for example:
#KIND_IMAGE=kindest/node:v1.18.2
#KIND_IMAGE=kindest/node:v1.18.0@sha256:0e20578828edd939d25eb98496a685c76c98d54084932f76069f886ec315d694
#KIND_IMAGE=kindest/node:v1.17.0@sha256:9512edae126da271b66b990b6fff768fbb7cd786c7d39e86bdf55906352fdf62
#KIND_IMAGE=kindest/node:v1.16.4@sha256:b91a2c2317a000f3a783489dfb755064177dbc3a0b2f4147d50f04825d016f55
KIND_IMAGE=${KIND_IMAGE:-"kindest/node:v1.18.2"}

# desired cluster name; default is "kured"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-kured}"

KURED_IMAGE_DEST=${KURED_IMAGE_DEST:-"docker.io/evrardjp/kured:latest"}

#############
# Functions #
#############

kubectl="kubectl --context kind-$KIND_CLUSTER_NAME"
# change nodecount if you change the default kind cluster size
nodecount=5

cleanup_tmpfiles() {
	echo "Cleaning up kured tmp folders"
	rm -rf /tmp/kured-*
}

teardown_kindcluster() {
	echo "Tearing down the kind cluster $KIND_CLUSTER_NAME"
	kind delete cluster --name $KIND_CLUSTER_NAME
}

cleanup() {
	if [ -z "$DO_NOT_TEARDOWN" ]; then
		echo "Tearing down cluster"
		cleanup_tmpfiles
		teardown_kindcluster
        else
		echo "Gathering cluster logs"
		kind export logs $tmp_dir/logs --name $KIND_CLUSTER_NAME
        fi
}

retry() {
	set +o errexit
	local -r -i max_attempts="$1"; shift
	local -r cmd="$@"
	local -i attempt_num=1

	until $cmd
	do
		if (( attempt_num == max_attempts ))
		then
			echo "Attempt $attempt_num failed and there are no more attempts left!"
			exit 1
		else
			echo "Attempt $attempt_num failed! Trying again in $attempt_num seconds..."
			sleep $(( attempt_num++ ))
		fi
	done
	set -o errexit
}
####################
# Kured build step #
####################

build_and_push_kuredimage() {
	make image
        docker tag docker.io/weaveworks/kured $KURED_IMAGE_DEST
        docker push $KURED_IMAGE_DEST
}

############################
# Kind cluster create step #
############################

gen_kind_manifest() {
	echo "Generating kind.yaml"
	# Please don't name your kind nodes ".*true.*" or "<none>", if you name them
	cat <<EOF > $tmp_dir/kind.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  image: $KIND_IMAGE
- role: control-plane
  image: $KIND_IMAGE
- role: control-plane
  image: $KIND_IMAGE
- role: worker
  image: $KIND_IMAGE
- role: worker
  image: $KIND_IMAGE
EOF
}

check_all_nodes_are_ready() {
	$kubectl get nodes | grep Ready | wc -l | grep $nodecount > /dev/null
}

spinup_cluster() {
	gen_kind_manifest
	kind create cluster --name "${KIND_CLUSTER_NAME}" --config=$tmp_dir/kind.yaml
	retry 20 check_all_nodes_are_ready
	echo "Cluster ready"
}

######################
# Kured install step #
######################

gen_kured_manifests() {
	echo "Generating kured manifests in folder $tmp_dir"
	cp kured-ds.yaml kured-rbac.yaml $tmp_dir/
	cat <<EOF > $tmp_dir/kustomization.yaml
#kustomize.yaml base
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - kured-ds.yaml
  - kured-rbac.yaml
patchesStrategicMerge:
  - kured-dev.yaml
EOF

	cat <<EOF > $tmp_dir/kured-dev.yaml
# kured-dev.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kured
  namespace: kube-system
spec:
  template:
    spec:
      containers:
      - name: kured
        image: $KURED_IMAGE_DEST
        imagePullPolicy: Always
        command:
        - /usr/bin/kured
        - --period=1m
EOF
}

check_kured_installed(){
	$kubectl get ds -n kube-system | egrep "kured.*$nodecount.*$nodecount.*$nodecount.*$nodecount.*$nodecount" > /dev/null
}

install_kured() {
	gen_kured_manifests
	$kubectl apply -k $tmp_dir
	retry 20 check_kured_installed
	echo "Kured is installed now"
}

######################
# Restart nodes step #
######################

create_reboot_sentinels() {
	echo "Creating reboot sentinel on all nodes"
	for podname in `$kubectl get pods -n kube-system -l name=kured -o name`; do
		$kubectl exec $podname -n kube-system -- /usr/bin/nsenter -m/proc/1/ns/mnt -- touch /var/run/reboot-required
	done
}

follow_coordinated_reboot() {
	set +o errexit
	declare -A was_unschedulable
	declare -A has_recovered

	local -r -i max_attempts="60" #20 minutes
	local -i attempt_num=1

	until [ ${#was_unschedulable[@]} == $nodecount ] && [ ${#has_recovered[@]} == $nodecount ]
	do
		echo "${#was_unschedulable[@]} nodes were removed from pool once: ${!was_unschedulable[@]}"
		echo "${#has_recovered[@]} nodes removed from the pool are now back: ${!has_recovered[@]}"

		$kubectl get nodes -o custom-columns=NAME:.metadata.name,SCHEDULABLE:.spec.unschedulable --no-headers > $tmp_dir/node_output
		while read node; do
			unschedulable=$(echo $node | grep true | cut -f 1 -d ' ')
			if [ -n "$unschedulable" ] && [ -z ${was_unschedulable["$unschedulable"]+x} ] ; then
				echo "$unschedulable is now unschedulable!"
				was_unschedulable["$unschedulable"]=1
			fi
			schedulable=$(echo $node | grep '<none>' | cut -f 1 -d ' ')
			if [ -n "$schedulable" ] && [ ${was_unschedulable["$schedulable"]+x} ] && [ -z ${has_recovered["$schedulable"]+x} ]; then
				echo "$schedulable has recovered!"
				has_recovered["$schedulable"]=1
			fi
		done < $tmp_dir/node_output

		if [ ${#has_recovered[@]} == $nodecount ]; then
			break
		else
			if (( attempt_num == max_attempts ))
			then
				echo "Attempt $attempt_num failed and there are no more attempts left!"
				exit 1
			else
				echo "Attempt $attempt_num failed! Trying again in 20 seconds..."
				sleep 20
			fi
		fi
		(( attempt_num++ ))
	done
	set -o errexit
	rm $tmp_dir/node_output
}

functional_test() {
	create_reboot_sentinels
	follow_coordinated_reboot
	echo "Test successful"
}

########
# MAIN #
########

trap 'cleanup' ERR EXIT

tmp_dir=$(mktemp -d -t kured-XXXX)

build_and_push_kuredimage
spinup_cluster
install_kured
functional_test
