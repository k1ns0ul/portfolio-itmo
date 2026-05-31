#pragma once

#include "types.hpp"

namespace me {

struct Trade {
    TradeId id;
    InstrumentId instrument;
    OrderId buy_order_id;
    OrderId sell_order_id;
    Price price;
    Quantity quantity;
    Timestamp executed_at;
};

}
