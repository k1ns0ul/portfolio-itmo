#include "connection.hpp"

#include <chrono>
#include <utility>

namespace lb {

ProxyConnection::ProxyConnection(Socket client, Socket backend, BackendPtr backend_info, std::string client_ip)
    : client_(std::move(client)),
      backend_(std::move(backend)),
      backend_info_(std::move(backend_info)),
      client_ip_(std::move(client_ip)),
      state_(ConnState::Connecting),
      created_at_(Clock::now()) {
    if (backend_info_) {
        backend_info_->inc_active();
        backend_info_->add_connection();
    }
}

ProxyConnection::~ProxyConnection() {
    if (backend_info_) {
        backend_info_->dec_active();
        backend_info_->add_bytes(bytes_in_ + bytes_out_);
    }
}

double ProxyConnection::age_seconds() const {
    return std::chrono::duration<double>(Clock::now() - created_at_).count();
}

}
