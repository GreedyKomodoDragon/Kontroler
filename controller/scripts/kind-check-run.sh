#!/usr/bin/env bash
set -euo pipefail

NAMESPACE=default

# get postgres password
SECRET_NAME=postgres-postgresql
PGPASSWORD=$(kubectl get secret --namespace ${NAMESPACE} ${SECRET_NAME} -o jsonpath="{.data.postgres-password}" | base64 --decode)

# show Task_Runs from kontroler DB
kubectl delete pod postgres-client --ignore-not-found -n ${NAMESPACE} || true
kubectl run postgres-client --rm -i --namespace ${NAMESPACE} --image registry-1.docker.io/bitnamicharts/postgresql:latest \
  --env="PGPASSWORD=${PGPASSWORD}" --restart='Never' --command -- psql --host postgres-postgresql -U postgres -d kontroler -c "SELECT task_run_id, run_id, task_id, status, claimed_by FROM Task_Runs ORDER BY task_run_id DESC LIMIT 20;"

# show controller logs
kubectl -n default logs -l app.kubernetes.io/name=kontroler-controller --tail=200

# show pods
kubectl -n default get pods -o wide
