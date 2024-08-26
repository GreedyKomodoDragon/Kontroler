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

Kontroler is a kubernetes native scheduler where it aims to provide a way to run containers based on CRDs.

## State

Kontroler is is not ready for any kind of production workload and has not go any of core functionalities implemented yet.

### Operator Progress

The operator is very much in its early days and a lot of learning has to be done before the operator can be in a state to be shared

## Aims of Kontroler

Kontroler aims to provide a way to manage scheduling containers in a simple manner via YAML files.

### Operator's Core Aims

Features we are aiming to cover are:

- Single Container Scheduling - Similar to the Native Kubernetes
- DAGs - Allow stages to be linked together
- BlackBox Containers - It does not matter what the image is, Kontroler will run it!
- Conditional Retries - Based on the exit code of the container Kontroler will return the image a set amount of times

### Server + UI

We are aiming to create a UI and a server that will allow for you to interact with the operator to perform actions such as:

- Creating Schedules via a web UI
- View results of schedules
- See the list of schedules you have

It is the aim that you can optionally deploy the UI, we will provide a REST API that you can connect to create whatever you'd like!

## Roadmap

There is no roadmap for Kontroler, and most features have not been outlined or thought about.
