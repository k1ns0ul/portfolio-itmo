#include "order_book.hpp"

#include <algorithm>
#include <chrono>

namespace me {

OrderBook::OrderBook(InstrumentId instrument) : instrument_(instrument) {}

template <typename OppMap>
void OrderBook::match_against(Order& incoming, OppMap& opposite, MatchResult& result) {
    const bool incoming_is_buy = incoming.side == Side::Buy;

    while (incoming.remaining > 0 && !opposite.empty()) {
        auto best = opposite.begin();
        PriceLevel& level = best->second;

        if (incoming.type == OrderType::Limit) {
            const bool crosses = incoming_is_buy ? level.price() <= incoming.price : level.price() >= incoming.price;
            if (!crosses) {
                break;
            }
        }

        while (incoming.remaining > 0 && !level.empty()) {
            Order* resting = level.front();
            const Quantity exec = std::min(incoming.remaining, resting->remaining);
            const Price trade_price = resting->price;

            incoming.remaining -= exec;
            resting->remaining -= exec;
            level.reduce(exec);

            Trade trade;
            trade.id = TradeId{next_trade_id()};
            trade.instrument = instrument_;
            trade.price = trade_price;
            trade.quantity = exec;
            trade.executed_at = std::chrono::steady_clock::now();
            if (incoming_is_buy) {
                trade.buy_order_id = incoming.id;
                trade.sell_order_id = resting->id;
            } else {
                trade.buy_order_id = resting->id;
                trade.sell_order_id = incoming.id;
            }
            result.trades.push_back(trade);

            if (resting->remaining == 0) {
                const OrderId resting_id = resting->id;
                resting->status = OrderStatus::Filled;
                level.pop_front();
                orders_.erase(resting_id);
            }
        }

        if (level.empty()) {
            opposite.erase(best);
        }
    }
}

void OrderBook::rest_order(Order& order) {
    if (order.side == Side::Buy) {
        auto it = bids_.try_emplace(order.price, order.price).first;
        it->second.add(&order);
    } else {
        auto it = asks_.try_emplace(order.price, order.price).first;
        it->second.add(&order);
    }
}

MatchResult OrderBook::add_order(OrderType type, Side side, Price price, Quantity qty) {
    const OrderId id = next_order_id();
    auto [it, inserted] = orders_.try_emplace(id, id, instrument_, side, type, price, qty);
    Order& order = it->second;

    MatchResult result;
    result.order_id = id;

    if (side == Side::Buy) {
        match_against(order, asks_, result);
    } else {
        match_against(order, bids_, result);
    }

    OrderStatus final_status;
    if (order.remaining == 0) {
        final_status = OrderStatus::Filled;
        order.status = final_status;
        orders_.erase(id);
    } else if (type == OrderType::Market) {
        final_status = result.trades.empty() ? OrderStatus::Cancelled : OrderStatus::PartiallyFilled;
        order.status = final_status;
        orders_.erase(id);
    } else {
        final_status = result.trades.empty() ? OrderStatus::New : OrderStatus::PartiallyFilled;
        order.status = final_status;
        rest_order(order);
    }

    result.status = final_status;
    return result;
}

bool OrderBook::cancel_order(OrderId id) {
    auto it = orders_.find(id);
    if (it == orders_.end()) {
        return false;
    }
    const Order& order = it->second;
    if (order.side == Side::Buy) {
        auto level = bids_.find(order.price);
        if (level != bids_.end()) {
            level->second.remove(id);
            if (level->second.empty()) {
                bids_.erase(level);
            }
        }
    } else {
        auto level = asks_.find(order.price);
        if (level != asks_.end()) {
            level->second.remove(id);
            if (level->second.empty()) {
                asks_.erase(level);
            }
        }
    }
    orders_.erase(it);
    return true;
}

BookSnapshot OrderBook::snapshot(int depth) const {
    BookSnapshot snap;
    int taken = 0;
    for (const auto& [price, level] : bids_) {
        if (taken++ >= depth) {
            break;
        }
        snap.bids.push_back({price, level.total_quantity(), level.order_count()});
    }
    taken = 0;
    for (const auto& [price, level] : asks_) {
        if (taken++ >= depth) {
            break;
        }
        snap.asks.push_back({price, level.total_quantity(), level.order_count()});
    }
    return snap;
}

BookStats OrderBook::stats() const {
    BookStats st;
    st.order_count = orders_.size();
    if (!bids_.empty()) {
        st.best_bid = bids_.begin()->first;
    }
    if (!asks_.empty()) {
        st.best_ask = asks_.begin()->first;
    }
    if (!bids_.empty() && !asks_.empty()) {
        st.spread = st.best_ask - st.best_bid;
    }
    for (const auto& [price, level] : bids_) {
        st.total_bid_quantity += level.total_quantity();
    }
    for (const auto& [price, level] : asks_) {
        st.total_ask_quantity += level.total_quantity();
    }
    return st;
}

}
