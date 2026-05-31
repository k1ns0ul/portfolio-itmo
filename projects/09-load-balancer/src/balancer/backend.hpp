#pragma once

#include <atomic>
#include <cstdint>
#include <memory>
#include <string>

#include "../core/types.hpp"
#include "../net/address.hpp"

namespace lb {

class Backend {
public:
    Backend(Address address, std::string label, int weight, int max_connections);

    const Address& address() const noexcept { return address_; }
    const std::string& label() const noexcept { return label_; }
    int weight() const noexcept { return weight_; }
    int max_connections() const noexcept { return max_connections_; }
    void set_weight(int weight) noexcept { weight_ = weight; }

    bool healthy() const noexcept { return healthy_.load(std::memory_order_acquire); }
    void set_healthy(bool value) noexcept { healthy_.store(value, std::memory_order_release); }

    int active_connections() const noexcept { return active_connections_.load(std::memory_order_relaxed); }
    void inc_active() noexcept { active_connections_.fetch_add(1, std::memory_order_relaxed); }
    void dec_active() noexcept { active_connections_.fetch_sub(1, std::memory_order_relaxed); }

    void add_connection() noexcept { total_connections_.fetch_add(1, std::memory_order_relaxed); }
    std::uint64_t total_connections() const noexcept { return total_connections_.load(std::memory_order_relaxed); }

    void add_bytes(std::uint64_t n) noexcept { total_bytes_.fetch_add(n, std::memory_order_relaxed); }
    std::uint64_t total_bytes() const noexcept { return total_bytes_.load(std::memory_order_relaxed); }

    void set_response_time(std::int64_t us) noexcept { response_time_us_.store(us, std::memory_order_relaxed); }
    std::int64_t response_time_us() const noexcept { return response_time_us_.load(std::memory_order_relaxed); }

    int consecutive_failures = 0;
    bool draining = false;
    TimePoint last_health_check{};

private:
    Address address_;
    std::string label_;
    int weight_;
    int max_connections_;

    std::atomic<bool> healthy_{false};
    std::atomic<int> active_connections_{0};
    std::atomic<std::uint64_t> total_connections_{0};
    std::atomic<std::uint64_t> total_bytes_{0};
    std::atomic<std::int64_t> response_time_us_{0};
};

using BackendPtr = std::shared_ptr<Backend>;

}
