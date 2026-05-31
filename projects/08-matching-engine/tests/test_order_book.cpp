#include <gtest/gtest.h>

#include "order_book.hpp"

using namespace me;

TEST(OrderBook, LimitBuyRestsWhenNoAsks) {
    OrderBook book(1);
    auto result = book.add_order(OrderType::Limit, Side::Buy, 100, 10);
    EXPECT_EQ(result.status, OrderStatus::New);
    EXPECT_TRUE(result.trades.empty());

    auto snap = book.snapshot();
    ASSERT_EQ(snap.bids.size(), 1u);
    EXPECT_EQ(snap.bids[0].price, 100);
    EXPECT_EQ(snap.bids[0].quantity, 10);
    EXPECT_TRUE(snap.asks.empty());
}

TEST(OrderBook, LimitSellRestsWhenNoBids) {
    OrderBook book(1);
    auto result = book.add_order(OrderType::Limit, Side::Sell, 105, 7);
    EXPECT_EQ(result.status, OrderStatus::New);

    auto snap = book.snapshot();
    ASSERT_EQ(snap.asks.size(), 1u);
    EXPECT_EQ(snap.asks[0].price, 105);
    EXPECT_EQ(snap.asks[0].quantity, 7);
}

TEST(OrderBook, BuyMatchesBestAsk) {
    OrderBook book(1);
    book.add_order(OrderType::Limit, Side::Sell, 101, 5);
    book.add_order(OrderType::Limit, Side::Sell, 100, 5);

    auto result = book.add_order(OrderType::Limit, Side::Buy, 100, 5);
    EXPECT_EQ(result.status, OrderStatus::Filled);
    ASSERT_EQ(result.trades.size(), 1u);
    EXPECT_EQ(result.trades[0].price, 100);
    EXPECT_EQ(result.trades[0].quantity, 5);

    auto snap = book.snapshot();
    ASSERT_EQ(snap.asks.size(), 1u);
    EXPECT_EQ(snap.asks[0].price, 101);
}

TEST(OrderBook, SellMatchesBestBid) {
    OrderBook book(1);
    book.add_order(OrderType::Limit, Side::Buy, 99, 5);
    book.add_order(OrderType::Limit, Side::Buy, 100, 5);

    auto result = book.add_order(OrderType::Limit, Side::Sell, 100, 5);
    EXPECT_EQ(result.status, OrderStatus::Filled);
    ASSERT_EQ(result.trades.size(), 1u);
    EXPECT_EQ(result.trades[0].price, 100);
}

TEST(OrderBook, PriceTimePriority) {
    OrderBook book(1);
    auto first = book.add_order(OrderType::Limit, Side::Buy, 100, 5);
    auto second = book.add_order(OrderType::Limit, Side::Buy, 100, 5);

    auto result = book.add_order(OrderType::Limit, Side::Sell, 100, 5);
    ASSERT_EQ(result.trades.size(), 1u);
    EXPECT_EQ(result.trades[0].buy_order_id, first.order_id);
    EXPECT_NE(result.trades[0].buy_order_id, second.order_id);
}

TEST(OrderBook, PartialFillRestsRemainder) {
    OrderBook book(1);
    book.add_order(OrderType::Limit, Side::Sell, 100, 3);

    auto result = book.add_order(OrderType::Limit, Side::Buy, 100, 10);
    EXPECT_EQ(result.status, OrderStatus::PartiallyFilled);
    ASSERT_EQ(result.trades.size(), 1u);
    EXPECT_EQ(result.trades[0].quantity, 3);

    auto snap = book.snapshot();
    ASSERT_EQ(snap.bids.size(), 1u);
    EXPECT_EQ(snap.bids[0].quantity, 7);
    EXPECT_TRUE(snap.asks.empty());
}

TEST(OrderBook, MarketSweepsMultipleLevels) {
    OrderBook book(1);
    book.add_order(OrderType::Limit, Side::Sell, 100, 5);
    book.add_order(OrderType::Limit, Side::Sell, 101, 5);
    book.add_order(OrderType::Limit, Side::Sell, 102, 5);

    auto result = book.add_order(OrderType::Market, Side::Buy, 0, 12);
    EXPECT_EQ(result.status, OrderStatus::Filled);
    ASSERT_EQ(result.trades.size(), 3u);
    EXPECT_EQ(result.trades[0].price, 100);
    EXPECT_EQ(result.trades[1].price, 101);
    EXPECT_EQ(result.trades[2].price, 102);
    EXPECT_EQ(result.trades[2].quantity, 2);
}

TEST(OrderBook, MarketWithoutLiquidityCancelled) {
    OrderBook book(1);
    auto result = book.add_order(OrderType::Market, Side::Buy, 0, 10);
    EXPECT_EQ(result.status, OrderStatus::Cancelled);
    EXPECT_TRUE(result.trades.empty());
    EXPECT_EQ(book.order_count(), 0u);
}

TEST(OrderBook, CancelRemovesResting) {
    OrderBook book(1);
    auto result = book.add_order(OrderType::Limit, Side::Buy, 100, 5);
    EXPECT_TRUE(book.cancel_order(result.order_id));

    auto snap = book.snapshot();
    EXPECT_TRUE(snap.bids.empty());
    EXPECT_FALSE(book.cancel_order(result.order_id));
}

TEST(OrderBook, SnapshotReflectsState) {
    OrderBook book(1);
    book.add_order(OrderType::Limit, Side::Buy, 100, 5);
    book.add_order(OrderType::Limit, Side::Buy, 100, 5);
    book.add_order(OrderType::Limit, Side::Buy, 99, 3);
    book.add_order(OrderType::Limit, Side::Sell, 101, 4);

    auto snap = book.snapshot();
    ASSERT_EQ(snap.bids.size(), 2u);
    EXPECT_EQ(snap.bids[0].price, 100);
    EXPECT_EQ(snap.bids[0].quantity, 10);
    EXPECT_EQ(snap.bids[0].order_count, 2u);
    EXPECT_EQ(snap.bids[1].price, 99);
    ASSERT_EQ(snap.asks.size(), 1u);
    EXPECT_EQ(snap.asks[0].price, 101);
}

TEST(OrderBook, StatsReportSpread) {
    OrderBook book(1);
    book.add_order(OrderType::Limit, Side::Buy, 100, 5);
    book.add_order(OrderType::Limit, Side::Sell, 103, 5);

    auto stats = book.stats();
    EXPECT_EQ(stats.best_bid, 100);
    EXPECT_EQ(stats.best_ask, 103);
    EXPECT_EQ(stats.spread, 3);
    EXPECT_EQ(stats.total_bid_quantity, 5);
    EXPECT_EQ(stats.total_ask_quantity, 5);
}
