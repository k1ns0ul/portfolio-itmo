#pragma once

#include <cstddef>
#include <cstdint>
#include <string>
#include <string_view>
#include <variant>
#include <vector>

#include "order_book.hpp"
#include "trade.hpp"
#include "types.hpp"

namespace me {

struct OrderCmd {
    InstrumentId instrument;
    Side side;
    OrderType type;
    Price price;
    Quantity quantity;
};

struct CancelCmd {
    InstrumentId instrument;
    OrderId order_id;
};

struct BookCmd {
    InstrumentId instrument;
    int depth;
};

struct TradesCmd {
    InstrumentId instrument;
    int limit;
};

struct StatsCmd {};

struct InvalidCmd {
    std::string message;
};

using Command = std::variant<OrderCmd, CancelCmd, BookCmd, TradesCmd, StatsCmd, InvalidCmd>;

struct OrderResponse {
    OrderId order_id;
    OrderStatus status;
    std::vector<Trade> trades;
};

struct CancelResponse {
    OrderId order_id;
};

struct BookResponse {
    InstrumentId instrument;
    BookSnapshot snapshot;
};

struct TradesResponse {
    std::vector<Trade> trades;
};

struct StatsResponse {
    std::size_t instruments;
    std::size_t orders;
    std::uint64_t trades;
};

struct ErrorResponse {
    std::string message;
};

using Response =
    std::variant<OrderResponse, CancelResponse, BookResponse, TradesResponse, StatsResponse, ErrorResponse>;

Command ParseCommand(std::string_view line);
std::string FormatResponse(const Response& response);

}
