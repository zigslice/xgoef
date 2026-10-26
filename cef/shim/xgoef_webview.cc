// Print and DevTools on the CEF UI thread.
// ShowDevTools calls OnAfterCreated before it returns. Expect is set only
// when HasDevTools is still false, and cancelled if the call did not open one.

#include "xgoef_webview.h"

#include "xgoef_handler.h"
#include "xgoef_window.h"

#include <string>

#include "include/base/cef_callback.h"
#include "include/cef_browser.h"
#include "include/cef_waitable_event.h"
#include "include/wrapper/cef_closure_task.h"
#include "include/wrapper/cef_helpers.h"

namespace xgoef {
namespace webview {

class WebViewCall : public CefBaseRefCounted {
 public:
  int rc = -1;
  CefRefPtr<CefWaitableEvent> event = CefWaitableEvent::CreateWaitableEvent(false, false);

 private:
  IMPLEMENT_REFCOUNTING(WebViewCall);
};

CefRefPtr<CefBrowser> BrowserFor(const std::string& label) {
  return WindowBrowser(label.empty() ? "main" : label);
}

int PrintOnUI(const std::string& label) {
  CEF_REQUIRE_UI_THREAD();
  CefRefPtr<CefBrowser> browser = BrowserFor(label);
  if (!browser || !browser->GetHost()) {
    return -1;
  }
  browser->GetHost()->Print();
  return 0;
}

int OpenOnUI(const std::string& label) {
  CEF_REQUIRE_UI_THREAD();
  CefRefPtr<CefBrowser> browser = BrowserFor(label);
  if (!browser || !browser->GetHost()) {
    return -1;
  }
  CefRefPtr<CefBrowserHost> host = browser->GetHost();
  if (host->HasDevTools()) {
    return 0;
  }
  ClientHandler* handler = ClientHandler::GetInstance();
  if (!handler) {
    return -1;
  }
  DevToolsExpect();
  CefWindowInfo window_info;
  CefBrowserSettings settings;
  CefPoint inspect_at;
  host->ShowDevTools(window_info, CefRefPtr<CefClient>(handler), settings, inspect_at);
  if (!host->HasDevTools()) {
    DevToolsCancel();
  }
  return 0;
}

int CloseOnUI(const std::string& label) {
  CEF_REQUIRE_UI_THREAD();
  CefRefPtr<CefBrowser> browser = BrowserFor(label);
  if (!browser || !browser->GetHost()) {
    return -1;
  }
  browser->GetHost()->CloseDevTools();
  return 0;
}

int IsOpenOnUI(const std::string& label) {
  CEF_REQUIRE_UI_THREAD();
  CefRefPtr<CefBrowser> browser = BrowserFor(label);
  if (!browser || !browser->GetHost()) {
    return -1;
  }
  return browser->GetHost()->HasDevTools() ? 1 : 0;
}

void PrintTask(std::string label, CefRefPtr<WebViewCall> call) {
  call->rc = PrintOnUI(label);
  call->event->Signal();
}

void OpenTask(std::string label, CefRefPtr<WebViewCall> call) {
  call->rc = OpenOnUI(label);
  call->event->Signal();
}

void CloseTask(std::string label, CefRefPtr<WebViewCall> call) {
  call->rc = CloseOnUI(label);
  call->event->Signal();
}

void IsOpenTask(std::string label, CefRefPtr<WebViewCall> call) {
  call->rc = IsOpenOnUI(label);
  call->event->Signal();
}

int CallOnUI(const std::string& label, void (*task)(std::string, CefRefPtr<WebViewCall>),
             int (*on_ui)(const std::string&)) {
  if (CefCurrentlyOn(TID_UI)) {
    return on_ui(label);
  }
  CefRefPtr<WebViewCall> call = new WebViewCall();
  CefPostTask(TID_UI, base::BindOnce(task, label, call));
  if (!call->event->TimedWait(2000)) {
    return -1;
  }
  return call->rc;
}

}  // namespace webview

int QueryPrint(const std::string& label) {
  return webview::CallOnUI(label, &webview::PrintTask, &webview::PrintOnUI);
}

int QueryDevToolsOpen(const std::string& label) {
  return webview::CallOnUI(label, &webview::OpenTask, &webview::OpenOnUI);
}

int QueryDevToolsClose(const std::string& label) {
  return webview::CallOnUI(label, &webview::CloseTask, &webview::CloseOnUI);
}

int QueryDevToolsOpenState(const std::string& label) {
  return webview::CallOnUI(label, &webview::IsOpenTask, &webview::IsOpenOnUI);
}

}  // namespace xgoef

extern "C" int xgoef_window_print(const char* label) {
  return xgoef::QueryPrint(label ? label : "");
}

extern "C" int xgoef_devtools_open(const char* label) {
  return xgoef::QueryDevToolsOpen(label ? label : "");
}

extern "C" int xgoef_devtools_close(const char* label) {
  return xgoef::QueryDevToolsClose(label ? label : "");
}

extern "C" int xgoef_devtools_is_open(const char* label) {
  return xgoef::QueryDevToolsOpenState(label ? label : "");
}
