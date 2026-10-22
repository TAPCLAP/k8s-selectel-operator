# CDN Resource

[Русская версия](ru/resource.md)

`Resource` (`cdn.selectel.tapclap.com/v1alpha1`) is the desired state of one Selectel HTTP CDN resource. It is namespaced. The Selectel project comes from `spec.projectRef.name`, which refers to a cluster-scoped [Project](project.md). The reference has no namespace field.

```yaml
apiVersion: cdn.selectel.tapclap.com/v1alpha1
kind: Resource
metadata:
  name: static
  namespace: media
spec:
  projectRef:
    name: production
  deletionPolicy: Deactivate
  name: static
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

A resource that serves traffic needs `origin.servers` with at least one entry. Selectel assigns `status.cdnDomain` (`<id>.selcdn.net`) and `status.id` (a UUID). Those fields, and the creation time, are status only.

`kubectl get resources` prints `Phase`, `Policy`, `Domain`, and `Names`. `Names` shows `spec.names` when personal domains are set.

## Operator fields

These fields belong to the operator. They are not sent to Selectel under the same names.

| Field | Default | Meaning |
| --- | --- | --- |
| `projectRef.name` | required | Cluster-scoped `Project`. |
| `deletionPolicy` | `Deactivate` | `Deactivate` or `Orphan`. See [deletion.md](deletion.md). Ignored while `managementPolicy` is `Observe`. |
| `managementPolicy` | `Manage` | `Manage` creates and updates the CDN resource. `Observe` links an existing resource by `resourceID` or by name and reports drift. `Observe` does not change Selectel. |
| `resourceID` | empty | Selectel UUID to import. You set it. The operator never writes this field. A created resource stores its UUID in `status.id`. |
| `certificate` | omitted | Object with `id`. Omitted means the default `*.selcdn.net` certificate. |
| `name` | `metadata.name` | Name stored in Selectel. Pattern `^[a-zA-Zа-яА-ЯёЁ0-9\-_ ]{1,50}$`. An invalid resolved name sets `Ready=False`, reason `InvalidName`, and the API is not called. |

CDN configuration fields use the Selectel JSON names (`https_only`, `allowed_http_methods`, `slice_size_megabytes`, and the rest). The OpenAPI document is [`api/openapi/cdn.yaml`](../api/openapi/cdn.yaml). The Go types are in `api/cdn.selectel/v1alpha1`.

Top-level configuration fields: `active`, `allowed_http_methods`, `auth`, `cache`, `compress`, `cors`, `follow_redirects`, `headers`, `http2https`, `https_only`, `ioss`, `limitations`, `locations`, `modern_tls_only`, `names`, `no_http2`, `origin`, `packaging`, `return`, `rewrite`, `robots`, `slice_size_megabytes`, `ssl_protocols`, `strong_ssl_ciphers`, `tuning`, `use_http3`.

Booleans and integers are optional so an explicit `false` or `0` is sent. Fields left unset are omitted. The controller compares the desired document with the API response and ignores extra keys the API adds as defaults, so server defaults do not cause a permanent update loop.

## Example

This resource serves only HTTPS on `cdn.example.com`, redirects HTTP to HTTPS, and disables HTTP/2. The origin is contacted over HTTPS. `origin.hostname` is the `Host` header sent to the origin, and `origin.sni_hostname` is the name used for SNI and origin certificate verification. `sni_hostname` requires `origin.https: true`. `hostname` cannot be set together with `origin.forward_host_header`.

`certificate.id` is the Selectel certificate sequence for the personal domain. The Project allowlist for this namespace must include that sequence. See [project.md](project.md).

Cache durations use `^(\d+)[smhd]$`. `365d` is one year. `0s` stores the response for zero time. `cache.valid.force: true` makes the CDN use these durations and ignore `Cache-Control` and `Expires` from the origin. `cache.disable: false` turns cache settings on. `consider_args: false` keeps the query string out of the cache key. `use_stale` serves the last cached copy for the listed origin failures.

```yaml
apiVersion: cdn.selectel.tapclap.com/v1alpha1
kind: Resource
metadata:
  name: static
  namespace: media
