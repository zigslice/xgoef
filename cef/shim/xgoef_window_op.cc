// Applies a Views window verb on the UI thread.
// Compiled with CEF by CMake. The policy test does not include this file.

#include "xgoef_window_op.h"
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

CefRefPtr<CefWindow> ViewsWindow(const std::string& label) {
  std::string lab = label.empty() ? "main" : label;
  CefRefPtr<CefBrowser> browser = WindowBrowser(lab);
  if (!browser) {
    return nullptr;
  }
  CefRefPtr<CefBrowserView> view = CefBrowserView::GetForBrowser(browser);
  if (!view) {
    return nullptr;
  }
  return view->GetWindow();
}

int ApplyOp(CefRefPtr<CefWindow> window, int op, int a, int b, const std::string& text) {
  switch (op) {
    case kWindowOpMinimize:
      window->Minimize();
      return 0;
    case kWindowOpMaximize:
      window->Maximize();
      return 0;
    case kWindowOpUnminimize:
      if (window->IsMinimized()) {
        window->Restore();
      }
      return 0;
    case kWindowOpUnmaximize:
      if (window->IsMaximized()) {
        window->Restore();
      }
      return 0;
    case kWindowOpToggleMaximize:
      if (window->IsMaximized()) {
        window->Restore();
      } else {
        window->Maximize();
      }
      return 0;
    case kWindowOpShow:
      window->Show();
      return 0;
    case kWindowOpHide:
      window->Hide();
      return 0;
    case kWindowOpCenter:
      window->CenterWindow(window->GetSize());
      return 0;
    case kWindowOpSetTitle:
      if (!TitleOK(text)) {
        return -2;
      }
      window->SetTitle(CefString(text));
      return 0;
    case kWindowOpSetAlwaysOnTop:
      window->SetAlwaysOnTop(a != 0);
      return 0;
    case kWindowOpSetFullscreen:
      window->SetFullscreen(a != 0);
      return 0;
    case kWindowOpSetSize:
      if (!SizeOK(a, b)) {
        return -2;
      }
      window->SetSize(CefSize(a, b));
      return 0;
    default:
      return -2;
  }
}

int QueryFlag(CefRefPtr<CefWindow> window, int query) {
  switch (query) {
    case kWindowQueryMinimized:
      return window->IsMinimized() ? 1 : 0;
    case kWindowQueryMaximized:
      return window->IsMaximized() ? 1 : 0;
    case kWindowQueryFullscreen:
      return window->IsFullscreen() ? 1 : 0;
    case kWindowQueryAlwaysOnTop:
      return window->IsAlwaysOnTop() ? 1 : 0;
    case kWindowQueryVisible:
      return window->IsVisible() ? 1 : 0;
    default:
      return -1;
  }
}

int OpOnUI(const std::string& label, int op, int a, int b, const std::string& text) {
  CEF_REQUIRE_UI_THREAD();
  CefRefPtr<CefWindow> window = ViewsWindow(label);
  if (!window) {
    return -1;
  }
  return ApplyOp(window, op, a, b, text);
}

int QueryOnUI(const std::string& label, int query) {
  CEF_REQUIRE_UI_THREAD();
  CefRefPtr<CefWindow> window = ViewsWindow(label);
  if (!window) {
    return -1;
  }
  return QueryFlag(window, query);
}

class WindowCall : public CefBaseRefCounted {
 public:
  int rc = -1;
  CefRefPtr<CefWaitableEvent> event = CefWaitableEvent::CreateWaitableEvent(false, false);

 private:
  IMPLEMENT_REFCOUNTING(WindowCall);
};

void OpTask(std::string label, int op, int a, int b, std::string text, CefRefPtr<WindowCall> call) {
  call->rc = OpOnUI(label, op, a, b, text);
  call->event->Signal();
}

void QueryTask(std::string label, int query, CefRefPtr<WindowCall> call) {
  call->rc = QueryOnUI(label, query);
  call->event->Signal();
}

int RunOp(const std::string& label, int op, int a, int b, const std::string& text) {
  if (CefCurrentlyOn(TID_UI)) {
    return OpOnUI(label, op, a, b, text);
  }
  CefRefPtr<WindowCall> call = new WindowCall();
  CefPostTask(TID_UI, base::BindOnce(&OpTask, label, op, a, b, text, call));
  if (!call->event->TimedWait(2000)) {
    return -1;
  }
  return call->rc;
}

int RunQuery(const std::string& label, int query) {
  if (CefCurrentlyOn(TID_UI)) {
    return QueryOnUI(label, query);
  }
  CefRefPtr<WindowCall> call = new WindowCall();
  CefPostTask(TID_UI, base::BindOnce(&QueryTask, label, query, call));
  if (!call->event->TimedWait(2000)) {
    return -1;
  }
  return call->rc;
}

}  // namespace
}  // namespace xgoef

extern "C" int xgoef_window_op(const char* label, int op, int a, int b, const char* text) {
  return xgoef::RunOp(label ? label : "", op, a, b, text ? text : "");
}

extern "C" int xgoef_window_query(const char* label, int query) {
  return xgoef::RunQuery(label ? label : "", query);
}
