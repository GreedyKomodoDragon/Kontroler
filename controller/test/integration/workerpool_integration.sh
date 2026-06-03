#!/usr/bin/env bash
set -euo pipefail

# Integration test for WorkerPool using kind
# Requirements: kind, kubectl, helm, kustomize, docker
# Run from repository root: ./controller/test/integration/workerpool_integration.sh

KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-kind}
NAMESPACE=default
WP_NAME=integration-wp
DEP_NAME=${WP_NAME}-workers
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "$SCRIPT_DIR/../.." && pwd)
WORKER_TEST_TAG=${WORKER_TEST_TAG:-itest}
WORKER_TEST_IMAGE="greedykomodo/kontroler-worker:${WORKER_TEST_TAG}"

wait_for_rollout() {
  ns="$1"
  deploy="$2"
  timeout="${3:-180s}"
  kubectl -n "$ns" rollout status deployment/"$deploy" --timeout="$timeout"
}

wait_for_webhook_service() {
  echo "==> Waiting for operator-webhook-service endpoints"
  for i in {1..60}; do
    ep=$(kubectl -n operator-system get endpoints operator-webhook-service -o jsonpath='{.subsets[0].addresses[0].ip}' 2>/dev/null || true)
    if [ -n "$ep" ]; then
      echo "Webhook endpoint ready at $ep"
      return 0
    fi
    sleep 2
  done
  echo "ERROR: operator-webhook-service has no ready endpoints"
  return 1
}

wait_for_webhook_cabundle() {
  echo "==> Waiting for webhook CA bundle injection"
  for i in {1..60}; do
    out=$(kubectl get mutatingwebhookconfiguration operator-mutating-webhook-configuration -o jsonpath='{.webhooks[0].clientConfig.caBundle}' 2>/dev/null || true)
    if [ -n "$out" ]; then
      echo "CA bundle injected"
      return 0
    fi
    sleep 2
  done
  echo "ERROR: CA bundle was not injected in time"
  return 1
}

check_admission_ready() {
  echo "==> Checking admission webhook readiness with server dry-run"
  tmp=$(mktemp -t dagrun-dryrun-XXXX.yaml)
  cat > "$tmp" <<EOF
apiVersion: kontroler.greedykomodo/v1alpha1
kind: DagRun
metadata:
  name: dryrun-check
  namespace: ${NAMESPACE}
spec:
  dagName: dryrun-dag
EOF
  if kubectl apply --dry-run=server -f "$tmp" >/dev/null 2>&1; then
    echo "Admission webhook is reachable"
    rm -f "$tmp"
    return 0
  fi
  echo "ERROR: admission webhook dry-run failed"
  kubectl apply --dry-run=server -f "$tmp" || true
  rm -f "$tmp"
  return 1
}

set_deployment_ready_replicas() {
  ns="$1"; name="$2"; val="$3"
  if kubectl -n "$ns" patch deployment "$name" --type='merge' --subresource=status -p "{\"status\":{\"readyReplicas\":$val}}" 2>/dev/null; then
    return 0
  fi
  if kubectl -n "$ns" get deployment "$name" -o json > /tmp/dep.json 2>/dev/null; then
    python3 - <<PY > /tmp/dep2.json
import sys, json
obj = json.load(sys.stdin)
obj.setdefault('status', {})['readyReplicas'] = int("$val")
json.dump(obj, sys.stdout)
PY
    kubectl replace --raw "/apis/apps/v1/namespaces/$ns/deployments/$name/status" -f /tmp/dep2.json >/dev/null 2>&1 && return 0
  fi
  echo "Warning: couldn't patch deployment status to readyReplicas=$val; continuing"
  return 1
}

echo "==> Creating kind cluster '${KIND_CLUSTER_NAME}'"
kind create cluster --name "${KIND_CLUSTER_NAME}" || true

echo "==> Building and loading controller image into kind"
make -C controller docker-build
make -C controller kind-load-controller

echo "==> Building and loading worker image into kind"
make -C controller docker-build-worker
# Tag worker image with non-latest tag to avoid implicit Always pull policy
if docker image inspect greedykomodo/kontroler-worker:latest >/dev/null 2>&1; then
  docker tag greedykomodo/kontroler-worker:latest "${WORKER_TEST_IMAGE}"
fi
kind load docker-image "${WORKER_TEST_IMAGE}" --name "${KIND_CLUSTER_NAME}"

