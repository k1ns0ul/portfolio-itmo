#include <gtest/gtest.h>

#include <memory>

#include "backend_pool.hpp"
#include "least_connections.hpp"

using namespace lb;

namespace {

BackendPtr make_backend(const std::string& label) {
    auto addr = Address::from_string(label);
    auto backend = std::make_shared<Backend>(*addr, label, 1, 0);
    backend->set_healthy(true);
    return backend;
}

}

TEST(LeastConnections, PicksIdleBackend) {
    BackendPool pool;
    auto busy = make_backend("127.0.0.1:3001");
    auto idle = make_backend("127.0.0.1:3002");
    busy->inc_active();
    busy->inc_active();
    pool.add(busy);
    pool.add(idle);

    LeastConnectionsStrategy strategy;
    auto chosen = strategy.select(pool, "10.0.0.1");
    ASSERT_TRUE(chosen != nullptr);
    EXPECT_EQ(chosen->label(), "127.0.0.1:3002");
}

TEST(LeastConnections, BalancesAsLoadShifts) {
    BackendPool pool;
    auto a = make_backend("127.0.0.1:3001");
    auto b = make_backend("127.0.0.1:3002");
    pool.add(a);
    pool.add(b);

    LeastConnectionsStrategy strategy;
    auto first = strategy.select(pool, "10.0.0.1");
    first->inc_active();
    auto second = strategy.select(pool, "10.0.0.1");
    EXPECT_NE(first->label(), second->label());
}