spec:
  projectRef:
    name: production
  no_http2: true
  https_only: true
  http2https: true
  names:
    - cdn.example.com
  certificate:
    id: 1
  origin:
    https: true
    hostname: origin.example.com
    sni_hostname: origin.example.com
    servers:
      origin.example.com: {}
  cache:
    disable: false
    consider_args: false
    use_stale: true
    stale_conditions:
      - error
      - timeout
      - http_503
      - http_502
      - http_504
    valid:
      force: true
      "2xx": 365d
      "3xx": 0s
      "4xx": 0s
      "5xx": 0s
      browser: 0s
```

## Create without import

Leave `spec.resourceID` empty. The operator does not fill it. With the default `managementPolicy: Manage`, the controller sends `POST /resources` and writes the UUID from the response to `status.id`. Later reconciles use `status.id` and do not create the resource again. The controller then `PUT`s until the spec matches.

## Import an existing CDN

Use this when a CDN resource already exists in Selectel and the custom resource is new, for example in a new cluster.

1. Copy the resource UUID from Selectel.
2. Create the `Resource` with `managementPolicy: Observe` and that UUID in `spec.resourceID`. Alternatively omit `resourceID` and set `spec.name` to the name already used in the Selectel project, or leave `spec.name` empty to use `metadata.name`. The operator lists resources and links the one with that name.
3. Wait until `status.id` is set. Selectel is not modified. Read `status.drift` and the `Ready` condition:
   - `Observing`: the spec matches. `Manage` would not send `PUT`.
   - `Drift`: `status.phase` is `Drift`. Each entry is a spec field `Manage` would update. `desired` and `actual` are short JSON values. Keys that exist only in the API response are not drift.
   - `WouldCreate`: nothing in Selectel matches. `status.id` stays empty and the search repeats about once a minute. `Manage` would send `POST`. `Observe` does not.
4. Edit the spec until `status.drift` is empty.
5. Set `managementPolicy: Manage`. The next reconcile applies a remaining difference once.

```yaml
spec:
  managementPolicy: Observe
  resourceID: 6d19b7a2-a493-48fa-a9ec-aea7ba9ffa28
  projectRef:
    name: production
  origin:
    https: true
    servers:
      origin.example.com: {}
```

While Selectel reports `Processing`, drift is not calculated. HTTP 404 on a known id sets `WouldRecreate` and does not create a replacement. Deleting the custom resource while `Observe` only removes the finalizer. Switch to `Manage` before deleting if the CDN resource should be deactivated. See [deletion.md](deletion.md).

## Certificate

`spec.certificate` is an object so a second source can be added later without changing the field type:

```yaml
certificate:
  id: 1
```

`id` is the CDN certificate `seq`. When `certificate` is present, `id` is required. There is no `ref` field in this version. A later Certificate custom resource is expected to add:

```yaml
certificate:
  ref:
    apiGroup: cdn.selectel.tapclap.com
    kind: Certificate
    name: cdn-example-com
