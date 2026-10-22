# Установка

[English](../installation.md) — основная версия этой страницы.

Оператор ставится Helm-чартом. Секреты с учётными данными читаются из namespace релиза. Установка через Kustomize из этого репозитория тоже доступна и использует namespace `selectel-infra`.

## Что нужно

- Кластер Kubernetes и `kubectl`, направленный на него
- Docker или другой инструмент, выбранный через `CONTAINER_TOOL`, чтобы собрать образ менеджера
- Проект Selectel и пользователь, который может получить project-scoped IAM-токен
- Go 1.25.4, если собирать или тестировать из исходников

## Helm

`selectel-operator` содержит CRD и контроллер. Этого чарта достаточно. `selectel-operator-crds` содержит те же CRD и больше ничего; его ставят первым, только если CRD ведут отдельно. Последующий upgrade чарта CRD не меняет CRD, которые Helm уже применил из его каталога `crds/`.

Тег вида `v*` запускает [`.github/workflows/release.yml`](../../.github/workflows/release.yml). Он публикует `ghcr.io/<owner>/<repository>:<tag>` и оба чарта в `oci://ghcr.io/<owner>/charts`.

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

`<version>` — это git-тег без начального `v`. Чарт в этом репозитории имеет версию `0.0.3`.

Из исходников укажите чарту собранный образ:

```bash
helm install selectel-operator helm/selectel-operator \
  --namespace selectel-infra \
  --create-namespace \
  --set image.repository=<registry>/selectel-operator \
  --set image.tag=latest
```

Для такой установки `Project.spec.credentialsSecretRef.namespace` должен быть `selectel-infra`. Чарт передаёт `--credentials-namespace` равным namespace релиза и даёт менеджеру `get`, `list` и `watch` на Secret в нём.

| Значение | По умолчанию | Смысл |
| --- | --- | --- |
| `image.repository` | `ghcr.io/tapclap/k8s-selectel-operator` | Образ менеджера. Workflow релиза переписывает его на репозиторий, который собрал чарт. |
| `image.tag` | `appVersion` чарта | Тег образа. |
| `leaderElection` | `true` | Передаёт `--leader-elect`. |
| `cdn.baseURL` | пусто | Передаёт `--cdn-base-url`, если задано. Пустое значение оставляет `https://api.selectel.ru/cdn/v3`. |
| `cdn.processingInterval` | `3m` | Передаёт `--processing-interval`. Оператор добавляет случайные 0–30 секунд. |
| `metrics.enabled` | `true` | HTTPS-метрики на `metrics.bindAddress` и Service. |
| `metrics.bindAddress` | `:8443` | Адрес метрик. Порт Service — числовая часть. |

## Сборка и установка через Kustomize

```sh
export IMG=<registry>/selectel-operator:latest
make docker-build docker-push IMG=$IMG
make deploy IMG=$IMG
```

`make deploy` ставит CRD, RBAC и Deployment. В аргументах контейнера есть `--leader-elect` и `--credentials-namespace=selectel-infra`.

Только CRD, без менеджера:

```sh
make install
```

Убрать менеджер и оставить CRD:

```sh
make undeploy
```

Убрать CRD:

```sh
make uninstall
```

Один манифест для установки без Makefile репозитория:

```sh
make build-installer IMG=<registry>/selectel-operator:latest
kubectl apply -f dist/install.yaml
```

## Где он работает

Установка через Kustomize (`make deploy`) использует такие имена:

| Объект | Имя |
| --- | --- |
| Namespace | `selectel-infra` |
| Deployment | `selectel-operator-controller-manager` |
| ServiceAccount | `selectel-operator-controller-manager` |
| Role на секреты | `selectel-operator-credentials-reader` |

Установка Helm с именем релиза `selectel-operator` называет Deployment, ServiceAccount и Role `selectel-operator` в namespace релиза. Role на секреты — `selectel-operator-credentials`.

Логи Deployment из Kustomize:

```sh
kubectl logs -n selectel-infra deployment/selectel-operator-controller-manager -c manager -f
```

## Флаги

| Флаг | По умолчанию | Смысл |
| --- | --- | --- |
| `--credentials-namespace` | `selectel-infra` | Читаются только Secret из этого namespace. `Project.spec.credentialsSecretRef.namespace` должен с ним совпадать. |
| `--cdn-base-url` | `https://api.selectel.ru/cdn/v3` | Корень CDN API. |
| `--processing-interval` | `3m` | Базовое ожидание, пока Selectel применяет конфигурацию. Добавляются случайные 0–30 секунд. |
| `--leader-elect` | `false` у бинаря, `true` у установленного Deployment | Один активный менеджер. |
| `--health-probe-bind-address` | `:8081` | Liveness и readiness. |

Адрес Keystone задаётся на каждом Project: `spec.authURL`, по умолчанию `https://cloud.api.selcloud.ru/identity/v3`.

Смена `--credentials-namespace` требует того же значения в `config/manager/manager.yaml` (или kustomize-патча) и RoleBinding на чтение секретов в новом namespace. Role из поставки живёт в namespace установки.

## RBAC

ClusterRole менеджера может get, list и watch объекты `Project` и CDN `Resource`, обновлять их status и finalizer и писать Events. Правила на Secret в этой ClusterRole нет.

Доступ к секретам — namespaced Role `credentials-reader` с `get`, `list` и `watch` на Secret в `selectel-infra`. Менеджер делает list и watch Secret только в `--credentials-namespace`, поэтому этой Role достаточно. `Project`, который ссылается на Secret в namespace тенанта, получает `Ready=False`, причина `SecretNamespaceForbidden`, и секрет не читается.

Пользователям тенанта, которые создают CDN-ресурсы, нужно право на `resources` группы `cdn.selectel.tapclap.com` в своём namespace. Создание `Project` — действие cluster-admin: там выбираются Secret и список namespace.

## Запуск из исходников

Поставьте CRD и запустите менеджер в текущем контексте kubeconfig:

```sh
make install
make run
```

`make run` запускает `cmd/main.go`. У бинаря `--credentials-namespace` по умолчанию равен `selectel-infra`. Процесс читает Secret из кластера, поэтому этот namespace и секрет должны существовать.

## Образцы

В [`config/samples`](../../config/samples) лежат Secret, `Project` и CDN `Resource`. Пароль в секрете — `replace-me`. Применение образцов с настоящими учётными данными создаёт CDN-ресурс в Selectel.

```sh
kubectl apply -k config/samples
kubectl get projects
kubectl get resources -n media
```
