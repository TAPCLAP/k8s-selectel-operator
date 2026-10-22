# Разработка

[English](../development.md) — основная версия этой страницы.

Модуль: `github.com/tapclap/k8s-selectel-operator`. Раскладка — kubebuilder multigroup. Один бинарь запускает все контроллеры. Следующий продукт Selectel — новая группа API и свой контроллер. Ему стоит переиспользовать `internal/selectel/iam`, а не копировать работу с токеном.

| Путь | Содержимое |
| --- | --- |
| `api/selectel/v1alpha1` | Типы `Project` |
| `api/cdn.selectel/v1alpha1` | Типы CDN `Resource` |
| `api/openapi/cdn.yaml` | OpenAPI-спека CDN Selectel |
| `internal/selectel/iam` | Клиент токена Keystone и кэш в процессе |
| `internal/selectel/access` | Списки namespace и сертификатов |
| `internal/selectel/cdn` | Обёртка клиента CDN |
| `internal/selectel/cdn/cdnapi` | Клиент, сгенерированный oapi-codegen. Не править вручную. |
| `internal/controller/selectel` | Контроллер `Project` |
| `internal/controller/cdn.selectel` | Контроллер CDN `Resource` |
| `config/` | CRD, RBAC, менеджер, образцы |
| `helm/selectel-operator` | Чарт контроллера. Включает те же CRD. |
| `helm/selectel-operator-crds` | Только CRD, для отдельной установки. |
| `.github/workflows/release.yml` | По тегу `v*` публикует образ, оба чарта и GitHub release. |

Типы CRD написаны вручную с теми же JSON-именами, что у CDN API. `controller-gen` не умеет DeepCopy для моделей oapi-codegen, поэтому custom resource и сгенерированный клиент — разные Go-типы. Контроллер переводит spec через `encoding/json`.

Сгенерированные файлы лежат в репозитории и вручную не правятся:

- `config/crd/bases/*.yaml`
- `config/rbac/role.yaml`
- `**/zz_generated.*.go`
- `internal/selectel/cdn/cdnapi/zz_generated.client.go`
- `PROJECT`

Комментарии `// +kubebuilder:scaffold:*` нужны CLI kubebuilder.

## Команды

```sh
make manifests   # CRD, ClusterRole и копии CRD в обоих Helm-чартах
make generate    # DeepCopy и клиент CDN
make test        # fmt, vet, httptest, envtest
make run         # менеджер в текущем kubeconfig
make lint-fix    # golangci-lint --fix
```

`make test` не ходит в Selectel. Тесты IAM и CDN используют `httptest`. Тесты контроллеров используют envtest. Сквозные тесты в `test/e2e` в этот запуск не входят. Им нужен отдельный кластер [Kind](https://kind.sigs.k8s.io/), не общий кластер разработки:

```sh
make test-e2e
```

После установки на живой проект проверьте, что ресурс создаётся, что появляется `status.cdnDomain`, что смена origin применяется, и свободно ли имя Selectel после `deletionPolicy: Deactivate`. Модульные тесты проверяют, что деактивация шлёт `{"active": false}`. Освобождает ли Selectel имя после этого, они показать не могут.

## Клиент CDN

`make generate` запускает oapi-codegen (`OAPI_CODEGEN_VERSION`, по умолчанию v2.5.1) с `api/openapi/oapi-codegen.yaml`. Обёртка в `internal/selectel/cdn` ставит `X-Auth-Token`, принимает 200/201/202, переводит 401 и 404 в sentinel-ошибки и учитывает `Retry-After` на 429. Список запрашивает и `active=true`, и `active=false`, потому что API по умолчанию отдаёт только активные ресурсы.

После правки `api/openapi/cdn.yaml` перегенерируйте клиент и запустите `make test`.

## Другой продукт Selectel

Добавьте группу API (`dns.selectel.tapclap.com` или похожую) через `kubebuilder create api`, контроллер в `internal/controller/<group>` и подключение в `cmd/main.go`. Передайте тот же `iam.Cache`, который уже делят контроллеры Project и CDN. URL продукта и логику ресурса держите в этой группе. Project, namespace учётных данных и кэш токена остаются общими.
