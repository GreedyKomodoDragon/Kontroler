for image in $(minikube cache list); do
    minikube cache delete $image
done
