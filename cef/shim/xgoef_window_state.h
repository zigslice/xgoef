// Line-oriented window places shared by the Go encoder and the native shim.
// No CEF headers, so this file can be compiled on its own.
#ifndef XGOEF_WINDOW_STATE_H_
#define XGOEF_WINDOW_STATE_H_

#include <string>
#include <vector>

namespace xgoef {

struct WindowPlace {
  std::string label;
  int x = 0;
  int y = 0;
  int width = 0;
  int height = 0;
  bool positioned = false;
  bool maximized = false;
  bool minimized = false;
};

// Parse the text produced by windowstate.Store.Encode.
// An empty string yields an empty list. A bad line returns false.
bool ParseWindowState(const std::string& flat,
                      std::vector<WindowPlace>* out,
                      std::string* err);

// Replace the process-wide table. A bad encoding clears it.
void WindowStateSet(const std::string& flat);
bool WindowStateFind(const std::string& label, WindowPlace* out);

// 2 = minimized, 1 = maximized, otherwise fallback clamped to 0..2.
// Callers map 2 and 1 onto their show-state enum and treat 0 as the fallback.
inline int ShowRestore(bool minimized, bool maximized, int fallback) {
  if (minimized) {
    return 2;
  }
  if (maximized) {
    return 1;
  }
  if (fallback < 0 || fallback > 2) {
    return 0;
  }
  return fallback;
}

// Invoke the note callback without holding the table lock.
void WindowStateNote(const std::string& label,
                     int x,
                     int y,
                     int width,
                     int height,
                     int maximized,
                     int minimized);

}  // namespace xgoef

#ifdef __cplusplus
extern "C" {
#endif

typedef void (*xgoef_window_state_fn)(const char* label,
                                      int x,
                                      int y,
                                      int width,
                                      int height,
                                      int maximized,
                                      int minimized);

void xgoef_window_state_set(const char* encoded);
void xgoef_window_state_set_note(xgoef_window_state_fn fn);

#ifdef __cplusplus
}
#endif

#endif  // XGOEF_WINDOW_STATE_H_
