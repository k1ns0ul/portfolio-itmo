#pragma once

#include <optional>
#include <utility>

#include "address.hpp"

namespace lb {

class Socket {
public:
    Socket() noexcept = default;
    explicit Socket(int fd) noexcept : fd_(fd) {}

    Socket(const Socket&) = delete;
    Socket& operator=(const Socket&) = delete;

    Socket(Socket&& other) noexcept : fd_(std::exchange(other.fd_, -1)) {}
    Socket& operator=(Socket&& other) noexcept {
        if (this != &other) {
            reset();
            fd_ = std::exchange(other.fd_, -1);
        }
        return *this;
    }

    ~Socket() { reset(); }

    static Socket create_tcp();
    static std::optional<Socket> listen_on(const Address& addr, int backlog);

    int fd() const noexcept { return fd_; }
    bool valid() const noexcept { return fd_ >= 0; }
    int release() noexcept { return std::exchange(fd_, -1); }
    void reset() noexcept;

    bool set_nonblocking();
    bool set_reuseaddr();
    bool set_nodelay();
    bool set_keepalive();

    std::optional<Socket> accept(Address& peer);

private:
    int fd_ = -1;
};

}
