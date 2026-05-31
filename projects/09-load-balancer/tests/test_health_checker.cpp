#include <gtest/gtest.h>

#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include <atomic>
#include <memory>
#include <thread>

#include "backend_pool.hpp"
#include "health_checker.hpp"

using namespace lb;

namespace {

struct ListenerGuard {
    int fd = -1;
    std::uint16_t port = 0;

    explicit ListenerGuard(bool accept_loop) {
        fd = ::socket(AF_INET, SOCK_STREAM, 0);
        int opt = 1;
        ::setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));
        sockaddr_in addr{};
        addr.sin_family = AF_INET;
        addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
        addr.sin_port = 0;
        ::bind(fd, reinterpret_cast<sockaddr*>(&addr), sizeof(addr));
        ::listen(fd, 8);
        socklen_t len = sizeof(addr);
        ::getsockname(fd, reinterpret_cast<sockaddr*>(&addr), &len);
        port = ntohs(addr.sin_port);
        if (accept_loop) {
            running_.store(true);
            thread_ = std::thread([this]() {
                while (running_.load()) {
                    const int c = ::accept(fd, nullptr, nullptr);
                    if (c >= 0) {
                        ::close(c);
                    }
                }
            });
        }
    }

    ~ListenerGuard() {
        running_.store(false);
        if (fd >= 0) {
            ::shutdown(fd, SHUT_RDWR);
            ::close(fd);
        }
        if (thread_.joinable()) {
            thread_.join();
        }
    }

    std::atomic<bool> running_{false};
    std::thread thread_;
};

BackendPtr make_backend(const std::string& label) {
    auto addr = Address::from_string(label);
    return std::make_shared<Backend>(*addr, label, 1, 0);
}

HealthCheckConfig fast_config() {
    HealthCheckConfig cfg;
    cfg.interval = std::chrono::milliseconds(50);
    cfg.timeout = std::chrono::milliseconds(500);
    cfg.threshold = 3;
    cfg.type = HealthCheckType::Tcp;
    return cfg;
}

}

TEST(HealthChecker, HealthyWhenAcceptable) {
    ListenerGuard listener(true);
    BackendPool pool;
    auto backend = make_backend("127.0.0.1:" + std::to_string(listener.port));
    pool.add(backend);

    HealthChecker checker(pool, fast_config(), nullptr);
    checker.check_once();
    EXPECT_TRUE(backend->healthy());
}

TEST(HealthChecker, DownAfterThresholdFailures) {
    BackendPool pool;
    auto backend = make_backend("127.0.0.1:1");
    backend->set_healthy(true);
    pool.add(backend);

    HealthChecker checker(pool, fast_config(), nullptr);
    checker.check_once();
    EXPECT_TRUE(backend->healthy());
    checker.check_once();
    EXPECT_TRUE(backend->healthy());
    checker.check_once();
    EXPECT_FALSE(backend->healthy());
}

TEST(HealthChecker, RecoversAfterSuccess) {
    ListenerGuard listener(true);
    BackendPool pool;
    auto backend = make_backend("127.0.0.1:" + std::to_string(listener.port));
    backend->set_healthy(false);
    backend->consecutive_failures = 5;
    pool.add(backend);

    HealthChecker checker(pool, fast_config(), nullptr);
    checker.check_once();
    EXPECT_TRUE(backend->healthy());
    EXPECT_EQ(backend->consecutive_failures, 0);
}
