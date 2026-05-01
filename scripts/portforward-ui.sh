export POD_NAME=$(kubectl get pods --namespace default -l "app.kubernetes.io/name=kontroler-ui,app.kubernetes.io/instance=kontroler" -o jsonpath="{.items[0].metadata.name}")
export CONTAINER_PORT=$(kubectl get pod --namespace default $POD_NAME -o jsonpath="{.spec.containers[0].ports[0].containerPort}")
echo "Visit http://127.0.0.1:3000 to use your application"
kubectl --namespace default port-forward $POD_NAME 3000:$CONTAINER_PORT