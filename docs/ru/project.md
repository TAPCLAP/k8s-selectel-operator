# Project и учётные данные

[English](../project.md) — основная версия этой страницы.

`Project` — cluster-scoped объект (`selectel.tapclap.com/v1alpha1`). Он не создаёт проект в Selectel. Он называет Secret, из которого берётся project-scoped IAM-токен, и перечисляет namespace, которым разрешено создавать CDN-ресурсы с этим токеном.

Вызовы CDN API требуют этот токен в `X-Auth-Token`. Account-scoped токены и статические токены не используются.

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

`authURL` необязателен. Значение по умолчанию — `https://cloud.api.selcloud.ru/identity/v3`.

## Секрет

Secret должен лежать в namespace учётных данных оператора (`selectel-infra` или значение `--credentials-namespace`). `credentialsSecretRef.namespace` обязан совпадать с этим namespace. Ссылка на любой другой namespace отклоняется до чтения секрета.

| Ключ | Смысл |
| --- | --- |
| `accountId` | Номер аккаунта Selectel. В Keystone уходит как `domain.name`. |
| `username` | Имя пользователя Keystone. |
| `password` | Пароль Keystone. |
| `projectName` | Проект Selectel, на который выдаётся токен. |

Нужны все четыре ключа. Токен хранится в процессе менеджера. В status `Project` он не пишется и в логи не попадает.

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

Контроллер запрашивает `POST {authURL}/auth/tokens` с password identity и project scope. Срок берётся из `token.expires_at` или равен 24 часам, если поля нет. Ключ кэша — имя `Project`. Токен обновляется, когда до истечения остаётся меньше 30 минут, или когда содержимое Secret изменилось. CDN `401` сбрасывает кэш, и вызов повторяется один раз.

`Project` становится `Ready=True` с причиной `TokenIssued` после выпуска токена. Контроллер проверяет его снова примерно раз в 30 минут.

## Список namespace

`spec.allowedNamespaces` — список объектов, не строк.

| `name` | Эффект |
| --- | --- |
| Точное имя, например `media` | Этот namespace может пользоваться Project. |
| `*` | Namespace без своей записи могут пользоваться Project. |
| Пустой список или поле опущено | Ни один namespace не может пользоваться Project. |

Точная запись заменяет `*`. Сертификаты из `*` к точной записи не добавляются. Другие шаблоны и label selector не поддерживаются.

```yaml
allowedNamespaces:
  - name: "*"
    certificates:
      - id: 9
  - name: media
    certificates:
      - id: 1
```

Здесь `media` может привязать только сертификат `1`. Сертификат `9` доступен остальным namespace и недоступен `media`. Namespace с пустым `certificates` может создать CDN-ресурс и не может задать `spec.certificate`.

Удаление namespace из списка останавливает create и update его CDN-ресурсов. Уже созданные в Selectel ресурсы остаются как есть. Удаление самого custom resource всё равно выполняет deletion policy, чтобы finalizer мог завершиться. См. [deletion.md](deletion.md).

Изменение `allowedNamespaces`, включая списки сертификатов, ставит в очередь каждый CDN `Resource`, у которого `spec.projectRef.name` совпадает с этим Project.

## Список сертификатов

`certificates[].id` — `seq` сертификата в CDN API: то же целое, которое принимает поле `certificate` у CDN-ресурса. Форма объекта оставлена, чтобы позже рядом с `id` разрешать объект кластера.

Оператор не вызывает `GET /certificate`, чтобы проверить, что seq существует. Список — разрешение внутри кластера. CDN `Resource`, чей `spec.certificate.id` отсутствует в подходящей записи, получает `Ready=False`, причина `CertificateNotAllowed`, и create или update не отправляются.

Снятие id со списка не отвязывает сертификат, уже привязанный в Selectel. Следующее желаемое обновление, которое всё ещё задаёт этот id, отклоняется. Обновление без поля `certificate` — обычное изменение spec и уходит в API, если сам namespace по-прежнему разрешён.

## Персональные домены

`spec.allowedDomains` ограничивает `spec.names` у CDN-ресурсов этого Project. Пустой список или отсутствие поля разрешает любой персональный домен. Непустой список требует, чтобы каждое имя совпало хотя бы с одним шаблоном. Выданный домен `*.selcdn.net` в эту проверку не входит.

| Шаблон | Совпадение |
| --- | --- |
| `cdn.example.com` | Только этот домен. Регистр не учитывается. |
| `*.example.com` | Одна метка, например `cdn.example.com`. `a.b.example.com` не совпадает. `*` не переходит через точку, `?` — один символ внутри метки. |
| `**.example.com` | Любой префикс, включая точки, например `a.b.example.com`. |
| `/^cdn\..+\.example\.com$/` | Регулярное выражение Go на весь домен. Шаблон обёрнут в `/`. Регистр учитывается. |

Имя без совпадения ставит `Ready=False`, причина `DomainNotAllowed`, и create или update не отправляются. Шаблон, который не компилируется, ставит `DomainPatternInvalid`. Удаление custom resource всё равно выполняет deletion policy.

## Status

`kubectl get projects` показывает условие `Ready`.

| Причина | Ready | Смысл |
| --- | --- | --- |
| `TokenIssued` | True | Project-scoped токен выпущен. |
| `SecretNotFound` | False | Secret отсутствует. |
| `SecretNamespaceForbidden` | False | Namespace секрета — не namespace учётных данных оператора. |
| `SecretInvalid` | False | Нет обязательного ключа. |
| `AuthFailed` | False | Keystone отклонил запрос. |
| `ResourcesRemain` | False, пока объект удаляется | CDN `Resource` всё ещё ссылается на этот Project, finalizer остаётся. |

Ошибки аутентификации повторяются примерно раз в минуту.

## Finalizer

Имя finalizer — `selectel.tapclap.com/project`. Он остаётся, пока любой CDN `Resource` имеет `spec.projectRef.name`, равный этому Project, в том числе Resource, который сам удаляется. Так учётные данные доступны, пока CDN-finalizer может деактивировать ресурс или отпустить его. Когда последнего ссылающегося Resource не остаётся, finalizer Project снимается, и Project может исчезнуть.
