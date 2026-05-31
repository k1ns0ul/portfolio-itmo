#include "weighted.hpp"

namespace lb {

BackendPtr WeightedStrategy::select(const BackendPool& pool, std::string_view) {
    const auto healthy = pool.healthy();
    if (healthy.empty()) {
        return nullptr;
    }

    std::uint64_t total_weight = 0;
    for (const auto& backend : healthy) {
        total_weight += static_cast<std::uint64_t>(backend->weight());
    }
    if (total_weight == 0) {
        return healthy.front();
    }

    const std::uint64_t slot = counter_.fetch_add(1, std::memory_order_relaxed) % total_weight;
    std::uint64_t running = 0;
    for (const auto& backend : healthy) {
        running += static_cast<std::uint64_t>(backend->weight());
        if (slot < running) {
            return backend;
        }
    }
    return healthy.back();
}

}
