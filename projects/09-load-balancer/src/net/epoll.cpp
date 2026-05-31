#include "epoll.hpp"

#include <unistd.h>

namespace lb {

namespace {
constexpr std::size_t kMaxEvents = 1024;
}

Epoll::Epoll() : events_(kMaxEvents) {
    fd_ = ::epoll_create1(0);
}

Epoll::~Epoll() {
    if (fd_ >= 0) {
        ::close(fd_);
    }
}

bool Epoll::add(int fd, std::uint32_t events, void* data) {
    epoll_event ev{};
    ev.events = events;
    ev.data.ptr = data;
    return ::epoll_ctl(fd_, EPOLL_CTL_ADD, fd, &ev) == 0;
}

bool Epoll::modify(int fd, std::uint32_t events, void* data) {
    epoll_event ev{};
    ev.events = events;
    ev.data.ptr = data;
    return ::epoll_ctl(fd_, EPOLL_CTL_MOD, fd, &ev) == 0;
}

bool Epoll::remove(int fd) {
    return ::epoll_ctl(fd_, EPOLL_CTL_DEL, fd, nullptr) == 0;
}

std::span<epoll_event> Epoll::wait(int timeout_ms) {
    const int n = ::epoll_wait(fd_, events_.data(), static_cast<int>(events_.size()), timeout_ms);
    if (n <= 0) {
        return {};
    }
    return std::span<epoll_event>(events_.data(), static_cast<std::size_t>(n));
}

}
