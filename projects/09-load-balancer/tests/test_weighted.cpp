#include <gtest/gtest.h>

#include <map>
#include <memory>

#include "backend_pool.hpp"
#include "weighted.hpp"

using namespace lb;

namespace {

BackendPtr make_backend(const std::string& label, int weight) {
    auto addr = Address::from_string(label);
    auto backend = std::make_shared<Backend>(*addr, label, weight, 0);
    backend->set_healthy(true);
    return backend;
}

}

TEST(Weighted, RespectsWeightRatio) {
    BackendPool pool;
    pool.add(make_backend("127.0.0.1:3001", 3));
    pool.add(make_backend("127.0.0.1:3002", 1));

    WeightedStrategy strategy;
    std::map<std::string, int> counts;
    const int total = 4000;
    for (int i = 0; i < total; ++i) {
        auto backend = strategy.select(pool, "10.0.0.1");
        counts[backend->label()] += 1;
    }

    const double heavy_ratio = static_cast<double>(counts["127.0.0.1:3001"]) / total;
    const double light_ratio = static_cast<double>(counts["127.0.0.1:3002"]) / total;
    EXPECT_NEAR(heavy_ratio, 0.75, 0.05);
    EXPECT_NEAR(light_ratio, 0.25, 0.05);
}
