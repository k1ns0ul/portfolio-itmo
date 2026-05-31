# Distributed KV

Распределенное key-value хранилище на Go с консенсусом Raft. Кластер из трех (или более) нод
автоматически выбирает лидера, реплицирует записи на все ноды и переживает отказ лидера без потери
данных. Поддерживает два режима чтения: быстрое локальное и строго консистентное через лидера.

## Что это

Каждая нода — это один бинарь `kv-node`, который держит in-memory key-value таблицу, поверх которой
работает Raft: лог операций реплицируется на все ноды и применяется к конечному автомату (FSM) в одном
и том же порядке. Клиенты обращаются к любой ноде по HTTP; запись всегда проходит через лидера, чтение
может идти с любой ноды.

## Как работает

Запись (PUT/DELETE) сериализуется в команду и подается в Raft через лидера. Raft реплицирует запись на
большинство нод, после чего она считается зафиксированной (committed) и применяется к FSM. Если клиент
попал на follower, нода форвардит запрос на лидера через gRPC и возвращает его ответ.

Если лидер падает, оставшиеся ноды замечают отсутствие heartbeat и запускают выборы нового лидера —
это занимает порядка секунды. Зафиксированные данные не теряются, потому что Raft-лог персистентен и
реплицирован. Когда упавшая нода возвращается, она догоняет лидера по логу или из снапшота.

## Гарантии консистентности

- **stale read** (по умолчанию) — чтение из локальной FSM. Быстро, но значение может слегка отставать
  от лидера на время репликации.
- **consistent read** (`?consistent=true`) — перед чтением нода вызывает `VerifyLeader()`. Если нода не
  лидер, возвращается 503; если лидер — чтение линеаризуемо (видит все зафиксированные записи).

## API

| Метод  | Путь                                  | Назначение                                              |
| ------ | ------------------------------------- | ------------------------------------------------------- |
| PUT    | `/api/v1/kv/:key`                     | Записать значение (raw body). Форвардится на лидера     |
| GET    | `/api/v1/kv/:key?consistent=true`     | Прочитать значение (stale или consistent)               |
| DELETE | `/api/v1/kv/:key`                     | Удалить ключ. Форвардится на лидера                      |
| GET    | `/api/v1/kv?prefix=foo`               | Список ключей по префиксу (stale)                        |
| GET    | `/api/v1/cluster/status`              | Состояние ноды: роль, лидер, term, индексы, пиры         |
| POST   | `/api/v1/cluster/join`                | Добавить ноду `{id, addr}` (только лидер)                |
| POST   | `/api/v1/cluster/remove`              | Убрать ноду `{id}` (только лидер)                        |
| GET    | `/api/v1/cluster/peers`               | Список нод с пометкой лидера                              |
| GET    | `/api/v1/health`                      | Запущен ли Raft                                          |
| GET    | `/api/v1/ready`                       | Синхронизирована ли нода (applied > 0)                   |
| GET    | `/metrics`                            | Метрики в формате Prometheus                              |

Примеры:

```bash
curl -X PUT --data-binary "blue" http://localhost:8081/api/v1/kv/color
curl http://localhost:8082/api/v1/kv/color
curl "http://localhost:8081/api/v1/kv/color?consistent=true"
curl -X DELETE http://localhost:8081/api/v1/kv/color
curl "http://localhost:8081/api/v1/kv?prefix=col"
curl http://localhost:8081/api/v1/cluster/status
```

## Управление кластером

Ноды можно добавлять и удалять на лету, обращаясь к лидеру:

```bash
curl -X POST http://localhost:8081/api/v1/cluster/join \
  -H 'Content-Type: application/json' \
  -d '{"id":"node4","addr":"kv-node-4:7000"}'

curl -X POST http://localhost:8081/api/v1/cluster/remove \
  -H 'Content-Type: application/json' -d '{"id":"node4"}'
```

Новая нода запускается без `--bootstrap`, после чего лидер добавляет ее как voter и реплицирует на нее
весь лог.

## Persistence

- Raft-лог хранится в BoltDB (`raft-boltdb/v2`) в каталоге данных каждой ноды.
- Периодически создаются снапшоты состояния FSM (хранятся три последних), что обрезает лог.
- После перезапуска нода восстанавливает состояние из снапшота и доигрывает оставшийся лог, затем
  догоняет лидера.

## Метрики

На `/metrics` отдается текст в формате Prometheus:

- `dkv_keys_total` — число ключей в FSM.
- `dkv_raft_state{state}` — текущая роль ноды (leader/follower/candidate).
- `dkv_raft_term`, `dkv_raft_commit_index`, `dkv_raft_applied_index` — состояние Raft.
- `dkv_requests_total{method,status}` — счетчик запросов.
- `dkv_request_duration_seconds` — гистограмма времени обработки запросов.

## Запуск

```bash
docker compose up --build
```

Поднимается кластер из трех нод. HTTP API доступен на портах 8081, 8082, 8083; метрики на 9101-9103.
После выбора лидера можно слать запросы на любую ноду.

## Демонстрация отказоустойчивости

```bash
curl -X PUT --data-binary "v1" http://localhost:8081/api/v1/kv/demo
docker stop kv-node-1
curl http://localhost:8082/api/v1/cluster/status
curl http://localhost:8082/api/v1/kv/demo
curl -X PUT --data-binary "v2" http://localhost:8082/api/v1/kv/demo2
docker start kv-node-1
curl http://localhost:8081/api/v1/kv/demo2
```

После остановки `kv-node-1` оставшиеся две ноды сохраняют кворум, выбирают нового лидера и продолжают
обслуживать запросы. Когда нода возвращается, она догоняет кластер и снова видит все данные.

## Структура проекта

```
cmd/
  kv-node/main.go          точка входа: флаги, инициализация, graceful shutdown
internal/
  config/                  разбор флагов командной строки
  store/                   FSM, команды, снапшоты (raft.FSM)
  raft/                    обертка над hashicorp/raft, TCP transport
  grpc/                    ручной protowire, ForwardService для форварда на лидера
  api/                     Gin router, middleware, handlers, форвардинг
  metrics/                 коллектор и Prometheus-эндпоинт
  common/                  graceful shutdown, retry-хелперы
tests/                     unit-тесты FSM/forward и интеграционный тест кластера
Dockerfile                 multi-stage сборка, distroless
docker-compose.yml         кластер из трех нод с персистентными томами
```
