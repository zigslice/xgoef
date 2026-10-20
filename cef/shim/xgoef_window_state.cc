#include "xgoef_window_state.h"

#include <cstdlib>
#include <mutex>
#include <sstream>
#include <string>
#include <vector>

namespace xgoef {
namespace {

std::mutex g_mu;
std::vector<WindowPlace> g_places;
xgoef_window_state_fn g_note = nullptr;

const int kMinSize = 1;
const int kMaxSize = 16384;
const int kMinPos = -32768;
const int kMaxPos = 32768;

std::vector<std::string> SplitTabs(const std::string& line) {
  std::vector<std::string> cols;
  std::string cur;
  for (char c : line) {
    if (c == '\t') {
      cols.push_back(cur);
      cur.clear();
    } else {
      cur.push_back(c);
    }
  }
  cols.push_back(cur);
  return cols;
}

bool ParseNumber(const std::string& text, int* out) {
  if (text.empty() || out == nullptr) {
    return false;
  }
  char* end = nullptr;
  const long n = std::strtol(text.c_str(), &end, 10);
  if (end == text.c_str() || *end != '\0' || n < kMinPos || n > kMaxPos) {
    return false;
  }
  *out = static_cast<int>(n);
  return true;
}

bool FlagsOK(const std::string& flags, bool* positioned, bool* maximized) {
  *positioned = false;
  *maximized = false;
  for (char c : flags) {
    if (c == 'p') {
      *positioned = true;
    } else if (c == 'm') {
      *maximized = true;
    } else {
      return false;
    }
  }
  return true;
}

bool InRange(const WindowPlace& p) {
  if (p.positioned) {
    if (p.width < kMinSize || p.height < kMinSize || p.width > kMaxSize || p.height > kMaxSize) {
      return false;
    }
    if (p.x < kMinPos || p.y < kMinPos || p.x > kMaxPos || p.y > kMaxPos) {
      return false;
    }
  }
  return p.positioned || p.maximized;
}

}  // namespace

bool ParseWindowState(const std::string& flat, std::vector<WindowPlace>* out, std::string* err) {
  out->clear();
  std::istringstream in(flat);
  std::string line;
  int n = 0;
  while (std::getline(in, line)) {
    if (!line.empty() && line.back() == '\r') {
      line.pop_back();
    }
    if (line.empty()) {
      continue;
    }
    ++n;
    const std::vector<std::string> cols = SplitTabs(line);
    if (cols.size() != 6 || cols[0].empty()) {
      if (err) {
        *err = "window state line " + std::to_string(n);
      }
      out->clear();
      return false;
    }
    WindowPlace p;
    p.label = cols[0];
    if (!ParseNumber(cols[1], &p.x) || !ParseNumber(cols[2], &p.y) ||
        !ParseNumber(cols[3], &p.width) || !ParseNumber(cols[4], &p.height) ||
        !FlagsOK(cols[5], &p.positioned, &p.maximized) || !InRange(p)) {
      if (err) {
        *err = "window state line " + std::to_string(n);
      }
      out->clear();
      return false;
    }
    for (const WindowPlace& prev : *out) {
      if (prev.label == p.label) {
        if (err) {
          *err = "window state duplicate " + p.label;
        }
        out->clear();
        return false;
      }
    }
    out->push_back(p);
  }
  return true;
}

void WindowStateSet(const std::string& flat) {
  std::vector<WindowPlace> parsed;
  std::string err;
  if (!ParseWindowState(flat, &parsed, &err)) {
    parsed.clear();
  }
  std::lock_guard<std::mutex> lock(g_mu);
  g_places = std::move(parsed);
}

bool WindowStateFind(const std::string& label, WindowPlace* out) {
  std::lock_guard<std::mutex> lock(g_mu);
  for (const WindowPlace& p : g_places) {
    if (p.label == label) {
      if (out) {
        *out = p;
      }
      return true;
    }
  }
  return false;
}

void WindowStateSetNote(xgoef_window_state_fn fn) {
  std::lock_guard<std::mutex> lock(g_mu);
  g_note = fn;
}

void WindowStateNote(const std::string& label, int x, int y, int width, int height, int maximized) {
  if (label.empty()) {
    return;
  }
  xgoef_window_state_fn fn = nullptr;
  {
    std::lock_guard<std::mutex> lock(g_mu);
    fn = g_note;
  }
  if (fn) {
    fn(label.c_str(), x, y, width, height, maximized);
  }
}

}  // namespace xgoef

extern "C" void xgoef_window_state_set(const char* encoded) {
  xgoef::WindowStateSet(encoded ? encoded : "");
}

extern "C" void xgoef_window_state_set_note(xgoef_window_state_fn fn) {
  xgoef::WindowStateSetNote(fn);
}
