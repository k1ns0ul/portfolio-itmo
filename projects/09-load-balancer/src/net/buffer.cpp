#include "buffer.hpp"

#include <algorithm>
#include <cstring>

namespace lb {

Buffer::Buffer(std::size_t capacity) : data_(capacity) {}

void Buffer::compact() {
    if (read_pos_ == 0) {
        return;
    }
    const std::size_t pending = readable();
    if (pending > 0) {
        std::memmove(data_.data(), data_.data() + read_pos_, pending);
    }
    read_pos_ = 0;
    write_pos_ = pending;
}

std::size_t Buffer::write(const char* data, std::size_t len) {
    if (writable() < len) {
        compact();
    }
    const std::size_t n = std::min(len, writable());
    if (n > 0) {
        std::memcpy(data_.data() + write_pos_, data, n);
        write_pos_ += n;
    }
    return n;
}

std::size_t Buffer::read(char* out, std::size_t len) {
    const std::size_t n = std::min(len, readable());
    if (n > 0) {
        std::memcpy(out, data_.data() + read_pos_, n);
        read_pos_ += n;
    }
    if (read_pos_ == write_pos_) {
        clear();
    }
    return n;
}

void Buffer::advance_read(std::size_t n) noexcept {
    read_pos_ += n;
    if (read_pos_ >= write_pos_) {
        clear();
    }
}

void Buffer::clear() noexcept {
    read_pos_ = 0;
    write_pos_ = 0;
}

}
