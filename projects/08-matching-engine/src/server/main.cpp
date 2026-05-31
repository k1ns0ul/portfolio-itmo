#include <csignal>
#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <iostream>
#include <string>
#include <string_view>

#include "matching_engine.hpp"
#include "tcp_server.hpp"

namespace {

me::TcpServer* g_server = nullptr;

void handle_signal(int) {
    if (g_server != nullptr) {
        g_server->stop();
    }
}

std::uint16_t parse_port(std::string_view value, std::uint16_t fallback) {
    try {
        const int parsed = std::stoi(std::string(value));
        if (parsed > 0 && parsed <= 65535) {
            return static_cast<std::uint16_t>(parsed);
        }
    } catch (...) {
    }
    return fallback;
}

}

int main(int argc, char** argv) {
    me::TcpServerConfig config;

    for (int i = 1; i < argc; ++i) {
        const std::string_view arg = argv[i];
        if (arg == "--port" && i + 1 < argc) {
            config.port = parse_port(argv[++i], config.port);
        } else if (arg == "--max-connections" && i + 1 < argc) {
            config.max_connections = std::atoi(argv[++i]);
        }
    }

    me::MatchingEngine engine;
    me::TcpServer server(engine, config);
    g_server = &server;

    std::signal(SIGINT, handle_signal);
    std::signal(SIGTERM, handle_signal);
    std::signal(SIGPIPE, SIG_IGN);

    std::cout << "matching_server listening on port " << config.port << '\n';
    server.run();
    std::cout << "matching_server shutting down\n";

    g_server = nullptr;
    return 0;
}
