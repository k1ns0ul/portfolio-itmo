#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstring>
#include <iomanip>
#include <iostream>
#include <thread>
#include <vector>

#include "../balancer/backend_pool.hpp"
#include "../balancer/strategy.hpp"
#include "../core/types.hpp"
#include "../metrics/collector.hpp"
#include "../net/address.hpp"
#include "../net/socket.hpp"
#include "../proxy/proxy_engine.hpp"

namespace {

using Clock = std::chrono::steady_clock;

constexpr int kEchoBackends = 3;
constexpr int kConnections = 200;
constexpr int kMessagesPerConn = 20;
constexpr int kMessageSize = 256;

struct EchoServer {
    lb::Socket sock;
    std::uint16_t port;
    std::thread thread;
    std::atomic<bool> running{true};
};

std::uint16_t start_echo(std::shared_ptr<EchoServer> server) {
    auto addr = lb::Address::from_string("127.0.0.1:0");
    auto listener = lb::Socket::listen_on(*addr, 128);
    if (!listener) {
        return 0;
    }
    sockaddr_in bound{};
    socklen_t len = sizeof(bound);
    ::getsockname(listener->fd(), reinterpret_cast<sockaddr*>(&bound), &len);
    server->port = ntohs(bound.sin_port);
    server->sock = std::move(*listener);

    server->thread = std::thread([server]() {
        while (server->running.load()) {
            lb::Address peer;
            auto client = server->sock.accept(peer);
            if (!client) {
                std::this_thread::sleep_for(std::chrono::milliseconds(1));
                continue;
            }
            std::thread([fd = client->release()]() {
                char buf[4096];
                while (true) {
                    const ssize_t n = ::recv(fd, buf, sizeof(buf), 0);
                    if (n <= 0) {
                        break;
                    }
                    ::send(fd, buf, static_cast<std::size_t>(n), 0);
                }
                ::close(fd);
            }).detach();
        }
    });
    return server->port;
}

double percentile(std::vector<double>& v, double p) {
    if (v.empty()) {
        return 0.0;
    }
    const std::size_t idx = static_cast<std::size_t>(p * (v.size() - 1));
    return v[idx];
}

struct Result {
    double throughput_conn_per_sec;
    double throughput_mb_per_sec;
    double p50;
    double p95;
    double p99;
};

Result run_load(std::uint16_t lb_port) {
    std::vector<double> latencies;
    latencies.reserve(kConnections * kMessagesPerConn);
    std::atomic<std::uint64_t> total_bytes{0};

    const auto start = Clock::now();
    std::vector<std::thread> clients;
    std::vector<std::vector<double>> per_thread(kConnections);

    for (int c = 0; c < kConnections; ++c) {
        clients.emplace_back([lb_port, c, &per_thread, &total_bytes]() {
            int fd = ::socket(AF_INET, SOCK_STREAM, 0);
            if (fd < 0) {
                return;
            }
            sockaddr_in addr{};
            addr.sin_family = AF_INET;
            addr.sin_port = htons(lb_port);
            ::inet_pton(AF_INET, "127.0.0.1", &addr.sin_addr);
            if (::connect(fd, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) != 0) {
                ::close(fd);
                return;
            }
            char payload[kMessageSize];
            std::memset(payload, 'x', sizeof(payload));
            char recv_buf[kMessageSize];
            for (int m = 0; m < kMessagesPerConn; ++m) {
                const auto t0 = Clock::now();
                if (::send(fd, payload, sizeof(payload), 0) <= 0) {
                    break;
                }
                std::size_t received = 0;
                while (received < sizeof(payload)) {
                    const ssize_t n = ::recv(fd, recv_buf, sizeof(recv_buf) - received, 0);
                    if (n <= 0) {
                        break;
                    }
                    received += static_cast<std::size_t>(n);
                }
                const auto t1 = Clock::now();
                per_thread[c].push_back(std::chrono::duration<double, std::milli>(t1 - t0).count());
                total_bytes.fetch_add(received * 2, std::memory_order_relaxed);
            }
            ::close(fd);
        });
    }
    for (auto& t : clients) {
        t.join();
    }
    const auto elapsed = std::chrono::duration<double>(Clock::now() - start).count();

    for (auto& v : per_thread) {
        latencies.insert(latencies.end(), v.begin(), v.end());
    }
    std::sort(latencies.begin(), latencies.end());

    Result result;
    result.throughput_conn_per_sec = elapsed > 0 ? kConnections / elapsed : 0;
    result.throughput_mb_per_sec =
        elapsed > 0 ? static_cast<double>(total_bytes.load()) / (1024.0 * 1024.0) / elapsed : 0;
    result.p50 = percentile(latencies, 0.50);
    result.p95 = percentile(latencies, 0.95);
    result.p99 = percentile(latencies, 0.99);
    return result;
}

Result benchmark_strategy(lb::Strategy kind, const std::vector<std::uint16_t>& backend_ports,
                          std::uint16_t lb_port) {
    lb::BackendPool pool;
    for (std::uint16_t port : backend_ports) {
        const std::string label = "127.0.0.1:" + std::to_string(port);
        auto addr = lb::Address::from_string(label);
        auto backend = std::make_shared<lb::Backend>(*addr, label, 1, 0);
        backend->set_healthy(true);
        pool.add(backend);
    }

    lb::MetricsCollector metrics;
    auto strategy = lb::make_strategy(kind);
    auto listen = lb::Address::from_string("127.0.0.1:" + std::to_string(lb_port));
    lb::ProxyEngine engine(*listen, pool, *strategy, metrics);
    if (!engine.start()) {
        return {};
    }

    std::atomic<bool> stop{false};
    std::thread loop([&engine, &stop]() {
        while (!stop.load()) {
            engine.run_once(50);
        }
    });

    std::this_thread::sleep_for(std::chrono::milliseconds(50));
    Result result = run_load(lb_port);
    stop.store(true);
    loop.join();
    return result;
}

}

int main() {
    std::cout << "load balancer benchmark\n";
    std::cout << "connections=" << kConnections << " messages/conn=" << kMessagesPerConn << "\n\n";

    std::vector<std::shared_ptr<EchoServer>> servers;
    std::vector<std::uint16_t> ports;
    for (int i = 0; i < kEchoBackends; ++i) {
        auto server = std::make_shared<EchoServer>();
        const std::uint16_t port = start_echo(server);
        servers.push_back(server);
        ports.push_back(port);
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(100));

    std::cout << std::left << std::setw(20) << "strategy" << std::right << std::setw(14) << "conn/sec"
              << std::setw(14) << "MB/sec" << std::setw(12) << "p50 ms" << std::setw(12) << "p95 ms"
              << std::setw(12) << "p99 ms" << "\n";

    const lb::Strategy strategies[] = {lb::Strategy::RoundRobin, lb::Strategy::LeastConnections,
                                       lb::Strategy::Weighted};
    std::uint16_t lb_port = 18080;
    for (lb::Strategy kind : strategies) {
        const Result r = benchmark_strategy(kind, ports, lb_port++);
        std::cout << std::left << std::setw(20) << lb::strategy_name(kind) << std::right << std::fixed
                  << std::setprecision(1) << std::setw(14) << r.throughput_conn_per_sec << std::setw(14)
                  << r.throughput_mb_per_sec << std::setw(12) << r.p50 << std::setw(12) << r.p95
                  << std::setw(12) << r.p99 << "\n";
    }

    for (auto& server : servers) {
        server->running.store(false);
        ::shutdown(server->sock.fd(), SHUT_RDWR);
        if (server->thread.joinable()) {
            server->thread.join();
        }
    }
    return 0;
}
