# Installation

[Русская версия](ru/installation.md)

The operator is installed with the Helm chart. Credential Secrets are read from the release namespace. A Kustomize install from this repository is also available and uses the namespace `selectel-infra`.

## Requirements

- Kubernetes cluster and `kubectl` aimed at it
- Docker, or another tool selected with `CONTAINER_TOOL`, to build the manager image
- A Selectel project and a user that can obtain a project-scoped IAM token
- Go 1.25.4 when building or testing from source

## Helm

`selectel-operator` includes the CRDs and the controller. Installing that chart is enough. `selectel-operator-crds` contains the same CRDs and nothing else; install it first only when CRDs are managed separately. A later upgrade of the CRD chart does not change CRDs that Helm already applied from its `crds/` directory.

A tag matching `v*` runs [`.github/workflows/release.yml`](../.github/workflows/release.yml). It publishes `ghcr.io/<owner>/<repository>:<tag>` and both charts to `oci://ghcr.io/<owner>/charts`.

```bash
helm install selectel-operator-crds \
  oci://ghcr.io/tapclap/charts/selectel-operator-crds \
  --version <version>

helm install selectel-operator \
  oci://ghcr.io/tapclap/charts/selectel-operator \
  --version <version> \
  --namespace selectel-infra \
  --create-namespace
```

`<version>` is the git tag without the leading `v`. The chart in this repository is `0.0.5`.

From a checkout, point the chart at an image you have built:

```bash
helm install selectel-operator helm/selectel-operator \
  --namespace selectel-infra \
  --create-namespace \
  --set image.repository=<registry>/selectel-operator \
  --set image.tag=latest
```

`Project.spec.credentialsSecretRef.namespace` must be `selectel-infra` for that install. The chart sets `--credentials-namespace` to the release namespace and grants the manager `get`, `list`, and `watch` on Secrets there.

| Value | Default | Meaning |
| --- | --- | --- |
| `image.repository` | `ghcr.io/tapclap/k8s-selectel-operator` | Manager image. The release workflow rewrites this to the repository that built the chart. |
| `image.tag` | chart `appVersion` | Image tag. |
| `leaderElection` | `true` | Passes `--leader-elect`. |
| `cdn.baseURL` | empty | Passes `--cdn-base-url` when set. Empty keeps `https://api.selectel.ru/cdn/v3`. |
| `cdn.processingInterval` | `3m` | Passes `--processing-interval`. The operator adds a random 0–30 seconds. |
| `metrics.enabled` | `true` | HTTPS metrics on `metrics.bindAddress` and a Service. |
| `metrics.bindAddress` | `:8443` | Metrics listen address. The Service port is the numeric part. |

## Build and deploy with Kustomize

```sh
export IMG=<registry>/selectel-operator:latest
make docker-build docker-push IMG=$IMG
make deploy IMG=$IMG
```

`make deploy` installs CRDs, RBAC, and the Deployment. The container args include `--leader-elect` and `--credentials-namespace=selectel-infra`.

Install CRDs without the manager:

```sh
make install
```

Remove the manager and keep CRDs:

```sh
make undeploy
```

Remove CRDs:

```sh
make uninstall
```

A single install manifest, for distribution without the repository Makefile:

```sh
make build-installer IMG=<registry>/selectel-operator:latest
kubectl apply -f dist/install.yaml
```

## Where it runs

A Kustomize install (`make deploy`) uses these names:

| Object | Name |
| --- | --- |
| Namespace | `selectel-infra` |
| Deployment | `selectel-operator-controller-manager` |
| ServiceAccount | `selectel-operator-controller-manager` |
| Credentials Role | `selectel-operator-credentials-reader` |

A Helm install named `selectel-operator` uses Deployment, ServiceAccount, and Roles named `selectel-operator` in the release namespace. The credentials Role is `selectel-operator-credentials`.

Logs for the Kustomize Deployment:

```sh
kubectl logs -n selectel-infra deployment/selectel-operator-controller-manager -c manager -f
```

## Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `--credentials-namespace` | `selectel-infra` | Only Secrets in this namespace are read. `Project.spec.credentialsSecretRef.namespace` must match it. |
| `--cdn-base-url` | `https://api.selectel.ru/cdn/v3` | CDN API root. |
| `--processing-interval` | `3m` | Base wait while Selectel applies configuration. A random 0–30 seconds is added. |
| `--leader-elect` | `false` in the binary, `true` in the deployed Deployment | One active manager. |
| `--health-probe-bind-address` | `:8081` | Liveness and readiness. |

The Keystone URL is per Project: `spec.authURL`, default `https://cloud.api.selcloud.ru/identity/v3`.

Changing `--credentials-namespace` requires the same change in `config/manager/manager.yaml` (or a kustomize patch) and a RoleBinding that grants secret read in that namespace. The bundled Role lives in the install namespace.

## RBAC

The manager ClusterRole can get, list, and watch `Project` and CDN `Resource` objects, update their status and finalizers, and write Events. It has no rule for Secrets.

Secret access is a namespaced Role, `credentials-reader`, with `get`, `list`, and `watch` on Secrets in `selectel-infra`. The manager lists and watches Secrets only in `--credentials-namespace`, so that Role is enough. A `Project` that points at a Secret in a tenant namespace gets `Ready=False`, reason `SecretNamespaceForbidden`, and the Secret is not read.

Tenant users who should create CDN resources need permission on `cdn.selectel.tapclap.com` `resources` in their namespace. Creating a `Project` is a cluster-admin action: it chooses the Secret and the namespace allowlist.

## Run from a checkout

Install CRDs, then run the manager against the current kubeconfig context:

```sh
make install
make run
```

`make run` starts `cmd/main.go`. The binary defaults `--credentials-namespace` to `selectel-infra`. The process still reads Secrets from the cluster, so that namespace and the Secret must exist.

## Samples

[`config/samples`](../config/samples) contains a Secret, a `Project`, and a CDN `Resource`. The Secret password is `replace-me`. Applying the samples with real credentials creates a CDN resource in Selectel.

```sh
kubectl apply -k config/samples
kubectl get projects
kubectl get resources -n media
```
