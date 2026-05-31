#include "prometheus.hpp"

#include <netinet/in.h>
#include <poll.h>
#include <sys/socket.h>
#include <unistd.h>

#include <cstring>
#include <string>

#include "../balancer/backend_pool.hpp"
#include "../core/logger.hpp"
#include "../net/socket.hpp"

namespace lb {

PrometheusExporter::PrometheusExporter(const MetricsCollector& collector, const BackendPool& pool, std::uint16_t port)
    : collector_(collector), pool_(pool), port_(port) {}

PrometheusExporter::~PrometheusExporter() {
    stop();
}

bool PrometheusExporter::start() {
    auto addr = Address::from_string("0.0.0.0:" + std::to_string(port_));
    if (!addr) {
        return false;
    }
    auto sock = Socket::listen_on(*addr, 16);
    if (!sock) {
        Logger::instance().error("metrics exporter failed to bind", {{"port", std::to_string(port_)}});
        return false;
    }
    listen_fd_ = sock->release();
    running_.store(true, std::memory_order_release);
    thread_ = std::thread(&PrometheusExporter::run, this);
    return true;
}

void PrometheusExporter::stop() {
    running_.store(false, std::memory_order_release);
    if (listen_fd_ >= 0) {
        ::shutdown(listen_fd_, SHUT_RDWR);
        ::close(listen_fd_);
        listen_fd_ = -1;
    }
    if (thread_.joinable()) {
        thread_.join();
    }
}

void PrometheusExporter::run() {
    while (running_.load(std::memory_order_acquire)) {
        pollfd pfd{};
        pfd.fd = listen_fd_;
        pfd.events = POLLIN;
        const int ready = ::poll(&pfd, 1, 200);
        if (ready <= 0) {
            continue;
        }
        const int client = ::accept(listen_fd_, nullptr, nullptr);
        if (client < 0) {
            continue;
        }
        handle_client(client);
        ::close(client);
    }
}

void PrometheusExporter::handle_client(int client_fd) {
    char buf[1024];
    const ssize_t got = ::recv(client_fd, buf, sizeof(buf), 0);
    (void)got;

    for (const auto& backend : pool_.snapshot()) {
        const_cast<MetricsCollector&>(collector_).set_backend_health(backend->label(), backend->healthy());
        const_cast<MetricsCollector&>(collector_).set_backend_response_time(backend->label(),
                                                                            backend->response_time_us());
    }

    const std::string body = collector_.render_prometheus();
    std::string response;
    response.reserve(body.size() + 128);
    response += "HTTP/1.1 200 OK\r\n";
    response += "Content-Type: text/plain; version=0.0.4\r\n";
    response += "Content-Length: " + std::to_string(body.size()) + "\r\n";
    response += "Connection: close\r\n\r\n";
    response += body;

    std::size_t sent = 0;
    while (sent < response.size()) {
        const ssize_t n = ::send(client_fd, response.data() + sent, response.size() - sent, 0);
        if (n <= 0) {
            break;
        }
        sent += static_cast<std::size_t>(n);
    }
}

}
