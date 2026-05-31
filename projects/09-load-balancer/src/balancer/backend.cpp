#include "backend.hpp"

#include <utility>

namespace lb {

Backend::Backend(Address address, std::string label, int weight, int max_connections)
    : address_(address),
      label_(std::move(label)),
      weight_(weight < 1 ? 1 : weight),
      max_connections_(max_connections) {}

}
