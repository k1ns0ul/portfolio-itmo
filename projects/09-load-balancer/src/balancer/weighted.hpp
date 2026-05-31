#pragma once

#include <atomic>

#include "strategy.hpp"

namespace lb {

class WeightedStrategy : public BalancingStrategy {
public:
    BackendPtr select(const BackendPool& pool, std::string_view client_ip) override;
    std::string_view name() const override { return "weighted"; }

private:
    std::atomic<std::uint64_t> counter_{0};
};

}
