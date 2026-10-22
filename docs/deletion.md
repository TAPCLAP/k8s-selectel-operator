# Deletion

[Русская версия](ru/deletion.md)

The CDN OpenAPI spec used by this operator (`api/openapi/cdn.yaml`, v0.0.1) has no `DELETE /resources/{id}`. The tag text mentions deletion, and the operations do not. Removing a `Resource` custom resource therefore cannot destroy the Selectel object.

`spec.deletionPolicy` chooses what the finalizer does. The finalizer name is `cdn.selectel.tapclap.com/resource`.

| Policy | Behavior |
| --- | --- |
| `Deactivate` | Default. `PATCH /resources/{id}` with `{"active": false}`, then the finalizer is removed. The Selectel resource stays, turned off. |
| `Orphan` | The finalizer is removed. Selectel is not called. |

`managementPolicy: Observe` removes the finalizer and does not call Selectel, including when `deletionPolicy` is `Deactivate`. Set `Manage` before deleting the custom resource when the CDN resource should be turned off.

```yaml
spec:
  deletionPolicy: Deactivate
```

Check on a live project whether `active: false` releases the resource name before you treat deletion of the custom resource as deletion of the CDN. The name can remain taken.

## When the finalizer still runs

Deletion runs the policy even if the namespace was removed from `Project.spec.allowedNamespaces`, and even if `certificate.id` is no longer on the allowlist. Deactivation does not change the certificate binding, and a stuck finalizer would block CR deletion. The namespace check applies to create, update, and adopt.

If Selectel already returns 404 for a known id, the finalizer is removed. There is nothing left to deactivate.

If the Project or its Secret is missing, `Deactivate` keeps the finalizer and retries. The controller needs the token to send `active: false`. `Orphan` does not need the token.

Removing a namespace from the allowlist does not turn off CDN resources that already exist. Only deletion of the custom resource, with `Deactivate`, sends `active: false`.

## Project deletion

`Project` has the finalizer `selectel.tapclap.com/project`. It stays while any CDN `Resource` references that Project name, including a Resource whose own deletion has started. After the last Resource is gone, the Project finalizer is removed.

Delete CDN resources first, or expect the Project to remain until their finalizers finish. `kubectl describe project <name>` shows `ResourcesRemain` in that state.
