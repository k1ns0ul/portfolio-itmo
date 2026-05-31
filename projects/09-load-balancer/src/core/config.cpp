#include "config.hpp"

#include <charconv>
#include <fstream>
#include <sstream>

namespace lb {

namespace {

struct Line {
    int indent;
    std::string key;
    std::string value;
    bool is_list_item;
};

std::string_view trim(std::string_view s) {
    while (!s.empty() && (s.front() == ' ' || s.front() == '\t' || s.front() == '\r')) {
        s.remove_prefix(1);
    }
    while (!s.empty() && (s.back() == ' ' || s.back() == '\t' || s.back() == '\r')) {
        s.remove_suffix(1);
    }
    return s;
}

int count_indent(std::string_view raw) {
    int n = 0;
    for (char c : raw) {
        if (c == ' ') {
            ++n;
        } else {
            break;
        }
    }
    return n;
}

std::optional<int> to_int(std::string_view sv) {
    int value = 0;
    const auto [ptr, ec] = std::from_chars(sv.data(), sv.data() + sv.size(), value);
    if (ec != std::errc{} || ptr != sv.data() + sv.size()) {
        return std::nullopt;
    }
    return value;
}

bool to_bool(std::string_view sv, bool fallback) {
    if (sv == "true" || sv == "yes" || sv == "1") {
        return true;
    }
    if (sv == "false" || sv == "no" || sv == "0") {
        return false;
    }
    return fallback;
}

std::vector<Line> tokenize(std::string_view text) {
    std::vector<Line> lines;
    std::size_t pos = 0;
    while (pos <= text.size()) {
        std::size_t nl = text.find('\n', pos);
        std::string_view raw = text.substr(pos, nl == std::string_view::npos ? std::string_view::npos : nl - pos);
        pos = nl == std::string_view::npos ? text.size() + 1 : nl + 1;

        const int indent = count_indent(raw);
        std::string_view content = trim(raw);
        if (content.empty() || content.front() == '#') {
            continue;
        }

        Line line;
        line.indent = indent;
        line.is_list_item = false;
        if (content.rfind("- ", 0) == 0) {
            line.is_list_item = true;
            content = trim(content.substr(2));
        }

        const std::size_t colon = content.find(':');
        if (colon == std::string_view::npos) {
            line.key = std::string(content);
        } else {
            line.key = std::string(trim(content.substr(0, colon)));
            line.value = std::string(trim(content.substr(colon + 1)));
        }
        lines.push_back(std::move(line));
    }
    return lines;
}

}

std::optional<Duration> parse_duration(std::string_view value) {
    if (value.empty()) {
        return std::nullopt;
    }
    const char unit = value.back();
    std::string_view digits = value;
    long multiplier = 1;
    if (unit == 's') {
        multiplier = 1000;
        digits.remove_suffix(1);
    } else if (unit == 'm') {
        multiplier = 60 * 1000;
        digits.remove_suffix(1);
    } else if (unit == 'h') {
        multiplier = 60 * 60 * 1000;
        digits.remove_suffix(1);
    } else if (unit >= '0' && unit <= '9') {
        multiplier = 1;
    } else {
        return std::nullopt;
    }
    auto number = to_int(digits);
    if (!number) {
        return std::nullopt;
    }
    return Duration{static_cast<long>(*number) * multiplier};
}

std::optional<Config> Config::from_string(std::string_view text, std::string& error) {
    const auto lines = tokenize(text);
    Config cfg;

    std::size_t i = 0;
    while (i < lines.size()) {
        const Line& line = lines[i];
        if (line.indent != 0) {
            ++i;
            continue;
        }

        if (line.key == "listen") {
            cfg.listen = line.value;
            ++i;
        } else if (line.key == "strategy") {
            if (line.value == "round_robin") {
                cfg.strategy = Strategy::RoundRobin;
            } else if (line.value == "least_connections") {
                cfg.strategy = Strategy::LeastConnections;
            } else if (line.value == "weighted") {
                cfg.strategy = Strategy::Weighted;
            } else if (line.value == "ip_hash") {
                cfg.strategy = Strategy::IpHash;
            } else {
                error = "unknown strategy: " + line.value;
                return std::nullopt;
            }
            ++i;
        } else if (line.key == "drain_timeout") {
            if (auto d = parse_duration(line.value)) {
                cfg.drain_timeout = *d;
            }
            ++i;
        } else if (line.key == "health_check") {
            ++i;
            while (i < lines.size() && lines[i].indent > line.indent) {
                const Line& sub = lines[i];
                if (sub.key == "interval") {
                    if (auto d = parse_duration(sub.value)) {
                        cfg.health_check.interval = *d;
                    }
                } else if (sub.key == "timeout") {
                    if (auto d = parse_duration(sub.value)) {
                        cfg.health_check.timeout = *d;
                    }
                } else if (sub.key == "type") {
                    cfg.health_check.type = sub.value == "http" ? HealthCheckType::Http : HealthCheckType::Tcp;
                } else if (sub.key == "threshold") {
                    if (auto n = to_int(sub.value)) {
                        cfg.health_check.threshold = *n;
                    }
                } else if (sub.key == "path") {
                    cfg.health_check.http_path = sub.value;
                }
                ++i;
            }
        } else if (line.key == "metrics") {
            ++i;
            while (i < lines.size() && lines[i].indent > line.indent) {
                const Line& sub = lines[i];
                if (sub.key == "enabled") {
                    cfg.metrics.enabled = to_bool(sub.value, true);
                } else if (sub.key == "port") {
                    if (auto n = to_int(sub.value)) {
                        cfg.metrics.port = static_cast<std::uint16_t>(*n);
                    }
                }
                ++i;
            }
        } else if (line.key == "backends") {
            ++i;
            while (i < lines.size() && lines[i].indent > line.indent) {
                if (lines[i].is_list_item) {
                    BackendConfig backend;
                    const int item_indent = lines[i].indent;
                    if (lines[i].key == "address") {
                        backend.address = lines[i].value;
                    }
                    ++i;
                    while (i < lines.size() && lines[i].indent >= item_indent && !lines[i].is_list_item) {
                        const Line& field = lines[i];
                        if (field.key == "address") {
                            backend.address = field.value;
                        } else if (field.key == "weight") {
                            if (auto n = to_int(field.value)) {
                                backend.weight = *n;
                            }
                        } else if (field.key == "max_connections") {
                            if (auto n = to_int(field.value)) {
                                backend.max_connections = *n;
                            }
                        }
                        ++i;
                    }
                    if (backend.address.empty()) {
                        error = "backend without address";
                        return std::nullopt;
                    }
                    if (backend.weight < 1) {
                        backend.weight = 1;
                    }
                    cfg.backends.push_back(std::move(backend));
                } else {
                    ++i;
                }
            }
        } else {
            ++i;
        }
    }

    if (cfg.backends.empty()) {
        error = "configuration has no backends";
        return std::nullopt;
    }
    return cfg;
}

std::optional<Config> Config::from_file(const std::string& path, std::string& error) {
    std::ifstream file(path);
    if (!file) {
        error = "cannot open config file: " + path;
        return std::nullopt;
    }
    std::ostringstream ss;
    ss << file.rdbuf();
    return from_string(ss.str(), error);
}

}
