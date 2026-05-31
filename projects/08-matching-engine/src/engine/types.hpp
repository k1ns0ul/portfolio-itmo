#pragma once

#include <chrono>
#include <cstdint>
#include <string_view>

namespace me {

enum class Side : std::uint8_t { Buy, Sell };
enum class OrderType : std::uint8_t { Limit, Market };
enum class OrderStatus : std::uint8_t { New, PartiallyFilled, Filled, Cancelled };

using OrderId = std::uint64_t;
using Price = std::int64_t;
using Quantity = std::int64_t;
using InstrumentId = std::uint32_t;
using Timestamp = std::chrono::steady_clock::time_point;

struct TradeId {
    std::uint64_t value;
};

constexpr std::string_view to_string(Side s) noexcept {
    return s == Side::Buy ? "BUY" : "SELL";
}

constexpr std::string_view to_string(OrderType t) noexcept {
    return t == OrderType::Limit ? "LIMIT" : "MARKET";
}

constexpr std::string_view to_string(OrderStatus s) noexcept {
    switch (s) {
        case OrderStatus::New:
            return "NEW";
        case OrderStatus::PartiallyFilled:
            return "PARTIALLY_FILLED";
        case OrderStatus::Filled:
            return "FILLED";
        case OrderStatus::Cancelled:
            return "CANCELLED";
    }
    return "UNKNOWN";
}

}
