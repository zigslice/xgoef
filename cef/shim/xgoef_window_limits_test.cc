#include "xgoef_window_limits.h"

#include <iostream>
#include <string>

namespace {

int Fail(const char* message) {
  std::cerr << message << "\n";
  return 1;
}

}  // namespace

int main() {
  static_assert(xgoef::kWindowLimitResizable == 1, "resize");
  static_assert(xgoef::kWindowLimitMinimizable == 2, "minable");
  static_assert(xgoef::kWindowLimitMaximizable == 3, "maxable");
  static_assert(xgoef::kWindowLimitClosable == 4, "close");
  static_assert(xgoef::kWindowLimitMinSize == 5, "min");
  static_assert(xgoef::kWindowLimitMaxSize == 6, "max");
  static_assert(xgoef::kWindowLimitSizeMax == 16384, "size");

  xgoef::WindowLimits limits;
  std::string err;
  if (!xgoef::ParseWindowLimits("", &limits, &err) || !limits.resizable || !limits.closable ||
      limits.min_width != 0) {
    return Fail("default");
  }
  const std::string text = "0,1,0,1,320,240,800,600";
  if (!xgoef::ParseWindowLimits(text, &limits, &err) || limits.resizable || !limits.minimizable ||
      limits.maximizable || !limits.closable || limits.min_width != 320 || limits.max_height != 600) {
    return Fail("parse");
  }
  if (xgoef::FormatWindowLimits(limits) != text) {
    return Fail("format");
  }
  if (xgoef::ParseWindowLimits("1,1,1,1,0,1,0,0", &limits, &err)) {
    return Fail("half size");
  }
  if (xgoef::ParseWindowLimits("1,1,1,1,800,600,320,240", &limits, &err)) {
    return Fail("min above max");
  }
  if (xgoef::ParseWindowLimits("2,1,1,1,0,0,0,0", &limits, &err)) {
    return Fail("flag");
  }
  if (xgoef::ParseWindowLimits("1,1,1,1,01,1,0,0", &limits, &err)) {
    return Fail("leading zero");
  }

  xgoef::WindowLimits row;
  if (!xgoef::ApplyWindowLimit(&row, xgoef::kWindowLimitResizable, 0, 0) || row.resizable) {
    return Fail("apply resize");
  }
  if (!xgoef::ApplyWindowLimit(&row, xgoef::kWindowLimitMinSize, 400, 300) || row.min_width != 400) {
    return Fail("apply min");
  }
  if (xgoef::ApplyWindowLimit(&row, xgoef::kWindowLimitMaxSize, 100, 100)) {
    return Fail("apply max under min");
  }
  if (row.max_width != 0) {
    return Fail("rejected max stuck");
  }
  if (!xgoef::ApplyWindowLimit(&row, xgoef::kWindowLimitMinSize, 0, 0) || row.min_width != 0) {
    return Fail("clear min");
  }
  if (xgoef::ApplyWindowLimit(&row, 9, 1, 0)) {
    return Fail("unknown field");
  }

  if (xgoef_window_limits_seed("aux", "0,1,1,1,0,0,0,0") != 0) {
    return Fail("seed");
  }
  xgoef::WindowLimits got = xgoef::WindowLimitsGet("aux");
  if (got.resizable || !got.minimizable) {
    return Fail("stored");
  }
  if (xgoef::WindowLimitsGet("missing").resizable != true) {
    return Fail("missing default");
  }
  if (xgoef_window_limits_seed("aux", "nope") != -2) {
    return Fail("bad seed");
  }
  got = xgoef::WindowLimitsGet("aux");
  if (got.resizable) {
    return Fail("bad seed replaced");
  }
  xgoef::WindowLimits blank = xgoef::LimitsForDelegate("");
  if (!blank.resizable || !blank.closable) {
    return Fail("unbound delegate");
  }
  return 0;
}
