#pragma once

#include <cstdint>
#include <optional>
#include <string>
#include <vector>

#include "types.hpp"

namespace lb {

struct BackendConfig {
    std::string address;
    int weight = 1;
    int max_connections = 0;
};

struct HealthCheckConfig {
    Duration interval{std::chrono::seconds(5)};
    Duration timeout{std::chrono::seconds(2)};
    HealthCheckType type = HealthCheckType::Tcp;
    int threshold = 3;
    std::string http_path = "/health";
};

struct MetricsConfig {
    bool enabled = true;
    std::uint16_t port = 9100;
};

struct Config {
    std::string listen = "0.0.0.0:8080";
    Strategy strategy = Strategy::RoundRobin;
    HealthCheckConfig health_check;
    MetricsConfig metrics;
    std::vector<BackendConfig> backends;
    Duration drain_timeout{std::chrono::seconds(30)};

    static std::optional<Config> from_file(const std::string& path, std::string& error);
    static std::optional<Config> from_string(std::string_view text, std::string& error);
};

std::optional<Duration> parse_duration(std::string_view value);

}
