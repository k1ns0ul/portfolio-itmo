#include "price_level.hpp"

#include <algorithm>

namespace me {

PriceLevel::PriceLevel(Price p) : price_(p), total_quantity_(0) {}

void PriceLevel::add(Order* order) {
    total_quantity_ += order->remaining;
    orders_.push_back(order);
}

void PriceLevel::remove(OrderId id) {
    auto it = std::find_if(orders_.begin(), orders_.end(), [id](const Order* o) { return o->id == id; });
    if (it == orders_.end()) {
        return;
    }
    total_quantity_ -= (*it)->remaining;
    orders_.erase(it);
}

Order* PriceLevel::front() {
    return orders_.front();
}

void PriceLevel::pop_front() {
    orders_.pop_front();
}

void PriceLevel::reduce(Quantity executed) {
    total_quantity_ -= executed;
}

bool PriceLevel::empty() const noexcept {
    return orders_.empty();
}

Price PriceLevel::price() const noexcept {
    return price_;
}

Quantity PriceLevel::total_quantity() const noexcept {
    return total_quantity_;
}

std::size_t PriceLevel::order_count() const noexcept {
    return orders_.size();
}

}
