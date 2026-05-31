#include "matching_engine.hpp"

#include <mutex>

namespace me {

MatchingEngine::MatchingEngine(std::size_t recent_trades_capacity)
    : recent_capacity_(recent_trades_capacity == 0 ? 1 : recent_trades_capacity) {}

OrderBook& MatchingEngine::book_for(InstrumentId inst) {
    auto it = books_.find(inst);
    if (it == books_.end()) {
        it = books_.try_emplace(inst, inst).first;
    }
    return it->second;
}

MatchResult MatchingEngine::submit_order(InstrumentId inst, OrderType type, Side side, Price price, Quantity qty) {
    std::unique_lock lock(mutex_);
    OrderBook& book = book_for(inst);
    MatchResult result = book.add_order(type, side, price, qty);

    for (const auto& trade : result.trades) {
        recent_trades_.push_back(trade);
        if (recent_trades_.size() > recent_capacity_) {
            recent_trades_.pop_front();
        }
    }
    total_trades_ += result.trades.size();
    return result;
}

bool MatchingEngine::cancel_order(InstrumentId inst, OrderId id) {
    std::unique_lock lock(mutex_);
    auto it = books_.find(inst);
    if (it == books_.end()) {
        return false;
    }
    return it->second.cancel_order(id);
}

BookSnapshot MatchingEngine::get_snapshot(InstrumentId inst, int depth) const {
    std::shared_lock lock(mutex_);
    auto it = books_.find(inst);
    if (it == books_.end()) {
        return BookSnapshot{};
    }
    return it->second.snapshot(depth);
}

std::vector<Trade> MatchingEngine::get_recent_trades(InstrumentId inst, int limit) const {
    std::shared_lock lock(mutex_);
    std::vector<Trade> out;
    if (limit <= 0) {
        return out;
    }
    out.reserve(static_cast<std::size_t>(limit));
    for (auto it = recent_trades_.rbegin(); it != recent_trades_.rend(); ++it) {
        if (it->instrument != inst) {
            continue;
        }
        out.push_back(*it);
        if (static_cast<int>(out.size()) >= limit) {
            break;
        }
    }
    return out;
}

EngineStats MatchingEngine::get_stats() const {
    std::shared_lock lock(mutex_);
    EngineStats stats;
    stats.instruments = books_.size();
    for (const auto& [inst, book] : books_) {
        stats.orders += book.order_count();
    }
    stats.trades = total_trades_;
    return stats;
}

}
