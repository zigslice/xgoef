// Taskbar skip and progress shared by the Go plugin and the native shim.
// No CEF headers, so this file can be compiled on its own.
#ifndef XGOEF_TASKBAR_H_
#define XGOEF_TASKBAR_H_

#include <string>

namespace xgoef {

// EWMH ClientMessage that adds or removes _NET_WM_STATE_SKIP_TASKBAR.
// action is 1 to add and 0 to remove. source is 1 (application).
struct SkipTaskbarMessage {
  const char* type;
  long action;
  const char* atom;
  long source;
};

inline SkipTaskbarMessage DescribeSkip(bool skip) {
  return {"_NET_WM_STATE", skip ? 1L : 0L, "_NET_WM_STATE_SKIP_TASKBAR", 1L};
}

// XApp progress property. progress -1 removes it. 0..100 sets the cardinal.
struct ProgressProperty {
  bool remove;
  const char* name;
  unsigned long value;
};

inline bool DescribeProgress(int progress, ProgressProperty* out) {
  if (out == nullptr || progress < -1 || progress > 100) {
    return false;
  }
  out->name = "_NET_WM_XAPP_PROGRESS";
  out->remove = progress < 0;
  out->value = progress < 0 ? 0 : static_cast<unsigned long>(progress);
  return true;
}

// Remember that the next Views window with this label should start skipped
// or shown. A second call for the same label replaces the first.
void TaskbarExpect(const std::string& label, bool skip);
// Consume one expectation. False when this label has none.
bool TaskbarTakeExpected(const std::string& label, bool* skip);

}  // namespace xgoef

#endif  // XGOEF_TASKBAR_H_
