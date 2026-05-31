# CFA Bonds

Бэкенд платформы для выпуска и обслуживания облигационных ЦФА — цифровых финансовых активов в форме
денежных требований по 259-ФЗ. Платформа закрывает полный жизненный цикл цифровой облигации: создание
выпуска, первичное размещение, вторичные сделки с расчетом накопленного купонного дохода, купонные
выплаты по расписанию и погашение по истечении срока. Каждая операция фиксируется в неизменяемом журнале
событий, что соответствует требованиям регулятора к учету операций с ЦФА.

## Что это

Эмитент создает выпуск облигаций с заданными номиналом, ставкой купона, частотой выплат и датой погашения.
Инвесторы покупают бумаги при первичном размещении, торгуют ими на вторичном рынке и получают купонный
доход. Система рассчитывает накопленный купонный доход (НКД) при каждой сделке, автоматически начисляет
купоны и возвращает номинал при погашении. Все расчеты ведутся в точной десятичной арифметике, а денежные
переводы между балансами инвесторов выполняются в транзакциях с уровнем изоляции Serializable.

## Бизнес-логика

Жизненный цикл выпуска описан конечным автоматом статусов:

```
draft ──> placement ──> active ──> matured
   └──────> cancelled <──┘
```

- **draft** — выпуск создан, сформирован график купонов, торги еще не идут.
- **placement** — идет первичное размещение: инвесторы выкупают бумаги, растет placed_quantity.
- **active** — выпуск полностью размещен, разрешены вторичные сделки и начисление купонов.
- **matured** — наступила дата погашения: инвесторам возвращен номинал, позиции обнулены.
- **cancelled** — выпуск отменен до размещения.

Вторичная сделка проходит через расчет (settlement): проверяются доступные бумаги у продавца и баланс у
покупателя, считается НКД, атомарно переводятся позиции и деньги, сделка фиксируется в журнале. Купонные
выплаты и погашения выполняются фоновыми воркерами по расписанию.

## Как устроено

Платформа состоит из пяти сервисов поверх общей PostgreSQL, Kafka и Redis.

- **api** — HTTP API на Gin: CRUD выпусков и эмитентов, размещение, сделки, портфели инвесторов, аналитика.
  Публикует событие `trade.submitted` в Kafka и не дожидается завершения расчета.
- **settlement-worker** — consumer Kafka. Обрабатывает каждую сделку: валидация, расчет НКД, перевод
  позиций и средств в одной транзакции Serializable, запись в журнал, публикация `trade.settled` или
  `trade.failed`. Поддерживает retry и dead letter queue.
- **coupon-worker** — по расписанию находит выпуски с наступившей датой купона и начисляет выплаты всем
  держателям. Каждый выпуск обрабатывается в отдельной транзакции, поэтому сбой одного не блокирует прочие.
- **maturity-worker** — по расписанию находит выпуски с наступившей датой погашения, возвращает номинал
  держателям, обнуляет позиции и закрывает выпуск.
- **metrics-exporter** — собирает бизнес-метрики из PostgreSQL и отдает их в формате Prometheus.

Поток событий через Kafka:

```
api ──trade.submitted──> settlement-worker ──trade.settled──> (downstream consumers)
                                            └─trade.failed──> DLQ
coupon-worker  ──coupon.paid──>
maturity-worker ──issue.matured──>
```

## Финансовая математика

**Накопленный купонный доход (НКД)** считается по конвенции ACT/365, стандартной для российского рынка:

```
НКД = Номинал * (Ставка / Частота) * (Дней с последнего купона / Дней в купонном периоде)
```

Дни считаются как фактическое число дней, деленное на 365; длина купонного периода равна 365 / частота.
НКД добавляется к цене сделки, формируя итоговую сумму расчета.

**Доходность к погашению (YTM)** вычисляется численно методом Ньютона-Рафсона из уравнения цены облигации:

```
Price = sum( C / (1 + y)^t ) + Nominal / (1 + y)^T
```

где `C` — купон за период, `y` — периодическая доходность, `T` — число оставшихся периодов. Итерации
сходятся обычно за десяток шагов; результат приводится к годовой доходности умножением на частоту купона.
Дополнительно доступна текущая доходность `CurrentYield = Ставка * Номинал / Цена`.

## Мониторинг

metrics-exporter публикует на `:9090/metrics` следующие показатели:

| Метрика                              | Тип       | Назначение                                    |
| ------------------------------------ | --------- | --------------------------------------------- |
| `cfa_issues_total{status}`           | gauge     | Число выпусков по статусам                     |
| `cfa_trades_total`                   | counter   | Всего рассчитанных сделок                       |
| `cfa_trades_volume_rub`              | counter   | Суммарный объем торгов в рублях                |
| `cfa_settlement_duration_seconds`    | histogram | Время обработки одной сделки                    |
| `cfa_settlement_outcomes_total`      | counter   | Исходы расчетов (settled / failed)             |
| `cfa_investors_total`                | gauge     | Число зарегистрированных инвесторов            |
| `cfa_coupons_paid_total`             | counter   | Число купонных выплат                           |
| `cfa_coupons_paid_amount_rub`        | counter   | Суммарный объем купонных выплат                 |
| `cfa_events_total{type}`             | counter   | События журнала по типам за 24 часа            |

Готовый Grafana-дашборд лежит в `deploy/grafana/dashboard.json`: объем торгов, частота сделок, латентность
расчета (p50/p95), активные выпуски, инвесторы, распределение событий и таблица исходов расчета. Алерты
настраиваются на латентность расчета выше 5 секунд и долю ошибок сделок выше 1%.

