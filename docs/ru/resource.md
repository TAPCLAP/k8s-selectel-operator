# CDN Resource

[English](../resource.md) — основная версия этой страницы.

`Resource` (`cdn.selectel.tapclap.com/v1alpha1`) — желаемое состояние одного HTTP CDN-ресурса Selectel. Объект namespaced. Проект Selectel берётся из `spec.projectRef.name` и указывает на cluster-scoped [Project](project.md). У ссылки нет поля namespace.

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

Ресурсу, который отдаёт трафик, нужен `origin.servers` хотя бы с одной записью. Selectel выдаёт `status.cdnDomain` (`<id>.selcdn.net`) и `status.id` (UUID). Эти поля и время создания есть только в status.

`kubectl get resources` показывает `Phase`, `Domain` и `ID`.

## Поля оператора

Эти поля принадлежат оператору. В Selectel они не уходят под теми же именами.

| Поле | По умолчанию | Смысл |
| --- | --- | --- |
| `projectRef.name` | обязательное | Cluster-scoped `Project`. |
| `deletionPolicy` | `Deactivate` | `Deactivate` или `Orphan`. См. [deletion.md](deletion.md). Не действует, пока `managementPolicy` равен `Observe`. |
| `managementPolicy` | `Manage` | `Manage` создаёт и обновляет CDN-ресурс. `Observe` связывает существующий ресурс по `resourceID` или по имени и сообщает об отличиях. `Observe` Selectel не меняет. |
| `resourceID` | пусто | UUID Selectel для импорта. Его задаёте вы. Оператор это поле не заполняет. UUID созданного ресурса пишется в `status.id`. |
| `certificate` | не задано | Объект с `id`. Если поля нет, используется сертификат `*.selcdn.net` по умолчанию. |
| `name` | `metadata.name` | Имя в Selectel. Паттерн `^[a-zA-Zа-яА-ЯёЁ0-9\-_ ]{1,50}$`. Неверное итоговое имя ставит `Ready=False`, причина `InvalidName`, и API не вызывается. |

Поля конфигурации CDN названы как в JSON Selectel (`https_only`, `allowed_http_methods`, `slice_size_megabytes` и остальные). OpenAPI-документ — [`api/openapi/cdn.yaml`](../../api/openapi/cdn.yaml). Go-типы лежат в `api/cdn.selectel/v1alpha1`.

Поля конфигурации верхнего уровня: `active`, `allowed_http_methods`, `auth`, `cache`, `compress`, `cors`, `follow_redirects`, `headers`, `http2https`, `https_only`, `ioss`, `limitations`, `locations`, `modern_tls_only`, `names`, `no_http2`, `origin`, `packaging`, `return`, `rewrite`, `robots`, `slice_size_megabytes`, `ssl_protocols`, `strong_ssl_ciphers`, `tuning`, `use_http3`.

Булевы значения и целые числа необязательны, поэтому явные `false` и `0` уходят в API. Незаданные поля опускаются. Контроллер сравнивает желаемый документ с ответом API и игнорирует лишние ключи, которые API добавляет как значения по умолчанию, чтобы серверные умолчания не вызывали бесконечный цикл обновлений.

## Создание без импорта

`spec.resourceID` остаётся пустым. Оператор его не заполняет. При политике по умолчанию `managementPolicy: Manage` контроллер шлёт `POST /resources` и записывает UUID из ответа в `status.id`. Дальше reconcile идёт по `status.id` и не создаёт ресурс повторно. Затем контроллер шлёт `PUT`, пока spec не совпадёт.

## Импорт существующего CDN

Этот сценарий нужен, когда CDN-ресурс уже есть в Selectel, а custom resource новый, например в новом кластере.

1. Скопируйте UUID ресурса из Selectel.
2. Создайте `Resource` с `managementPolicy: Observe` и этим UUID в `spec.resourceID`. Либо не задавайте `resourceID`, а укажите в `spec.name` имя, которое уже занято в проекте Selectel. Пустое `spec.name` означает `metadata.name`. Оператор найдёт ресурс в списке и свяжет его.
3. Дождитесь `status.id`. Selectel при этом не меняется. Смотрите `status.drift` и условие `Ready`:
   - `Observing`: spec совпадает. `Manage` не отправил бы `PUT`.
   - `Drift`: `status.phase` равен `Drift`. Каждая запись — поле spec, которое обновил бы `Manage`. `desired` и `actual` — короткие JSON-значения. Ключи, которые есть только в ответе API, дрифтом не считаются.
   - `WouldCreate`: в Selectel ничего не совпало. `status.id` остаётся пустым, поиск повторяется примерно раз в минуту. `Manage` отправил бы `POST`. `Observe` этого не делает.
4. Правьте spec, пока `status.drift` не станет пустым.
5. Поставьте `managementPolicy: Manage`. Следующий reconcile один раз применяет оставшееся отличие.

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

Пока Selectel сообщает `Processing`, дрифт не считается. HTTP 404 на известный id ставит `WouldRecreate` и не создаёт замену. Удаление custom resource в `Observe` только снимает finalizer. Чтобы выключить CDN, сначала переключитесь на `Manage` и затем удалите объект. См. [deletion.md](deletion.md).

## Сертификат

`spec.certificate` — объект, чтобы позже добавить второй источник без смены типа поля:

```yaml
certificate:
  id: 1
```

`id` — `seq` сертификата CDN. Если `certificate` задан, `id` обязателен. В этой версии поля `ref` нет. Позже custom resource Certificate должен добавить такую форму:

```yaml
certificate:
  ref:
    apiGroup: cdn.selectel.tapclap.com
    kind: Certificate
    name: cdn-example-com
```

