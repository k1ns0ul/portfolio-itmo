#include "address.hpp"

#include <arpa/inet.h>

#include <charconv>

namespace lb {

std::optional<Address> Address::from_string(std::string_view host_port) {
    const std::size_t colon = host_port.rfind(':');
    if (colon == std::string_view::npos || colon == 0 || colon + 1 >= host_port.size()) {
        return std::nullopt;
    }
    const std::string_view host = host_port.substr(0, colon);
    const std::string_view port_sv = host_port.substr(colon + 1);

    int port = 0;
    const auto [ptr, ec] = std::from_chars(port_sv.data(), port_sv.data() + port_sv.size(), port);
    if (ec != std::errc{} || ptr != port_sv.data() + port_sv.size() || port < 1 || port > 65535) {
        return std::nullopt;
    }

    Address result;
    result.addr_.sin_family = AF_INET;
    result.addr_.sin_port = htons(static_cast<std::uint16_t>(port));

    const std::string host_str(host);
    if (host_str == "*" || host_str == "0.0.0.0") {
        result.addr_.sin_addr.s_addr = htonl(INADDR_ANY);
    } else if (::inet_pton(AF_INET, host_str.c_str(), &result.addr_.sin_addr) != 1) {
        return std::nullopt;
    }
    return result;
}

std::string Address::host() const {
    char buf[INET_ADDRSTRLEN]{};
    ::inet_ntop(AF_INET, &addr_.sin_addr, buf, sizeof(buf));
    return std::string(buf);
}

std::uint16_t Address::port() const noexcept {
    return ntohs(addr_.sin_port);
}

std::string Address::to_string() const {
    return host() + ":" + std::to_string(port());
}

}
