#include "logger.hpp"

#include <array>
#include <chrono>
#include <ctime>
#include <iostream>

namespace lb {

namespace {

std::string_view level_label(LogLevel level) {
    switch (level) {
        case LogLevel::Debug:
            return "DEBUG";
        case LogLevel::Info:
            return "INFO";
        case LogLevel::Warn:
            return "WARN";
        case LogLevel::Error:
            return "ERROR";
    }
    return "INFO";
}

std::string timestamp() {
    const auto now = std::chrono::system_clock::now();
    const auto secs = std::chrono::system_clock::to_time_t(now);
    const auto ms = std::chrono::duration_cast<std::chrono::milliseconds>(now.time_since_epoch()) % 1000;
    std::tm tm_buf{};
    ::gmtime_r(&secs, &tm_buf);
    std::array<char, 32> buf{};
    const std::size_t n = std::strftime(buf.data(), buf.size(), "%Y-%m-%dT%H:%M:%S", &tm_buf);
    std::string out(buf.data(), n);
    out += '.';
    const long m = ms.count();
    out += static_cast<char>('0' + (m / 100) % 10);
    out += static_cast<char>('0' + (m / 10) % 10);
    out += static_cast<char>('0' + m % 10);
    out += 'Z';
    return out;
}

}

Logger& Logger::instance() {
    static Logger logger;
    return logger;
}

void Logger::log(LogLevel level, std::string_view message, std::initializer_list<LogField> fields) {
    if (level < level_) {
        return;
    }
    std::string line;
    line.reserve(128);
    line += timestamp();
    line += " [";
    line += level_label(level);
    line += "] ";
    line += message;
    for (const auto& [key, value] : fields) {
        line += ' ';
        line += key;
        line += '=';
        line += value;
    }
    line += '\n';

    std::lock_guard<std::mutex> guard(mutex_);
    std::cerr << line;
}

std::string field_int(long value) {
    return std::to_string(value);
}

}
