#!/bin/bash

for i in {1..10}
do
  cat <<EOF | kubectl apply -f -
apiVersion: kontroler.greedykomodo/v1alpha1
kind: DagRun
metadata:
  labels:
    app.kubernetes.io/name: dagrun
    app.kubernetes.io/instance: dagrun-sample
    app.kubernetes.io/part-of: operator
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/created-by: operator
  name: dagrun-sample-${i}
spec:
  dagName: dag-sample-long
  parameters: []
EOF
  echo "Created DAGRun ${i} of 10"
done
