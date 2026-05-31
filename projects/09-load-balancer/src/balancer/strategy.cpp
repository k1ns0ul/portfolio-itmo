#include "strategy.hpp"

#include "ip_hash.hpp"
#include "least_connections.hpp"
#include "round_robin.hpp"
#include "weighted.hpp"

namespace lb {

std::unique_ptr<BalancingStrategy> make_strategy(Strategy kind) {
    switch (kind) {
        case Strategy::RoundRobin:
            return std::make_unique<RoundRobinStrategy>();
        case Strategy::LeastConnections:
            return std::make_unique<LeastConnectionsStrategy>();
        case Strategy::Weighted:
            return std::make_unique<WeightedStrategy>();
        case Strategy::IpHash:
            return std::make_unique<IpHashStrategy>();
    }
    return std::make_unique<RoundRobinStrategy>();
}

}
