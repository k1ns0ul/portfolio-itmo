#include "ip_hash.hpp"

#include <functional>
#include <string>

namespace lb {

BackendPtr IpHashStrategy::select(const BackendPool& pool, std::string_view client_ip) {
    const auto healthy = pool.healthy();
    if (healthy.empty()) {
        return nullptr;
    }
    const std::size_t digest = std::hash<std::string_view>{}(client_ip);
    return healthy[digest % healthy.size()];
}

}
