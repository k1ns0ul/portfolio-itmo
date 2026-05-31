#include "protocol.hpp"

#include <charconv>
#include <optional>

namespace me {

namespace {

std::vector<std::string_view> tokenize(std::string_view line) {
    std::vector<std::string_view> tokens;
    std::size_t i = 0;
    while (i < line.size()) {
        while (i < line.size() && (line[i] == ' ' || line[i] == '\t' || line[i] == '\r')) {
            ++i;
        }
        const std::size_t start = i;
        while (i < line.size() && line[i] != ' ' && line[i] != '\t' && line[i] != '\r') {
            ++i;
        }
        if (i > start) {
            tokens.push_back(line.substr(start, i - start));
        }
    }
    return tokens;
}

template <typename T>
std::optional<T> parse_int(std::string_view sv) {
    T value{};
    const auto* begin = sv.data();
    const auto* end = sv.data() + sv.size();
    const auto [ptr, ec] = std::from_chars(begin, end, value);
    if (ec != std::errc{} || ptr != end) {
        return std::nullopt;
    }
    return value;
}

bool iequals(std::string_view a, std::string_view b) {
    if (a.size() != b.size()) {
        return false;
    }
    for (std::size_t i = 0; i < a.size(); ++i) {
        char ca = a[i];
        char cb = b[i];
        if (ca >= 'a' && ca <= 'z') {
            ca = static_cast<char>(ca - 'a' + 'A');
        }
        if (cb >= 'a' && cb <= 'z') {
            cb = static_cast<char>(cb - 'a' + 'A');
        }
        if (ca != cb) {
            return false;
        }
    }
    return true;
}

Command parse_order(const std::vector<std::string_view>& t) {
    if (t.size() != 6) {
        return InvalidCmd{"ORDER expects: ORDER <instrument> <BUY|SELL> <LIMIT|MARKET> <price> <quantity>"};
    }
    auto instrument = parse_int<InstrumentId>(t[1]);
    if (!instrument) {
        return InvalidCmd{"invalid instrument id"};
    }

    Side side;
    if (iequals(t[2], "BUY")) {
        side = Side::Buy;
    } else if (iequals(t[2], "SELL")) {
        side = Side::Sell;
    } else {
        return InvalidCmd{"side must be BUY or SELL"};
    }

    OrderType type;
    if (iequals(t[3], "LIMIT")) {
        type = OrderType::Limit;
    } else if (iequals(t[3], "MARKET")) {
        type = OrderType::Market;
    } else {
        return InvalidCmd{"type must be LIMIT or MARKET"};
    }

    auto price = parse_int<Price>(t[4]);
    if (!price) {
        return InvalidCmd{"invalid price"};
    }
    auto quantity = parse_int<Quantity>(t[5]);
    if (!quantity || *quantity <= 0) {
        return InvalidCmd{"quantity must be a positive integer"};
    }
    const Price effective_price = type == OrderType::Market ? 0 : *price;
    return OrderCmd{*instrument, side, type, effective_price, *quantity};
}

Command parse_cancel(const std::vector<std::string_view>& t) {
    if (t.size() != 3) {
        return InvalidCmd{"CANCEL expects: CANCEL <instrument> <order_id>"};
    }
    auto instrument = parse_int<InstrumentId>(t[1]);
    auto order_id = parse_int<OrderId>(t[2]);
    if (!instrument || !order_id) {
        return InvalidCmd{"invalid CANCEL arguments"};
    }
    return CancelCmd{*instrument, *order_id};
}

Command parse_book(const std::vector<std::string_view>& t) {
    if (t.size() < 2 || t.size() > 3) {
        return InvalidCmd{"BOOK expects: BOOK <instrument> [depth]"};
    }
    auto instrument = parse_int<InstrumentId>(t[1]);
    if (!instrument) {
        return InvalidCmd{"invalid instrument id"};
    }
    int depth = 10;
    if (t.size() == 3) {
        auto parsed = parse_int<int>(t[2]);
        if (!parsed || *parsed <= 0) {
            return InvalidCmd{"depth must be a positive integer"};
        }
        depth = *parsed;
    }
    return BookCmd{*instrument, depth};
}

Command parse_trades(const std::vector<std::string_view>& t) {
    if (t.size() < 2 || t.size() > 3) {
        return InvalidCmd{"TRADES expects: TRADES <instrument> [limit]"};
    }
    auto instrument = parse_int<InstrumentId>(t[1]);
    if (!instrument) {
        return InvalidCmd{"invalid instrument id"};
    }
    int limit = 50;
    if (t.size() == 3) {
        auto parsed = parse_int<int>(t[2]);
        if (!parsed || *parsed <= 0) {
            return InvalidCmd{"limit must be a positive integer"};
        }
        limit = *parsed;
    }
    return TradesCmd{*instrument, limit};
}

void append_int(std::string& out, std::int64_t value) {
    char buf[24];
    const auto [ptr, ec] = std::to_chars(buf, buf + sizeof(buf), value);
    if (ec == std::errc{}) {
        out.append(buf, ptr);
    }
}

void append_uint(std::string& out, std::uint64_t value) {
    char buf[24];
    const auto [ptr, ec] = std::to_chars(buf, buf + sizeof(buf), value);
    if (ec == std::errc{}) {
        out.append(buf, ptr);
    }
}

}

Command ParseCommand(std::string_view line) {
    const auto tokens = tokenize(line);
    if (tokens.empty()) {
        return InvalidCmd{"empty command"};
    }
    const std::string_view verb = tokens[0];
    if (iequals(verb, "ORDER")) {
        return parse_order(tokens);
    }
    if (iequals(verb, "CANCEL")) {
        return parse_cancel(tokens);
    }
    if (iequals(verb, "BOOK")) {
        return parse_book(tokens);
    }
    if (iequals(verb, "TRADES")) {
        return parse_trades(tokens);
    }
    if (iequals(verb, "STATS")) {
        return StatsCmd{};
    }
    return InvalidCmd{"unknown command"};
}

std::string FormatResponse(const Response& response) {
    std::string out;
    std::visit(
        [&out](const auto& r) {
            using T = std::decay_t<decltype(r)>;
            if constexpr (std::is_same_v<T, OrderResponse>) {
                out += "OK ORDER ";
                append_uint(out, r.order_id);
                out += ' ';
                out += to_string(r.status);
                if (!r.trades.empty()) {
                    out += " TRADES ";
                    for (std::size_t i = 0; i < r.trades.size(); ++i) {
                        if (i != 0) {
                            out += ", ";
                        }
                        append_uint(out, r.trades[i].id.value);
                        out += ':';
                        append_int(out, r.trades[i].price);
                        out += ':';
                        append_int(out, r.trades[i].quantity);
                    }
                }
            } else if constexpr (std::is_same_v<T, CancelResponse>) {
                out += "OK CANCEL ";
                append_uint(out, r.order_id);
            } else if constexpr (std::is_same_v<T, BookResponse>) {
                out += "OK BOOK ";
                append_uint(out, r.instrument);
                out += " BIDS ";
                for (std::size_t i = 0; i < r.snapshot.bids.size(); ++i) {
                    if (i != 0) {
                        out += ',';
                    }
                    append_int(out, r.snapshot.bids[i].price);
                    out += ':';
                    append_int(out, r.snapshot.bids[i].quantity);
                }
                out += " ASKS ";
                for (std::size_t i = 0; i < r.snapshot.asks.size(); ++i) {
                    if (i != 0) {
                        out += ',';
                    }
                    append_int(out, r.snapshot.asks[i].price);
                    out += ':';
                    append_int(out, r.snapshot.asks[i].quantity);
                }
            } else if constexpr (std::is_same_v<T, TradesResponse>) {
                out += "OK TRADES ";
                for (std::size_t i = 0; i < r.trades.size(); ++i) {
                    if (i != 0) {
                        out += ", ";
                    }
                    append_uint(out, r.trades[i].id.value);
                    out += ':';
                    append_uint(out, r.trades[i].buy_order_id);
                    out += ':';
                    append_uint(out, r.trades[i].sell_order_id);
                    out += ':';
                    append_int(out, r.trades[i].price);
                    out += ':';
                    append_int(out, r.trades[i].quantity);
                }
            } else if constexpr (std::is_same_v<T, StatsResponse>) {
                out += "OK STATS instruments:";
                append_uint(out, r.instruments);
                out += " orders:";
                append_uint(out, r.orders);
                out += " trades:";
                append_uint(out, r.trades);
            } else if constexpr (std::is_same_v<T, ErrorResponse>) {
                out += "ERR ";
                out += r.message;
            }
        },
        response);
    return out;
}

}
