#include <gtest/gtest.h>

#include <memory>
#include <set>
#include <string>

#include "backend_pool.hpp"
#include "ip_hash.hpp"

using namespace lb;

namespace {

BackendPtr make_backend(const std::string& label) {
    auto addr = Address::from_string(label);
    auto backend = std::make_shared<Backend>(*addr, label, 1, 0);
    backend->set_healthy(true);
    return backend;
}

}

TEST(IpHash, StickyForSameClient) {
    BackendPool pool;
    pool.add(make_backend("127.0.0.1:3001"));
    pool.add(make_backend("127.0.0.1:3002"));
    pool.add(make_backend("127.0.0.1:3003"));

    IpHashStrategy strategy;
    auto first = strategy.select(pool, "203.0.113.7");
    ASSERT_TRUE(first != nullptr);
    for (int i = 0; i < 20; ++i) {
        EXPECT_EQ(strategy.select(pool, "203.0.113.7")->label(), first->label());
    }
}

TEST(IpHash, DifferentClientsSpread) {
    BackendPool pool;
    pool.add(make_backend("127.0.0.1:3001"));
    pool.add(make_backend("127.0.0.1:3002"));
    pool.add(make_backend("127.0.0.1:3003"));

    IpHashStrategy strategy;
    std::set<std::string> seen;
    for (int i = 0; i < 50; ++i) {
        seen.insert(strategy.select(pool, "10.0.0." + std::to_string(i))->label());
    }
    EXPECT_GT(seen.size(), 1u);
}

TEST(IpHash, RedistributesOnRemoval) {
    BackendPool pool;
    pool.add(make_backend("127.0.0.1:3001"));
    pool.add(make_backend("127.0.0.1:3002"));

    IpHashStrategy strategy;
    auto before = strategy.select(pool, "198.51.100.5")->label();
    pool.remove(before);
    auto after = strategy.select(pool, "198.51.100.5");
    ASSERT_TRUE(after != nullptr);
    EXPECT_NE(after->label(), before);
}
