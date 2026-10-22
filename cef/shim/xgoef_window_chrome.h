// decorations,theme,transparent text shared by the Go encoder and the native shim.
// No CEF headers, so this file can be compiled on its own.
// Example: "1,system,0" (decorated, system theme, opaque) or "0,dark,1".
#ifndef XGOEF_WINDOW_CHROME_H_
#define XGOEF_WINDOW_CHROME_H_

#include <string>

namespace xgoef {

struct WindowChrome {
  bool decorations = true;
  std::string theme = "system";
  bool transparent = false;
};

// Parse the text produced by cef.WindowChrome.Encode.
// An empty string yields the default. Any other shape returns false.
bool ParseWindowChrome(const std::string& text, WindowChrome* out, std::string* err);

}  // namespace xgoef

#endif  // XGOEF_WINDOW_CHROME_H_
