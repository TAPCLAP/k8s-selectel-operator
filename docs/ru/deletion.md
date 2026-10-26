# Удаление

[English](../deletion.md) — основная версия этой страницы.

OpenAPI CDN, на которой собран оператор (`api/openapi/cdn.yaml`, v0.0.1), не содержит `DELETE /resources/{id}`. Текст тега упоминает удаление, операций удаления нет. Удаление custom resource `Resource` поэтому не уничтожает объект в Selectel.

`spec.deletionPolicy` выбирает, что делает finalizer. Имя finalizer — `cdn.selectel.tapclap.com/resource`.

| Политика | Поведение |
| --- | --- |
| `Deactivate` | Значение по умолчанию. `PATCH /resources/{id}` с телом `{"active": false}`, затем finalizer снимается. Ресурс Selectel остаётся выключенным. |
| `Orphan` | Finalizer снимается. Selectel не вызывается. |

`managementPolicy: Observe` снимает finalizer и не вызывает Selectel, в том числе когда `deletionPolicy` равен `Deactivate`. Поставьте `Manage` перед удалением custom resource, если CDN-ресурс нужно выключить.

```yaml
spec:
  deletionPolicy: Deactivate
```

На живом проекте проверьте, освобождает ли `active: false` имя ресурса, прежде чем считать удаление custom resource удалением CDN. Имя может остаться занятым.

## Когда finalizer всё равно выполняется

Удаление выполняет политику, даже если namespace убрали из `Project.spec.allowedNamespaces`, даже если `certificate.id` больше нет в списке, и даже если `spec.names` больше не совпадает с `domains`. Деактивация не меняет привязку сертификата, а застрявший finalizer заблокировал бы удаление CR. Проверки namespace, сертификата и домена относятся к create, update и adopt.

Если Selectel уже отвечает 404 на известный id, finalizer снимается. Деактивировать нечего.

Если нет Project или его Secret, `Deactivate` оставляет finalizer и повторяет попытку. Чтобы отправить `active: false`, контроллеру нужен токен. `Orphan` токен не требует.

Удаление namespace из списка не выключает CDN-ресурсы, которые уже созданы. `active: false` уходит только при удалении custom resource с политикой `Deactivate`.

## Удаление Project

У `Project` finalizer `selectel.tapclap.com/project`. Он остаётся, пока любой CDN `Resource` ссылается на имя этого Project, в том числе Resource, чьё собственное удаление уже началось. Когда последнего Resource не остаётся, finalizer Project снимается.

Сначала удалите CDN-ресурсы, либо ожидайте, что Project останется, пока их finalizer не завершатся. `kubectl describe project <name>` в этом состоянии показывает `ResourcesRemain`.
