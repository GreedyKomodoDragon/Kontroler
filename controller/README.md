# operator
// TODO(user): Add simple overview of use/purpose

## Description
// TODO(user): An in-depth paragraph about your project and overview of use

## Getting Started

### Prerequisites
- go version v1.20.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified. 
And it is required to have access to pull the image from the working environment. 
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin 
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

### Running the Tests

```sh
make test
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## Defaults and local testing notes

This project ships a small set of defaults and conveniences to make local
and CI testing straightforward. Below are the most important defaults and
how to override them for production.

- Configuration file (configpath):
  - If the manager is started without the `--configpath` flag or an
    external config file, the binary will use a sensible built-in default
    configuration (memory-based workers, a single default namespace watcher,
    and a filesystem log store at `/tmp/kontroler-logs`). This is intended
    to make local e2e and dev runs easier. For production, supply a full
    YAML config and pass `--configpath` to the manager.

- Database selection and defaults:
  - DB_TYPE: must be set to either `postgresql` or `sqlite`.
  - For local tests and single-node deployments use `sqlite`.
  - When using `sqlite` set `SQLITE_PATH` (for tests you can use `:memory:`).
  - When using `postgresql` set `DB_NAME`, `DB_USER`, `DB_ENDPOINT`, and
    `DB_PASSWORD` and optionally `DB_SSL_MODE`.

- Concurrency tuning:
  - Bounded concurrency for background batch operations is controlled by a
    package-level constant `defaultConcurrency` (set to 8). Change this
    value in `internal/controller/dag_controller.go` if you need a higher
    or lower bound when adding/removing finalizers or deleting pods.

- Webhooks and cert-manager:
  - The operator's webhook requires a TLS certificate. The repository's
    default manifests include integration points for cert-manager to issue
    the webhook certs via an Issuer/Certificate resource.
  - In local e2e runs ensure cert-manager is installed and its webhook
    deployment is Healthy before applying the operator manifests. The
    e2e scripts install cert-manager (see `test/e2e`) but in CI you should
    also ensure it is available.

- Image handling for Kind/local testing:
  - To run the e2e test locally you typically build the image and load it
    into the Kind cluster:
    - make docker-build IMG=example.com/operator:vX.Y.Z
    - kind load docker-image example.com/operator:vX.Y.Z --name kind
  - The deployment imagePullPolicy in the config is set to `IfNotPresent`
    to favour local `kind load` usage. For production set an appropriate
    policy (for immutable tags `IfNotPresent` is fine; for moving tags use
    `Always`).

- Common environment variables used by the deployment (config/manager/manager.yaml):
  - DB_TYPE (sqlite|postgresql)
  - SQLITE_PATH (e.g. `:memory:` for tests)
  - LEADER_ELECTION_ID (defaults to `kontroler` when not provided and no config)
  - LOG_DIR (when using filesystem log store — defaults to `/tmp/kontroler-logs` in the built-in config)

Running local e2e (quick checklist)

1. Build the manager image and load it into Kind:
   - make docker-build IMG=example.com/operator:v0.0.1
   - kind load docker-image example.com/operator:v0.0.1 --name kind

2. Install CRDs and (optionally) cert-manager:
   - make install
   - For cert-manager installation follow the official docs: https://cert-manager.io/docs/installation/
    (or use the latest release manifest: https://github.com/jetstack/cert-manager/releases/latest/download/cert-manager.yaml)
   - kubectl -n cert-manager wait deployment.apps/cert-manager-webhook --for condition=Available --timeout=5m

3. Deploy the manager with the loaded image:
   - make deploy IMG=example.com/operator:v0.0.1

4. Verify operator pods are Running:
   - kubectl -n operator-system get pods
   - kubectl -n operator-system describe pod <pod> (if Pending / CrashLoopBackOff)

> **NOTE**: To change the built-in defaults you can mount a ConfigMap with a
> full config file and pass `--configpath` to the manager, or open a PR/issue
> to update the repository defaults. This keeps runtime behaviour explicit for
> production deployments.

## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

