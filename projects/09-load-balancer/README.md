# Load Balancer

L4 (транспортного уровня) TCP-балансировщик нагрузки на C++20. Принимает входящие TCP-соединения на
фронтенд-порту и распределяет их по набору бэкендов согласно выбранной стратегии. Работает на epoll
event loop с неблокирующим вводом-выводом, проверяет живость бэкендов, отдает метрики в Prometheus и
умеет перечитывать конфигурацию без перезапуска.

## Алгоритмы балансировки

- **round_robin** — соединения раздаются бэкендам по кругу, поровну между живыми.
- **least_connections** — новое соединение уходит бэкенду с наименьшим числом активных соединений.
- **weighted** — взвешенный round-robin: бэкенд с весом 3 получает втрое больше соединений, чем с весом 1.
- **ip_hash** — детерминированный выбор бэкенда по хешу IP клиента, обеспечивает sticky sessions.

## Архитектура

Ядро — однопоточный epoll event loop в `ProxyEngine`. Listen-сокет, клиентские и бэкенд-сокеты живут
в одном epoll; весь ввод-вывод неблокирующий. На каждое принятое соединение выбирается бэкенд через
стратегию, открывается неблокирующий connect, создается `ProxyConnection` с двумя кольцевыми буферами
(клиент -> бэкенд и бэкенд -> клиент). Данные перекачиваются по событиям EPOLLIN/EPOLLOUT; при
EPOLLHUP/EPOLLERR оба конца закрываются.

Все системные ресурсы обернуты в RAII-классы (`Socket`, `Epoll`), что исключает утечки дескрипторов;
типы move-only там, где владение единоличное. Health checker и Prometheus-экспортер работают в отдельных
потоках, а доступ к пулу бэкендов защищен `std::shared_mutex` (много читателей, один писатель).

Библиотеки разнесены по слоям: `lb_core` (конфиг, логгер, сигналы), `lb_net` (сокеты, адреса, epoll,
буферы), `lb_balancer` (бэкенды, стратегии, health checks), `lb_metrics`, `lb_proxy`, `lb_reload`.

## Health checks

Отдельный поток периодически (по умолчанию каждые 5 секунд) проверяет каждый бэкенд. Два режима:

- **tcp** — неблокирующий connect с таймаутом; успешное соединение означает, что бэкенд жив.
- **http** — после connect отправляется `GET /health HTTP/1.0`, ответ должен начинаться с `HTTP/1.x` и
  содержать код 200.

После N подряд неудач (по умолчанию 3) бэкенд помечается down и выводится из ротации. Первая же удачная
проверка возвращает его обратно. Смена состояния логируется.

## Hot reload

По сигналу SIGHUP балансировщик перечитывает файл конфигурации и применяет разницу под write-локом пула:

- новые бэкенды добавляются в ротацию;
- удаленные переводятся в режим draining (новые соединения на них не идут) и удаляются, когда активные
  соединения завершатся;
- изменение весов применяется на лету;
- смена стратегии или порта логируется (порт требует перезапуска).

Так конфигурацию бэкендов можно менять без рестарта и без разрыва текущих соединений.

## Мониторинг

Prometheus-метрики отдаются на отдельном порту (`:9100/metrics`):

- `lb_connections_total{backend}` — принятые соединения по бэкендам.
- `lb_connections_active` — текущие активные соединения.
- `lb_bytes_total{direction,backend}` — объем трафика по направлению и бэкенду.
- `lb_backend_healthy{backend}` — состояние бэкенда (0 или 1).
- `lb_backend_response_time_microseconds{backend}` — время ответа последней health-проверки.
- `lb_connection_duration_seconds_bucket{le}` — гистограмма длительности соединений.
- `lb_health_checks_total{backend,result}` — счетчики health-проверок по результату.

## Стек

C++20, CMake (>= 3.20). Стандартная библиотека и POSIX (epoll, sockets, splice). Тесты на GoogleTest,
подтягиваются через FetchContent. Внешних рантайм-зависимостей нет.

## Сборка и запуск

```bash
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build --parallel
ctest --test-dir build --output-on-failure
./build/src/server/load_balancer --config config/example.conf
./build/src/benchmark/lb_benchmark
```

Сборка серверного бинаря рассчитана на Linux (epoll). Через docker-compose поднимаются три echo-бэкенда
и балансировщик:

```bash
docker compose up --build
```

После старта запросы на `localhost:8080` распределяются по бэкендам, метрики доступны на `localhost:9100/metrics`.

## Конфигурация

Конфиг в простом отступном формате (без внешних YAML-библиотек):

```
listen: 0.0.0.0:8080          # адрес фронтенда
strategy: round_robin         # round_robin | least_connections | weighted | ip_hash
health_check:
  interval: 5s                # период проверки
  timeout: 2s                 # таймаут одной проверки
  type: tcp                   # tcp | http
  threshold: 3                # неудач подряд до пометки down
metrics:
  enabled: true
  port: 9100
backends:
  - address: 127.0.0.1:3001
    weight: 3
  - address: 127.0.0.1:3002
    weight: 1
  - address: 127.0.0.1:3003
    weight: 1
drain_timeout: 30s            # сколько ждать завершения соединений при остановке
```

## Бенчмарки

`lb_benchmark` поднимает три встроенных echo-сервера, запускает балансировщик программно и генерирует
поток конкурентных соединений, каждое из которых отправляет несколько сообщений. Замеряются:

- throughput (соединений в секунду, МБ в секунду);
- latency запроса (p50/p95/p99);
- сравнение стратегий round_robin / least_connections / weighted.

Результаты выводятся таблицей в stdout.

## Структура проекта

```
CMakeLists.txt              корневой CMake (BUILD_TESTS, BUILD_BENCHMARK)
src/
  core/                    конфиг, structured logger, обработка сигналов
    types.hpp config.* logger.* signal_handler.*
  net/                     RAII-сокеты, адреса, epoll, кольцевой буфер
    socket.* address.* epoll.* buffer.*
  balancer/                бэкенды, пул, стратегии, health checker
    backend.* backend_pool.* strategy.hpp round_robin.* least_connections.*
    weighted.* ip_hash.* health_checker.*
  proxy/                   проксирование соединений, event loop
    connection.* proxy_engine.*
  metrics/                 сбор метрик и Prometheus-экспортер
    collector.* prometheus.*
  reload/                  перечитывание конфига по SIGHUP
    config_watcher.*
  server/                  бинарь load_balancer
    main.cpp
  benchmark/               бинарь lb_benchmark
    main.cpp
tests/                     юнит- и интеграционные тесты (GoogleTest)
config/example.conf        пример конфигурации
Dockerfile                 multi-stage сборка
docker-compose.yml         балансировщик и три echo-бэкенда
```
