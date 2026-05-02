PODS=$(kubectl get pods -o jsonpath='{.items[*].metadata.name}')

for POD_NAME in $PODS; do
    echo "Processing pod: $POD_NAME"

    # Get the current pod details in JSON format
    POD_DETAILS=$(kubectl get pod "$POD_NAME" -o json)

    # Check if the pod has finalizers
    FINALIZERS=$(echo "$POD_DETAILS" | jq -r '.metadata.finalizers')

    if [ "$FINALIZERS" != "null" ] && [ -n "$FINALIZERS" ]; then
        echo "Finalizers found: $FINALIZERS"

        # Remove finalizers by patching the pod
        kubectl patch pod "$POD_NAME" -p '{"metadata":{"finalizers":null}}'

        echo "Finalizers removed from pod $POD_NAME in namespace $NAMESPACE."
    else
        echo "No finalizers found for pod $POD_NAME in namespace $NAMESPACE."
    fi

    kubectl delete pod $POD_NAME
done
