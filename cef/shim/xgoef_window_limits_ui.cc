// Applies a window limit on the UI thread once a Views window exists.
// Compiled with CEF by CMake. The policy test does not include this file.

#include "xgoef_window_limits.h"
#include "xgoef_window.h"

#include <string>

#include "include/base/cef_callback.h"
#include "include/cef_task.h"
#include "include/cef_waitable_event.h"
#include "include/views/cef_browser_view.h"
#include "include/views/cef_window.h"
#include "include/wrapper/cef_closure_task.h"
#include "include/wrapper/cef_helpers.h"

namespace xgoef {
namespace {

std::string MainLabel(const std::string& label) { return label.empty() ? "main" : label; }

CefRefPtr<CefWindow> ViewsWindow(const std::string& label) {
  CefRefPtr<CefBrowser> browser = WindowBrowser(MainLabel(label));
  if (!browser) {
    return nullptr;
  }
  CefRefPtr<CefBrowserView> view = CefBrowserView::GetForBrowser(browser);
  if (!view) {
    return nullptr;
  }
  return view->GetWindow();
}

int Pack(int a, int b) { return (a << 16) | (b & 0xffff); }

int ApplyOnUI(const std::string& label, int field, int a, int b) {
  CEF_REQUIRE_UI_THREAD();
  if (!ViewsWindow(label)) {
    return -1;
  }
  std::string lab = MainLabel(label);
  WindowLimits limits = WindowLimitsGet(lab);
  if (!ApplyWindowLimit(&limits, field, a, b)) {
    return -2;
  }
  WindowLimitsSet(lab, limits);
  return 0;
}

int ReadOnUI(const std::string& label, int field) {
  CEF_REQUIRE_UI_THREAD();
  if (!ViewsWindow(label)) {
    return -1;
  }
  WindowLimits limits = WindowLimitsGet(MainLabel(label));
  switch (field) {
    case kWindowLimitResizable:
      return Pack(limits.resizable ? 1 : 0, 0);
    case kWindowLimitMinimizable:
      return Pack(limits.minimizable ? 1 : 0, 0);
    case kWindowLimitMaximizable:
      return Pack(limits.maximizable ? 1 : 0, 0);
    case kWindowLimitClosable:
      return Pack(limits.closable ? 1 : 0, 0);
    case kWindowLimitMinSize:
      return Pack(limits.min_width, limits.min_height);
    case kWindowLimitMaxSize:
      return Pack(limits.max_width, limits.max_height);
    default:
      return -2;
  }
}

class LimitCall : public CefBaseRefCounted {
 public:
  int rc = -1;
  CefRefPtr<CefWaitableEvent> event = CefWaitableEvent::CreateWaitableEvent(false, false);

 private:
  IMPLEMENT_REFCOUNTING(LimitCall);
};

void ApplyTask(std::string label, int field, int a, int b, CefRefPtr<LimitCall> call) {
  call->rc = ApplyOnUI(label, field, a, b);
  call->event->Signal();
}

void ReadTask(std::string label, int field, CefRefPtr<LimitCall> call) {
  call->rc = ReadOnUI(label, field);
  call->event->Signal();
}

int RunApply(const std::string& label, int field, int a, int b) {
  if (CefCurrentlyOn(TID_UI)) {
    return ApplyOnUI(label, field, a, b);
  }
  CefRefPtr<LimitCall> call = new LimitCall();
  CefPostTask(TID_UI, base::BindOnce(&ApplyTask, label, field, a, b, call));
  if (!call->event->TimedWait(2000)) {
    return -1;
  }
  return call->rc;
}

int RunRead(const std::string& label, int field) {
  if (CefCurrentlyOn(TID_UI)) {
    return ReadOnUI(label, field);
  }
  CefRefPtr<LimitCall> call = new LimitCall();
  CefPostTask(TID_UI, base::BindOnce(&ReadTask, label, field, call));
  if (!call->event->TimedWait(2000)) {
    return -1;
  }
  return call->rc;
}

}  // namespace
}  // namespace xgoef

extern "C" int xgoef_window_limit(const char* label, int field, int a, int b) {
  return xgoef::RunApply(label ? label : "", field, a, b);
}

extern "C" int xgoef_window_limit_get(const char* label, int field) {
  return xgoef::RunRead(label ? label : "", field);
}
