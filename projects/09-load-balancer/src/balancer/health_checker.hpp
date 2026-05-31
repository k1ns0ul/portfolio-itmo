#pragma once

#include <atomic>
#include <functional>
#include <thread>

#include "../core/config.hpp"
#include "backend_pool.hpp"

namespace lb {

class MetricsCollector;

class HealthChecker {
public:
    HealthChecker(BackendPool& pool, HealthCheckConfig config, MetricsCollector* metrics);
    ~HealthChecker();

    HealthChecker(const HealthChecker&) = delete;
    HealthChecker& operator=(const HealthChecker&) = delete;

    void start();
    void stop();

    void check_once();

    bool probe(const Backend& backend) const;

private:
    void run();
    void evaluate(const BackendPtr& backend);

    BackendPool& pool_;
    HealthCheckConfig config_;
    MetricsCollector* metrics_;
    std::thread thread_;
    std::atomic<bool> running_{false};
};

}
