#include <gtest/gtest.h>

#include "address.hpp"

using lb::Address;

TEST(Address, ParsesHostPort) {
    auto addr = Address::from_string("127.0.0.1:8080");
    ASSERT_TRUE(addr.has_value());
    EXPECT_EQ(addr->host(), "127.0.0.1");
    EXPECT_EQ(addr->port(), 8080);
    EXPECT_EQ(addr->to_string(), "127.0.0.1:8080");
}

TEST(Address, ParsesWildcard) {
    auto addr = Address::from_string("0.0.0.0:9100");
    ASSERT_TRUE(addr.has_value());
    EXPECT_EQ(addr->port(), 9100);
}

TEST(Address, RejectsMissingPort) {
    EXPECT_FALSE(Address::from_string("127.0.0.1").has_value());
}

TEST(Address, RejectsEmptyPort) {
    EXPECT_FALSE(Address::from_string("127.0.0.1:").has_value());
}

TEST(Address, RejectsNonNumericPort) {
    EXPECT_FALSE(Address::from_string("127.0.0.1:abc").has_value());
}

TEST(Address, RejectsOutOfRangePort) {
    EXPECT_FALSE(Address::from_string("127.0.0.1:70000").has_value());
}

TEST(Address, RejectsBadHost) {
    EXPECT_FALSE(Address::from_string("999.999.999.999:80").has_value());
}
