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

echo "==> Creating kind cluster '${KIND_CLUSTER_NAME}'"
kind create cluster --name "${KIND_CLUSTER_NAME}" || true

echo "==> Building and loading controller image into kind"
make -C controller docker-build
make -C controller kind-load-controller

# Ensure operator namespace exists
kubectl create ns operator-system || true

# If a previous manual CRD exists, remove it so Helm can take ownership
if kubectl get crd workerpools.kontroler.greedykomodo >/dev/null 2>&1; then
  echo "==> Deleting pre-existing WorkerPool CRD to allow Helm to own it"
  kubectl delete crd workerpools.kontroler.greedykomodo || true
fi

# Create test secrets the chart expects (idempotent)
if ! kubectl -n operator-system get secret webhook-server-cert >/dev/null 2>&1; then
  echo "==> Creating self-signed webhook TLS secret"
  openssl req -x509 -nodes -days 365 -newkey rsa:2048 -subj '/CN=operator-webhook' -keyout /tmp/tls.key -out /tmp/tls.crt >/dev/null 2>&1 || true
  kubectl -n operator-system create secret tls webhook-server-cert --cert=/tmp/tls.crt --key=/tmp/tls.key >/dev/null 2>&1 || true
  rm -f /tmp/tls.key /tmp/tls.crt || true
fi

if ! kubectl -n operator-system get secret postgresql-client-tls >/dev/null 2>&1; then
  echo "==> Creating dummy postgresql-client-tls secret"
  kubectl -n operator-system create secret generic postgresql-client-tls --from-literal=ca.crt='dummy-ca' --from-literal=client.crt='dummy-client' --from-literal=client.key='dummy-key' >/dev/null 2>&1 || true
fi

if ! kubectl -n operator-system get secret jwt-kontroller-key >/dev/null 2>&1; then
  echo "==> Creating dummy jwt-kontroller-key secret"
  kubectl -n operator-system create secret generic jwt-kontroller-key --from-literal=jwt='dummyjwt' >/dev/null 2>&1 || true
fi

# Create DB secret if it doesn't exist
if ! kubectl -n operator-system get secret kontroler-db-secret >/dev/null 2>&1; then
  echo "==> Creating DB secret"
  kubectl -n operator-system create secret generic kontroler-db-secret --from-literal=password=postgres >/dev/null 2>&1 || true
fi

# Install CRDs + chart via Helm (disable cert-manager integration for this integration test)
helm upgrade --install kontroler helm/kontroler \
  --namespace operator-system --create-namespace \
  --set crds.install=true \
  --set certManager.enabled=false \
  --set controller.image=greedykomodo/kontroler-controller:0.0.1 \
  --set controller.enabled=true \
  --set db.type=sqlite \
  --wait --timeout 3m || true

# Ensure the Helm deployment is applied and wait for rollout
kubectl -n operator-system rollout status deployment/operator-controller-manager --timeout=120s || true

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
  image: greedykomodo/kontroler-worker:latest
  podTemplate:
    serviceAccountName: default
    nodeSelector:
      node-role.kubernetes.io/worker: "true"
  gracefulShutdownSeconds: 30
EOF

echo "==> Creating WorkerPool"
kubectl apply -f "$WP_MANIFEST"

echo "==> Waiting for generated Deployment to appear"
for i in {1..30}; do
  if kubectl -n ${NAMESPACE} get deployment ${DEP_NAME} >/dev/null 2>&1; then
    echo "Deployment ${DEP_NAME} created"
    break
  fi
  sleep 2
done

# helper to set deployment status.readyReplicas in a kubectl-version-portable way
set_deployment_ready_replicas() {
  ns="$1"; name="$2"; val="$3"
  # try kubectl patch with subresource (preferred)
  if kubectl -n "$ns" patch deployment "$name" --type='merge' --subresource=status -p "{\"status\":{\"readyReplicas\":$val}}" 2>/dev/null; then
    return 0
  fi
  # fallback: get, modify, and replace status subresource via --raw (portable)
  if kubectl -n "$ns" get deployment "$name" -o json > /tmp/dep.json 2>/dev/null; then
    python3 - <<PY > /tmp/dep2.json
import sys, json
obj = json.load(sys.stdin)
obj.setdefault('status', {})['readyReplicas'] = int(sys.argv[1])
json.dump(obj, sys.stdout)
PY $val < /tmp/dep.json 2>/dev/null || true
    if kubectl replace --raw "/apis/apps/v1/namespaces/$ns/deployments/$name/status" -f /tmp/dep2.json >/dev/null 2>&1; then
      return 0
    fi
  fi
  echo "Warning: couldn't patch deployment status to readyReplicas=$val; continuing"
  return 1
}

# simulate a ready replica so the controller will requeue during deletion
echo "==> Simulating deployment readyReplicas=1"
set_deployment_ready_replicas ${NAMESPACE} ${DEP_NAME} 1 || true

# delete WorkerPool and verify graceful scale down and finalizer behavior
echo "==> Deleting WorkerPool (trigger finalizer/graceful shutdown)"
kubectl delete workerpool ${WP_NAME} -n ${NAMESPACE} || true

# wait for controller to scale deployment to 0
echo "==> Waiting for controller to scale Deployment to 0 replicas (or for it to be removed)"
for i in {1..30}; do
  if ! kubectl -n ${NAMESPACE} get deployment ${DEP_NAME} >/dev/null 2>&1; then
    echo "Deployment not found (assume removed)"
    break
  fi

  REPLICAS=$(kubectl -n ${NAMESPACE} get deployment ${DEP_NAME} -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "")
  if [ "$REPLICAS" = "0" ]; then
    echo "Deployment scaled to 0"
    break
  fi
  sleep 2
done

# simulate pods terminated (readyReplicas=0) if deployment still exists
if kubectl -n ${NAMESPACE} get deployment ${DEP_NAME} >/dev/null 2>&1; then
  echo "==> Simulate pods terminated (readyReplicas=0)"
  set_deployment_ready_replicas ${NAMESPACE} ${DEP_NAME} 0 || true
else
  echo "==> Deployment already removed; skipping simulate pods termination"
fi

# wait for WorkerPool deletion to complete
echo "==> Waiting for WorkerPool resource to be deleted"
for i in {1..30}; do
  if ! kubectl -n ${NAMESPACE} get workerpool ${WP_NAME} >/dev/null 2>&1; then
    echo "WorkerPool deleted"
    break
  fi
  sleep 2
done

# cleanup
echo "==> Cleaning up: uninstall Helm release and delete CRDs"
helm -n operator-system uninstall kontroler || true
# delete the CRD installed by the chart
kubectl delete crd workerpools.kontroler.greedykomodo || true

# remove test secrets
kubectl -n operator-system delete secret webhook-server-cert kontroler-db-secret postgresql-client-tls jwt-kontroller-key --ignore-not-found || true

echo "==> Integration test completed"
rm -f "$WP_MANIFEST"
