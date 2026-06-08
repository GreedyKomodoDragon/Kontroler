# Kontroler examples

This directory contains copy/pasteable manifests that mirror the README DAG
examples.

## Prerequisites

- A Kubernetes cluster with Kontroler installed
- `kubectl` configured for that cluster

If you are running Kontroler from source, install the CRDs before applying these
examples:

```sh
cd controller
make install
```

## Apply an example DAG

From the repository root:

```sh
kubectl apply -f examples/dag-event-driven.yaml
kubectl apply -f examples/dag-schedule.yaml
kubectl apply -f examples/dag-dsl-example.yaml
```

## Start a DAG run manually

The sample `DagRun` references `dag-schedule`, which is defined in
`dag-schedule.yaml`.

```sh
kubectl apply -f examples/dagrun-sample.yaml
kubectl get dagruns
```

## Clean up

```sh
kubectl delete -f examples/dagrun-sample.yaml --ignore-not-found
kubectl delete -f examples/dag-dsl-example.yaml --ignore-not-found
kubectl delete -f examples/dag-schedule.yaml --ignore-not-found
kubectl delete -f examples/dag-event-driven.yaml --ignore-not-found
```
