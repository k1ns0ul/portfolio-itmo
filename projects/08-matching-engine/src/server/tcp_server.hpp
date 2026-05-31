#pragma once

#include <cstdint>
#include <memory>
#include <unordered_map>

#include "matching_engine.hpp"
#include "session.hpp"

namespace me {

struct TcpServerConfig {
    std::uint16_t port = 9999;
    int max_connections = 1024;
    int backlog = 512;
};

class TcpServer {
public:
    TcpServer(MatchingEngine& engine, TcpServerConfig config);
    ~TcpServer();

    TcpServer(const TcpServer&) = delete;
    TcpServer& operator=(const TcpServer&) = delete;

    void run();
    void stop();

private:
    void setup();
    void accept_connections();
    void handle_readable(int fd);
    void flush_writes(Session& session);
    void close_session(int fd);

    MatchingEngine& engine_;
    TcpServerConfig config_;
    int listen_fd_ = -1;
    int epoll_fd_ = -1;
    int wake_pipe_[2] = {-1, -1};
    std::unordered_map<int, std::unique_ptr<Session>> sessions_;
};

}
