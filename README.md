<p align="center">
<img src="./ui/public/logo.svg" width="150" />
</p>
<h1 align="center">
    Always in control - Always on time
    <p align="center">
        <img src="https://img.shields.io/badge/Go-00ADD8?style=&logo=go&logoColor=white" alt="Golang">
    </p>
</h1>

# Kontroler

Kontroler is a Kubernetes scheduling engine for managing Directed Acyclic Graphs (DAGs) through cron-based jobs or event-driven execution. 

This system allows for running containers as tasks within a DAG, while providing an optional web-based UI for creating, managing, and visualizing DAG runs.

## Getting Started

Helm Charts coming soon! See the Building/Running from Source section further down for now

## State

Kontroler is in a very early alpha state and if it is used in production expect bugs along with breaking changes coming in future releases.

## Aims of Kontroler

Kontroler aims to provide a way to manage scheduling containers in a simple manner via YAML files.

## Features

Features we are aiming to cover are:

* DAG Execution via CronJobs: Define and schedule your DAGs to run at specified intervals using cron.
* Event-Driven DAGs: Execute DAGs based on external events such as a message from a queue or a webhook trigger.
* Container Support: Easily run any containerized task within your DAG.
* Optional UI: A web-based interface is available for creating and viewing DAG runs, simplifying DAG management.
* Optional Server: An included server can be deployed to power the UI, providing a full-featured platform for scheduling tasks.
* Pod Templates: Allows pods to use secrets, pvcs, serviceAccounts, setting affinity along with much more (see the example below for all the options!)


## Server + UI Overview

The Server+UI allows you to:

* Create DAGs: Use the interface to visually create and configure DAGs
* View DAG Runs: Track the status, success, or failure of DAG runs

Planned Features:

* Trigger DAGs Manually: Execute DAGs directly from the UI
* User Roles: Restrict what a user can do
* Better Password Management: Improve Security/Creation workflow
* Adding mTLS: Server + UI can communicate over mTLS

## Example of DAG

Here are two examples, one event-driven & one that runs on a schedule:

Event Driven
```yaml
apiVersion: kontroler.greedykomodo/v1alpha1
kind: DAG
metadata:
  name: event-driven
spec:
  schedule: ""
  parameters:
    - name: first
      defaultFromSecret: secret-name
    - name: second
      defaultValue: value
  task:
    - name: "random"
      command: ["sh", "-c"]
      args:
        [
          "if [ $((RANDOM%2)) -eq 0 ]; then echo $second; else exit 1; fi",
        ]
      image: "alpine:latest"
      backoff:
        limit: 3
      parameters:
        - second
      conditional:
        enabled: true
        retryCodes: [1]
      podTemplate:
        volumes:
          - name: example-pvc
            persistentVolumeClaim:
              claimName: example-claim  # The name of the PVC
        volumeMounts:
          - name: example-pvc
            mountPath: /data  # Path inside the container where the PVC is mounted
        imagePullSecrets:
          - name: my-registry-secret
        securityContext:
          runAsUser: 1000
          runAsGroup: 3000
          fsGroup: 2000
        nodeSelector:
          disktype: ssd
        tolerations:
          - key: "key1"
            operator: "Equal"
            value: "value1"
            effect: "NoSchedule"
        affinity:
          nodeAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
              nodeSelectorTerms:
                - matchExpressions:
                    - key: "kubernetes.io/e2e-az-name"
                      operator: In
                      values:
                        - e2e-az1
                        - e2e-az2
        serviceAccountName: "custom-service-account"
        automountServiceAccountToken: false
    - name: "random-b"
      command: ["sh", "-c"]
      args:
        [
          "if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi",
        ]
      image: "alpine:latest"
      runAfter: ["random"]
      backoff:
        limit: 3
      conditional:
        enabled: true
        retryCodes: [1]
      parameters:
        - first
        - second
    - name: "random-c"
      command: ["sh", "-c"]
      args:
        [
          "if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi",
        ]
      image: "alpine:latest"
      runAfter: ["random"]
      backoff:
        limit: 3
      conditional:
        enabled: true
        retryCodes: [1]
```

Schedule:
```yaml
apiVersion: kontroler.greedykomodo/v1alpha1
kind: DAG
metadata:
  name: dag-schedule
spec:
  schedule: "*/1 * * * *"
  task:
    - name: "random"
      command: ["sh", "-c"]
      args:
        [
          "echo 'Hello, World!'",
        ]
      image: "alpine:latest"
      backoff:
        limit: 3
      conditional:
        enabled: true
        retryCodes: [8]
    - name: "random-b"
      command: ["sh", "-c"]
      args:
        [
          "echo 'Hello, World!'",
        ]
      image: "alpine:latest"
      runAfter: ["random"]
      backoff:
        limit: 3
      conditional:
        enabled: true
        retryCodes: [8]
    - name: "random-c"
      command: ["sh", "-c"]
      args:
        [
          "echo 'Hello, World!'",
        ]
      image: "alpine:latest"
      runAfter: ["random"]
      backoff:
        limit: 3
      conditional:
        enabled: true
        retryCodes: [8]
```

