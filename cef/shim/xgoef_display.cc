#include "xgoef_display.h"

#include <cmath>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <sstream>
#include <vector>

namespace xgoef {
namespace {

std::string FormatScale(double scale) {
  char buf[64];
  std::snprintf(buf, sizeof(buf), "%.6f", scale);
  return buf;
}

bool ParseScale(const std::string& raw, bool allow_zero, double* out) {
  if (raw.empty()) {
    return false;
  }
  char* end = nullptr;
  double scale = std::strtod(raw.c_str(), &end);
  if (end == raw.c_str() || *end != '\0' || !std::isfinite(scale)) {
    return false;
  }
  if (scale == 0 && allow_zero) {
    *out = 0;
    return true;
  }
  if (scale <= 0 || scale >= 100) {
    return false;
  }
  *out = scale;
  return true;
}

bool ParseI64(const std::string& raw, int64_t* out) {
  if (raw.empty()) {
    return false;
  }
  char* end = nullptr;
  long long value = std::strtoll(raw.c_str(), &end, 10);
  if (end == raw.c_str() || *end != '\0') {
    return false;
  }
  *out = static_cast<int64_t>(value);
  return true;
}

bool ParseInt(const std::string& raw, int* out) {
  int64_t value = 0;
  if (!ParseI64(raw, &value) || value < INT32_MIN || value > INT32_MAX) {
    return false;
  }
  *out = static_cast<int>(value);
  return true;
}

std::vector<std::string> Fields(const std::string& line) {
  std::vector<std::string> out;
  std::string cur;
  for (char c : line) {
    if (c == ' ' || c == '\t') {
      if (!cur.empty()) {
        out.push_back(cur);
        cur.clear();
      }
      continue;
    }
    cur.push_back(c);
  }
  if (!cur.empty()) {
    out.push_back(cur);
  }
  return out;
}

bool ParseDisplayLine(const std::string& line, DisplayInfo* out, std::string* err) {
  auto fail = [&](const char* why) {
    if (err) {
      *err = std::string("display ") + why;
    }
    return false;
  };
  std::vector<std::string> fields = Fields(line);
  if (fields.size() != 20) {
    return fail("fields");
  }
  DisplayInfo info;
  if (!ParseI64(fields[0], &info.id) || !ParseScale(fields[1], false, &info.scale)) {
    return fail("id");
  }
  int* nums[] = {
      &info.x,           &info.y,           &info.width,           &info.height,
      &info.work_x,      &info.work_y,      &info.work_width,      &info.work_height,
      &info.pixel_x,     &info.pixel_y,     &info.pixel_width,     &info.pixel_height,
      &info.work_pixel_x, &info.work_pixel_y, &info.work_pixel_width, &info.work_pixel_height,
      &info.rotation};
  for (int i = 0; i < 17; ++i) {
    if (!ParseInt(fields[static_cast<size_t>(i + 2)], nums[i])) {
      return fail("number");
    }
  }
  int primary = 0;
  if (!ParseInt(fields[19], &primary) || (primary != 0 && primary != 1)) {
    return fail("primary");
  }
  info.primary = primary == 1;
  if (info.width <= 0 || info.height <= 0 || info.work_width <= 0 || info.work_height <= 0 ||
      info.pixel_width <= 0 || info.pixel_height <= 0 || info.work_pixel_width <= 0 ||
      info.work_pixel_height <= 0) {
    return fail("size");
  }
  if (info.rotation != 0 && info.rotation != 90 && info.rotation != 180 && info.rotation != 270) {
    return fail("rotation");
  }
  *out = info;
  return true;
}

}  // namespace

std::string FormatDisplayTable(const DisplayTable& table) {
  std::ostringstream out;
  out << "window ";
  if (table.has_window) {
    out << table.window_id;
  } else {
    out << '-';
  }
  out << ' ' << FormatScale(table.has_window ? table.window_scale : 0) << '\n';
  for (const DisplayInfo& d : table.displays) {
    out << d.id << ' ' << FormatScale(d.scale) << ' ' << d.x << ' ' << d.y << ' ' << d.width
        << ' ' << d.height << ' ' << d.work_x << ' ' << d.work_y << ' ' << d.work_width << ' '
        << d.work_height << ' ' << d.pixel_x << ' ' << d.pixel_y << ' ' << d.pixel_width << ' '
        << d.pixel_height << ' ' << d.work_pixel_x << ' ' << d.work_pixel_y << ' '
        << d.work_pixel_width << ' ' << d.work_pixel_height << ' ' << d.rotation << ' '
        << (d.primary ? 1 : 0) << '\n';
  }
  return out.str();
}

bool ParseDisplayTable(const std::string& text, DisplayTable* out, std::string* err) {
  auto fail = [&](const char* why) {
    if (err) {
      *err = std::string("display ") + why;
    }
    return false;
  };
  if (!out) {
    return fail("out");
  }
  DisplayTable table;
  bool saw_header = false;
  std::string line;
  auto take = [&](const std::string& raw) -> bool {
    if (raw.empty()) {
      return true;
    }
    if (!saw_header) {
      std::vector<std::string> fields = Fields(raw);
      if (fields.size() != 3 || fields[0] != "window") {
        return fail("window");
      }
      if (fields[1] == "-") {
        table.has_window = false;
      } else {
        table.has_window = true;
        if (!ParseI64(fields[1], &table.window_id)) {
          return fail("window");
        }
      }
      if (!ParseScale(fields[2], !table.has_window, &table.window_scale)) {
        return fail("window");
      }
      saw_header = true;
      return true;
    }
    DisplayInfo info;
    if (!ParseDisplayLine(raw, &info, err)) {
      return false;
    }
    for (const DisplayInfo& have : table.displays) {
      if (have.id == info.id) {
        return fail("id");
      }
    }
    table.displays.push_back(info);
    return true;
  };
  for (char c : text) {
    if (c == '\n') {
      if (!line.empty() && line.back() == '\r') {
        line.pop_back();
      }
      if (!take(line)) {
        return false;
      }
      line.clear();
      continue;
    }
    line.push_back(c);
  }
  if (!line.empty() && line.back() == '\r') {
    line.pop_back();
  }
  if (!take(line)) {
    return false;
  }
  if (!saw_header) {
    return fail("window");
  }
  int primaries = 0;
  for (const DisplayInfo& d : table.displays) {
    if (d.primary) {
      ++primaries;
    }
  }
  if (!table.displays.empty() && primaries != 1) {
    return fail("primary");
  }
  *out = table;
  if (err) {
    err->clear();
  }
  return true;
}

}  // namespace xgoef
