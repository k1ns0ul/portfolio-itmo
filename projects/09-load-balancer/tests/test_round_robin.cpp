#include <gtest/gtest.h>

#include <map>
#include <memory>

#include "backend_pool.hpp"
#include "round_robin.hpp"

using namespace lb;

namespace {

BackendPtr make_backend(const std::string& label, bool healthy = true) {
    auto addr = Address::from_string(label);
    auto backend = std::make_shared<Backend>(*addr, label, 1, 0);
    backend->set_healthy(healthy);
    return backend;
}

}

TEST(RoundRobin, EvenDistribution) {
    BackendPool pool;
    pool.add(make_backend("127.0.0.1:3001"));
    pool.add(make_backend("127.0.0.1:3002"));
    pool.add(make_backend("127.0.0.1:3003"));

    RoundRobinStrategy strategy;
    std::map<std::string, int> counts;
    for (int i = 0; i < 9; ++i) {
        auto backend = strategy.select(pool, "10.0.0.1");
        ASSERT_TRUE(backend != nullptr);
        counts[backend->label()] += 1;
    }
    EXPECT_EQ(counts.size(), 3u);
    for (const auto& [label, n] : counts) {
        EXPECT_EQ(n, 3);
    }
}

TEST(RoundRobin, SkipsUnhealthy) {
    BackendPool pool;
    pool.add(make_backend("127.0.0.1:3001"));
    pool.add(make_backend("127.0.0.1:3002", false));
    pool.add(make_backend("127.0.0.1:3003"));

    RoundRobinStrategy strategy;
    for (int i = 0; i < 10; ++i) {
        auto backend = strategy.select(pool, "10.0.0.1");
        ASSERT_TRUE(backend != nullptr);
        EXPECT_NE(backend->label(), "127.0.0.1:3002");
    }
}

TEST(RoundRobin, NullWhenAllDown) {
    BackendPool pool;
    pool.add(make_backend("127.0.0.1:3001", false));
    RoundRobinStrategy strategy;
    EXPECT_EQ(strategy.select(pool, "10.0.0.1"), nullptr);
}
