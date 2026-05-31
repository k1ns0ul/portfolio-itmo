#pragma once

#include <chrono>
#include <cstdint>
#include <string>

namespace lb {

using Clock = std::chrono::steady_clock;
using TimePoint = Clock::time_point;
using Duration = std::chrono::milliseconds;

enum class Strategy : std::uint8_t { RoundRobin, LeastConnections, Weighted, IpHash };

enum class HealthCheckType : std::uint8_t { Tcp, Http };

constexpr int kDefaultBufferSize = 64 * 1024;

inline std::string strategy_name(Strategy s) {
    switch (s) {
        case Strategy::RoundRobin:
            return "round_robin";
        case Strategy::LeastConnections:
            return "least_connections";
        case Strategy::Weighted:
            return "weighted";
        case Strategy::IpHash:
            return "ip_hash";
    }
    return "unknown";
}

}
