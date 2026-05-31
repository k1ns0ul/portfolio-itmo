#include "order.hpp"

#include <atomic>

namespace me {

namespace {
std::atomic<std::uint64_t> g_order_counter{1};
std::atomic<std::uint64_t> g_trade_counter{1};
}

Order::Order(OrderId id, InstrumentId instrument, Side side, OrderType type, Price price, Quantity quantity)
    : id(id),
      instrument(instrument),
      side(side),
      type(type),
      price(price),
      quantity(quantity),
      remaining(quantity),
      status(OrderStatus::New),
      created_at(std::chrono::steady_clock::now()) {}

OrderId next_order_id() noexcept {
    return g_order_counter.fetch_add(1, std::memory_order_relaxed);
}

std::uint64_t next_trade_id() noexcept {
    return g_trade_counter.fetch_add(1, std::memory_order_relaxed);
}

}
