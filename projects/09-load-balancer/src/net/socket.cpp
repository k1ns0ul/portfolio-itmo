#include "socket.hpp"

#include <fcntl.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <sys/socket.h>
#include <unistd.h>

namespace lb {

void Socket::reset() noexcept {
    if (fd_ >= 0) {
        ::close(fd_);
        fd_ = -1;
    }
}

Socket Socket::create_tcp() {
    return Socket(::socket(AF_INET, SOCK_STREAM, 0));
}

bool Socket::set_nonblocking() {
    const int flags = ::fcntl(fd_, F_GETFL, 0);
    if (flags < 0) {
        return false;
    }
    return ::fcntl(fd_, F_SETFL, flags | O_NONBLOCK) == 0;
}

bool Socket::set_reuseaddr() {
    int opt = 1;
    return ::setsockopt(fd_, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt)) == 0;
}

bool Socket::set_nodelay() {
    int opt = 1;
    return ::setsockopt(fd_, IPPROTO_TCP, TCP_NODELAY, &opt, sizeof(opt)) == 0;
}

bool Socket::set_keepalive() {
    int opt = 1;
    return ::setsockopt(fd_, SOL_SOCKET, SO_KEEPALIVE, &opt, sizeof(opt)) == 0;
}

std::optional<Socket> Socket::listen_on(const Address& addr, int backlog) {
    Socket sock = create_tcp();
    if (!sock.valid()) {
        return std::nullopt;
    }
    sock.set_reuseaddr();
    sock.set_nonblocking();

    const sockaddr_in& raw = addr.raw();
    if (::bind(sock.fd(), reinterpret_cast<const sockaddr*>(&raw), sizeof(raw)) != 0) {
        return std::nullopt;
    }
    if (::listen(sock.fd(), backlog) != 0) {
        return std::nullopt;
    }
    return sock;
}

std::optional<Socket> Socket::accept(Address& peer) {
    sockaddr_in raw{};
    socklen_t len = sizeof(raw);
    const int client = ::accept(fd_, reinterpret_cast<sockaddr*>(&raw), &len);
    if (client < 0) {
        return std::nullopt;
    }
    peer.raw() = raw;
    return Socket(client);
}

}
