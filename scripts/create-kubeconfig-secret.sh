#!/bin/bash

# Set variables
KUBECONFIG_PATH="$HOME/.kube/config"
SECRET_NAME="kubeconfig"
NAMESPACE="default"

# Check if kubeconfig file exists
if [ ! -f "$KUBECONFIG_PATH" ]; then
    echo "Kubeconfig file not found at $KUBECONFIG_PATH"
    exit 1
fi

# Encode kubeconfig in base64
ENCODED_KUBECONFIG=$(base64 < "$KUBECONFIG_PATH" | tr -d '\n')

# Create a secret YAML file
cat <<EOF > kubeconfig-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: $SECRET_NAME
  namespace: $NAMESPACE
type: Opaque
data:
  kubeconfig: $ENCODED_KUBECONFIG
EOF

# Apply the secret to Kubernetes
kubectl apply -f kubeconfig-secret.yaml

# Output the created secret
kubectl get secret $SECRET_NAME -n $NAMESPACE

echo "Kubeconfig secret '$SECRET_NAME' created successfully in namespace '$NAMESPACE'."
