# Project and credentials

[Русская версия](ru/project.md)

`Project` is a cluster-scoped object (`selectel.tapclap.com/v1alpha1`). It does not create a project in Selectel. It names the Secret used to obtain a project-scoped IAM token, and it lists the namespaces allowed to create CDN resources with that token.

CDN API calls require that token in `X-Auth-Token`. Account-scoped tokens and static tokens are not used.

```yaml
apiVersion: selectel.tapclap.com/v1alpha1
kind: Project
metadata:
  name: production
spec:
  authURL: https://cloud.api.selcloud.ru/identity/v3
  allowedNamespaces:
    - name: media
      certificates:
        - id: 1
        - id: 4
    - name: games
      certificates:
        - id: 4
  credentialsSecretRef:
    name: selectel-production
    namespace: selectel-infra
```

`authURL` is optional. The default is `https://cloud.api.selcloud.ru/identity/v3`.

## Secret

The Secret must live in the operator credentials namespace (`selectel-infra`, or the value of `--credentials-namespace`). `credentialsSecretRef.namespace` must be that same namespace. A reference to any other namespace is rejected before the Secret is read.

| Key | Meaning |
| --- | --- |
| `accountId` | Selectel account number. Sent to Keystone as `domain.name`. |
| `username` | Keystone user name. |
| `password` | Keystone password. |
| `projectName` | Selectel project the token is scoped to. |

All four keys are required. The token is kept in the manager process. It is not written to `Project` status and it is not logged.

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

The controller requests `POST {authURL}/auth/tokens` with a password identity and a project scope. Expiry comes from `token.expires_at`, or 24 hours when that field is absent. The cache key is the `Project` name. A token is refreshed when it expires within 30 minutes, or when the Secret contents change. A CDN `401` drops the cached token and the call is retried once.

`Project` becomes `Ready=True`, reason `TokenIssued`, after a token is issued. The controller checks again about every 30 minutes.

## Namespace allowlist

`spec.allowedNamespaces` is a list of objects, not a list of strings.

| `name` | Effect |
| --- | --- |
| Exact namespace, for example `media` | That namespace may use the Project. |
| `*` | Namespaces that have no exact entry may use the Project. |
| Empty list, or field omitted | No namespace may use the Project. |

An exact entry replaces `*`. Certificates from `*` are not added to the exact entry. Other patterns and label selectors are not supported.

```yaml
allowedNamespaces:
  - name: "*"
    certificates:
      - id: 9
  - name: media
    certificates:
      - id: 1
```

Here `media` may bind certificate `1` only. Certificate `9` is available to other namespaces, and is not available to `media`. A namespace with an empty `certificates` list may create a CDN resource and may not set `spec.certificate`.

Removing a namespace from the list stops create and update for its CDN resources. Resources already created in Selectel stay as they are. Deleting the custom resource still runs its deletion policy, so the finalizer can complete. See [deletion.md](deletion.md).

Changing `allowedNamespaces`, including the certificate lists, requeues every CDN `Resource` whose `spec.projectRef.name` matches this Project.

## Certificate allowlist

`certificates[].id` is the CDN API certificate `seq`: the same integer the CDN resource field `certificate` accepts. The object form leaves room to allow a cluster object later, next to `id`.

The operator does not call `GET /certificate` to check that the sequence exists. The list is an in-cluster permission. A CDN `Resource` whose `spec.certificate.id` is absent from the matching entry gets `Ready=False`, reason `CertificateNotAllowed`, and no create or update is sent.

Taking an id off the list does not detach a certificate already bound in Selectel. The next desired update that still sets that id is rejected. An update that omits `certificate` is a normal spec change and is sent when the namespace itself is still allowed.

## Status

`kubectl get projects` prints the `Ready` condition.

| Reason | Ready | Meaning |
| --- | --- | --- |
| `TokenIssued` | True | A project-scoped token was issued. |
| `SecretNotFound` | False | The Secret is missing. |
| `SecretNamespaceForbidden` | False | The Secret namespace is not the operator credentials namespace. |
| `SecretInvalid` | False | A required key is missing. |
| `AuthFailed` | False | Keystone rejected the request. |
| `ResourcesRemain` | False, while deleting | A CDN `Resource` still references this Project, so the finalizer stays. |

Auth failures are retried about once a minute.

## Finalizer

The finalizer is `selectel.tapclap.com/project`. It stays while any CDN `Resource` has `spec.projectRef.name` equal to this Project, including a Resource that is itself being deleted. That keeps the credentials available until the CDN finalizer can deactivate or orphan the Selectel resource. When the last referencing Resource is gone, the Project finalizer is removed and the Project can disappear.
