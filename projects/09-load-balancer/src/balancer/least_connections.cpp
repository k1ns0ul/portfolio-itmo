#include "least_connections.hpp"

namespace lb {

BackendPtr LeastConnectionsStrategy::select(const BackendPool& pool, std::string_view) {
    const auto healthy = pool.healthy();
    BackendPtr best;
    int best_active = 0;
    for (const auto& backend : healthy) {
        const int active = backend->active_connections();
        if (!best || active < best_active) {
            best = backend;
            best_active = active;
        }
    }
    return best;
}

}
