#include "xgoef_app.h"

#include <string>

#include "include/cef_browser.h"
#include "include/cef_command_line.h"
#include "include/cef_scheme.h"
#include "include/views/cef_browser_view.h"
#include "include/views/cef_window.h"
#include "include/wrapper/cef_helpers.h"

#include "xgoef_bridge.h"
#include "xgoef_crash.h"
#include "xgoef_csp.h"
#include "xgoef_handler.h"
#include "xgoef_menu_views.h"
#include "xgoef_window.h"
#include "xgoef_window_limits.h"

#include <cstdio>

namespace xgoef {
void RegisterAssetScheme(const std::string& root, bool csp_disabled, const std::string& csp);
}

namespace xgoef {
namespace {

class WindowDelegate : public CefWindowDelegate {
 public:
  WindowDelegate(CefRefPtr<CefBrowserView> browser_view,
                 cef_runtime_style_t runtime_style,
                 cef_show_state_t initial_show_state,
                 const WindowChrome& chrome)
      : browser_view_(browser_view),
        runtime_style_(runtime_style),
        initial_show_state_(initial_show_state),
        chrome_(chrome) {}

  bool IsFrameless(CefRefPtr<CefWindow> window) override {
    (void)window;
    return !chrome_.decorations;
  }

  bool CanResize(CefRefPtr<CefWindow> window) override {
    (void)window;
    return LimitsForDelegate(WindowCreatingLabel(browser_view_)).resizable;
  }

  bool CanMinimize(CefRefPtr<CefWindow> window) override {
    (void)window;
    return LimitsForDelegate(WindowCreatingLabel(browser_view_)).minimizable;
  }

  bool CanMaximize(CefRefPtr<CefWindow> window) override {
    (void)window;
    return LimitsForDelegate(WindowCreatingLabel(browser_view_)).maximizable;
  }

  CefSize GetMinimumSize(CefRefPtr<CefView> view) override {
    (void)view;
    WindowLimits limits = LimitsForDelegate(WindowCreatingLabel(browser_view_));
    return CefSize(limits.min_width, limits.min_height);
  }

  CefSize GetMaximumSize(CefRefPtr<CefView> view) override {
    (void)view;
    WindowLimits limits = LimitsForDelegate(WindowCreatingLabel(browser_view_));
    return CefSize(limits.max_width, limits.max_height);
  }

  void OnWindowCreated(CefRefPtr<CefWindow> window) override {
    window->AddChildView(WrapWithAppMenu(window, browser_view_));
    ApplySavedPlace(window, WindowCreatingLabel(browser_view_));
    if (initial_show_state_ != CEF_SHOW_STATE_HIDDEN) {
      window->Show();
    }
  }

  void OnWindowDestroyed(CefRefPtr<CefWindow> window) override {
    (void)window;
    browser_view_ = nullptr;
  }

  void OnWindowBoundsChanged(CefRefPtr<CefWindow> window, const CefRect& new_bounds) override {
    (void)new_bounds;
    NoteWindowGeometry(window, browser_view_);
  }

  bool CanClose(CefRefPtr<CefWindow> window) override {
    if (browser_view_ && !LimitsForDelegate(WindowCreatingLabel(browser_view_)).closable) {
      return false;
    }
    NoteWindowGeometry(window, browser_view_);
    if (!browser_view_) {
      return true;
    }
    CefRefPtr<CefBrowser> browser = browser_view_->GetBrowser();
    if (browser) {
      return browser->GetHost()->TryCloseBrowser();
    }
    return true;
  }

  CefSize GetPreferredSize(CefRefPtr<CefView> view) override {
    (void)view;
    int width = 0;
    int height = 0;
    DefaultWindowSize(&width, &height);
    int saved_w = 0;
    int saved_h = 0;
    if (SavedSize(WindowCreatingLabel(browser_view_), &saved_w, &saved_h)) {
      width = saved_w;
      height = saved_h;
    }
    return CefSize(width, height);
  }

  cef_show_state_t GetInitialShowState(CefRefPtr<CefWindow> window) override {
    (void)window;
    return InitialShowState(WindowCreatingLabel(browser_view_), initial_show_state_);
  }

  cef_runtime_style_t GetWindowRuntimeStyle() override {
    return runtime_style_;
  }

 private:
  CefRefPtr<CefBrowserView> browser_view_;
  const cef_runtime_style_t runtime_style_;
  const cef_show_state_t initial_show_state_;
  const WindowChrome chrome_;
  IMPLEMENT_REFCOUNTING(WindowDelegate);
};

class BrowserViewDelegate : public CefBrowserViewDelegate {
 public:
  BrowserViewDelegate(cef_runtime_style_t runtime_style, const WindowChrome& chrome)
      : runtime_style_(runtime_style), chrome_(chrome) {}

  bool OnPopupBrowserViewCreated(CefRefPtr<CefBrowserView> browser_view,
                                 CefRefPtr<CefBrowserView> popup_browser_view,
                                 bool is_devtools) override {
    (void)browser_view;
    (void)is_devtools;
    CefWindow::CreateTopLevelWindow(new WindowDelegate(
        popup_browser_view, runtime_style_, CEF_SHOW_STATE_NORMAL, chrome_));
    return true;
  }

  cef_runtime_style_t GetBrowserRuntimeStyle() override {
    return runtime_style_;
  }

