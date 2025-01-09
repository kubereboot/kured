#!/usr/bin/env bash

kubectl_flags=( )
[[ "$1" != "" ]] && kubectl_flags=("${kubectl_flags[@]}" --context "$1")

function gather_logs_and_cleanup {
      for id in $(docker ps -q); do
          echo "############################################################"
          echo "docker logs for container $id:"
          docker logs "$id"
      done
      ${KUBECTL_CMD:-kubectl} "${kubectl_flags[@]}" logs ds/kured --all-pods -n kube-system
}
trap gather_logs_and_cleanup EXIT

set +o errexit
worker=$(${KUBECTL_CMD:-kubectl} "${kubectl_flags[@]}" get nodes -o custom-columns=name:metadata.name --no-headers | grep worker | head -n 1)

${KUBECTL_CMD:-kubectl} "${kubectl_flags[@]}" label nodes "$worker" blocked-host=yes

${KUBECTL_CMD:-kubectl} "${kubectl_flags[@]}" apply -f - << EOF
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  labels:
    app: blocker
spec:
  containers:
    - name: nginx
      image: nginx
      imagePullPolicy: IfNotPresent
  nodeSelector:
    blocked-host: "yes"
EOF

docker exec "$worker" touch "${SENTINEL_FILE:-/var/run/reboot-required}"

set -o errexit
max_attempts="100"
attempt_num=1
sleep_time=5

until ${KUBECTL_CMD:-kubectl} "${kubectl_flags[@]}" logs ds/kured --all-pods -n kube-system  | grep -i -e "Reboot.*blocked"
do
  if (( attempt_num == max_attempts )); then
      echo "Attempt $attempt_num failed and there are no more attempts left!"
      exit 1
  else
      echo "Did not find 'reboot blocked' in the log, retrying in $sleep_time seconds (Attempt #$attempt_num)"
      sleep "$sleep_time"
  fi
  (( attempt_num++ ))
done
