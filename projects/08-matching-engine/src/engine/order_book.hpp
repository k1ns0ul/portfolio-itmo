#pragma once

#include <cstddef>
#include <functional>
#include <map>
#include <unordered_map>
#include <vector>

#include "order.hpp"
#include "price_level.hpp"
#include "trade.hpp"
#include "types.hpp"

namespace me {

struct MatchResult {
    OrderId order_id;
    OrderStatus status;
    std::vector<Trade> trades;
};

struct BookSnapshot {
    struct Level {
        Price price;
        Quantity quantity;
        std::size_t order_count;
    };
    std::vector<Level> bids;
    std::vector<Level> asks;
};

struct BookStats {
    Price best_bid = 0;
    Price best_ask = 0;
    Price spread = 0;
    Quantity total_bid_quantity = 0;
    Quantity total_ask_quantity = 0;
    std::size_t order_count = 0;
};

class OrderBook {
public:
    explicit OrderBook(InstrumentId instrument);

    OrderBook(const OrderBook&) = delete;
    OrderBook& operator=(const OrderBook&) = delete;
    OrderBook(OrderBook&&) = default;
    OrderBook& operator=(OrderBook&&) = default;
    ~OrderBook() = default;

    MatchResult add_order(OrderType type, Side side, Price price, Quantity qty);
    bool cancel_order(OrderId id);
    BookSnapshot snapshot(int depth = 10) const;
    BookStats stats() const;

    InstrumentId instrument() const noexcept { return instrument_; }
    std::size_t order_count() const noexcept { return orders_.size(); }

private:
    template <typename OppMap>
    void match_against(Order& incoming, OppMap& opposite, MatchResult& result);
    void rest_order(Order& order);

    InstrumentId instrument_;
    std::map<Price, PriceLevel, std::greater<>> bids_;
    std::map<Price, PriceLevel, std::less<>> asks_;
    std::unordered_map<OrderId, Order> orders_;
};

}
