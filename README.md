<p align="center">
<img src="./ui/src/assets/logo.svg" width="350" />
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

Helm Charts coming soon!

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