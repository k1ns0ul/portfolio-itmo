#include <gtest/gtest.h>

#include <thread>
#include <vector>

#include "matching_engine.hpp"

using namespace me;

TEST(MatchingEngine, MultiInstrumentIsolation) {
    MatchingEngine engine;
    engine.submit_order(1, OrderType::Limit, Side::Sell, 100, 10);
    auto result = engine.submit_order(2, OrderType::Limit, Side::Buy, 100, 10);

    EXPECT_TRUE(result.trades.empty());
    EXPECT_EQ(result.status, OrderStatus::New);

    auto snap1 = engine.get_snapshot(1);
    auto snap2 = engine.get_snapshot(2);
    EXPECT_EQ(snap1.asks.size(), 1u);
    EXPECT_EQ(snap2.bids.size(), 1u);
}

TEST(MatchingEngine, CrossInstrumentMatchesWhenSame) {
    MatchingEngine engine;
    engine.submit_order(7, OrderType::Limit, Side::Sell, 100, 10);
    auto result = engine.submit_order(7, OrderType::Limit, Side::Buy, 100, 10);
    EXPECT_EQ(result.status, OrderStatus::Filled);
    ASSERT_EQ(result.trades.size(), 1u);
    EXPECT_EQ(result.trades[0].instrument, 7u);
}

TEST(MatchingEngine, ConcurrentSubmitNoCrash) {
    MatchingEngine engine;
    constexpr int kThreads = 8;
    constexpr int kPerThread = 5000;

    std::vector<std::thread> workers;
    for (int t = 0; t < kThreads; ++t) {
        workers.emplace_back([&engine, t]() {
            const Side side = (t % 2 == 0) ? Side::Buy : Side::Sell;
            for (int i = 0; i < kPerThread; ++i) {
                const InstrumentId inst = static_cast<InstrumentId>(i % 4);
                engine.submit_order(inst, OrderType::Limit, side, 10000 + (i % 100), 1 + (i % 10));
            }
        });
    }
    for (auto& w : workers) {
        w.join();
    }

    auto stats = engine.get_stats();
    EXPECT_GT(stats.instruments, 0u);
}

TEST(MatchingEngine, StatsAfterOperations) {
    MatchingEngine engine;
    engine.submit_order(1, OrderType::Limit, Side::Buy, 100, 10);
    engine.submit_order(1, OrderType::Limit, Side::Sell, 100, 4);
    engine.submit_order(2, OrderType::Limit, Side::Buy, 50, 5);

    auto stats = engine.get_stats();
    EXPECT_EQ(stats.instruments, 2u);
    EXPECT_EQ(stats.trades, 1u);
    EXPECT_GE(stats.orders, 2u);

    auto trades = engine.get_recent_trades(1);
    ASSERT_EQ(trades.size(), 1u);
    EXPECT_EQ(trades[0].quantity, 4);
}

TEST(MatchingEngine, RecentTradesFilteredByInstrument) {
    MatchingEngine engine;
    engine.submit_order(1, OrderType::Limit, Side::Sell, 100, 5);
    engine.submit_order(1, OrderType::Limit, Side::Buy, 100, 5);
    engine.submit_order(2, OrderType::Limit, Side::Sell, 200, 5);
    engine.submit_order(2, OrderType::Limit, Side::Buy, 200, 5);

    EXPECT_EQ(engine.get_recent_trades(1).size(), 1u);
    EXPECT_EQ(engine.get_recent_trades(2).size(), 1u);
    EXPECT_EQ(engine.get_recent_trades(3).size(), 0u);
}
