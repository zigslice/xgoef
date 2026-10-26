// Views window resize limits shared by the Go encoder and the native shim.
// No CEF headers, so this file can be compiled on its own.
// Keep the numbers aligned with cef/window_limits.go.
#ifndef XGOEF_WINDOW_LIMITS_H_
#define XGOEF_WINDOW_LIMITS_H_

#include <string>

namespace xgoef {

constexpr int kWindowLimitResizable = 1;
constexpr int kWindowLimitMinimizable = 2;
constexpr int kWindowLimitMaximizable = 3;
constexpr int kWindowLimitClosable = 4;
constexpr int kWindowLimitMinSize = 5;
constexpr int kWindowLimitMaxSize = 6;

constexpr int kWindowLimitSizeMax = 16384;

struct WindowLimits {
  bool resizable = true;
  bool minimizable = true;
  bool maximizable = true;
  bool closable = true;
  int min_width = 0;
  int min_height = 0;
  int max_width = 0;
  int max_height = 0;
};

// Empty text is the default (every action allowed, no size bounds).
bool ParseWindowLimits(const std::string& text, WindowLimits* out, std::string* err);
std::string FormatWindowLimits(const WindowLimits& limits);

// Replace or read the process-wide table. An unknown label reads as the default.
// An empty label is main.
void WindowLimitsSet(const std::string& label, const WindowLimits& limits);
WindowLimits WindowLimitsGet(const std::string& label);

// Change one field. Returns false when the field or the resulting bounds are rejected.
bool ApplyWindowLimit(WindowLimits* limits, int field, int a, int b);

// Empty label means the window is not bound yet, so the caller keeps the default
// and does not read another window's row.
inline WindowLimits LimitsForDelegate(const std::string& label) {
  if (label.empty()) {
    return WindowLimits();
  }
  return WindowLimitsGet(label);
}

}  // namespace xgoef

#ifdef __cplusplus
extern "C" {
#endif

/* Store limits for |label| before that window exists.
 * |text| is FormatWindowLimits. Empty |label| is main. Empty |text| is the default.
 * Returns 0, or -2 when the text is rejected. */
int xgoef_window_limits_seed(const char* label, const char* text);

/* Change one field on the Views window bound to |label|.
 * Returns 0, -1 when that label has no Views window or the UI thread does
 * not answer within 2 seconds, or -2 when the field or the size is rejected. */
int xgoef_window_limit(const char* label, int field, int a, int b);

/* Read one field. On success the value is (a << 16) | b, with a and b in 0..16384.
 * A flag uses a = 0 or 1 and b = 0. An unset size is 0.
 * Returns -1 when that label has no Views window or the UI thread does not answer,
 * or -2 when the field is unknown. */
int xgoef_window_limit_get(const char* label, int field);

#ifdef __cplusplus
}
#endif

#endif  // XGOEF_WINDOW_LIMITS_H_
