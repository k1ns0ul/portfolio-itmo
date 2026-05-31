# Matching Engine

Ядро биржевого движка матчинга ордеров на C++20. Сводит заявки покупателей и продавцов по принципу
price-time priority с поддержкой limit и market ордеров. Движок оформлен как библиотека, поверх которой
работает TCP-сервер на epoll, а отдельный бенчмарк замеряет throughput и latency операций.

## Как работает матчинг

Каждый инструмент имеет свой стакан (order book) из двух сторон: биды (заявки на покупку) и аски (заявки
на продажу). Приоритет исполнения — сначала по цене, затем по времени поступления (price-time priority).

- **Buy limit** матчится с асками, у которых цена `<= ` цены заявки, начиная с лучшего (самого низкого) аска.
- **Sell limit** матчится с бидами, у которых цена `>= ` цены заявки, начиная с лучшего (самого высокого) бида.
- **Market** берет лучшую доступную цену с противоположной стороны независимо от уровня.
- На каждом ценовом уровне заявки стоят в FIFO-очереди: первым исполняется тот, кто пришел раньше.

Сделка исполняется по цене пассивной (стоящей в стакане) заявки. Частично исполненная limit-заявка
дослеживает остаток в стакане; market-заявка без ликвидности отменяется (остаток не сохраняется).

## Архитектура

Три компонента:

1. **matching_engine** — статическая библиотека: структуры ордера, ценовой уровень, стакан, движок над
   несколькими стаканами с reader-writer блокировкой (`std::shared_mutex`).
2. **matching_server** — TCP-сервер на epoll (Linux). Неблокирующий IO, по сессии на соединение, graceful
   shutdown через self-pipe. Принимает команды по простому текстовому протоколу.
3. **matching_benchmark** — замеры производительности без внешних зависимостей.

Движок ничего не знает о сети: сервер — это тонкий слой, который парсит протокол и вызывает методы движка.

## Протокол

Одна команда — одна строка, завершенная `\n`.

Входящие:

```
ORDER <instrument_id> <BUY|SELL> <LIMIT|MARKET> <price> <quantity>
CANCEL <instrument_id> <order_id>
BOOK <instrument_id> [depth]
TRADES <instrument_id> [limit]
STATS
```

Ответы:

```
OK ORDER <order_id> <status> [TRADES <trade_id>:<price>:<qty>, ...]
OK CANCEL <order_id>
OK BOOK <instrument_id> BIDS <price>:<qty>,... ASKS <price>:<qty>,...
OK TRADES <trade_id>:<buy_id>:<sell_id>:<price>:<qty>, ...
OK STATS instruments:<n> orders:<n> trades:<n>
ERR <message>
```

Цены и количества — целые числа. Цена выражена в минимальных единицах (копейки/центы), чтобы не работать
с плавающей точкой. Для market-ордера поле цены игнорируется, но должно присутствовать в команде.

## Структуры данных

- `std::map<Price, PriceLevel, std::greater<>>` для бидов и `std::less<>` для асков — упорядоченные уровни
  цен, доступ к лучшей цене и вставка за `O(log N)`.
- `std::deque<Order*>` на каждом ценовом уровне — FIFO-очередь, обеспечивающая time priority.
- `std::unordered_map<OrderId, Order>` — владение живыми ордерами и поиск по id за `O(1)` (для отмены).
  Указатели на элементы `unordered_map` стабильны, поэтому уровни цен безопасно хранят `Order*`.

Цены — `int64_t`, никаких `float`. Идентификаторы ордеров и сделок выдаются атомарными счетчиками.

## Производительность

`matching_benchmark` прогоняет warm-up (1000 операций), затем измерение (100000 операций) и печатает:

- **insertion throughput** — вставка ордеров в пустой стакан, ops/sec.
- **matching throughput** — поток встречных (crossing) ордеров, матчей/sec.
- **cancel throughput** — массовая отмена ранее вставленных ордеров.
- **mixed workload** — 60% insert, 20% market, 10% cancel, 10% snapshot.
- **latency** — поэлементный замер `add_order` с перцентилями p50/p95/p99/p999 в наносекундах.

Данные генерируются случайно: 10 инструментов, цены по нормальному распределению вокруг 10000 (±500),
количества равномерно 1..100.

## Сборка и запуск

### CMake

```bash
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build --parallel
ctest --test-dir build --output-on-failure
./build/src/benchmark/matching_benchmark
```

TCP-сервер использует epoll и собирается на Linux (на других платформах библиотека, тесты и бенчмарк
собираются, а бинарь сервера пропускается). Запуск сервера:

```bash
./build/src/server/matching_server --port 9999
```

### Docker

```bash
docker compose up --build
```

## Примеры использования

Подключение через `nc` (или `telnet`):

```bash
nc localhost 9999
```

Отправить limit-ордера и посмотреть стакан:

```
ORDER 1 SELL LIMIT 10100 50
OK ORDER 1 NEW
ORDER 1 BUY LIMIT 10000 30
OK ORDER 2 NEW
BOOK 1
OK BOOK 1 BIDS 10000:30 ASKS 10100:50
```

Свести встречную заявку (получаем сделку):

```
ORDER 1 BUY LIMIT 10100 20
OK ORDER 3 FILLED TRADES 1:10100:20
TRADES 1
OK TRADES 1:3:1:10100:20
STATS
OK STATS instruments:1 orders:2 trades:1
```

Market-ордер и отмена:

```
ORDER 1 SELL MARKET 0 10
OK ORDER 4 FILLED TRADES 2:10000:10
CANCEL 1 1
OK CANCEL 1
```

## Структура проекта

```
CMakeLists.txt              корневой CMake (опции BUILD_TESTS, BUILD_BENCHMARK)
src/
  engine/                  библиотека matching_engine
    types.hpp              базовые типы, алиасы, enums
    order.hpp/.cpp         структура ордера, генераторы id
    trade.hpp             результат матчинга
    price_level.hpp/.cpp   ценовой уровень с FIFO-очередью
    order_book.hpp/.cpp    стакан одного инструмента, алгоритм матчинга
    matching_engine.hpp/.cpp  движок над несколькими стаканами
  server/                  matching_server
    protocol.hpp/.cpp      разбор и формирование сообщений
    session.hpp/.cpp       клиентская сессия, буферизация
    tcp_server.hpp/.cpp    epoll event loop
    main.cpp              точка входа, разбор аргументов, сигналы
  benchmark/
    main.cpp              набор бенчмарков
tests/                     юнит-тесты на GoogleTest (FetchContent)
Dockerfile                 multi-stage сборка
docker-compose.yml         запуск сервера на порту 9999
```
