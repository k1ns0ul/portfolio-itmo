#pragma once

#include <atomic>
#include <cstdint>
#include <thread>

#include "collector.hpp"

namespace lb {

class BackendPool;

class PrometheusExporter {
public:
    PrometheusExporter(const MetricsCollector& collector, const BackendPool& pool, std::uint16_t port);
    ~PrometheusExporter();

    PrometheusExporter(const PrometheusExporter&) = delete;
    PrometheusExporter& operator=(const PrometheusExporter&) = delete;

    bool start();
    void stop();

private:
    void run();
    void handle_client(int client_fd);

    const MetricsCollector& collector_;
    const BackendPool& pool_;
    std::uint16_t port_;
    int listen_fd_ = -1;
    std::thread thread_;
    std::atomic<bool> running_{false};
};

}
