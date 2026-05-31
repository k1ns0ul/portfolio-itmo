#include <gtest/gtest.h>

#include <cstring>
#include <string>

#include "buffer.hpp"

using lb::Buffer;

TEST(Buffer, WriteThenRead) {
    Buffer buf(64);
    const char* data = "hello";
    EXPECT_EQ(buf.write(data, 5), 5u);
    EXPECT_EQ(buf.readable(), 5u);

    char out[16]{};
    EXPECT_EQ(buf.read(out, 16), 5u);
    EXPECT_EQ(std::string(out, 5), "hello");
    EXPECT_TRUE(buf.empty());
}

TEST(Buffer, PartialRead) {
    Buffer buf(64);
    buf.write("abcdef", 6);
    char out[3]{};
    EXPECT_EQ(buf.read(out, 3), 3u);
    EXPECT_EQ(std::string(out, 3), "abc");
    EXPECT_EQ(buf.readable(), 3u);
    EXPECT_EQ(buf.read(out, 3), 3u);
    EXPECT_EQ(std::string(out, 3), "def");
}

TEST(Buffer, OverflowIsBounded) {
    Buffer buf(8);
    const std::size_t written = buf.write("0123456789", 10);
    EXPECT_EQ(written, 8u);
    EXPECT_EQ(buf.readable(), 8u);
}

TEST(Buffer, CompactReclaimsSpace) {
    Buffer buf(8);
    buf.write("abcd", 4);
    char out[2]{};
    buf.read(out, 2);
    const std::size_t written = buf.write("efghij", 6);
    EXPECT_GE(written, 4u);
    EXPECT_LE(buf.readable(), buf.capacity());
}

TEST(Buffer, ClearResets) {
    Buffer buf(16);
    buf.write("data", 4);
    buf.clear();
    EXPECT_TRUE(buf.empty());
    EXPECT_EQ(buf.readable(), 0u);
    EXPECT_EQ(buf.writable(), 16u);
}

TEST(Buffer, AdvanceReadWrite) {
    Buffer buf(16);
    std::memcpy(buf.write_ptr(), "xyz", 3);
    buf.advance_write(3);
    EXPECT_EQ(buf.readable(), 3u);
    buf.advance_read(3);
    EXPECT_TRUE(buf.empty());
}
