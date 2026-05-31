#pragma once

#include <memory>
#include <string_view>

#include "backend.hpp"
#include "backend_pool.hpp"

namespace lb {

class BalancingStrategy {
public:
    virtual ~BalancingStrategy() = default;
    virtual BackendPtr select(const BackendPool& pool, std::string_view client_ip) = 0;
    virtual std::string_view name() const = 0;
};

std::unique_ptr<BalancingStrategy> make_strategy(Strategy kind);

}
