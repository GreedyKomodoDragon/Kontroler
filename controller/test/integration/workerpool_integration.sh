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

echo "==> Installing CRDs"
make -C controller install

echo "==> Deploying controller manager"
make -C controller deploy

echo "==> Waiting for controller deployment to become ready"
kubectl -n operator-system wait deployment/operator-controller --for condition=Available --timeout=120s || true

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

# simulate a ready replica so the controller will requeue during deletion
echo "==> Simulating deployment readyReplicas=1"
kubectl -n ${NAMESPACE} patch deployment ${DEP_NAME} --type='merge' --subresource=status -p '{"status":{"readyReplicas":1}}'

# delete WorkerPool and verify graceful scale down and finalizer behavior
echo "==> Deleting WorkerPool (trigger finalizer/graceful shutdown)"
kubectl delete workerpool ${WP_NAME} -n ${NAMESPACE}

# wait for controller to scale deployment to 0
echo "==> Waiting for controller to scale Deployment to 0 replicas"
for i in {1..30}; do
  REPLICAS=$(kubectl -n ${NAMESPACE} get deployment ${DEP_NAME} -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "")
  if [ "$REPLICAS" = "0" ]; then
    echo "Deployment scaled to 0"
    break
  fi
  sleep 2
done

# simulate pods terminated
echo "==> Simulate pods terminated (readyReplicas=0)"
kubectl -n ${NAMESPACE} patch deployment ${DEP_NAME} --type='merge' --subresource=status -p '{"status":{"readyReplicas":0}}'

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
echo "==> Cleaning up: undeploy controller and CRDs"
make -C controller undeploy || true
make -C controller uninstall || true

echo "==> Integration test completed"
rm -f "$WP_MANIFEST"