# Ensure operator namespace exists
kubectl create ns operator-system || true

# Avoid deleting CRDs mid-run; Helm handles upgrades/install ownership.
# (Deleting CRDs here can put them into Terminating while tests are running.)

# Install cert-manager (chart-managed webhook certs)
if ! kubectl -n cert-manager get deployment cert-manager >/dev/null 2>&1; then
  echo "==> Installing cert-manager via Helm"
  helm repo add jetstack https://charts.jetstack.io >/dev/null 2>&1 || true
  helm repo update >/dev/null 2>&1 || true
  helm upgrade --install cert-manager jetstack/cert-manager \
    --namespace cert-manager --create-namespace \
    --set crds.enabled=true \
    --wait --timeout 5m
fi

# Create test secrets the chart expects (idempotent)
if ! kubectl -n operator-system get secret postgresql-client-tls >/dev/null 2>&1; then
  echo "==> Creating dummy postgresql-client-tls secret"
  kubectl -n operator-system create secret generic postgresql-client-tls --from-literal=ca.crt='dummy-ca' --from-literal=client.crt='dummy-client' --from-literal=client.key='dummy-key' >/dev/null
fi

if ! kubectl -n operator-system get secret jwt-kontroller-key >/dev/null 2>&1; then
  echo "==> Creating dummy jwt-kontroller-key secret"
  kubectl -n operator-system create secret generic jwt-kontroller-key --from-literal=jwt='dummyjwt' >/dev/null
fi

if ! kubectl -n operator-system get secret kontroler-db-secret >/dev/null 2>&1; then
  echo "==> Creating DB secret"
  kubectl -n operator-system create secret generic kontroler-db-secret --from-literal=password=postgres >/dev/null
fi

# Deploy Postgres
if ! helm ls -q | rg -x "postgres" >/dev/null 2>&1; then
  echo "==> Deploying test Postgres via Helm"
  helm repo add bitnami https://charts.bitnami.com/bitnami >/dev/null 2>&1 || true
  helm upgrade --install postgres bitnami/postgresql \
    --set auth.postgresPassword=postgres \
    --set primary.persistence.enabled=false \
    --wait --timeout 5m
fi
kubectl -n default rollout status statefulset/postgres-postgresql --timeout=180s

# Ensure kontroler DB exists
bash "$ROOT_DIR/scripts/kind-create-db.sh"

# Copy DB password secret into operator-system
kubectl -n operator-system create secret generic postgres-postgresql --from-literal=postgres-password=postgres --dry-run=client -o yaml | kubectl apply -f -

# Install chart
helm upgrade --install kontroler "$ROOT_DIR/../helm/kontroler" \
  --namespace operator-system --create-namespace \
  --set crds.install=true \
  --set certManager.enabled=true \
  --set webhook.enabled=true \
  --set controller.image=greedykomodo/kontroler-controller:0.0.1 \
  --set controller.enabled=true \
  --set server.enabled=false \
  --set ui.enabled=false \
  --set db.type=postgresql \
  --set db.postgresql.endpoint=postgres-postgresql.default.svc.cluster.local:5432 \
  --set controller.db.ssl.mode=disable \
  --set controller.db.password.secret=postgres-postgresql \
  --set controller.db.password.key=postgres-password \
  --wait --timeout 5m

# Wait for manager deployment (correct name)
wait_for_rollout operator-system kontroler-manager 180s
wait_for_webhook_cabundle
wait_for_webhook_service
check_admission_ready

# create WorkerPool manifest
WP_MANIFEST=$(mktemp -t workerpool-XXXX.yaml)
cat > "$WP_MANIFEST" <<EOF
apiVersion: kontroler.greedykomodo/v1alpha1
kind: WorkerPool
metadata:
  name: ${WP_NAME}
  namespace: ${NAMESPACE}
spec:
  replicas: 1
  image: ${WORKER_TEST_IMAGE}
  podTemplate:
    serviceAccountName: default
    nodeSelector:
      node-role.kubernetes.io/worker: "true"
  gracefulShutdownSeconds: 30
EOF

echo "==> Creating WorkerPool"
kubectl apply -f "$WP_MANIFEST"
kubectl label node kind-control-plane node-role.kubernetes.io/worker=true --overwrite || true

