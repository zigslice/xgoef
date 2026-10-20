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
};

// Parse the text produced by windowstate.Store.Encode.
// An empty string yields an empty list. A bad line returns false.
bool ParseWindowState(const std::string& flat,
                      std::vector<WindowPlace>* out,
                      std::string* err);

// Replace the process-wide table. A bad encoding clears it.
void WindowStateSet(const std::string& flat);
bool WindowStateFind(const std::string& label, WindowPlace* out);

// Invoke the note callback without holding the table lock.
void WindowStateNote(const std::string& label, int x, int y, int width, int height, int maximized);

}  // namespace xgoef

#ifdef __cplusplus
extern "C" {
#endif

typedef void (*xgoef_window_state_fn)(const char* label,
                                      int x,
                                      int y,
                                      int width,
                                      int height,
                                      int maximized);

void xgoef_window_state_set(const char* encoded);
void xgoef_window_state_set_note(xgoef_window_state_fn fn);

#ifdef __cplusplus
}
#endif

#endif  // XGOEF_WINDOW_STATE_H_
