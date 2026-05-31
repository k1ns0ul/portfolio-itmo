#include "session.hpp"

#include <utility>

#include "protocol.hpp"

namespace me {

Session::Session(int fd, MatchingEngine& engine) : fd_(fd), engine_(engine) {}

void Session::on_data(const char* data, std::size_t len) {
    read_buffer_.append(data, len);

    std::size_t newline;
    while ((newline = read_buffer_.find('\n')) != std::string::npos) {
        std::string_view line(read_buffer_.data(), newline);
        if (!line.empty() && line.back() == '\r') {
            line.remove_suffix(1);
        }
        write_buffer_ += process_line(line);
        write_buffer_ += '\n';
        read_buffer_.erase(0, newline + 1);
    }
}

void Session::consume_write(std::size_t n) {
    if (n >= write_buffer_.size()) {
        write_buffer_.clear();
    } else {
        write_buffer_.erase(0, n);
    }
}

std::string Session::process_line(std::string_view line) {
    const Command cmd = ParseCommand(line);

    Response response = std::visit(
        [this](const auto& c) -> Response {
            using T = std::decay_t<decltype(c)>;
            if constexpr (std::is_same_v<T, OrderCmd>) {
                MatchResult result = engine_.submit_order(c.instrument, c.type, c.side, c.price, c.quantity);
                return OrderResponse{result.order_id, result.status, std::move(result.trades)};
            } else if constexpr (std::is_same_v<T, CancelCmd>) {
                const bool ok = engine_.cancel_order(c.instrument, c.order_id);
                if (!ok) {
                    return ErrorResponse{"order not found"};
                }
                return CancelResponse{c.order_id};
            } else if constexpr (std::is_same_v<T, BookCmd>) {
                return BookResponse{c.instrument, engine_.get_snapshot(c.instrument, c.depth)};
            } else if constexpr (std::is_same_v<T, TradesCmd>) {
                return TradesResponse{engine_.get_recent_trades(c.instrument, c.limit)};
            } else if constexpr (std::is_same_v<T, StatsCmd>) {
                const EngineStats stats = engine_.get_stats();
                return StatsResponse{stats.instruments, stats.orders, stats.trades};
            } else {
                return ErrorResponse{c.message};
            }
        },
        cmd);

    return FormatResponse(response);
}

}
