# Оператор Selectel

Оператор Kubernetes создаёт CDN-ресурсы [Selectel](https://selectel.ru) и приводит их к custom resource. Аутентификация общая: cluster-scoped `Project` выпускает project-scoped IAM-токен и задаёт, каким namespace можно пользоваться этим проектом и какие номера сертификатов им можно привязывать.

Эта версия управляет CDN-ресурсами. Загрузка PEM, заказ Let's Encrypt и сброс кэша — отдельные custom resource, которые появятся позже. Поле `certificate` у CDN-ресурса уже объект, поэтому ссылку на будущий Certificate можно добавить рядом с `id`.

Основная документация на английском: [README.md](README.md).

## Документация

| Тема | English | Русский |
| --- | --- | --- |
| Установка и запуск | [docs/installation.md](docs/installation.md) | [docs/ru/installation.md](docs/ru/installation.md) |
| Project и учётные данные | [docs/project.md](docs/project.md) | [docs/ru/project.md](docs/ru/project.md) |
| CDN Resource | [docs/resource.md](docs/resource.md) | [docs/ru/resource.md](docs/ru/resource.md) |
| Удаление | [docs/deletion.md](docs/deletion.md) | [docs/ru/deletion.md](docs/ru/deletion.md) |
| Разработка | [docs/development.md](docs/development.md) | [docs/ru/development.md](docs/ru/development.md) |

## Установка

`selectel-operator` содержит CRD и контроллер. Этого чарта достаточно. `selectel-operator-crds` — те же CRD без оператора; его ставят первым, только если CRD ведут отдельно. При последующем `helm upgrade` Helm не обновляет CRD из каталога `crds/` этого чарта.

Namespace релиза — тот, откуда оператор читает секреты с учётными данными. `Project.spec.credentialsSecretRef.namespace` должен совпадать с ним.

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

`<version>` — это git-тег без начального `v`. Тег `v*` публикует образ `ghcr.io/<owner>/<repo>:<tag>` и оба чарта. Чарт в этом репозитории имеет версию `0.0.5`. Из исходников:

```bash
helm install selectel-operator helm/selectel-operator \
  --namespace selectel-infra \
  --create-namespace \
  --set image.repository=<registry>/selectel-operator \
  --set image.tag=latest
```

Установка через Kustomize из этого репозитория описана в [docs/ru/installation.md](docs/ru/installation.md).

## CDN-ресурс

1. Положите учётные данные Selectel в `selectel-infra`. `accountId` — номер аккаунта Selectel. В Keystone он уходит как `domain.name`.

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

2. Создайте cluster-scoped `Project`. `allowedNamespaces` перечисляет namespace тенантов, которым можно им пользоваться. `certificates[].id` — это `seq` сертификата в CDN API. Пустой список не разрешает ни один namespace.

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

3. Создайте namespaced `Resource`. `projectRef.name` указывает на cluster Project. Без `certificate` ресурс отдаётся на сертификате `*.selcdn.net` по умолчанию.

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

Такие же примеры лежат в [`config/samples`](config/samples). Перед применением замените пароль.

```sh
kubectl apply -k config/samples
kubectl get projects
kubectl get resources -n media
```

`status.phase` становится `Ready`, когда Selectel сообщает `Completed`. `status.cdnDomain` — выданное имя `*.selcdn.net`. `status.id` — UUID ресурса в Selectel. `spec.resourceID` остаётся пустым: это поле только для импорта.

## Импорт существующего CDN

Поставьте `managementPolicy: Observe` и UUID Selectel до того, как оператору можно что-то менять. `status.drift` перечисляет, что обновил бы `Manage`. Полная последовательность — в [docs/ru/resource.md](docs/ru/resource.md).

```yaml
spec:
  managementPolicy: Observe
  resourceID: 6d19b7a2-a493-48fa-a9ec-aea7ba9ffa28
  projectRef:
    name: production
```

Переключите политику на `Manage`, когда `status.drift` пуст. Не задавайте `resourceID` и укажите `spec.name`, если ресурс Selectel нужно найти по имени. Удаление custom resource в `Observe` не деактивирует CDN-ресурс.

## Что делает контроллер

- Создаёт CDN-ресурс один раз и записывает `status.id` до опроса готовности. Повторный reconcile не шлёт второй `POST`.
- Усыновляет существующий ресурс, если задан `spec.resourceID`, или если `Observe` находит то же имя. `managementPolicy: Observe` останавливается после привязки и пишет отличие в `status.drift`.
- Шлёт `PUT /resources/{id}`, когда spec отличается от API. Пока Selectel сообщает `Processing`, контроллер ждёт и не отправляет следующее изменение конфигурации.
- Отказывает в create и update, если namespace нет в `allowedNamespaces` или `certificate.id` нет в списке этого namespace. `Ready` равен `False`, причина `NamespaceNotAllowed` или `CertificateNotAllowed`.
- При удалении `deletionPolicy: Deactivate` (значение по умолчанию) шлёт `{"active": false}` и снимает finalizer. `Orphan` снимает finalizer и оставляет ресурс Selectel как есть. В этой спеке CDN API нет операции удаления, объект в Selectel остаётся. Освобождает ли `active: false` имя, нужно проверить на живом проекте, прежде чем считать удаление CR удалением CDN. Подробности в [docs/ru/deletion.md](docs/ru/deletion.md).

## Лицензия

Apache License 2.0. Уведомление стоит в заголовках исходных файлов.
