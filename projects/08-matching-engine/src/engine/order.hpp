#pragma once

#include "types.hpp"

namespace me {

struct Order {
    OrderId id;
    InstrumentId instrument;
    Side side;
    OrderType type;
    Price price;
    Quantity quantity;
    Quantity remaining;
    OrderStatus status;
    Timestamp created_at;

    Order(OrderId id, InstrumentId instrument, Side side, OrderType type, Price price, Quantity quantity);

    Order(const Order&) = delete;
    Order& operator=(const Order&) = delete;
    Order(Order&&) = default;
    Order& operator=(Order&&) = default;
    ~Order() = default;
};

OrderId next_order_id() noexcept;
std::uint64_t next_trade_id() noexcept;

}
