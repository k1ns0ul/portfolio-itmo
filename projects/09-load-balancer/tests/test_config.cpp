#include <gtest/gtest.h>

#include <string>

#include "config.hpp"

using namespace lb;

TEST(Config, ParsesFullExample) {
    const std::string text =
        "listen: 0.0.0.0:8080\n"
        "strategy: weighted\n"
        "health_check:\n"
        "  interval: 10s\n"
        "  timeout: 3s\n"
        "  type: http\n"
        "  threshold: 5\n"
        "metrics:\n"
        "  enabled: true\n"
        "  port: 9100\n"
        "backends:\n"
        "  - address: 127.0.0.1:3001\n"
        "    weight: 3\n"
        "  - address: 127.0.0.1:3002\n"
        "    weight: 1\n"
        "drain_timeout: 45s\n";

    std::string error;
    auto cfg = Config::from_string(text, error);
    ASSERT_TRUE(cfg.has_value()) << error;
    EXPECT_EQ(cfg->listen, "0.0.0.0:8080");
    EXPECT_EQ(cfg->strategy, Strategy::Weighted);
    EXPECT_EQ(cfg->health_check.type, HealthCheckType::Http);
    EXPECT_EQ(cfg->health_check.threshold, 5);
    EXPECT_EQ(cfg->health_check.interval, std::chrono::milliseconds(10000));
    EXPECT_TRUE(cfg->metrics.enabled);
    EXPECT_EQ(cfg->metrics.port, 9100);
    ASSERT_EQ(cfg->backends.size(), 2u);
    EXPECT_EQ(cfg->backends[0].address, "127.0.0.1:3001");
    EXPECT_EQ(cfg->backends[0].weight, 3);
    EXPECT_EQ(cfg->drain_timeout, std::chrono::milliseconds(45000));
}

TEST(Config, AppliesDefaults) {
    const std::string text =
        "backends:\n"
        "  - address: 127.0.0.1:3001\n";
    std::string error;
    auto cfg = Config::from_string(text, error);
    ASSERT_TRUE(cfg.has_value()) << error;
    EXPECT_EQ(cfg->strategy, Strategy::RoundRobin);
    EXPECT_EQ(cfg->health_check.threshold, 3);
    EXPECT_EQ(cfg->backends[0].weight, 1);
}

TEST(Config, RejectsNoBackends) {
    const std::string text = "strategy: round_robin\n";
    std::string error;
    auto cfg = Config::from_string(text, error);
    EXPECT_FALSE(cfg.has_value());
    EXPECT_FALSE(error.empty());
}

TEST(Config, RejectsUnknownStrategy) {
    const std::string text =
        "strategy: magic\n"
        "backends:\n"
        "  - address: 127.0.0.1:3001\n";
    std::string error;
    EXPECT_FALSE(Config::from_string(text, error).has_value());
}

TEST(Duration, ParsesUnits) {
    EXPECT_EQ(parse_duration("5s"), std::chrono::milliseconds(5000));
    EXPECT_EQ(parse_duration("2m"), std::chrono::milliseconds(120000));
    EXPECT_EQ(parse_duration("1h"), std::chrono::milliseconds(3600000));
    EXPECT_EQ(parse_duration("500"), std::chrono::milliseconds(500));
    EXPECT_FALSE(parse_duration("abc").has_value());
}
