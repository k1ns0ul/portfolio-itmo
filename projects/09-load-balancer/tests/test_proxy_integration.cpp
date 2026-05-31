#include <gtest/gtest.h>

#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include <atomic>
#include <cstring>
#include <memory>
#include <string>
#include <thread>

#include "backend_pool.hpp"
#include "collector.hpp"
#include "proxy_engine.hpp"
#include "round_robin.hpp"
#include "socket.hpp"

using namespace lb;

namespace {

struct EchoBackend {
    Socket sock;
    std::uint16_t port = 0;
    std::thread thread;
    std::atomic<bool> running{true};

    EchoBackend() {
        auto addr = Address::from_string("127.0.0.1:0");
        auto listener = Socket::listen_on(*addr, 16);
        sockaddr_in bound{};
        socklen_t len = sizeof(bound);
        ::getsockname(listener->fd(), reinterpret_cast<sockaddr*>(&bound), &len);
        port = ntohs(bound.sin_port);
        sock = std::move(*listener);
        thread = std::thread([this]() {
            while (running.load()) {
                Address peer;
                auto client = sock.accept(peer);
                if (!client) {
                    std::this_thread::sleep_for(std::chrono::milliseconds(2));
                    continue;
                }
                const int fd = client->release();
                std::thread([fd]() {
                    char buf[1024];
                    const ssize_t n = ::recv(fd, buf, sizeof(buf), 0);
                    if (n > 0) {
                        ::send(fd, buf, static_cast<std::size_t>(n), 0);
                    }
                    ::close(fd);
                }).detach();
            }
        });
    }

    ~EchoBackend() {
        running.store(false);
        ::shutdown(sock.fd(), SHUT_RDWR);
        if (thread.joinable()) {
            thread.join();
        }
    }
};

}

TEST(ProxyIntegration, EchoThroughBalancer) {
    EchoBackend backend;
    std::this_thread::sleep_for(std::chrono::milliseconds(50));

    BackendPool pool;
    const std::string label = "127.0.0.1:" + std::to_string(backend.port);
    auto addr = Address::from_string(label);
    auto entry = std::make_shared<Backend>(*addr, label, 1, 0);
    entry->set_healthy(true);
    pool.add(entry);

    MetricsCollector metrics;
    RoundRobinStrategy strategy;
    auto listen = Address::from_string("127.0.0.1:18099");
    ProxyEngine engine(*listen, pool, strategy, metrics);
    ASSERT_TRUE(engine.start());

    std::atomic<bool> stop{false};
    std::thread loop([&engine, &stop]() {
        while (!stop.load()) {
            engine.run_once(20);
        }
    });
    std::this_thread::sleep_for(std::chrono::milliseconds(50));

    int fd = ::socket(AF_INET, SOCK_STREAM, 0);
    sockaddr_in target{};
    target.sin_family = AF_INET;
    target.sin_port = htons(18099);
    ::inet_pton(AF_INET, "127.0.0.1", &target.sin_addr);
    ASSERT_EQ(::connect(fd, reinterpret_cast<sockaddr*>(&target), sizeof(target)), 0);

    const std::string payload = "ping-through-lb";
    ASSERT_GT(::send(fd, payload.data(), payload.size(), 0), 0);

    char buf[64]{};
    const ssize_t n = ::recv(fd, buf, sizeof(buf), 0);
    ASSERT_GT(n, 0);
    EXPECT_EQ(std::string(buf, static_cast<std::size_t>(n)), payload);

    ::close(fd);
    stop.store(true);
    loop.join();
}
