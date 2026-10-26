#include "xgoef_window_limits.h"

#include <mutex>
#include <unordered_map>

namespace xgoef {
namespace {

std::mutex g_mu;
std::unordered_map<std::string, WindowLimits> g_limits;

bool Fail(std::string* err, const char* why) {
  if (err) {
    *err = std::string("window limits ") + why;
  }
  return false;
}

bool ParseCanon(const std::string& text, int* out) {
  if (text == "0") {
    *out = 0;
    return true;
  }
  if (text.empty() || text[0] == '0' || text.size() > 5) {
    return false;
  }
  int value = 0;
  for (char c : text) {
    if (c < '0' || c > '9') {
      return false;
    }
    value = value * 10 + (c - '0');
  }
  *out = value;
  return true;
}

bool SizeOK(int width, int height) {
  if (width == 0 && height == 0) {
    return true;
  }
  return width >= 1 && width <= kWindowLimitSizeMax && height >= 1 && height <= kWindowLimitSizeMax;
}

bool BoundsOK(const WindowLimits& limits) {
  if (!SizeOK(limits.min_width, limits.min_height) || !SizeOK(limits.max_width, limits.max_height)) {
    return false;
  }
  if (limits.min_width > 0 && limits.max_width > 0 && limits.min_width > limits.max_width) {
    return false;
  }
  if (limits.min_height > 0 && limits.max_height > 0 && limits.min_height > limits.max_height) {
    return false;
  }
  return true;
}

std::string LabelOrMain(const std::string& label) { return label.empty() ? "main" : label; }

}  // namespace

bool ParseWindowLimits(const std::string& text, WindowLimits* out, std::string* err) {
  WindowLimits limits;
  if (!text.empty()) {
    std::string fields[8];
    int commas = 0;
    for (char c : text) {
      if (c == ',') {
        if (commas >= 7) {
          return Fail(err, "fields");
        }
        ++commas;
        continue;
      }
      fields[commas].push_back(c);
    }
    if (commas != 7) {
      return Fail(err, "fields");
    }
    bool* flags[4] = {&limits.resizable, &limits.minimizable, &limits.maximizable, &limits.closable};
    for (int i = 0; i < 4; ++i) {
      if (fields[i] == "1") {
        *flags[i] = true;
      } else if (fields[i] == "0") {
        *flags[i] = false;
      } else {
        return Fail(err, "flag");
      }
    }
    int nums[4] = {0, 0, 0, 0};
    for (int i = 0; i < 4; ++i) {
      if (!ParseCanon(fields[i + 4], &nums[i])) {
        return Fail(err, "size");
      }
    }
    limits.min_width = nums[0];
    limits.min_height = nums[1];
    limits.max_width = nums[2];
    limits.max_height = nums[3];
    if (!BoundsOK(limits)) {
      return Fail(err, "bounds");
    }
  }
  if (out) {
    *out = limits;
  }
  if (err) {
    err->clear();
  }
  return true;
}

std::string FormatWindowLimits(const WindowLimits& limits) {
  auto bit = [](bool on) { return on ? '1' : '0'; };
  return std::string(1, bit(limits.resizable)) + "," + bit(limits.minimizable) + "," +
         bit(limits.maximizable) + "," + bit(limits.closable) + "," + std::to_string(limits.min_width) +
         "," + std::to_string(limits.min_height) + "," + std::to_string(limits.max_width) + "," +
         std::to_string(limits.max_height);
}

void WindowLimitsSet(const std::string& label, const WindowLimits& limits) {
  std::lock_guard<std::mutex> lock(g_mu);
  g_limits[LabelOrMain(label)] = limits;
}

WindowLimits WindowLimitsGet(const std::string& label) {
  std::lock_guard<std::mutex> lock(g_mu);
  auto found = g_limits.find(LabelOrMain(label));
  if (found == g_limits.end()) {
    return WindowLimits();
  }
  return found->second;
}

bool ApplyWindowLimit(WindowLimits* limits, int field, int a, int b) {
  if (!limits) {
    return false;
  }
  WindowLimits next = *limits;
  switch (field) {
    case kWindowLimitResizable:
    case kWindowLimitMinimizable:
    case kWindowLimitMaximizable:
    case kWindowLimitClosable:
      if ((a != 0 && a != 1) || b != 0) {
        return false;
      }
      switch (field) {
        case kWindowLimitResizable:
          next.resizable = a == 1;
          break;
        case kWindowLimitMinimizable:
          next.minimizable = a == 1;
          break;
        case kWindowLimitMaximizable:
          next.maximizable = a == 1;
          break;
        default:
          next.closable = a == 1;
          break;
      }
      break;
    case kWindowLimitMinSize:
      next.min_width = a;
      next.min_height = b;
      break;
    case kWindowLimitMaxSize:
      next.max_width = a;
      next.max_height = b;
      break;
    default:
      return false;
  }
  if (!BoundsOK(next)) {
    return false;
  }
  *limits = next;
  return true;
}

}  // namespace xgoef

extern "C" int xgoef_window_limits_seed(const char* label, const char* text) {
  xgoef::WindowLimits limits;
  std::string err;
  if (!xgoef::ParseWindowLimits(text ? text : "", &limits, &err)) {
    return -2;
  }
  xgoef::WindowLimitsSet(label ? label : "", limits);
  return 0;
}
