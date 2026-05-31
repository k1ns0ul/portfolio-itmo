# MPC Cluster

Платформа оркестрации защищенных многосторонних вычислений (Multi-Party Computation) поверх Kubernetes.
Несколько участников совместно вычисляют функцию над своими приватными данными так, что ни одна сторона
не видит входы остальных. Платформа разбивает секреты на криптографические доли, распределяет их по нодам
и собирает итоговый результат, не раскрывая исходные значения.

## Что это

Система решает задачу конфиденциальных вычислений: стороны хотят узнать общий результат (сумму, среднее,
максимум, результат сравнения), но не готовы раскрывать собственные числа. Координатор шарит входы на доли,
поднимает воркеры-ноды, раздает им доли и Beaver triples, после чего ноды считают результат через протоколы
secret sharing и собирают финальное значение обратно у координатора.

## Как устроено

Два бинаря:

- **coordinator** — управляющий сервис. REST API на Gin для пользователей и gRPC-сервер для нод. Хранит
  состояние сессий в PostgreSQL, использует Redis для distributed locks и pub/sub-координации. В Kubernetes
  разворачивается как Deployment.
- **node** — MPC-воркер. Регистрируется у координатора, забирает задачу, общается с пирами напрямую через
  peer-to-peer gRPC и отправляет результирующую долю обратно. В Kubernetes создается как Job на каждую сторону.

Жизненный цикл сессии: `created` → `nodes_ready` → `computing` → `completed` (или `failed`). Координатор
ждет регистрации всех нод, затем рассылает доли, дожидается результатов и рекомбинирует их.

```
client ──REST──> coordinator ──gRPC──> node-0
                     │                    │ peer-to-peer gRPC
                     ├──gRPC──> node-1 <──┤
                     └──gRPC──> node-2 <──┘
              PostgreSQL + Redis
```

## Операции

| Операция     | Тип          | Как считается                                                                 |
| ------------ | ------------ | ----------------------------------------------------------------------------- |
| Сумма        | `sum`        | Каждая нода складывает свои доли локально, координатор рекомбинирует сумму     |
| Среднее      | `average`    | Та же сумма долей, координатор делит результат на число сторон                 |
| Сравнение    | `comparison` | Открытие разности под защитой долей, результат — доля бита `x > y`             |
| Максимум     | `max`        | Цепочка сравнений и защищенных умножений (Beaver) для выбора большего значения |

Сложение и скалярное умножение выполняются локально без обмена. Умножение долей требует одного раунда
коммуникации и одного Beaver triple.

## Криптографические примитивы

Низкоуровневая криптография взята из готовых библиотек, протоколы построены поверх них.

- **Конечное поле GF(p)** — арифметика поверх `math/big`, модуль по умолчанию простое число порядка 2^127
  (`2^127 - 1`). Операции `Add`, `Sub`, `Mul`, `Inv`, `Neg`, `Rand` приведены по модулю p, случайность из `crypto/rand`.
- **Additive secret sharing** — секрет делится на n долей: n-1 случайных элементов поля и замыкающая доля
  `secret - sum`. Рекомбинация — сумма долей по модулю p.
- **Shamir secret sharing** — обертка над `github.com/hashicorp/vault/shamir` (`ShamirSplit` / `ShamirCombine`)
  для пороговых сценариев.
- **Beaver triples** — тройки `(a, b, c)`, где `c = a * b`, генерируются и шарятся координатором. Защищенное
  умножение раскрывает только маскированные разности `d = x - a` и `e = y - b`, восстанавливая `x * y` из долей.

## Kubernetes

Координатор сам создает и удаляет Job-ы нод через client-go. Helm-чарт в `deploy/helm`:

- `coordinator-deployment.yaml` — Deployment с readiness/liveness пробами на `/api/v1/health`, портами 8080 и 9090.
- `coordinator-service.yaml` — ClusterIP для HTTP и gRPC.
- `coordinator-configmap.yaml` — конфигурация окружения.
- `rbac.yaml` — ServiceAccount и Role с правами create/delete/list/watch на Jobs и Pods.
- `networkpolicy.yaml` — поды `mpc-node` изолированы: трафик разрешен только между нодами и координатором (плюс DNS).

## Стек

Go 1.22, Gin, google.golang.org/grpc (с ручным protowire-кодеком, без protoc), pgx v5, go-redis v9,
client-go, hashicorp/vault/shamir, slog. Graceful shutdown и context propagation по всем сервисам.

## Запуск

### docker-compose

```bash
docker compose up --build
```

Поднимаются PostgreSQL, Redis, координатор (8080 / 9090) и три ноды, заранее настроенные на сессию `demo`.
Ноды повторяют попытку регистрации, пока сессия не создана, поэтому порядок запуска не важен.

### Helm

```bash
helm install mpc deploy/helm --namespace mpc --create-namespace
```

В Kubernetes координатор сам поднимает Job-ы нод при создании сессии, поэтому статические ноды не нужны.

## Примеры запросов

Создать сессию на суммирование трех чисел (в docker-compose используем фиксированный id `demo`):

```bash
curl -X POST http://localhost:8080/api/v1/sessions \
  -H 'Content-Type: application/json' \
  -d '{"id":"demo","type":"sum","party_count":3}'
```

Запустить вычисление:

```bash
curl -X POST http://localhost:8080/api/v1/sessions/demo/execute \
  -H 'Content-Type: application/json' \
  -d '{"inputs":{"0":"123","1":"456","2":"789"}}'
```

Ответ:

```json
{"session_id":"demo","result":"1368"}
```

Сравнение двух значений:

```bash
curl -X POST http://localhost:8080/api/v1/sessions \
  -H 'Content-Type: application/json' \
  -d '{"id":"cmp","type":"comparison","party_count":2}'

curl -X POST http://localhost:8080/api/v1/sessions/cmp/execute \
  -H 'Content-Type: application/json' \
  -d '{"inputs":{"0":"500","1":"200"}}'
```

Прочие эндпоинты:

```bash
curl http://localhost:8080/api/v1/sessions/demo
curl "http://localhost:8080/api/v1/sessions?limit=20&offset=0"
curl http://localhost:8080/api/v1/sessions/demo/rounds
curl -X DELETE http://localhost:8080/api/v1/sessions/demo
curl http://localhost:8080/api/v1/health
```

## Структура проекта

```
cmd/
  coordinator/        точка входа координатора (HTTP + gRPC, graceful shutdown)
  node/               точка входа ноды (peer-сервер, регистрация, выполнение задачи)
internal/
  config/             загрузка конфигурации из env
  protocol/           поле GF(p), secret sharing, Beaver triples, MPC-операции
  session/            модель, PostgreSQL-стор, менеджер сессий
  coordinator/        gRPC-сервер для нод, Gin-обработчики, router
  node/               воркер, peer-сеть, peer-to-peer gRPC-сервер
  grpc/               ручной protowire marshal, кодек, клиенты к координатору и пирам
  k8s/                обертка client-go, формирование Job manifest
  redis/              клиент, distributed lock, pub/sub
  db/                 пул pgx, embed-мигратор
  common/             graceful shutdown, retry с backoff
migrations/           SQL-миграции (sessions, rounds, session_nodes)
deploy/helm/          Helm-чарт: Deployment, Service, ConfigMap, RBAC, NetworkPolicy
docker-compose.yml    локальный запуск всего стека
Dockerfile            multi-stage сборка, distroless, ARG TARGET
```
