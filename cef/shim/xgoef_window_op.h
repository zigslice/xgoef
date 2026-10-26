// Views window verbs shared by the Go bridge and the native shim.
// No CEF headers, so this file can be compiled on its own.
// Keep the numbers aligned with cef/window_op.go.
#ifndef XGOEF_WINDOW_OP_H_
#define XGOEF_WINDOW_OP_H_

#include <cstddef>
#include <string>

namespace xgoef {

constexpr int kWindowOpMinimize = 1;
constexpr int kWindowOpMaximize = 2;
constexpr int kWindowOpUnminimize = 3;
constexpr int kWindowOpUnmaximize = 4;
constexpr int kWindowOpToggleMaximize = 5;
constexpr int kWindowOpShow = 6;
constexpr int kWindowOpHide = 7;
constexpr int kWindowOpCenter = 8;
constexpr int kWindowOpSetTitle = 9;
constexpr int kWindowOpSetAlwaysOnTop = 10;
constexpr int kWindowOpSetFullscreen = 11;
constexpr int kWindowOpSetSize = 12;

constexpr int kWindowQueryMinimized = 1;
constexpr int kWindowQueryMaximized = 2;
constexpr int kWindowQueryFullscreen = 3;
constexpr int kWindowQueryAlwaysOnTop = 4;
constexpr int kWindowQueryVisible = 5;

constexpr int kWindowTitleMax = 256;
constexpr int kWindowSizeMin = 1;
constexpr int kWindowSizeMax = 16384;

inline bool TitleContinuation(unsigned char c) { return (c & 0xC0) == 0x80; }

// Empty is allowed. UTF-8, at most kWindowTitleMax bytes, no byte below 0x20.
inline bool TitleOK(const std::string& title) {
  if (title.size() > static_cast<std::size_t>(kWindowTitleMax)) {
    return false;
  }
  const auto* s = reinterpret_cast<const unsigned char*>(title.data());
  const std::size_t n = title.size();
  std::size_t i = 0;
  while (i < n) {
    const unsigned char c = s[i];
    if (c < 0x20) {
      return false;
    }
    if (c < 0x80) {
      i += 1;
      continue;
    }
    int need = 0;
    if (c >= 0xC2 && c <= 0xDF) {
      need = 2;
    } else if (c >= 0xE0 && c <= 0xEF) {
      need = 3;
    } else if (c >= 0xF0 && c <= 0xF4) {
      need = 4;
    } else {
      return false;
    }
    if (i + static_cast<std::size_t>(need) > n) {
      return false;
    }
    for (int k = 1; k < need; ++k) {
      if (!TitleContinuation(s[i + static_cast<std::size_t>(k)])) {
        return false;
      }
    }
    if (need == 3 && c == 0xE0 && s[i + 1] < 0xA0) {
      return false;
    }
    if (need == 3 && c == 0xED && s[i + 1] >= 0xA0) {
      return false;
    }
    if (need == 4 && c == 0xF0 && s[i + 1] < 0x90) {
      return false;
    }
    if (need == 4 && c == 0xF4 && s[i + 1] > 0x8F) {
      return false;
    }
    i += static_cast<std::size_t>(need);
  }
  return true;
}

inline bool SizeOK(int width, int height) {
  return width >= kWindowSizeMin && width <= kWindowSizeMax && height >= kWindowSizeMin &&
         height <= kWindowSizeMax;
}

}  // namespace xgoef

#ifdef __cplusplus
extern "C" {
#endif

/* Apply one verb to the Views window bound to |label|.
 * Empty |label| is the main window. |text| is the title for set_title.
 * Returns 0, -1 when that label has no Views window or the UI thread does
 * not answer within 2 seconds, or -2 when the verb, title, or size is rejected. */
int xgoef_window_op(const char* label, int op, int a, int b, const char* text);

/* Read one flag. Returns 1, 0, or -1 when that label has no Views window
 * or the UI thread does not answer. An unknown query is -1. */
int xgoef_window_query(const char* label, int query);

#ifdef __cplusplus
}
#endif

#endif  // XGOEF_WINDOW_OP_H_
