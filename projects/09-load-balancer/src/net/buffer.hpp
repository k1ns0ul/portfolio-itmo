#pragma once

#include <cstddef>
#include <vector>

#include "../core/types.hpp"

namespace lb {

class Buffer {
public:
    explicit Buffer(std::size_t capacity = kDefaultBufferSize);

    std::size_t write(const char* data, std::size_t len);
    std::size_t read(char* out, std::size_t len);

    std::size_t readable() const noexcept { return write_pos_ - read_pos_; }
    std::size_t writable() const noexcept { return data_.size() - write_pos_; }
    bool empty() const noexcept { return read_pos_ == write_pos_; }
    bool full() const noexcept { return write_pos_ == data_.size() && read_pos_ == 0; }

    char* write_ptr() noexcept { return data_.data() + write_pos_; }
    const char* read_ptr() const noexcept { return data_.data() + read_pos_; }
    void advance_write(std::size_t n) noexcept { write_pos_ += n; }
    void advance_read(std::size_t n) noexcept;

    void compact();
    void clear() noexcept;
    std::size_t capacity() const noexcept { return data_.size(); }

private:
    std::vector<char> data_;
    std::size_t read_pos_ = 0;
    std::size_t write_pos_ = 0;
};

}
