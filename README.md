# Selectel Operator

Kubernetes operator that creates [Selectel](https://selectel.ru) CDN resources and keeps them aligned with a custom resource. Authentication is shared: a cluster-scoped `Project` issues a project-scoped IAM token and decides which namespaces may use that project, and which certificate sequence numbers they may bind.

This version manages CDN resources. Uploading a PEM certificate, ordering Let's Encrypt, and purging the cache are later custom resources. The `certificate` field on a CDN resource is already an object, so a reference to a future Certificate can be added beside `id`.

[Русская версия](README.ru.md)

## Documentation

| Topic | English | Russian |
| --- | --- | --- |
| Install and run | [docs/installation.md](docs/installation.md) | [docs/ru/installation.md](docs/ru/installation.md) |
| Project and credentials | [docs/project.md](docs/project.md) | [docs/ru/project.md](docs/ru/project.md) |
| CDN Resource | [docs/resource.md](docs/resource.md) | [docs/ru/resource.md](docs/ru/resource.md) |
| Deletion | [docs/deletion.md](docs/deletion.md) | [docs/ru/deletion.md](docs/ru/deletion.md) |
| Development | [docs/development.md](docs/development.md) | [docs/ru/development.md](docs/ru/development.md) |

English pages are the primary documentation.

## Install

`selectel-operator` includes the CRDs and the controller. Installing that chart is enough. `selectel-operator-crds` is the same CRDs without the operator; install it first only when CRDs are managed on their own. Helm does not upgrade CRDs from that chart's `crds/` directory on a later upgrade.

The release namespace is where the operator reads credential Secrets. `Project.spec.credentialsSecretRef.namespace` must be that namespace.

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

`<version>` is the git tag without the leading `v`. A tag `v*` publishes the image `ghcr.io/<owner>/<repo>:<tag>` and both charts. The chart in this repository is `0.0.4`. From a checkout:

```bash
helm install selectel-operator helm/selectel-operator \
  --namespace selectel-infra \
  --create-namespace \
  --set image.repository=<registry>/selectel-operator \
  --set image.tag=latest
```

Kustomize install from this repository is described in [docs/installation.md](docs/installation.md).

## Create a CDN resource

1. Put Selectel credentials in `selectel-infra`. `accountId` is the Selectel account number. Keystone receives it as `domain.name`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: selectel-production
  namespace: selectel-infra
stringData:
  accountId: "123456"
  username: cdn-operator
  password: replace-me
  projectName: production
```

2. Create a cluster-scoped `Project`. `allowedNamespaces` names the tenant namespaces that may use it. `certificates[].id` is the CDN API certificate `seq`. An empty list denies every namespace.

```yaml
apiVersion: selectel.tapclap.com/v1alpha1
kind: Project
metadata:
  name: production
spec:
  allowedNamespaces:
    - name: media
      certificates:
        - id: 1
  credentialsSecretRef:
    name: selectel-production
    namespace: selectel-infra
```

3. Create a namespaced `Resource`. `projectRef.name` points at the cluster Project. Omit `certificate` to serve the default `*.selcdn.net` certificate.

```yaml
apiVersion: cdn.selectel.tapclap.com/v1alpha1
kind: Resource
metadata:
  name: static
  namespace: media
spec:
  projectRef:
    name: production
  origin:
    https: true
    servers:
      origin.example.com: {}
  names:
    - cdn.example.com
  certificate:
    id: 1
  http2https: true
```

Examples with the same shape live in [`config/samples`](config/samples). Replace the password before applying them.

```sh
kubectl apply -k config/samples
kubectl get projects
kubectl get resources -n media
```

`status.phase` becomes `Ready` after Selectel reports `Completed`. `status.cdnDomain` is the assigned `*.selcdn.net` name. `status.id` is the Selectel resource UUID. `spec.resourceID` stays empty: that field is only for import.

## Import an existing CDN

Set `managementPolicy: Observe` and the Selectel UUID before the operator is allowed to change anything. `status.drift` lists what `Manage` would update. The full sequence is in [docs/resource.md](docs/resource.md).

```yaml
spec:
  managementPolicy: Observe
  resourceID: 6d19b7a2-a493-48fa-a9ec-aea7ba9ffa28
  projectRef:
    name: production
```

Switch the policy to `Manage` after `status.drift` is empty. Omit `resourceID` and set `spec.name` when the Selectel resource should be found by name. Deleting the custom resource while it is `Observe` does not deactivate the CDN resource.

## What the controller does

- Creates the CDN resource once and writes `status.id` before polling. A repeated reconcile does not send a second `POST`.
- Adopts an existing resource when `spec.resourceID` is set, or when `Observe` finds the same name. `managementPolicy: Observe` stops after linking the resource and writes the difference in `status.drift`.
- Sends `PUT /resources/{id}` when the spec differs from the API. While Selectel reports `Processing`, it waits and does not send another configuration change.
- Refuses create and update when the namespace is missing from `allowedNamespaces`, or when `certificate.id` is not on that namespace's list. `Ready` is `False` with reason `NamespaceNotAllowed` or `CertificateNotAllowed`.
- On delete, `deletionPolicy: Deactivate` (the default) sends `{"active": false}` and then drops the finalizer. `Orphan` drops the finalizer and leaves the Selectel resource unchanged. The CDN API in this spec has no delete operation, so the Selectel object remains. Whether `active: false` frees the name has to be checked on a live project before treating CR deletion as CDN deletion. Details are in [docs/deletion.md](docs/deletion.md).

## License

Apache License 2.0. Source file headers carry the license notice.
