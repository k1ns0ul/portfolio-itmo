#pragma once

#include <cstddef>
#include <string>
#include <string_view>

#include "matching_engine.hpp"

namespace me {

class Session {
public:
    Session(int fd, MatchingEngine& engine);

    void on_data(const char* data, std::size_t len);

    int fd() const noexcept { return fd_; }
    bool has_pending_write() const noexcept { return !write_buffer_.empty(); }
    const std::string& write_buffer() const noexcept { return write_buffer_; }
    void consume_write(std::size_t n);

private:
    std::string process_line(std::string_view line);

    int fd_;
    std::string read_buffer_;
    std::string write_buffer_;
    MatchingEngine& engine_;
};

}
