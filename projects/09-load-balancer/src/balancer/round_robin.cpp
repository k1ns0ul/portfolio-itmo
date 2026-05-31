#include "round_robin.hpp"

namespace lb {

BackendPtr RoundRobinStrategy::select(const BackendPool& pool, std::string_view) {
    const auto healthy = pool.healthy();
    if (healthy.empty()) {
        return nullptr;
    }
    const std::uint64_t index = counter_.fetch_add(1, std::memory_order_relaxed);
    return healthy[index % healthy.size()];
}

}
