// Runs a trusted script on the UI thread.
// The source is host or config text. The live frame URL has to match the
// pinned origin or this returns -2 and does not call ExecuteJavaScript.

#include "xgoef_eval_run.h"

#include "xgoef_eval.h"

#include "xgoef_webview.h"
#include "xgoef_window.h"

#include <string>

#include "include/base/cef_callback.h"
#include "include/cef_browser.h"
#include "include/cef_frame.h"
#include "include/cef_waitable_event.h"
#include "include/wrapper/cef_closure_task.h"
#include "include/wrapper/cef_helpers.h"

namespace xgoef {
namespace {

class EvalCall : public CefBaseRefCounted {
 public:
  int rc = -1;
  CefRefPtr<CefWaitableEvent> event = CefWaitableEvent::CreateWaitableEvent(false, false);

 private:
  IMPLEMENT_REFCOUNTING(EvalCall);
};

int RunOnUI(const std::string& label, const std::string& allowed, const std::string& script) {
  CEF_REQUIRE_UI_THREAD();
  if (!ScriptAccepted(script) || !OriginAllowed(allowed, allowed)) {
    return !ScriptAccepted(script) ? -3 : -2;
  }
  CefRefPtr<CefBrowser> browser = WindowBrowser(label.empty() ? "main" : label);
  if (!browser || !browser->GetMainFrame() || DevToolsIs(browser->GetIdentifier())) {
    return -1;
  }
  CefRefPtr<CefFrame> frame = browser->GetMainFrame();
  if (!frame->IsMain()) {
    return -1;
  }
  std::string url = frame->GetURL().ToString();
  if (!OriginAllowed(url, allowed)) {
    return -2;
  }
  frame->ExecuteJavaScript(script, frame->GetURL(), 0);
  return 0;
}

void RunTask(std::string label, std::string allowed, std::string script, CefRefPtr<EvalCall> call) {
  call->rc = RunOnUI(label, allowed, script);
  call->event->Signal();
}

}  // namespace

void EvalRunInit(const std::string& label, CefRefPtr<CefFrame> frame) {
  CEF_REQUIRE_UI_THREAD();
  if (!frame || !frame->IsMain()) {
    return;
  }
  std::string allowed;
  std::vector<std::string> scripts;
  if (!EvalLookup(label, &allowed, &scripts)) {
    return;
  }
  std::string url = frame->GetURL().ToString();
  if (!OriginAllowed(url, allowed)) {
    return;
  }
  for (const std::string& script : scripts) {
    if (!ScriptAccepted(script)) {
      continue;
    }
    frame->ExecuteJavaScript(script, frame->GetURL(), 0);
  }
}

int QueryEval(const std::string& label, const std::string& allowed, const std::string& script) {
  if (CefCurrentlyOn(TID_UI)) {
    return RunOnUI(label, allowed, script);
  }
  CefRefPtr<EvalCall> call = new EvalCall();
  CefPostTask(TID_UI, base::BindOnce(&RunTask, label, allowed, script, call));
  if (!call->event->TimedWait(2000)) {
    return -1;
  }
  return call->rc;
}

}  // namespace xgoef

extern "C" int xgoef_eval_run(const char* label, const char* allowed_url, const char* script) {
  return xgoef::QueryEval(label ? label : "", allowed_url ? allowed_url : "", script ? script : "");
}