## Creating a DagRun via YAML

Regardless of if a Dag is scheduled or event driven you can execute a run of the dag. You can do this by creating a DagRun object. 

Here is an example:

```yaml
apiVersion: kontroler.greedykomodo/v1alpha1
kind: DagRun
metadata:
  labels:
    app.kubernetes.io/name: dagrun
    app.kubernetes.io/instance: kontroler-DagRun
  name: dagrun-sample3
spec:
  dagName: dag-schedule
  parameters:
    - name: first
      fromSecret: secret-name-new
    - name: second
      value: value_new
```

## Building/Running from Source

Currently there are no official artefacts within Kontroler project (we plan to fix this soon!), for now we recommend building from source and using our makefile to deploy the controller directly into your cluster.

It is worth noting that the install will not be production ready so use at your own risk!

### Prerequisites Required

#### Tools

To start building and deploying you will need:

* [kustomize >=5.4.3](https://kubectl.docs.kubernetes.io/installation/kustomize/) - deploy scripts relay on kustomize to work
* [cert-manager >=1.15.1](https://cert-manager.io/) - Without editting the kustomize files it will use cert-manager to handle thw webhook certs
* [docker](https://www.docker.com/) - used to create the docker images for each service (most modern version of docker should work)
* [PostgreSQL >=16 Installed](https://github.com/bitnami/charts/tree/main/bitnami/postgresql) - We use the bitanmi chart for providing a PostgreSQL instance, postgresql is used as backend store for kontroler

#### Database

Kontroler requires a database to already exist to insert the required tables to manage DAGs and DagRun results.

Once you have created a the database must then update the envs in `controller/config/manager/manager.yaml` to allow kontroler to connect. For testing we use the postgres user but in production you should use have two users that can:

Controller user:
* Create Tables
* Insert into Tables
* Select records from Tables
* Delete records in tables

Server user:
* Create Tables
* Insert into Tables
* Select records from Tables
* Delete records in tables

### Building/Running the Controller

You will need to perform the following to build the docker & publish it to your registry of choice:

```sh
cd controller

export VERSION=YOUR_TAG
export IMAGE_TAG_BASE=YOUR_NAMESPACE/kontroler-controller

make docker-build docker-push
```

After a successful build and push get the full docker image URI and place it into `controller/config/manager/manager.yaml` for the deployment to use.

If you want to change the default namespace from `operator-system` you will need to update the file `controller/config/default/kustomization.yaml`. The field `namespace` in `controller/config/default/kustomization.yaml` controls which namespace it installs into.

Upon these changes you can then run which will use your default kubectl's cluster destination:

```sh
make deploy
```

After the script finishes you can use the following script to follow the controler logs:

```sh
kubectl logs $(kubectl get pods --all-namespaces | grep operator-controller | awk '{print $2}') -f -n operator-system
```

### Building/Running the Server

You will need to perform the following to build the docker & publish it to your registry of choice:

```sh
cd server

export VERSION=YOUR_TAG
export IMAGE_TAG_BASE=YOUR_NAMESPACE/kontroler-server

make docker-build docker-push
```

Currently there is no Helm chart to handle this so for now you will need to create your own, this is coming soon!

To help we can provide the role need to run the server at the cluster level:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kontroler-schedule-reader
rules:
- apiGroups: ["kontroler.greedykomodo"]
  resources: ["dags"]
  verbs: ["create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kontroler-schedule-reader-binding
subjects:
- kind: ServiceAccount
  name: serverAccountName
  namespace: NamespaceServerIsIn
roleRef:
  kind: ClusterRole
  name: kontroler-schedule-reader
  apiGroup: rbac.authorization.k8s.io
```

You will so need to include the following envs in your deployment object:

```yaml
env:
  - name: DB_NAME
    value: kontroler
  - name: DB_USER
    value: postgres
  - name: DB_PASSWORD
    value: YOUR_DB_PASSWORD
  - name: JWT_KEY
    value: RANDOM_SECRET_KEY
  - name: DB_ENDPOINT
    value: my-release-postgresql.default.svc.cluster.local:5432
```

### Building/Running the UI

You will need to perform the following to build the docker & publish it to your registry of choice:

```sh
cd ui

export VERSION=YOUR_TAG
export IMAGE_TAG_BASE=YOUR_NAMESPACE/kontroler-ui

make docker-build docker-push
```

Currently there is no Helm chart to handle this so for now you will need to create your own, this is coming soon!

# Contributing

There are many ways in which you can contribute to Kontroler, it doesn't have to be providing contributes to the codebase. Some examples are:

* Reporting bugs
  * Raise as issue as detailed as possible so we can get onto fixing it!
* Add to technical documentation (Docs coming soon!)
* Just talk and share Kontroler
  * Awareness for the project is one of the best things you can do for small projects such as Kontroler

As with most open source projects, we cannot accept all changes due to a range of factors such as not aligning with the goals of the project, or the changes are not quite ready to be merged in.