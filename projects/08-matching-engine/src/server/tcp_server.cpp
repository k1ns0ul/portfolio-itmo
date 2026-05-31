#include "tcp_server.hpp"

#include <arpa/inet.h>
#include <fcntl.h>
#include <netinet/in.h>
#include <sys/epoll.h>
#include <sys/socket.h>
#include <unistd.h>

#include <array>
#include <cerrno>
#include <cstring>
#include <stdexcept>
#include <string>

namespace me {

namespace {

constexpr int kMaxEvents = 256;

void set_nonblocking(int fd) {
    const int flags = ::fcntl(fd, F_GETFL, 0);
    if (flags < 0 || ::fcntl(fd, F_SETFL, flags | O_NONBLOCK) < 0) {
        throw std::runtime_error(std::string("fcntl O_NONBLOCK failed: ") + std::strerror(errno));
    }
}

}

TcpServer::TcpServer(MatchingEngine& engine, TcpServerConfig config) : engine_(engine), config_(config) {
    setup();
}

TcpServer::~TcpServer() {
    for (auto& [fd, session] : sessions_) {
        ::close(fd);
    }
    if (listen_fd_ >= 0) {
        ::close(listen_fd_);
    }
    if (epoll_fd_ >= 0) {
        ::close(epoll_fd_);
    }
    if (wake_pipe_[0] >= 0) {
        ::close(wake_pipe_[0]);
    }
    if (wake_pipe_[1] >= 0) {
        ::close(wake_pipe_[1]);
    }
}

void TcpServer::setup() {
    listen_fd_ = ::socket(AF_INET, SOCK_STREAM, 0);
    if (listen_fd_ < 0) {
        throw std::runtime_error(std::string("socket failed: ") + std::strerror(errno));
    }
    int opt = 1;
    ::setsockopt(listen_fd_, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));

    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = htonl(INADDR_ANY);
    addr.sin_port = htons(config_.port);

    if (::bind(listen_fd_, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) < 0) {
        throw std::runtime_error(std::string("bind failed: ") + std::strerror(errno));
    }
    if (::listen(listen_fd_, config_.backlog) < 0) {
        throw std::runtime_error(std::string("listen failed: ") + std::strerror(errno));
    }
    set_nonblocking(listen_fd_);

    if (::pipe(wake_pipe_) < 0) {
        throw std::runtime_error(std::string("pipe failed: ") + std::strerror(errno));
    }
    set_nonblocking(wake_pipe_[0]);

    epoll_fd_ = ::epoll_create1(0);
    if (epoll_fd_ < 0) {
        throw std::runtime_error(std::string("epoll_create1 failed: ") + std::strerror(errno));
    }

    epoll_event ev{};
    ev.events = EPOLLIN;
    ev.data.fd = listen_fd_;
    ::epoll_ctl(epoll_fd_, EPOLL_CTL_ADD, listen_fd_, &ev);

    epoll_event wake_ev{};
    wake_ev.events = EPOLLIN;
    wake_ev.data.fd = wake_pipe_[0];
    ::epoll_ctl(epoll_fd_, EPOLL_CTL_ADD, wake_pipe_[0], &wake_ev);
}

void TcpServer::run() {
    std::array<epoll_event, kMaxEvents> events{};
    bool running = true;

    while (running) {
        const int n = ::epoll_wait(epoll_fd_, events.data(), kMaxEvents, -1);
        if (n < 0) {
            if (errno == EINTR) {
                continue;
            }
            break;
        }
        for (int i = 0; i < n; ++i) {
            const int fd = events[i].data.fd;
            if (fd == wake_pipe_[0]) {
                running = false;
                break;
            }
            if (fd == listen_fd_) {
                accept_connections();
                continue;
            }
            if (events[i].events & (EPOLLHUP | EPOLLERR)) {
                close_session(fd);
                continue;
            }
            if (events[i].events & EPOLLIN) {
                handle_readable(fd);
            }
        }
    }
}

void TcpServer::stop() {
    if (wake_pipe_[1] >= 0) {
        const char byte = 'x';
        ssize_t ignored = ::write(wake_pipe_[1], &byte, 1);
        (void)ignored;
    }
}

void TcpServer::accept_connections() {
    while (true) {
        sockaddr_in client{};
        socklen_t len = sizeof(client);
        const int fd = ::accept(listen_fd_, reinterpret_cast<sockaddr*>(&client), &len);
        if (fd < 0) {
            if (errno == EAGAIN || errno == EWOULDBLOCK) {
                break;
            }
            break;
        }
        if (static_cast<int>(sessions_.size()) >= config_.max_connections) {
            ::close(fd);
            continue;
        }
        set_nonblocking(fd);

        epoll_event ev{};
        ev.events = EPOLLIN;
        ev.data.fd = fd;
        ::epoll_ctl(epoll_fd_, EPOLL_CTL_ADD, fd, &ev);
        sessions_.emplace(fd, std::make_unique<Session>(fd, engine_));
    }
}

void TcpServer::handle_readable(int fd) {
    auto it = sessions_.find(fd);
    if (it == sessions_.end()) {
        return;
    }
    Session& session = *it->second;

    std::array<char, 16384> buffer{};
    while (true) {
        const ssize_t got = ::read(fd, buffer.data(), buffer.size());
        if (got > 0) {
            session.on_data(buffer.data(), static_cast<std::size_t>(got));
            continue;
        }
        if (got == 0) {
            close_session(fd);
            return;
        }
        if (errno == EAGAIN || errno == EWOULDBLOCK) {
            break;
        }
        close_session(fd);
        return;
    }

    flush_writes(session);
}

void TcpServer::flush_writes(Session& session) {
    while (session.has_pending_write()) {
        const std::string& data = session.write_buffer();
        const ssize_t sent = ::write(session.fd(), data.data(), data.size());
        if (sent > 0) {
            session.consume_write(static_cast<std::size_t>(sent));
            continue;
        }
        if (sent < 0 && (errno == EAGAIN || errno == EWOULDBLOCK)) {
            break;
        }
        close_session(session.fd());
        return;
    }
}

void TcpServer::close_session(int fd) {
    ::epoll_ctl(epoll_fd_, EPOLL_CTL_DEL, fd, nullptr);
    ::close(fd);
    sessions_.erase(fd);
}

}