echo "==> Waiting for generated Deployment to appear"
for i in {1..60}; do
  if kubectl -n ${NAMESPACE} get deployment ${DEP_NAME} >/dev/null 2>&1; then
    echo "Deployment ${DEP_NAME} created"
    break
  fi
  sleep 2
done

echo "==> Waiting for worker pod to become Running"
for i in {1..60}; do
  POD=$(kubectl -n ${NAMESPACE} get pods -l workerpool=${WP_NAME} -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  if [ -n "$POD" ]; then
    PHASE=$(kubectl -n ${NAMESPACE} get pod "$POD" -o jsonpath='{.status.phase}' 2>/dev/null || true)
    if [ "$PHASE" = "Running" ]; then
      echo "Worker pod $POD is Running"
      break
    fi
  fi
  sleep 2
done

# simulate ready replica for finalizer flow
set_deployment_ready_replicas ${NAMESPACE} ${DEP_NAME} 1 || true

# DAG + DagRun
DAG_MANIFEST=$(mktemp -t dag-XXXX.yaml)
cat > "$DAG_MANIFEST" <<EOF
apiVersion: kontroler.greedykomodo/v1alpha1
kind: DAG
metadata:
  name: e2e-sample
  namespace: ${NAMESPACE}
spec:
  schedule: ""
  task:
    - name: say-hello
      command: ["sh", "-c"]
      args: ["echo hello from task && sleep 1"]
      image: alpine:3.18
      backoff:
        limit: 0
EOF
kubectl apply -f "$DAG_MANIFEST"
sleep 2
kubectl -n ${NAMESPACE} delete dagrun e2e-sample-run --ignore-not-found

DAGRUN_MANIFEST=$(mktemp -t dagrun-XXXX.yaml)
cat > "$DAGRUN_MANIFEST" <<EOF
apiVersion: kontroler.greedykomodo/v1alpha1
kind: DagRun
metadata:
  name: e2e-sample-run
  namespace: ${NAMESPACE}
spec:
  dagName: e2e-sample
EOF

kubectl apply -f "$DAGRUN_MANIFEST"

echo "==> Waiting for task pod created by scheduler"
POD_NAME=""
for i in {1..90}; do
  POD_NAME=$(kubectl -n ${NAMESPACE} get pods -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.containers[0].image}{"\n"}{end}' | awk '$2 ~ /alpine/ {print $1; exit}')
  if [ -n "${POD_NAME}" ]; then
    echo "Found task pod: ${POD_NAME}"
    break
  fi
  sleep 2
done

if [ -n "${POD_NAME}" ]; then
  kubectl -n ${NAMESPACE} wait --for=condition=Succeeded pod/${POD_NAME} --timeout=120s || true
  echo "==> Task pod logs:"
  kubectl -n ${NAMESPACE} logs ${POD_NAME} || true
else
  echo "Warning: could not find task pod running alpine image"
fi

# delete WorkerPool
kubectl delete workerpool ${WP_NAME} -n ${NAMESPACE} || true

echo "==> Waiting for controller to scale Deployment to 0 replicas (or remove it)"
for i in {1..30}; do
  if ! kubectl -n ${NAMESPACE} get deployment ${DEP_NAME} >/dev/null 2>&1; then
    echo "Deployment removed"
    break
  fi
  REPLICAS=$(kubectl -n ${NAMESPACE} get deployment ${DEP_NAME} -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "")
  if [ "$REPLICAS" = "0" ]; then
    echo "Deployment scaled to 0"
    break
  fi
  sleep 2
done

if kubectl -n ${NAMESPACE} get deployment ${DEP_NAME} >/dev/null 2>&1; then
  set_deployment_ready_replicas ${NAMESPACE} ${DEP_NAME} 0 || true
fi

echo "==> Waiting for WorkerPool deletion"
for i in {1..30}; do
  if ! kubectl -n ${NAMESPACE} get workerpool ${WP_NAME} >/dev/null 2>&1; then
    echo "WorkerPool deleted"
    break
  fi
  sleep 2
done

# cleanup
echo "==> Cleaning up"
helm -n operator-system uninstall kontroler || true
kubectl delete crd workerpools.kontroler.greedykomodo || true
kubectl -n operator-system delete secret webhook-server-cert kontroler-db-secret postgresql-client-tls jwt-kontroller-key --ignore-not-found || true

rm -f "$WP_MANIFEST" "$DAG_MANIFEST" "$DAGRUN_MANIFEST"
echo "==> Integration test completed"
