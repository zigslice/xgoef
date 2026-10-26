#include "xgoef_taskbar.h"
#include "xgoef_window.h"

#include <string>

#include "include/base/cef_callback.h"
#include "include/cef_browser.h"
#include "include/cef_command_line.h"
#include "include/cef_waitable_event.h"
#include "include/views/cef_browser_view.h"
#include "include/views/cef_window.h"
#include "include/wrapper/cef_closure_task.h"
#include "include/wrapper/cef_helpers.h"

#if defined(CEF_X11)
#include <X11/Xlib.h>
#endif

namespace xgoef {

#if defined(CEF_X11)
void SendSkipTaskbar(Display* display, Window window, bool skip);
void SetTaskbarProgress(Display* display, Window window, int progress);
#endif

namespace {

class TaskbarCall : public CefBaseRefCounted {
 public:
  TaskbarCall() : event(CefWaitableEvent::CreateWaitableEvent(false, false)) {}
  CefRefPtr<CefWaitableEvent> event;
  int rc = -1;

 private:
  IMPLEMENT_REFCOUNTING(TaskbarCall);
};

#if defined(CEF_X11)
CefWindowHandle NativeHandle(CefRefPtr<CefBrowser> browser) {
  if (!browser) {
    return kNullWindowHandle;
  }
  CefRefPtr<CefBrowserView> view = CefBrowserView::GetForBrowser(browser);
  if (view) {
    CefRefPtr<CefWindow> window = view->GetWindow();
    if (window) {
      return window->GetWindowHandle();
    }
  }
  return browser->GetHost()->GetWindowHandle();
}

int ApplySkipOnUI(const std::string& label, bool skip) {
  CEF_REQUIRE_UI_THREAD();
  const std::string lab = label.empty() ? "main" : label;
  CefRefPtr<CefBrowser> browser = WindowBrowser(lab);
  const CefWindowHandle handle = NativeHandle(browser);
  Display* display = cef_get_xdisplay();
  if (handle == kNullWindowHandle || display == nullptr) {
    return -1;
  }
  SendSkipTaskbar(display, handle, skip);
  return 0;
}

int ApplyProgressOnUI(const std::string& label, int progress) {
  CEF_REQUIRE_UI_THREAD();
  ProgressProperty prop;
  if (!DescribeProgress(progress, &prop)) {
    return -2;
  }
  const std::string lab = label.empty() ? "main" : label;
  CefRefPtr<CefBrowser> browser = WindowBrowser(lab);
  const CefWindowHandle handle = NativeHandle(browser);
  Display* display = cef_get_xdisplay();
  if (handle == kNullWindowHandle || display == nullptr) {
    return -1;
  }
  SetTaskbarProgress(display, handle, progress);
  return 0;
}
#endif

void SkipTask(std::string label, bool skip, CefRefPtr<TaskbarCall> call) {
#if defined(CEF_X11)
  call->rc = ApplySkipOnUI(label, skip);
#else
  (void)label;
  (void)skip;
  call->rc = -1;
#endif
  call->event->Signal();
}

void ProgressTask(std::string label, int progress, CefRefPtr<TaskbarCall> call) {
#if defined(CEF_X11)
  call->rc = ApplyProgressOnUI(label, progress);
#else
  (void)label;
  (void)progress;
  call->rc = -1;
#endif
  call->event->Signal();
}

int WaitSkip(const std::string& label, bool skip) {
#if defined(CEF_X11)
  if (CefCurrentlyOn(TID_UI)) {
    return ApplySkipOnUI(label, skip);
  }
  CefRefPtr<TaskbarCall> call = new TaskbarCall();
  CefPostTask(TID_UI, base::BindOnce(&SkipTask, label, skip, call));
  if (!call->event->TimedWait(2000)) {
    return -1;
  }
  return call->rc;
#else
  (void)label;
  (void)skip;
  return -1;
#endif
}

int WaitProgress(const std::string& label, int progress) {
#if defined(CEF_X11)
  if (CefCurrentlyOn(TID_UI)) {
    return ApplyProgressOnUI(label, progress);
  }
  CefRefPtr<TaskbarCall> call = new TaskbarCall();
  CefPostTask(TID_UI, base::BindOnce(&ProgressTask, label, progress, call));
  if (!call->event->TimedWait(2000)) {
    return -1;
  }
  return call->rc;
#else
  (void)label;
  (void)progress;
  return -1;
#endif
}

}  // namespace

void ApplyTaskbarHint(CefRefPtr<CefWindow> window, const std::string& label) {
  bool skip = false;
  const bool from_expect = TaskbarTakeExpected(label, &skip);
  if (!from_expect) {
    static bool launch_done = false;
    if (launch_done) {
      return;
    }
    launch_done = true;
    CefRefPtr<CefCommandLine> cmd = CefCommandLine::GetGlobalCommandLine();
    if (!cmd || !cmd->HasSwitch("xgoef-skip-taskbar")) {
      return;
    }
    skip = true;
  }
#if defined(CEF_X11)
  if (!window) {
    return;
  }
  const CefWindowHandle handle = window->GetWindowHandle();
  Display* display = cef_get_xdisplay();
  if (handle == kNullWindowHandle || display == nullptr) {
    return;
  }
  SendSkipTaskbar(display, handle, skip);
#else
  (void)window;
  (void)skip;
#endif
}

}  // namespace xgoef

extern "C" int xgoef_taskbar_skip(const char* label, int skip) {
  return xgoef::WaitSkip(label ? label : "", skip != 0);
}

extern "C" int xgoef_taskbar_progress(const char* label, int progress) {
  return xgoef::WaitProgress(label ? label : "", progress);
}
