#pragma once

#include <netinet/in.h>

#include <cstdint>
#include <optional>
#include <string>
#include <string_view>

namespace lb {

class Address {
public:
    Address() = default;

    static std::optional<Address> from_string(std::string_view host_port);

    std::string to_string() const;
    std::string host() const;
    std::uint16_t port() const noexcept;

    const sockaddr_in& raw() const noexcept { return addr_; }
    sockaddr_in& raw() noexcept { return addr_; }

private:
    sockaddr_in addr_{};
};

}
