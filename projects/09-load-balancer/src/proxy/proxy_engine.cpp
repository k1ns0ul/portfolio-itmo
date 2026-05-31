#include "proxy_engine.hpp"

#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include <cerrno>

#include "../core/logger.hpp"

namespace lb {

ProxyEngine::ProxyEngine(const Address& listen_addr, BackendPool& pool, BalancingStrategy& strategy,
                         MetricsCollector& metrics)
    : listen_addr_(listen_addr), pool_(pool), strategy_(&strategy), metrics_(metrics) {}

bool ProxyEngine::start() {
    auto sock = Socket::listen_on(listen_addr_, 1024);
    if (!sock) {
        Logger::instance().error("failed to bind listen socket", {{"address", listen_addr_.to_string()}});
        return false;
    }
    listen_sock_ = std::move(*sock);
    if (!epoll_.valid()) {
        return false;
    }
    if (!epoll_.add(listen_sock_.fd(), EPOLLIN, nullptr)) {
        return false;
    }
    running_ = true;
    Logger::instance().info("proxy listening", {{"address", listen_addr_.to_string()}});
    return true;
}

void ProxyEngine::run_once(int timeout_ms) {
    auto events = epoll_.wait(timeout_ms);
    for (const auto& ev : events) {
        if (ev.data.ptr == nullptr) {
            accept_clients();
            continue;
        }
        auto* ref = static_cast<FdRef*>(ev.data.ptr);
        on_event(*ref, ev.events);
    }
}

void ProxyEngine::accept_clients() {
    while (true) {
        Address peer;
        auto client = listen_sock_.accept(peer);
        if (!client) {
            break;
        }
        client->set_nonblocking();
        client->set_nodelay();
        establish_backend(std::move(*client), peer.host());
    }
}

void ProxyEngine::establish_backend(Socket client, const std::string& client_ip) {
    BackendPtr backend = strategy_->select(pool_, client_ip);
    if (!backend) {
        metrics_.on_failed_connection();
        Logger::instance().warn("no healthy backend for client", {{"client", client_ip}});
        return;
    }

    Socket upstream = Socket::create_tcp();
    if (!upstream.valid()) {
        metrics_.on_failed_connection();
        return;
    }
    upstream.set_nonblocking();
    upstream.set_nodelay();

    const sockaddr_in& raw = backend->address().raw();
    const int rc = ::connect(upstream.fd(), reinterpret_cast<const sockaddr*>(&raw), sizeof(raw));
    if (rc != 0 && errno != EINPROGRESS) {
        metrics_.on_failed_connection();
        return;
    }

    auto conn = std::make_unique<ProxyConnection>(std::move(client), std::move(upstream), backend, client_ip);
    ProxyConnection* ptr = conn.get();
    conn->set_state(ConnState::Active);

    metrics_.on_connection_open();
    metrics_.on_backend_connection(backend->label());

    auto client_ref = std::make_unique<FdRef>(FdRef{ptr, Side::Client});
    auto backend_ref = std::make_unique<FdRef>(FdRef{ptr, Side::Backend});

    epoll_.add(ptr->client_fd(), EPOLLIN, client_ref.get());
    epoll_.add(ptr->backend_fd(), EPOLLIN, backend_ref.get());

    fd_refs_[ptr->client_fd()] = std::move(client_ref);
    fd_refs_[ptr->backend_fd()] = std::move(backend_ref);
    connections_[ptr] = std::move(conn);
}

void ProxyEngine::on_event(const FdRef& ref, std::uint32_t events) {
    ProxyConnection* conn = ref.conn;
    if (events & (EPOLLHUP | EPOLLERR)) {
        close_connection(*conn);
        return;
    }
    if (events & EPOLLIN) {
        if (!pump(*conn, ref.side)) {
            close_connection(*conn);
            return;
        }
    }
    if (events & EPOLLOUT) {
        const Side flush_from = ref.side == Side::Client ? Side::Backend : Side::Client;
        if (!pump(*conn, flush_from)) {
            close_connection(*conn);
            return;
        }
    }
    update_epoll(*conn);
}

bool ProxyEngine::pump(ProxyConnection& conn, Side from) {
    const int src_fd = from == Side::Client ? conn.client_fd() : conn.backend_fd();
    const int dst_fd = from == Side::Client ? conn.backend_fd() : conn.client_fd();
    Buffer& buffer = from == Side::Client ? conn.client_to_backend() : conn.backend_to_client();

    char temp[16384];
    while (true) {
        const ssize_t n = ::recv(src_fd, temp, sizeof(temp), 0);
        if (n > 0) {
            buffer.write(temp, static_cast<std::size_t>(n));
        } else if (n == 0) {
            if (buffer.empty()) {
                return false;
            }
            break;
        } else {
            if (errno == EAGAIN || errno == EWOULDBLOCK) {
                break;
            }
            return false;
        }
    }

    while (!buffer.empty()) {
        const ssize_t sent = ::send(dst_fd, buffer.read_ptr(), buffer.readable(), 0);
        if (sent > 0) {
            buffer.advance_read(static_cast<std::size_t>(sent));
            if (from == Side::Client) {
                conn.add_bytes_in(static_cast<std::uint64_t>(sent));
                metrics_.add_bytes_in(conn.backend_info()->label(), static_cast<std::uint64_t>(sent));
            } else {
                conn.add_bytes_out(static_cast<std::uint64_t>(sent));
                metrics_.add_bytes_out(conn.backend_info()->label(), static_cast<std::uint64_t>(sent));
            }
        } else {
            if (errno == EAGAIN || errno == EWOULDBLOCK) {
                break;
            }
            return false;
        }
    }
    return true;
}

void ProxyEngine::update_epoll(ProxyConnection& conn) {
    std::uint32_t client_events = EPOLLIN;
    std::uint32_t backend_events = EPOLLIN;
    if (!conn.client_to_backend().empty()) {
        backend_events |= EPOLLOUT;
    }
    if (!conn.backend_to_client().empty()) {
        client_events |= EPOLLOUT;
    }
    auto client_it = fd_refs_.find(conn.client_fd());
    auto backend_it = fd_refs_.find(conn.backend_fd());
    if (client_it != fd_refs_.end()) {
        epoll_.modify(conn.client_fd(), client_events, client_it->second.get());
    }
    if (backend_it != fd_refs_.end()) {
        epoll_.modify(conn.backend_fd(), backend_events, backend_it->second.get());
    }
}

void ProxyEngine::close_connection(ProxyConnection& conn) {
    const double age = conn.age_seconds();
    epoll_.remove(conn.client_fd());
    epoll_.remove(conn.backend_fd());
    fd_refs_.erase(conn.client_fd());
    fd_refs_.erase(conn.backend_fd());
    conn.set_state(ConnState::Closed);
    metrics_.on_connection_close(age);
    connections_.erase(&conn);
}

}
