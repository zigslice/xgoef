#include "xgoef_handler.h"

#include <sstream>
#include <string>
#include <vector>

#include "include/base/cef_callback.h"
#include "include/cef_app.h"
#include "include/cef_parser.h"
#include "include/views/cef_browser_view.h"
#include "include/views/cef_window.h"
#include "include/wrapper/cef_closure_task.h"
#include "include/wrapper/cef_helpers.h"

#include "xgoef_eval_run.h"
#include "xgoef_webview.h"
#include "xgoef_window.h"

namespace xgoef {
namespace {

ClientHandler* g_instance = nullptr;

std::string GetDataURI(const std::string& data, const std::string& mime_type) {
  return "data:" + mime_type + ";base64," +
         CefURIEncode(CefBase64Encode(data.data(), data.size()), false)
             .ToString();
}

}  // namespace

ClientHandler::ClientHandler(bool is_alloy_style)
    : is_alloy_style_(is_alloy_style) {
  DCHECK(!g_instance);
  g_instance = this;
}

ClientHandler::~ClientHandler() {
  g_instance = nullptr;
}

// static
ClientHandler* ClientHandler::GetInstance() {
  return g_instance;
}

bool ClientHandler::OnProcessMessageReceived(
    CefRefPtr<CefBrowser> browser,
    CefRefPtr<CefFrame> frame,
    CefProcessId source_process,
    CefRefPtr<CefProcessMessage> message) {
  CEF_REQUIRE_UI_THREAD();
  if (message_router_ &&
      message_router_->OnProcessMessageReceived(browser, frame, source_process,
                                                message)) {
    return true;
  }
  return false;
}

void ClientHandler::OnTitleChange(CefRefPtr<CefBrowser> browser,
                                  const CefString& title) {
  CEF_REQUIRE_UI_THREAD();

  if (auto browser_view = CefBrowserView::GetForBrowser(browser)) {
    CefRefPtr<CefWindow> window = browser_view->GetWindow();
    if (window) {
      window->SetTitle(title);
    }
  } else if (is_alloy_style_) {
    PlatformTitleChange(browser, title);
  }
}

void ClientHandler::OnAfterCreated(CefRefPtr<CefBrowser> browser) {
  CEF_REQUIRE_UI_THREAD();

  // ShowDevTools calls this reentrantly. That browser must not take the
  // pending window label or keep the process alive after the last app window.
  const bool devtools = DevToolsConsume();
  if (!devtools) {
    CHECK_EQ(is_alloy_style_ ? CEF_RUNTIME_STYLE_ALLOY : CEF_RUNTIME_STYLE_CHROME,
             browser->GetHost()->GetRuntimeStyle());
  }

  if (!message_router_) {
    CefMessageRouterConfig config;
    message_router_ = CefMessageRouterBrowserSide::Create(config);
    message_router_->AddHandler(&ipc_handler_, false);
  }

  browser_list_.push_back(browser);
  if (devtools) {
    DevToolsNote(browser->GetIdentifier());
    return;
  }
  WindowBind(WindowTakePendingLabel(), browser);
}

bool ClientHandler::DoClose(CefRefPtr<CefBrowser> browser) {
  CEF_REQUIRE_UI_THREAD();
  if (browser && DevToolsIs(browser->GetIdentifier())) {
    return false;
  }
  if (CountAppBrowsers() <= 1) {
    is_closing_ = true;
  }
  return false;
}

void ClientHandler::OnBeforeClose(CefRefPtr<CefBrowser> browser) {
  CEF_REQUIRE_UI_THREAD();

  if (message_router_) {
    message_router_->OnBeforeClose(browser);
  }

  const bool devtools = browser && DevToolsIs(browser->GetIdentifier());
  if (devtools) {
    DevToolsForget(browser->GetIdentifier());
  } else {
    WindowUnbind(browser);
  }

  for (auto bit = browser_list_.begin(); bit != browser_list_.end(); ++bit) {
    if ((*bit)->IsSame(browser)) {
      browser_list_.erase(bit);
      break;
    }
  }

  if (CountAppBrowsers() == 0) {
    std::vector<CefRefPtr<CefBrowser>> tools;
    for (const auto& item : browser_list_) {
      if (item && DevToolsIs(item->GetIdentifier())) {
        tools.push_back(item);
      }
    }
    for (const auto& tool : tools) {
      if (DevToolsIs(tool->GetIdentifier()) && tool->GetHost()) {
        tool->GetHost()->CloseBrowser(true);
      }
    }
  }

  if (browser_list_.empty() && !quit_posted_) {
    quit_posted_ = true;
    if (message_router_) {
      message_router_->RemoveHandler(&ipc_handler_);
      message_router_ = nullptr;
    }
    CefQuitMessageLoop();
  }
}

int ClientHandler::CountAppBrowsers() const {
  int apps = 0;
  for (const auto& item : browser_list_) {
    if (!item || DevToolsIs(item->GetIdentifier())) {
      continue;
    }
    ++apps;
  }
  return apps;
}

void ClientHandler::OnLoadEnd(CefRefPtr<CefBrowser> browser,
                              CefRefPtr<CefFrame> frame,
                              int httpStatusCode) {
  CEF_REQUIRE_UI_THREAD();
  (void)httpStatusCode;
  if (!frame || !frame->IsMain() || !browser) {
    return;
  }
  if (DevToolsIs(browser->GetIdentifier())) {
    return;
  }
  std::string label;
  if (!WindowBoundLabel(browser, &label)) {
    return;
  }
  EvalRunInit(label, frame);
}

void ClientHandler::OnLoadError(CefRefPtr<CefBrowser> browser,
                                CefRefPtr<CefFrame> frame,
                                ErrorCode errorCode,
                                const CefString& errorText,
                                const CefString& failedUrl) {
  CEF_REQUIRE_UI_THREAD();
  (void)browser;

  if (!is_alloy_style_) {
    return;
  }
  if (errorCode == ERR_ABORTED) {
    return;
  }

  std::stringstream ss;
  ss << "<html><body bgcolor=\"white\">"
        "<h2>Failed to load URL "
     << std::string(failedUrl) << " with error " << std::string(errorText)
     << " (" << errorCode << ").</h2></body></html>";
  frame->LoadURL(GetDataURI(ss.str(), "text/html"));
}

bool ClientHandler::OnBeforeBrowse(CefRefPtr<CefBrowser> browser,
                                   CefRefPtr<CefFrame> frame,
                                   CefRefPtr<CefRequest> request,
                                   bool user_gesture,
                                   bool is_redirect) {
  CEF_REQUIRE_UI_THREAD();
  (void)request;
  (void)user_gesture;
  (void)is_redirect;
  if (message_router_) {
    message_router_->OnBeforeBrowse(browser, frame);
  }
  return false;
}

void ClientHandler::OnRenderProcessTerminated(CefRefPtr<CefBrowser> browser,
                                              TerminationStatus status,
                                              int error_code,
                                              const CefString& error_string) {
  CEF_REQUIRE_UI_THREAD();
  (void)status;
  (void)error_code;
  (void)error_string;
  if (message_router_) {
    message_router_->OnRenderProcessTerminated(browser);
  }
}

void ClientHandler::CloseAllBrowsers(bool force_close) {
  if (!CefCurrentlyOn(TID_UI)) {
    CefPostTask(TID_UI, base::BindOnce(&ClientHandler::CloseAllBrowsers, this,
                                       force_close));
    return;
  }
  if (browser_list_.empty()) {
    return;
  }
  for (const auto& browser : browser_list_) {
    browser->GetHost()->CloseBrowser(force_close);
  }
}

}  // namespace xgoef