## Kubernetes

Helm-чарт в `deploy/helm`:

- `api-deployment.yaml` — Deployment API с HPA (min 2, max 8, цель 70% CPU), readiness `/api/v1/ready`,
  liveness `/api/v1/health`.
- `api-service.yaml` — ClusterIP и Ingress.
- `settlement-deployment.yaml` — Deployment расчетного воркера на 2 реплики для доступности.
- `coupon-cronjob.yaml` — CronJob `0 6 * * *`, restartPolicy Never, запуск с флагом `--once`.
- `maturity-cronjob.yaml` — CronJob `0 7 * * *`.
- `metrics-deployment.yaml` / `metrics-service.yaml` — экспортер метрик.
- `servicemonitor.yaml` — ServiceMonitor для Prometheus Operator (автоматический scrape).
- `networkpolicy.yaml` — settlement-worker принимает трафик только от Kafka, API — только через ingress.
- `configmap.yaml` — конфигурация и секреты (DSN, JWT, пароль Redis).

## Стек

Go 1.22, Gin, pgx v5 (PostgreSQL), IBM/sarama (Kafka), go-redis v9, shopspring/decimal для денежной
арифметики, golang-jwt для аутентификации, prometheus/client_golang для метрик. Везде structured logging
через slog, graceful shutdown и проброс context.

## Запуск

### docker-compose

```bash
docker compose up --build
```

Поднимаются PostgreSQL, Redis, Kafka (KRaft), api (8080), settlement-worker, metrics-exporter (9090),
Prometheus (9091) и Grafana (3000 с уже подключенным дашбордом). Купонный и погасительный воркеры запускаются
отдельно под профилем tools одним прогоном:

```bash
docker compose --profile tools run --rm coupon-worker
docker compose --profile tools run --rm maturity-worker
```

### Helm

```bash
helm install cfa deploy/helm --namespace cfa --create-namespace
```

## Примеры запросов

Создать эмитента:

```bash
curl -X POST http://localhost:8080/api/v1/issuers \
  -H 'Content-Type: application/json' \
  -d '{"name":"ООО Эмитент","inn":"7700000000","contact_email":"bonds@issuer.ru"}'
```

Создать выпуск (генерируется график купонов):

```bash
curl -X POST http://localhost:8080/api/v1/issues \
  -H 'Content-Type: application/json' \
  -d '{
    "issuer_id":"<ISSUER_UUID>",
    "name":"Облигация-2027",
    "isin":"RU000A0JX001",
    "nominal":"1000",
    "coupon_rate":"0.12",
    "coupon_frequency":2,
    "issue_date":"2026-01-01",
    "maturity_date":"2027-01-01",
    "total_quantity":10000
  }'
```

Перевести выпуск в размещение и разместить бумаги инвестору:

```bash
curl -X PUT http://localhost:8080/api/v1/issues/<ISSUE_UUID>/status \
  -H 'Content-Type: application/json' -d '{"status":"placement"}'

curl -X POST http://localhost:8080/api/v1/investors/<INVESTOR_UUID>/deposit \
  -H 'Content-Type: application/json' -d '{"amount":"10000000"}'

curl -X POST http://localhost:8080/api/v1/issues/<ISSUE_UUID>/place \
  -H 'Content-Type: application/json' \
  -d '{"investor_id":"<INVESTOR_UUID>","quantity":10000,"price":"1000"}'
```

Совершить вторичную сделку (обработается settlement-worker асинхронно):

```bash
curl -X POST http://localhost:8080/api/v1/trades \
  -H 'Content-Type: application/json' \
  -d '{
    "issue_id":"<ISSUE_UUID>",
    "seller_id":"<SELLER_UUID>",
    "buyer_id":"<BUYER_UUID>",
    "quantity":100,
    "price":"1010"
  }'
```

Посмотреть портфель инвестора (кешируется в Redis):

```bash
curl http://localhost:8080/api/v1/investors/<INVESTOR_UUID>/portfolio
```

Аналитика по выпуску с доходностью к погашению:

```bash
curl http://localhost:8080/api/v1/analytics/issues/<ISSUE_UUID>
```

## Структура проекта

```
cmd/
  api/                точка входа HTTP API
  settlement-worker/  consumer расчета сделок
  coupon-worker/      начисление купонов по расписанию
  maturity-worker/    погашение выпусков по расписанию
  metrics-exporter/   экспорт метрик в Prometheus
internal/
  config/             конфигурация из env
  models/             доменные модели (выпуск, инвестор, сделка, купон, событие)
  db/                 пул pgx, embed-мигратор, хелперы decimal/NUMERIC
  repo/               репозитории поверх PostgreSQL
  kafka/              async producer и consumer group с DLQ
  redis/              кеш портфелей и котировок, rate limiting
  settlement/         расчет сделок, формула НКД
  coupon/             сервис купонных выплат
  maturity/           сервис погашения
  metrics/            Prometheus-коллектор и экспортер
  api/                router, middleware, обработчики
  auth/               JWT и role-based middleware
  analytics/          YTM и текущая доходность
  common/             graceful shutdown, retry с backoff
migrations/           SQL-миграции (эмитенты, инвесторы, выпуски, купоны, позиции, сделки, журнал, витрины)
deploy/helm/          Helm-чарт: Deployments, CronJobs, HPA, Service, Ingress, ServiceMonitor, NetworkPolicy
deploy/grafana/       дашборд и provisioning
docker-compose.yml    полный локальный стек с Prometheus и Grafana
Dockerfile            multi-stage сборка, distroless, ARG TARGET
```
