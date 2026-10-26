#include "xgoef_renderer.h"

#include <cstdio>
#include <string>

#include "include/cef_command_line.h"
#include "include/cef_v8.h"
#include "include/wrapper/cef_helpers.h"

#include "xgoef_bridge.h"

namespace xgoef {
namespace {

void LogBridge(const std::string& line) {
  static std::string last;
  if (line == last) {
    return;
  }
  last = line;
  std::fprintf(stderr, "xgoef: %s\n", line.c_str());
}

void InjectBridge(CefRefPtr<CefBrowser> browser,
                  CefRefPtr<CefFrame> frame,
                  CefRefPtr<CefV8Context> context) {
  if (!frame || !context) {
    return;
  }
  bool disabled = false;
  std::string root;
  std::string remote;
  CefRefPtr<CefCommandLine> command_line = CefCommandLine::GetGlobalCommandLine();
  if (command_line) {
    disabled = command_line->HasSwitch(kNoBridgeSwitch);
    root = command_line->GetSwitchValue(kAssetRootSwitch).ToString();
    remote = command_line->GetSwitchValue(kRemoteSwitch).ToString();
  }
  std::string main_url;
  if (browser && browser->GetMainFrame()) {
    main_url = browser->GetMainFrame()->GetURL().ToString();
  }
  const std::string url = frame->GetURL().ToString();
  if (!ShouldInjectBridge(disabled, frame->IsMain(), url, main_url, remote)) {
    return;
  }
  const std::string path = BridgeScriptPath(root);
  std::string script;
  std::string err;
  if (!ReadBridgeScript(path, &script, &err)) {
    LogBridge("bridge " + (path.empty() ? std::string("path") : path) + ": " + err);
    return;
  }
  if (!context->Enter()) {
    LogBridge("bridge eval: context enter failed");
    return;
  }
  CefRefPtr<CefV8Value> retval;
  CefRefPtr<CefV8Exception> exception;
  const bool ok =
      context->Eval(CefString(script), CefString("xgoef://bridge/xgoef.js"), 1, retval, exception);
  context->Exit();
  if (ok) {
    return;
  }
  std::string detail = "eval failed";
  int line = 0;
  if (exception) {
    detail = exception->GetMessage().ToString();
    line = exception->GetLineNumber();
  }
  std::fprintf(stderr, "xgoef: bridge eval xgoef://bridge/xgoef.js:%d: %s\n", line, detail.c_str());
}

}  // namespace

void RendererHandler::OnWebKitInitialized() {
  CefMessageRouterConfig config;
  router_ = CefMessageRouterRendererSide::Create(config);
}

void RendererHandler::OnContextCreated(CefRefPtr<CefBrowser> browser,
                                       CefRefPtr<CefFrame> frame,
                                       CefRefPtr<CefV8Context> context) {
  if (router_) {
    router_->OnContextCreated(browser, frame, context);
  }
  InjectBridge(browser, frame, context);
}

void RendererHandler::OnContextReleased(CefRefPtr<CefBrowser> browser,
                                        CefRefPtr<CefFrame> frame,
                                        CefRefPtr<CefV8Context> context) {
  if (router_) {
    router_->OnContextReleased(browser, frame, context);
  }
}

bool RendererHandler::OnProcessMessageReceived(
    CefRefPtr<CefBrowser> browser,
    CefRefPtr<CefFrame> frame,
    CefProcessId source_process,
    CefRefPtr<CefProcessMessage> message) {
  if (router_) {
    return router_->OnProcessMessageReceived(browser, frame, source_process,
                                             message);
  }
  return false;
}

}  // namespace xgoef
