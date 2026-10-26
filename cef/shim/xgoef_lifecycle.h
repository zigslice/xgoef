// Decide when a finished load is a host page-load event.
// No CEF headers, so this file can be compiled on its own.
// OnLoadEnd calls xgoef_page_load_note. The callback is the host hook.
#ifndef XGOEF_LIFECYCLE_H_
#define XGOEF_LIFECYCLE_H_

#include <string>

namespace xgoef {

inline constexpr int kPageLoadLabelMax = 64;
inline constexpr int kPageLoadURLMax = 4096;

// PageLoad is one accepted main-frame load.
struct PageLoad {
  bool notify = false;
  std::string label;
  std::string url;
};

// PageLoadTextOK reports whether text can cross the page-load callback.
// Empty text, text longer than max_bytes, a NUL, or a byte below 0x20 is rejected.
inline bool PageLoadTextOK(const std::string& text, int max_bytes) {
  if (text.empty() || max_bytes < 0 || text.size() > static_cast<size_t>(max_bytes) ||
      text.find('\0') != std::string::npos) {
    return false;
  }
  for (unsigned char c : text) {
    if (c < 0x20) {
      return false;
    }
  }
  return true;
}

// PageLoadFor plans a page-load notification.
// is_main is CefFrame::IsMain(). is_devtools is the developer-tools browser.
// A subframe, a developer-tools browser, or an unsafe label or URL does not notify.
inline PageLoad PageLoadFor(bool is_main,
                            bool is_devtools,
                            const std::string& label,
                            const std::string& url) {
  PageLoad out;
  if (!is_main || is_devtools) {
    return out;
  }
  if (!PageLoadTextOK(label, kPageLoadLabelMax) || !PageLoadTextOK(url, kPageLoadURLMax)) {
    return out;
  }
  out.notify = true;
  out.label = label;
  out.url = url;
  return out;
}

}  // namespace xgoef

#ifdef __cplusplus
extern "C" {
#endif

typedef void (*xgoef_page_load_fn)(const char* label, const char* url);

/* Replace the host callback. NULL clears it. */
void xgoef_page_load_set(xgoef_page_load_fn fn);

/* Apply PageLoadFor and invoke the callback when the load is accepted.
 * is_main and is_devtools are non-zero when true. */
void xgoef_page_load_note(int is_main, int is_devtools, const char* label, const char* url);

#ifdef __cplusplus
}
#endif

#endif  // XGOEF_LIFECYCLE_H_
