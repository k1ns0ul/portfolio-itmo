#pragma once

#include "strategy.hpp"

namespace lb {

class LeastConnectionsStrategy : public BalancingStrategy {
public:
    BackendPtr select(const BackendPool& pool, std::string_view client_ip) override;
    std::string_view name() const override { return "least_connections"; }
};

}
