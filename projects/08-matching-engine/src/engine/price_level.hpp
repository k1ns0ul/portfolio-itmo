#pragma once

#include <cstddef>
#include <deque>

#include "order.hpp"
#include "types.hpp"

namespace me {

class PriceLevel {
public:
    explicit PriceLevel(Price p);

    void add(Order* order);
    void remove(OrderId id);
    Order* front();
    void pop_front();
    void reduce(Quantity executed);

    bool empty() const noexcept;
    Price price() const noexcept;
    Quantity total_quantity() const noexcept;
    std::size_t order_count() const noexcept;

private:
    Price price_;
    std::deque<Order*> orders_;
    Quantity total_quantity_;
};

}
