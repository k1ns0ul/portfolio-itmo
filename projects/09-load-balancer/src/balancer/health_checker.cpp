#include "health_checker.hpp"

#include <fcntl.h>
#include <netinet/in.h>
#include <poll.h>
#include <sys/socket.h>
#include <unistd.h>

#include <cerrno>
#include <cstring>

#include "../core/logger.hpp"
#include "../metrics/collector.hpp"
#include "../net/socket.hpp"

namespace lb {

namespace {

bool connect_with_timeout(const Backend& backend, int timeout_ms, Socket& out_sock) {
    Socket sock = Socket::create_tcp();
    if (!sock.valid()) {
        return false;
    }
    sock.set_nonblocking();

    const sockaddr_in& raw = backend.address().raw();
    const int rc = ::connect(sock.fd(), reinterpret_cast<const sockaddr*>(&raw), sizeof(raw));
    if (rc == 0) {
        out_sock = std::move(sock);
        return true;
    }
    if (errno != EINPROGRESS) {
        return false;
    }

    pollfd pfd{};
    pfd.fd = sock.fd();
    pfd.events = POLLOUT;
    const int ready = ::poll(&pfd, 1, timeout_ms);
    if (ready <= 0) {
        return false;
    }

    int err = 0;
    socklen_t len = sizeof(err);
    if (::getsockopt(sock.fd(), SOL_SOCKET, SO_ERROR, &err, &len) != 0 || err != 0) {
        return false;
    }
    out_sock = std::move(sock);
    return true;
}

bool http_probe(Socket& sock, const std::string& path, int timeout_ms) {
    std::string request = "GET " + path + " HTTP/1.0\r\n\r\n";
    if (::send(sock.fd(), request.data(), request.size(), 0) < 0) {
        return false;
    }

    pollfd pfd{};
    pfd.fd = sock.fd();
    pfd.events = POLLIN;
    if (::poll(&pfd, 1, timeout_ms) <= 0) {
        return false;
    }

    char buf[64]{};
    const ssize_t n = ::recv(sock.fd(), buf, sizeof(buf) - 1, 0);
    if (n <= 0) {
        return false;
    }
    const std::string_view response(buf, static_cast<std::size_t>(n));
    return response.rfind("HTTP/1.", 0) == 0 && response.find(" 200") != std::string_view::npos;
}

}

HealthChecker::HealthChecker(BackendPool& pool, HealthCheckConfig config, MetricsCollector* metrics)
    : pool_(pool), config_(config), metrics_(metrics) {}

HealthChecker::~HealthChecker() {
    stop();
}

void HealthChecker::start() {
    running_.store(true, std::memory_order_release);
    thread_ = std::thread(&HealthChecker::run, this);
}

void HealthChecker::stop() {
    running_.store(false, std::memory_order_release);
    if (thread_.joinable()) {
        thread_.join();
    }
}

void HealthChecker::run() {
    while (running_.load(std::memory_order_acquire)) {
        check_once();
        const auto deadline = Clock::now() + config_.interval;
        while (running_.load(std::memory_order_acquire) && Clock::now() < deadline) {
            std::this_thread::sleep_for(std::chrono::milliseconds(100));
        }
    }
}

void HealthChecker::check_once() {
    for (const auto& backend : pool_.snapshot()) {
        evaluate(backend);
    }
}

bool HealthChecker::probe(const Backend& backend) const {
    Socket sock;
    const int timeout_ms = static_cast<int>(config_.timeout.count());
    if (!connect_with_timeout(backend, timeout_ms, sock)) {
        return false;
    }
    if (config_.type == HealthCheckType::Http) {
        return http_probe(sock, config_.http_path, timeout_ms);
    }
    return true;
}

void HealthChecker::evaluate(const BackendPtr& backend) {
    const auto start = Clock::now();
    const bool ok = probe(*backend);
    const auto elapsed = std::chrono::duration_cast<std::chrono::microseconds>(Clock::now() - start);
    backend->last_health_check = Clock::now();

    if (metrics_ != nullptr) {
        metrics_->record_health_check(backend->label(), ok);
    }

    if (ok) {
        backend->set_response_time(elapsed.count());
        if (!backend->healthy()) {
            backend->set_healthy(true);
            backend->consecutive_failures = 0;
            Logger::instance().info("backend recovered", {{"backend", backend->label()}});
        } else {
            backend->consecutive_failures = 0;
        }
        return;
    }

    backend->consecutive_failures += 1;
    if (backend->healthy() && backend->consecutive_failures >= config_.threshold) {
        backend->set_healthy(false);
        Logger::instance().warn("backend marked down",
                                {{"backend", backend->label()},
                                 {"failures", field_int(backend->consecutive_failures)}});
    }
}

}
