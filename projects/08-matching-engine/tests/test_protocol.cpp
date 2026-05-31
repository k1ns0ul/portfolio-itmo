#include <gtest/gtest.h>

#include <variant>

#include "protocol.hpp"

using namespace me;

TEST(Protocol, ParseOrderLimit) {
    auto cmd = ParseCommand("ORDER 1 BUY LIMIT 10000 50");
    ASSERT_TRUE(std::holds_alternative<OrderCmd>(cmd));
    const auto& o = std::get<OrderCmd>(cmd);
    EXPECT_EQ(o.instrument, 1u);
    EXPECT_EQ(o.side, Side::Buy);
    EXPECT_EQ(o.type, OrderType::Limit);
    EXPECT_EQ(o.price, 10000);
    EXPECT_EQ(o.quantity, 50);
}

TEST(Protocol, ParseOrderMarketIgnoresPrice) {
    auto cmd = ParseCommand("ORDER 2 SELL MARKET 9999 5");
    ASSERT_TRUE(std::holds_alternative<OrderCmd>(cmd));
    const auto& o = std::get<OrderCmd>(cmd);
    EXPECT_EQ(o.type, OrderType::Market);
    EXPECT_EQ(o.price, 0);
}

TEST(Protocol, ParseCancel) {
    auto cmd = ParseCommand("CANCEL 3 42");
    ASSERT_TRUE(std::holds_alternative<CancelCmd>(cmd));
    EXPECT_EQ(std::get<CancelCmd>(cmd).instrument, 3u);
    EXPECT_EQ(std::get<CancelCmd>(cmd).order_id, 42u);
}

TEST(Protocol, ParseBookWithDepth) {
    auto cmd = ParseCommand("BOOK 5 20");
    ASSERT_TRUE(std::holds_alternative<BookCmd>(cmd));
    EXPECT_EQ(std::get<BookCmd>(cmd).instrument, 5u);
    EXPECT_EQ(std::get<BookCmd>(cmd).depth, 20);

    auto def = ParseCommand("BOOK 5");
    ASSERT_TRUE(std::holds_alternative<BookCmd>(def));
    EXPECT_EQ(std::get<BookCmd>(def).depth, 10);
}

TEST(Protocol, ParseTrades) {
    auto cmd = ParseCommand("TRADES 9 25");
    ASSERT_TRUE(std::holds_alternative<TradesCmd>(cmd));
    EXPECT_EQ(std::get<TradesCmd>(cmd).instrument, 9u);
    EXPECT_EQ(std::get<TradesCmd>(cmd).limit, 25);
}

TEST(Protocol, ParseStats) {
    auto cmd = ParseCommand("STATS");
    EXPECT_TRUE(std::holds_alternative<StatsCmd>(cmd));
}

TEST(Protocol, InvalidCommands) {
    EXPECT_TRUE(std::holds_alternative<InvalidCmd>(ParseCommand("FOO 1 2")));
    EXPECT_TRUE(std::holds_alternative<InvalidCmd>(ParseCommand("ORDER 1 BUY LIMIT 100")));
    EXPECT_TRUE(std::holds_alternative<InvalidCmd>(ParseCommand("ORDER 1 HOLD LIMIT 100 5")));
    EXPECT_TRUE(std::holds_alternative<InvalidCmd>(ParseCommand("ORDER 1 BUY SWAP 100 5")));
    EXPECT_TRUE(std::holds_alternative<InvalidCmd>(ParseCommand("ORDER x BUY LIMIT 100 5")));
    EXPECT_TRUE(std::holds_alternative<InvalidCmd>(ParseCommand("")));
}

TEST(Protocol, FormatOrderResponseWithTrades) {
    Trade t;
    t.id = TradeId{7};
    t.price = 10000;
    t.quantity = 5;
    OrderResponse r{42, OrderStatus::Filled, {t}};
    EXPECT_EQ(FormatResponse(r), "OK ORDER 42 FILLED TRADES 7:10000:5");
}

TEST(Protocol, FormatOrderResponseNoTrades) {
    OrderResponse r{42, OrderStatus::New, {}};
    EXPECT_EQ(FormatResponse(r), "OK ORDER 42 NEW");
}

TEST(Protocol, FormatCancelResponse) {
    EXPECT_EQ(FormatResponse(CancelResponse{99}), "OK CANCEL 99");
}

TEST(Protocol, FormatBookResponse) {
    BookSnapshot snap;
    snap.bids.push_back({100, 10, 1});
    snap.asks.push_back({101, 5, 1});
    BookResponse r{1, snap};
    EXPECT_EQ(FormatResponse(r), "OK BOOK 1 BIDS 100:10 ASKS 101:5");
}

TEST(Protocol, FormatTradesResponse) {
    Trade t;
    t.id = TradeId{3};
    t.buy_order_id = 10;
    t.sell_order_id = 20;
    t.price = 500;
    t.quantity = 2;
    TradesResponse r{{t}};
    EXPECT_EQ(FormatResponse(r), "OK TRADES 3:10:20:500:2");
}

TEST(Protocol, FormatStatsResponse) {
    EXPECT_EQ(FormatResponse(StatsResponse{3, 12, 100}), "OK STATS instruments:3 orders:12 trades:100");
}

TEST(Protocol, FormatErrorResponse) {
    EXPECT_EQ(FormatResponse(ErrorResponse{"bad"}), "ERR bad");
}
