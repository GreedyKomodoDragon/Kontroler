#!/usr/bin/env bash
set -euo pipefail

NAMESPACE=default
SECRET_NAME=postgres-postgresql

# helper: try to create DB by exec into postgres pod
function try_exec_pod() {
  local pod=$1
  local pw=$2
  # Check if the database exists, create if missing
  exists=$(kubectl -n ${NAMESPACE} exec "${pod}" -- env PGPASSWORD="${pw}" psql -U postgres -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='kontroler';" 2>/dev/null || true)
  if [ "${exists}" = "1" ]; then
    return 0
  fi
  kubectl -n ${NAMESPACE} exec "${pod}" -- env PGPASSWORD="${pw}" psql -U postgres -d postgres -c "CREATE DATABASE kontroler;"
}

# get postgres password
PGPASSWORD=$(kubectl get secret --namespace ${NAMESPACE} ${SECRET_NAME} -o jsonpath='{.data.postgres-password}' | base64 --decode)

# find postgres pod
POSTGRES_POD=$(kubectl -n ${NAMESPACE} get pod -l app.kubernetes.io/name=postgresql -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

if [ -n "${POSTGRES_POD}" ]; then
  echo "Found postgres pod: ${POSTGRES_POD}, attempting to exec and create DB..."
  if try_exec_pod "${POSTGRES_POD}" "${PGPASSWORD}"; then
    echo "Database 'kontroler' ensured via pod exec."
    exit 0
  else
    echo "pod exec method failed, will fall back to running a temporary client pod"
  fi
else
  echo "No postgres pod found, will run a temporary client pod"
fi

# remove any previous postgres-client pod to avoid ImagePullBackOff leftovers
kubectl -n ${NAMESPACE} delete pod postgres-client --ignore-not-found=true || true

# Use official postgres client image (tags should exist) as fallback
CLIENT_IMAGE=postgres:16

echo "Running temporary postgres client pod to create DB (image=${CLIENT_IMAGE})..."

# Run a client pod that checks if the DB exists and creates it if not
kubectl run postgres-client --rm -i --namespace ${NAMESPACE} --image ${CLIENT_IMAGE} \
  --env="PGPASSWORD=${PGPASSWORD}" --restart='Never' --command -- sh -c "\
    exists=\$(psql --host postgres-postgresql -U postgres -d postgres -tAc \"SELECT 1 FROM pg_database WHERE datname='kontroler';\") || true; \
    if [ \"\$exists\" = \"1\" ]; then echo 'Database exists'; else psql --host postgres-postgresql -U postgres -d postgres -c \"CREATE DATABASE kontroler;\"; fi"

echo "Database 'kontroler' ensured." 
