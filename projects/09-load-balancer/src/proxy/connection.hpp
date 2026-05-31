#pragma once

#include <cstdint>

#include "../balancer/backend.hpp"
#include "../core/types.hpp"
#include "../net/buffer.hpp"
#include "../net/socket.hpp"

namespace lb {

enum class ConnState : std::uint8_t { Connecting, Active, Draining, Closed };

class ProxyConnection {
public:
    ProxyConnection(Socket client, Socket backend, BackendPtr backend_info, std::string client_ip);
    ~ProxyConnection();

    ProxyConnection(const ProxyConnection&) = delete;
    ProxyConnection& operator=(const ProxyConnection&) = delete;

    int client_fd() const noexcept { return client_.fd(); }
    int backend_fd() const noexcept { return backend_.fd(); }

    ConnState state() const noexcept { return state_; }
    void set_state(ConnState state) noexcept { state_ = state; }

    const BackendPtr& backend_info() const noexcept { return backend_info_; }
    const std::string& client_ip() const noexcept { return client_ip_; }

    Buffer& client_to_backend() noexcept { return client_to_backend_; }
    Buffer& backend_to_client() noexcept { return backend_to_client_; }

    double age_seconds() const;

    std::uint64_t bytes_in() const noexcept { return bytes_in_; }
    std::uint64_t bytes_out() const noexcept { return bytes_out_; }
    void add_bytes_in(std::uint64_t n) noexcept { bytes_in_ += n; }
    void add_bytes_out(std::uint64_t n) noexcept { bytes_out_ += n; }

private:
    Socket client_;
    Socket backend_;
    BackendPtr backend_info_;
    std::string client_ip_;
    ConnState state_;
    TimePoint created_at_;
    Buffer client_to_backend_;
    Buffer backend_to_client_;
    std::uint64_t bytes_in_ = 0;
    std::uint64_t bytes_out_ = 0;
};

}
