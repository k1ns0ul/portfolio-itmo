#pragma once

#include <cstdint>
#include <memory>
#include <unordered_map>

#include "../balancer/backend_pool.hpp"
#include "../balancer/strategy.hpp"
#include "../core/config.hpp"
#include "../metrics/collector.hpp"
#include "../net/epoll.hpp"
#include "../net/socket.hpp"
#include "connection.hpp"

namespace lb {

class ProxyEngine {
public:
    ProxyEngine(const Address& listen_addr, BackendPool& pool, BalancingStrategy& strategy,
                MetricsCollector& metrics);

    bool start();
    void run_once(int timeout_ms);
    void stop() noexcept { running_ = false; }
    bool running() const noexcept { return running_; }

    void set_strategy(BalancingStrategy& strategy) noexcept { strategy_ = &strategy; }
    std::size_t active_connections() const noexcept { return connections_.size(); }

private:
    enum class Side : std::uint8_t { Client, Backend };

    struct FdRef {
        ProxyConnection* conn;
        Side side;
    };

    void accept_clients();
    void establish_backend(Socket client, const std::string& client_ip);
    void on_event(const FdRef& ref, std::uint32_t events);
    bool pump(ProxyConnection& conn, Side from);
    void update_epoll(ProxyConnection& conn);
    void close_connection(ProxyConnection& conn);

    Address listen_addr_;
    BackendPool& pool_;
    BalancingStrategy* strategy_;
    MetricsCollector& metrics_;

    Socket listen_sock_;
    Epoll epoll_;
    bool running_ = false;

    std::unordered_map<ProxyConnection*, std::unique_ptr<ProxyConnection>> connections_;
    std::unordered_map<int, std::unique_ptr<FdRef>> fd_refs_;
};

}