Сейчас она не принимается. Когда она появится, задаётся ровно одно из `id` или `ref`, а seq, полученный из `ref`, проверяется тем же списком Project. Разрешение остаётся на номере seq.

Seq должен быть в записи `allowedNamespaces`, которая подходит этому namespace. См. [project.md](project.md). Отсутствие поля `certificate` пропускает эту проверку. Seq вне списка ставит `Ready=False`, причина `CertificateNotAllowed`. Create и update не отправляются. Удаление список сертификатов не смотрит: деактивация не меняет привязку.

В теле API уходит целое число в поле `certificate`. Оператор не загружает PEM и не заказывает Let's Encrypt.

## Status

| Поле | Смысл |
| --- | --- |
| `id` | UUID ресурса Selectel. Записывается сразу после принятия создания, до готовности ресурса. |
| `cdnDomain` | Выданный домен `*.selcdn.net`. |
| `creationTimestamp` | Время создания в Selectel (`creation_ts`). |
| `phase` | `Pending`, `Processing`, `Ready`, `Error` или `Drift`. |
| `drift` | Поля spec, которые обновил бы `Manage`. Пусто, когда spec совпадает. |
| `taskID` | Id последней задачи конфигурации Selectel. |
| `conditions` | `Ready` равен `True`, только когда ресурс завершён и совпадает со spec. |

| Фаза | Когда |
| --- | --- |
| `Pending` | Id Selectel ещё нет. |
| `Processing` | Статус Selectel — `Processing`, включая `dns processing` и `configuration processing`. |
| `Ready` | Статус Selectel — `Completed`, и spec совпадает. |
| `Drift` | `managementPolicy` равен `Observe`, Selectel завершил применение, а spec отличается. |
| `Error` | Статус Selectel — `Error`, либо последнее создание или усыновление отклонено. |

| Причина | Ready | Смысл |
| --- | --- | --- |
| `Synced` | True | Spec совпадает с API при `Manage`. Следующий reconcile примерно через 5 минут. |
| `Observing` | True | Spec совпадает, и `managementPolicy` равен `Observe`. Selectel не меняется. |
| `Drift` | False | `Observe` нашёл поля, которые обновил бы `Manage`. Они перечислены в `status.drift`. |
| `WouldCreate` | False | `Observe` не нашёл ресурс Selectel. `Manage` отправил бы `POST`. Поиск повторяется примерно раз в минуту. |
| `WouldRecreate` | False | Связанный ресурс Selectel исчез. `Observe` не создаёт его заново. |
| `Processing` | False | Ожидание. Следующий reconcile примерно через 15 секунд. Конфигурация не меняется. |
| `NamespaceNotAllowed` | False | Namespace нет в `allowedNamespaces`. |
| `CertificateNotAllowed` | False | `certificate.id` нет в записи namespace. |
| `ProjectNotFound` | False | `projectRef` не существует. |
| `ProjectNotReady` | False | Project ещё не может выпустить токен. |
| `SecretNamespaceForbidden` | False | Secret проекта лежит вне namespace оператора. |
| `SecretNotFound` | False | Secret проекта отсутствует. |
| `SecretInvalid` | False | В Secret проекта нет ключа. |
| `InvalidName` | False | Итоговое имя Selectel не совпадает с паттерном API. |
| `CreateRejected` | False | Ответ создания — `error`, или в нём нет id ресурса. Второй `POST` не уходит, пока не изменится `metadata.generation`. |
| `AdoptFailed` | False | `resourceID` не найден. `POST` не уходит, пока не изменится `metadata.generation`. |
| `SelectelError` | False | Фаза Selectel — `Error`, и spec уже совпадает. Повтор примерно раз в минуту. Изменение spec всё равно уходит через `PUT`, чтобы плохую конфигурацию можно было исправить. |
| `RateLimited` | False | HTTP 429. Ожидание по `Retry-After` или 30 секунд. |
| `AuthFailed` | False | Запрос токена не удался, или CDN снова вернул 401 после одного обновления. |
| `APIError` | False | Другая HTTP-ошибка CDN API. |

## Reconcile

1. Если Resource удаляется и `managementPolicy` равен `Manage`, выполняется deletion policy, даже когда namespace или сертификат уже не разрешены. `Observe` только снимает finalizer. См. [deletion.md](deletion.md).
2. Находится Project. Если его нет или он не Ready, CDN API не вызывается.
3. Namespace или сертификат отклоняются, если так говорит список разрешений.
4. Если `status.id` пуст и задан `spec.resourceID`, выполняется `GET` этого id, и он сохраняется. HTTP 404 ставит `AdoptFailed` для этого generation.
5. Если id всё ещё нет, выполняется `GET /resources` и поиск по `spec.name`, включая неактивные ресурсы. Совпадение связывается. `Manage` шлёт `POST /resources`, если совпадения нет. `Observe` ставит `WouldCreate` и повторяет попытку.
6. При `status: accept` сразу записываются `id`, `cdnDomain` и `taskID`. Создание с ответом `error` ставит `CreateRejected` для этого generation.
7. Опрашивается статус. `Processing` ждёт. `Completed` сравнивает spec с `GET`. При `Manage` отличие отправляет `PUT /resources/{id}` с полным желаемым телом, без `id`, `cdn_domain` и `creation_ts`. `Observe` записывает это отличие в `status.drift`. `PATCH` оставлен для деактивации: API описывает patch как изменение полей первого уровня, и вложенный объект так легко затереть.
8. HTTP 404 на известный id при `Manage` очищает `status.id`, пишет Event `Recreated` и создаёт ресурс заново. `Observe` ставит `WouldRecreate` и оставляет id.
9. HTTP 401 сбрасывает кэш токена и повторяет вызов один раз.

Готовый ресурс читается из API примерно раз в 5 минут, а не ждёт стандартный resync informer.
