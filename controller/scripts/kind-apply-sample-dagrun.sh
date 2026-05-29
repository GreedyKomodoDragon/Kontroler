#!/usr/bin/env bash
set -euo pipefail

NAMESPACE=default
DAGRUN_FILE=/tmp/kontroler-sample-dagrun.yaml

cat > ${DAGRUN_FILE} <<'EOF'
apiVersion: kontroler.greedykomodo/v1alpha1
kind: DagRun
metadata:
  name: sample-dagrun-1
spec:
  dagName: sample-dag
EOF

kubectl apply -f ${DAGRUN_FILE} -n ${NAMESPACE}

kubectl get dagruns.kontroler.greedykomodo -n ${NAMESPACE}
kubectl describe dagrun sample-dagrun-1 -n ${NAMESPACE}

echo "Sample DagRun applied: ${DAGRUN_FILE}"
