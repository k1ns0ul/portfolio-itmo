#pragma once

#include <atomic>

#include "strategy.hpp"

namespace lb {

class RoundRobinStrategy : public BalancingStrategy {
public:
    BackendPtr select(const BackendPool& pool, std::string_view client_ip) override;
    std::string_view name() const override { return "round_robin"; }

private:
    std::atomic<std::uint64_t> counter_{0};
};

}
