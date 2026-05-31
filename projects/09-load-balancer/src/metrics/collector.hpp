#pragma once

#include <array>
#include <atomic>
#include <cstdint>
#include <map>
#include <mutex>
#include <string>
#include <vector>

#include "../core/types.hpp"

namespace lb {

class MetricsCollector {
public:
    static constexpr std::size_t kBucketCount = 6;
    static constexpr std::array<double, kBucketCount - 1> kBucketBounds = {0.001, 0.01, 0.1, 1.0, 10.0};

    struct BackendMetrics {
        std::atomic<std::uint64_t> connections{0};
        std::atomic<std::uint64_t> bytes_in{0};
        std::atomic<std::uint64_t> bytes_out{0};
        std::atomic<std::int64_t> response_time_us{0};
        std::atomic<int> healthy{0};
        std::atomic<std::uint64_t> health_success{0};
        std::atomic<std::uint64_t> health_failure{0};
    };

    void on_connection_open();
    void on_connection_close(double duration_seconds);
    void on_failed_connection();

    void add_bytes_in(const std::string& backend, std::uint64_t n);
    void add_bytes_out(const std::string& backend, std::uint64_t n);
    void on_backend_connection(const std::string& backend);
    void set_backend_health(const std::string& backend, bool healthy);
    void set_backend_response_time(const std::string& backend, std::int64_t us);
    void record_health_check(const std::string& backend, bool success);

    std::uint64_t total_connections() const { return total_connections_.load(); }
    std::int64_t active_connections() const { return active_connections_.load(); }
    std::uint64_t total_bytes_in() const { return total_bytes_in_.load(); }
    std::uint64_t total_bytes_out() const { return total_bytes_out_.load(); }
    std::uint64_t failed_connections() const { return failed_connections_.load(); }

    std::string render_prometheus() const;

private:
    BackendMetrics& backend_metrics(const std::string& backend);

    std::atomic<std::uint64_t> total_connections_{0};
    std::atomic<std::int64_t> active_connections_{0};
    std::atomic<std::uint64_t> total_bytes_in_{0};
    std::atomic<std::uint64_t> total_bytes_out_{0};
    std::atomic<std::uint64_t> failed_connections_{0};
    std::atomic<std::uint64_t> health_checks_total_{0};
    std::atomic<std::uint64_t> health_checks_failed_{0};

    std::array<std::atomic<std::uint64_t>, kBucketCount> duration_buckets_{};

    mutable std::mutex backends_mutex_;
    std::map<std::string, std::unique_ptr<BackendMetrics>> backends_;
};

}
