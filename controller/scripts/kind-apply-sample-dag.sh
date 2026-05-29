#!/usr/bin/env bash
set -euo pipefail

NAMESPACE=default
DAG_FILE=/tmp/kontroler-sample-dag.yaml

cat > ${DAG_FILE} <<'EOF'
apiVersion: kontroler.greedykomodo/v1alpha1
kind: DAG
metadata:
  name: sample-dag
spec:
  schedule: ""
  task:
    - name: hello
      image: busybox
      command: ["/bin/sh", "-c"]
      args: ["echo hello && sleep 1"]
EOF

kubectl apply -f ${DAG_FILE} -n ${NAMESPACE}

kubectl get dags.kontroler.greedykomodo -n ${NAMESPACE}
kubectl describe dag sample-dag -n ${NAMESPACE}

echo "Sample DAG applied: ${DAG_FILE}"
