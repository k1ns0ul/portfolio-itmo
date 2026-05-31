#include <algorithm>
#include <chrono>
#include <cstdint>
#include <iomanip>
#include <iostream>
#include <random>
#include <string>
#include <vector>

#include "matching_engine.hpp"
#include "order_book.hpp"

namespace {

using Clock = std::chrono::steady_clock;

constexpr int kWarmup = 1000;
constexpr int kMeasure = 100000;
constexpr int kInstruments = 10;
constexpr me::Price kBasePrice = 10000;

struct RandomGen {
    std::mt19937_64 rng{0x9e3779b97f4a7c15ULL};
    std::normal_distribution<double> price_dist{static_cast<double>(kBasePrice), 500.0};
    std::uniform_int_distribution<int> qty_dist{1, 100};
    std::uniform_int_distribution<int> inst_dist{0, kInstruments - 1};
    std::uniform_int_distribution<int> side_dist{0, 1};

    me::Price price() { return static_cast<me::Price>(price_dist(rng)); }
    me::Quantity qty() { return qty_dist(rng); }
    me::InstrumentId instrument() { return static_cast<me::InstrumentId>(inst_dist(rng)); }
    me::Side side() { return side_dist(rng) == 0 ? me::Side::Buy : me::Side::Sell; }
};

double ops_per_sec(int ops, std::chrono::nanoseconds elapsed) {
    const double seconds = std::chrono::duration<double>(elapsed).count();
    if (seconds <= 0.0) {
        return 0.0;
    }
    return static_cast<double>(ops) / seconds;
}

void print_throughput(const std::string& name, int ops, std::chrono::nanoseconds elapsed) {
    const double rate = ops_per_sec(ops, elapsed);
    std::cout << std::left << std::setw(28) << name << std::right << std::setw(14) << ops << " ops "
              << std::setw(16) << std::fixed << std::setprecision(0) << rate << " ops/sec\n";
}

double percentile(std::vector<double>& samples, double p) {
    if (samples.empty()) {
        return 0.0;
    }
    const std::size_t idx = static_cast<std::size_t>(p * (samples.size() - 1));
    return samples[idx];
}

void bench_insertion() {
    me::OrderBook book(1);
    RandomGen gen;
    for (int i = 0; i < kWarmup; ++i) {
        book.add_order(me::OrderType::Limit, me::Side::Buy, gen.price() - 2000, gen.qty());
    }

    const auto start = Clock::now();
    for (int i = 0; i < kMeasure; ++i) {
        book.add_order(me::OrderType::Limit, me::Side::Buy, gen.price() - 2000, gen.qty());
    }
    const auto elapsed = Clock::now() - start;
    print_throughput("insertion (no match)", kMeasure, elapsed);
}

void bench_matching() {
    me::OrderBook book(1);
    RandomGen gen;
    for (int i = 0; i < kWarmup; ++i) {
        book.add_order(me::OrderType::Limit, me::Side::Sell, kBasePrice, 100);
        book.add_order(me::OrderType::Limit, me::Side::Buy, kBasePrice, 100);
    }

    const auto start = Clock::now();
    for (int i = 0; i < kMeasure; ++i) {
        book.add_order(me::OrderType::Limit, me::Side::Sell, kBasePrice, 50);
        book.add_order(me::OrderType::Limit, me::Side::Buy, kBasePrice, 50);
    }
    const auto elapsed = Clock::now() - start;
    print_throughput("matching (crossing)", kMeasure * 2, elapsed);
}

void bench_cancel() {
    me::OrderBook book(1);
    RandomGen gen;
    std::vector<me::OrderId> ids;
    ids.reserve(kMeasure);
    for (int i = 0; i < kMeasure; ++i) {
        auto result = book.add_order(me::OrderType::Limit, me::Side::Buy, kBasePrice - 1000 - i % 500, gen.qty());
        ids.push_back(result.order_id);
    }

    const auto start = Clock::now();
    for (me::OrderId id : ids) {
        book.cancel_order(id);
    }
    const auto elapsed = Clock::now() - start;
    print_throughput("cancel", kMeasure, elapsed);
}

void bench_mixed() {
    me::MatchingEngine engine;
    RandomGen gen;
    std::vector<std::pair<me::InstrumentId, me::OrderId>> live;
    live.reserve(kMeasure);

    std::uniform_int_distribution<int> action{0, 99};
    const auto start = Clock::now();
    for (int i = 0; i < kMeasure; ++i) {
        const int roll = action(gen.rng);
        const me::InstrumentId inst = gen.instrument();
        if (roll < 60) {
            auto result = engine.submit_order(inst, me::OrderType::Limit, gen.side(), gen.price(), gen.qty());
            if (result.status == me::OrderStatus::New || result.status == me::OrderStatus::PartiallyFilled) {
                live.emplace_back(inst, result.order_id);
            }
        } else if (roll < 80) {
            engine.submit_order(inst, me::OrderType::Market, gen.side(), 0, gen.qty());
        } else if (roll < 90 && !live.empty()) {
            const auto& target = live.back();
            engine.cancel_order(target.first, target.second);
            live.pop_back();
        } else {
            engine.get_snapshot(inst, 10);
        }
    }
    const auto elapsed = Clock::now() - start;
    print_throughput("mixed workload", kMeasure, elapsed);
}

void bench_latency() {
    me::OrderBook book(1);
    RandomGen gen;
    for (int i = 0; i < kWarmup; ++i) {
        book.add_order(me::OrderType::Limit, gen.side(), gen.price(), gen.qty());
    }

    std::vector<double> samples;
    samples.reserve(kMeasure);
    for (int i = 0; i < kMeasure; ++i) {
        const auto t0 = Clock::now();
        book.add_order(me::OrderType::Limit, gen.side(), gen.price(), gen.qty());
        const auto t1 = Clock::now();
        samples.push_back(std::chrono::duration<double, std::nano>(t1 - t0).count());
    }
    std::sort(samples.begin(), samples.end());

    std::cout << "\nlatency per add_order (nanoseconds)\n";
    std::cout << "  p50  " << std::fixed << std::setprecision(1) << percentile(samples, 0.50) << '\n';
    std::cout << "  p95  " << percentile(samples, 0.95) << '\n';
    std::cout << "  p99  " << percentile(samples, 0.99) << '\n';
    std::cout << "  p999 " << percentile(samples, 0.999) << '\n';
}

}

int main() {
    std::cout << "matching engine benchmark\n";
    std::cout << "warmup=" << kWarmup << " measure=" << kMeasure << "\n\n";

    bench_insertion();
    bench_matching();
    bench_cancel();
    bench_mixed();
    bench_latency();

    return 0;
}
