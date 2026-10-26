// Decide the Content-Security-Policy header for an xgoef://app/ response.
// No CEF headers, so this file can be compiled on its own.
// The host always passes one switch. This file does not invent a default.
// --xgoef-no-csp wins when both switches are present.
#ifndef XGOEF_CSP_H_
#define XGOEF_CSP_H_

#include <string>

namespace xgoef {

inline constexpr const char kCSPSwitch[] = "xgoef-csp";
inline constexpr const char kNoCSPSwitch[] = "xgoef-no-csp";
inline constexpr int kCSPMaxBytes = 2048;

// CSPTextOK reports whether configured can be a response header value.
// Empty text, text longer than kCSPMaxBytes, or any byte below 0x20
// (including NUL, CR, and LF) is rejected.
inline bool CSPTextOK(const std::string& text) {
  if (text.empty() || text.size() > static_cast<size_t>(kCSPMaxBytes)) {
    return false;
  }
  for (unsigned char c : text) {
    if (c < 0x20) {
      return false;
    }
  }
  return true;
}

// CSPHeaderValue plans the header value.
// disabled is true when --xgoef-no-csp is present.
// configured is the value of --xgoef-csp.
// Returns false when no header should be sent. On success, out is configured.
inline bool CSPHeaderValue(bool disabled, const std::string& configured, std::string* out) {
  if (out != nullptr) {
    out->clear();
  }
  if (disabled || !CSPTextOK(configured)) {
    return false;
  }
  if (out != nullptr) {
    *out = configured;
  }
  return true;
}

}  // namespace xgoef

#endif  // XGOEF_CSP_H_
