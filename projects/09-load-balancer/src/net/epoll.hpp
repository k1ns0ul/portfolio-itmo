#pragma once

#include <sys/epoll.h>

#include <cstdint>
#include <span>
#include <utility>
#include <vector>

namespace lb {

class Epoll {
public:
    Epoll();
    ~Epoll();

    Epoll(const Epoll&) = delete;
    Epoll& operator=(const Epoll&) = delete;
    Epoll(Epoll&& other) noexcept : fd_(std::exchange(other.fd_, -1)), events_(std::move(other.events_)) {}

    bool valid() const noexcept { return fd_ >= 0; }

    bool add(int fd, std::uint32_t events, void* data);
    bool modify(int fd, std::uint32_t events, void* data);
    bool remove(int fd);

    std::span<epoll_event> wait(int timeout_ms);

private:
    int fd_ = -1;
    std::vector<epoll_event> events_;
};

}