 private:
  const cef_runtime_style_t runtime_style_;
  const WindowChrome chrome_;
  IMPLEMENT_REFCOUNTING(BrowserViewDelegate);
};

}  // namespace

App::App() : renderer_(new RendererHandler) {}

void App::OnRegisterCustomSchemes(CefRawPtr<CefSchemeRegistrar> registrar) {
  // Same options in the browser process and xgoef-helper. STANDARD makes the
  // origin xgoef://app; the file handler itself runs in the browser process.
  registrar->AddCustomScheme(
      "xgoef", CEF_SCHEME_OPTION_STANDARD | CEF_SCHEME_OPTION_SECURE |
                   CEF_SCHEME_OPTION_CORS_ENABLED |
                   CEF_SCHEME_OPTION_FETCH_ENABLED);
}

void App::OnBeforeCommandLineProcessing(
    const CefString& process_type,
    CefRefPtr<CefCommandLine> command_line) {
  if (!command_line) {
    return;
  }
  const CrashSwitches plan = CrashSwitchesFor(
      process_type.ToString(),
      command_line->GetSwitchValue(kCrashDumpsSwitch).ToString(),
      command_line->HasSwitch(kCrashDumpsDirSwitch),
      command_line->HasSwitch(kEnableCrashReporterSwitch));
  if (plan.append_dir) {
    command_line->AppendSwitchWithValue(kCrashDumpsDirSwitch, CefString(plan.dir));
  }
  if (plan.append_enable) {
    command_line->AppendSwitch(kEnableCrashReporterSwitch);
  }
}

void App::OnBeforeChildProcessLaunch(CefRefPtr<CefCommandLine> command_line) {
  if (!command_line) {
    return;
  }
  CefRefPtr<CefCommandLine> browser = CefCommandLine::GetGlobalCommandLine();
  if (!browser) {
    return;
  }
  const std::string root = browser->GetSwitchValue(kAssetRootSwitch).ToString();
  if (!root.empty() && !command_line->HasSwitch(kAssetRootSwitch)) {
    command_line->AppendSwitchWithValue(kAssetRootSwitch, CefString(root));
  }
  if (browser->HasSwitch(kNoBridgeSwitch) && !command_line->HasSwitch(kNoBridgeSwitch)) {
    command_line->AppendSwitch(kNoBridgeSwitch);
  }
  const std::string remote = browser->GetSwitchValue(kRemoteSwitch).ToString();
  if (!remote.empty() && !command_line->HasSwitch(kRemoteSwitch)) {
    command_line->AppendSwitchWithValue(kRemoteSwitch, CefString(remote));
  }
}

void App::OnContextInitialized() {
  CEF_REQUIRE_UI_THREAD();

  CefRefPtr<CefCommandLine> command_line =
      CefCommandLine::GetGlobalCommandLine();

  std::string asset_root =
      command_line->GetSwitchValue("xgoef-asset-root").ToString();
  if (!asset_root.empty()) {
    RegisterAssetScheme(asset_root, command_line->HasSwitch(kNoCSPSwitch),
                        command_line->GetSwitchValue(kCSPSwitch).ToString());
  }

  cef_runtime_style_t runtime_style = CEF_RUNTIME_STYLE_DEFAULT;
  bool use_alloy_style = command_line->HasSwitch("use-alloy-style");
  if (use_alloy_style) {
    runtime_style = CEF_RUNTIME_STYLE_ALLOY;
  }

  CefRefPtr<ClientHandler> handler(new ClientHandler(use_alloy_style));
  CefBrowserSettings browser_settings;
  WindowChrome chrome =
      ChromeOrDefault(command_line->GetSwitchValue("xgoef-chrome").ToString());
  ApplyBrowserBackground(&browser_settings, chrome);
  CefRefPtr<CefRequestContext> request_context = ChromeContext(chrome);

  std::string url = command_line->GetSwitchValue("url");
  if (url.empty()) {
    url = "about:blank";
  }

  std::string label = command_line->GetSwitchValue("xgoef-label");
  if (label.empty()) {
    label = "main";
  }
  WindowSetPendingLabel(label);

  const bool use_views = !command_line->HasSwitch("use-native");
  const std::string limits_text = command_line->GetSwitchValue("xgoef-limits").ToString();
  if (use_views && !limits_text.empty()) {
    WindowLimits limits;
    std::string err;
    if (!ParseWindowLimits(limits_text, &limits, &err)) {
      std::fprintf(stderr, "xgoef: %s\n", err.c_str());
    } else {
      WindowLimitsSet(label, limits);
    }
  }

  if (use_views) {
    CefRefPtr<CefBrowserView> browser_view = CefBrowserView::CreateBrowserView(
        handler, url, browser_settings, nullptr, request_context,
        new BrowserViewDelegate(runtime_style, chrome));

    cef_show_state_t initial_show_state = CEF_SHOW_STATE_NORMAL;
    const std::string& show_state_value =
        command_line->GetSwitchValue("initial-show-state");
    if (show_state_value == "minimized") {
      initial_show_state = CEF_SHOW_STATE_MINIMIZED;
    } else if (show_state_value == "maximized") {
      initial_show_state = CEF_SHOW_STATE_MAXIMIZED;
    }

    CefWindow::CreateTopLevelWindow(
        new WindowDelegate(browser_view, runtime_style, initial_show_state, chrome));
  } else {
    // Native windows have no Views delegate, so IsFrameless does not apply.
    CefWindowInfo window_info;
    window_info.runtime_style = runtime_style;
    CefBrowserHost::CreateBrowser(window_info, handler, url, browser_settings,
                                  nullptr, request_context);
  }
}

CefRefPtr<CefClient> App::GetDefaultClient() {
  return ClientHandler::GetInstance();
}

}  // namespace xgoef