```

That form is not accepted yet. When it exists, exactly one of `id` or `ref` will be set, and the sequence resolved from `ref` will be checked against the same Project allowlist. Permission stays on the sequence number.

The sequence must appear on the `allowedNamespaces` entry that matches this namespace. See [project.md](project.md). A missing `certificate` field skips that check. A sequence outside the list sets `Ready=False`, reason `CertificateNotAllowed`. Create and update are not sent. Deletion does not consult the certificate list: deactivation does not change the binding.

The API body receives the integer as `certificate`. The operator does not upload PEM material and does not order Let's Encrypt.

## Status

| Field | Meaning |
| --- | --- |
| `id` | Selectel resource UUID. Recorded as soon as create is accepted, before the resource is ready. |
| `cdnDomain` | Assigned `*.selcdn.net` domain. |
| `creationTimestamp` | Selectel creation time (`creation_ts`). |
| `phase` | `Pending`, `Processing`, `Ready`, `Error`, or `Drift`. |
| `drift` | Spec fields `Manage` would update. Empty when the spec matches. |
| `taskID` | Last Selectel configuration task id. |
| `conditions` | `Ready` is `True` only when the resource is completed and matches the spec. |

| Phase | When |
| --- | --- |
| `Pending` | No Selectel id yet. |
| `Processing` | Selectel status is `Processing`, including `dns processing` and `configuration processing`. |
| `Ready` | Selectel status is `Completed` and the spec matches. |
| `Drift` | `managementPolicy` is `Observe`, Selectel is completed, and the spec differs. |
| `Error` | Selectel status is `Error`, or the last create or adopt was rejected. |

| Reason | Ready | Meaning |
| --- | --- | --- |
| `Synced` | True | Spec matches the API under `Manage`. Reconcile again in about 5 minutes. |
| `Observing` | True | Spec matches and `managementPolicy` is `Observe`. Selectel is not changed. |
| `Drift` | False | `Observe` found fields `Manage` would update. Listed in `status.drift`. |
| `WouldCreate` | False | `Observe` found no Selectel resource. `Manage` would send `POST`. The search repeats about once a minute. |
| `WouldRecreate` | False | The linked Selectel resource is gone. `Observe` does not create it again. |
| `Processing` | False | Waiting. Reconcile again in about 15 seconds. Configuration is not changed. |
| `NamespaceNotAllowed` | False | Namespace is not in `allowedNamespaces`. |
| `CertificateNotAllowed` | False | `certificate.id` is not on the namespace entry. |
| `ProjectNotFound` | False | `projectRef` does not exist. |
| `ProjectNotReady` | False | Project cannot issue a token yet. |
| `SecretNamespaceForbidden` | False | Project Secret is outside the operator namespace. |
| `SecretNotFound` | False | Project Secret is missing. |
| `SecretInvalid` | False | Project Secret is missing a key. |
| `InvalidName` | False | Resolved Selectel name does not match the API pattern. |
| `CreateRejected` | False | Create response was `error`, or it had no resource id. No second `POST` until `metadata.generation` changes. |
| `AdoptFailed` | False | `resourceID` was not found. No `POST` until `metadata.generation` changes. |
| `SelectelError` | False | Selectel phase is `Error` and the spec already matches. Retried about once a minute. A spec change is still sent with `PUT`, so a bad configuration can be corrected. |
| `RateLimited` | False | HTTP 429. Waits for `Retry-After`, or 30 seconds. |
| `AuthFailed` | False | Token request failed, or CDN still returns 401 after one refresh. |
| `APIError` | False | Other HTTP error from the CDN API. |

## Reconcile

1. If the Resource is being deleted and `managementPolicy` is `Manage`, run the deletion policy even when the namespace or certificate is no longer allowed. `Observe` only removes the finalizer. See [deletion.md](deletion.md).
2. Resolve the Project. A missing or unready Project leaves the CDN API untouched.
3. Reject the namespace or certificate when the allowlist says so.
4. If `status.id` is empty and `spec.resourceID` is set, `GET` that id and store it. HTTP 404 sets `AdoptFailed` for this generation.
5. If there is still no id, `GET /resources` and look for `spec.name`, including inactive resources. A match is linked. `Manage` sends `POST /resources` when nothing matches. `Observe` sets `WouldCreate` and tries again.
6. On `status: accept`, write `id`, `cdnDomain`, and `taskID` immediately. A create that returns `error` sets `CreateRejected` for this generation.
7. Poll status. `Processing` waits. `Completed` compares the spec with `GET`. Under `Manage`, a difference sends `PUT /resources/{id}` with the full desired body, without `id`, `cdn_domain`, or `creation_ts`. `Observe` records that difference in `status.drift`. `PATCH` is reserved for deactivation: the API describes patch as first-level updates, which can wipe a nested object.
8. HTTP 404 on a known id under `Manage` clears `status.id`, emits the Event `Recreated`, and creates the resource again. `Observe` sets `WouldRecreate` and keeps the id.
9. HTTP 401 drops the token cache and retries the call once.

A ready resource is read from the API about every 5 minutes, rather than waiting for the default informer resync.
