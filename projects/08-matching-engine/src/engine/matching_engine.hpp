#pragma once

#include <cstddef>
#include <cstdint>
#include <deque>
#include <shared_mutex>
#include <unordered_map>
#include <vector>

#include "order_book.hpp"
#include "trade.hpp"
#include "types.hpp"

namespace me {

struct EngineStats {
    std::size_t instruments = 0;
    std::size_t orders = 0;
    std::uint64_t trades = 0;
};

class MatchingEngine {
public:
    explicit MatchingEngine(std::size_t recent_trades_capacity = 10000);

    MatchResult submit_order(InstrumentId inst, OrderType type, Side side, Price price, Quantity qty);
    bool cancel_order(InstrumentId inst, OrderId id);

    BookSnapshot get_snapshot(InstrumentId inst, int depth = 10) const;
    std::vector<Trade> get_recent_trades(InstrumentId inst, int limit = 50) const;
    EngineStats get_stats() const;

private:
    OrderBook& book_for(InstrumentId inst);

    mutable std::shared_mutex mutex_;
    std::unordered_map<InstrumentId, OrderBook> books_;
    std::deque<Trade> recent_trades_;
    std::size_t recent_capacity_;
    std::uint64_t total_trades_ = 0;
};

}
