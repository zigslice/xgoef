#include "xgoef_window.h"

#include <map>
#include <mutex>
#include <string>

#include "include/base/cef_callback.h"
#include "include/cef_command_line.h"
#include "include/cef_request_context_handler.h"
#include "include/views/cef_browser_view.h"
#include "include/views/cef_window.h"
#include "include/wrapper/cef_closure_task.h"
#include "include/wrapper/cef_helpers.h"

#include "xgoef_handler.h"
#include "xgoef_menu_views.h"
#include "xgoef_window_state.h"

#include <cstdio>

namespace xgoef {
namespace {

std::mutex g_mu;
std::string g_pending_label = "main";
std::map<std::string, CefRefPtr<CefBrowser>> g_by_label;
std::map<int, std::string> g_label_by_id;

class SizedWindowDelegate : public CefWindowDelegate {
 public:
  SizedWindowDelegate(CefRefPtr<CefBrowserView> browser_view,
                      cef_runtime_style_t runtime_style,
                      int width,
                      int height,
                      const std::string& title,
                      const WindowChrome& chrome)
      : browser_view_(browser_view),
        runtime_style_(runtime_style),
        width_(width > 0 ? width : 960),
        height_(height > 0 ? height : 640),
        title_(title),
        chrome_(chrome) {}

  bool IsFrameless(CefRefPtr<CefWindow> window) override {
    (void)window;
    return !chrome_.decorations;
  }

  void OnWindowCreated(CefRefPtr<CefWindow> window) override {
    window->AddChildView(WrapWithAppMenu(window, browser_view_));
    if (!title_.empty()) {
      window->SetTitle(title_);
    }
    ApplySavedPlace(window, WindowCreatingLabel(browser_view_));
    window->Show();
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
    int width = width_;
    int height = height_;
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
    if (SavedMaximized(WindowCreatingLabel(browser_view_))) {
      return CEF_SHOW_STATE_MAXIMIZED;
    }
    return CEF_SHOW_STATE_NORMAL;
  }

  cef_runtime_style_t GetWindowRuntimeStyle() override {
    return runtime_style_;
  }

 private:
  CefRefPtr<CefBrowserView> browser_view_;
  const cef_runtime_style_t runtime_style_;
  const int width_;
  const int height_;
  const std::string title_;
  const WindowChrome chrome_;
  IMPLEMENT_REFCOUNTING(SizedWindowDelegate);
};

class StyleBrowserViewDelegate : public CefBrowserViewDelegate {
 public:
  StyleBrowserViewDelegate(cef_runtime_style_t runtime_style, const WindowChrome& chrome)
      : runtime_style_(runtime_style), chrome_(chrome) {}

  bool OnPopupBrowserViewCreated(CefRefPtr<CefBrowserView> browser_view,
                                 CefRefPtr<CefBrowserView> popup_browser_view,
                                 bool is_devtools) override {
    (void)browser_view;
    (void)is_devtools;
    CefWindow::CreateTopLevelWindow(new SizedWindowDelegate(
        popup_browser_view, runtime_style_, 960, 640, "", chrome_));
    return true;
  }

  cef_runtime_style_t GetBrowserRuntimeStyle() override {
    return runtime_style_;
  }

 private:
  const cef_runtime_style_t runtime_style_;
  const WindowChrome chrome_;
  IMPLEMENT_REFCOUNTING(StyleBrowserViewDelegate);
};

void EmitOnUI(CefRefPtr<CefBrowser> browser, std::string json) {
  CEF_REQUIRE_UI_THREAD();
  if (!browser) {
    return;
  }
  CefRefPtr<CefFrame> frame = browser->GetMainFrame();
  if (!frame) {
    return;
  }
  // json is a JSON object; embed directly into the dispatch call.
  const std::string code =
      "window.__xgoefDispatch&&window.__xgoefDispatch(" + json + ");";
  frame->ExecuteJavaScript(code, frame->GetURL(), 0);
}

CefRefPtr<CefBrowser> FindLocked(const std::string& label) {
  auto it = g_by_label.find(label);
  if (it == g_by_label.end()) {
    return nullptr;
  }
  return it->second;
}

}  // namespace

void WindowSetPendingLabel(const std::string& label) {
  std::lock_guard<std::mutex> lock(g_mu);
  g_pending_label = label.empty() ? "main" : label;
}

std::string WindowTakePendingLabel() {
  std::lock_guard<std::mutex> lock(g_mu);
  std::string out = g_pending_label.empty() ? "main" : g_pending_label;
  g_pending_label.clear();
  return out;
}

std::string WindowPeekPendingLabel() {
  std::lock_guard<std::mutex> lock(g_mu);
  return g_pending_label;
}

void WindowBind(const std::string& label, CefRefPtr<CefBrowser> browser) {
  if (!browser) {
    return;
  }
  std::string use = label.empty() ? "main" : label;
  std::lock_guard<std::mutex> lock(g_mu);
  // If label already taken, keep unique by appending browser id.
  if (g_by_label.count(use) && !g_by_label[use]->IsSame(browser)) {
    use = use + "-" + std::to_string(browser->GetIdentifier());
  }
  g_by_label[use] = browser;
  g_label_by_id[browser->GetIdentifier()] = use;
}

CefRefPtr<CefBrowser> WindowBrowser(const std::string& label) {
  std::lock_guard<std::mutex> lock(g_mu);
  return FindLocked(label.empty() ? "main" : label);
}

void WindowUnbind(CefRefPtr<CefBrowser> browser) {
  if (!browser) {
    return;
  }
  std::lock_guard<std::mutex> lock(g_mu);
  auto it = g_label_by_id.find(browser->GetIdentifier());
  if (it == g_label_by_id.end()) {
    return;
  }
  g_by_label.erase(it->second);
  g_label_by_id.erase(it);
}

std::string WindowLabelFor(CefRefPtr<CefBrowser> browser) {
  std::string label;
  if (!WindowBoundLabel(browser, &label)) {
    return "main";
  }
  return label;
}

bool WindowBoundLabel(CefRefPtr<CefBrowser> browser, std::string* out) {
  if (!browser) {
    return false;
  }
  std::lock_guard<std::mutex> lock(g_mu);
  auto it = g_label_by_id.find(browser->GetIdentifier());
  if (it == g_label_by_id.end()) {
    return false;
  }
  if (out) {
    *out = it->second;
  }
  return true;
}

std::string WindowCreatingLabel(CefRefPtr<CefBrowserView> view) {
  if (view) {
    std::string bound;
    if (WindowBoundLabel(view->GetBrowser(), &bound)) {
      return bound;
    }
  }
  return WindowPeekPendingLabel();
}

void DefaultWindowSize(int* width, int* height) {
  int w = 960;
  int h = 640;
  CefRefPtr<CefCommandLine> command_line = CefCommandLine::GetGlobalCommandLine();
  if (command_line) {
    const std::string spec = command_line->GetSwitchValue("xgoef-size").ToString();
    int a = 0;
    int b = 0;
    if (std::sscanf(spec.c_str(), "%d,%d", &a, &b) == 2 && a > 0 && b > 0 && a <= 16384 &&
        b <= 16384) {
      w = a;
      h = b;
    }
  }
  if (width) {
    *width = w;
  }
  if (height) {
    *height = h;
  }
}

bool SavedSize(const std::string& label, int* width, int* height) {
  WindowPlace place;
  if (label.empty() || !WindowStateFind(label, &place) || !place.positioned) {
    return false;
  }
  if (width) {
    *width = place.width;
  }
  if (height) {
    *height = place.height;
  }
  return true;
}

bool SavedMaximized(const std::string& label) {
  WindowPlace place;
  return !label.empty() && WindowStateFind(label, &place) && place.maximized;
}

void ApplySavedPlace(CefRefPtr<CefWindow> window, const std::string& label) {
  if (!window || label.empty()) {
    return;
  }
  WindowPlace place;
  if (!WindowStateFind(label, &place)) {
    return;
  }
  if (place.positioned) {
    window->SetBounds(CefRect(place.x, place.y, place.width, place.height));
  }
  if (place.maximized) {
    window->Maximize();
  }
}

void NoteWindowGeometry(CefRefPtr<CefWindow> window, CefRefPtr<CefBrowserView> view) {
  if (!window || !view) {
    return;
  }
  std::string label;
  if (!WindowBoundLabel(view->GetBrowser(), &label)) {
    return;
  }
  if (window->IsMinimized()) {
    return;
  }
  if (window->IsMaximized()) {
    WindowStateNote(label, 0, 0, 0, 0, 1);
    return;
  }
  const CefRect bounds = window->GetBoundsInScreen();
  WindowStateNote(label, bounds.x, bounds.y, bounds.width, bounds.height, 0);
}

WindowChrome ChromeOrDefault(const std::string& text) {
  WindowChrome chrome;
  std::string err;
  if (!ParseWindowChrome(text, &chrome, &err)) {
    std::fprintf(stderr, "xgoef: %s\n", err.c_str());
    return WindowChrome();
  }
  return chrome;
}

CefRefPtr<CefRequestContext> ChromeContext(const WindowChrome& chrome) {
  if (chrome.theme != "light" && chrome.theme != "dark") {
    return nullptr;
  }
  // A separate context so SetChromeColorScheme does not flip other windows.
  // Storage stays shared with the global context.
  CefRefPtr<CefRequestContext> shared = CefRequestContext::GetGlobalContext();
  if (!shared) {
    return nullptr;
  }
  CefRefPtr<CefRequestContext> ctx = CefRequestContext::CreateContext(shared, nullptr);
  if (!ctx) {
    return nullptr;
  }
  cef_color_variant_t variant = CEF_COLOR_VARIANT_LIGHT;
  if (chrome.theme == "dark") {
    variant = CEF_COLOR_VARIANT_DARK;
  }
  ctx->SetChromeColorScheme(variant, 0);
  return ctx;
}

void ApplyBrowserBackground(CefBrowserSettings* settings, const WindowChrome& chrome) {
  if (!settings) {
    return;
  }
  if (chrome.transparent) {
    // Alpha 0 asks for a transparent page. Windowed CEF paints that opaque white.
    settings->background_color = 0;
    return;
  }
  if (chrome.theme == "dark") {
    settings->background_color = static_cast<cef_color_t>(0xFF1E1E1E);
  } else if (chrome.theme == "light") {
    settings->background_color = static_cast<cef_color_t>(0xFFFFFFFF);
  }
}

void DoWindowCreate(std::string lab,
                    std::string tit,
                    std::string u,
                    int width,
                    int height,
                    WindowChrome chrome) {
  CEF_REQUIRE_UI_THREAD();
  ClientHandler* raw = ClientHandler::GetInstance();
  if (!raw) {
    return;
  }
  CefRefPtr<ClientHandler> handler = raw;
  WindowSetPendingLabel(lab);

  CefRefPtr<CefCommandLine> command_line =
      CefCommandLine::GetGlobalCommandLine();
  cef_runtime_style_t runtime_style = CEF_RUNTIME_STYLE_DEFAULT;
  if (command_line && command_line->HasSwitch("use-alloy-style")) {
    runtime_style = CEF_RUNTIME_STYLE_ALLOY;
  }

  CefBrowserSettings browser_settings;
  ApplyBrowserBackground(&browser_settings, chrome);
  CefRefPtr<CefBrowserView> browser_view = CefBrowserView::CreateBrowserView(
      handler, u, browser_settings, nullptr, ChromeContext(chrome),
      new StyleBrowserViewDelegate(runtime_style, chrome));
  CefWindow::CreateTopLevelWindow(
      new SizedWindowDelegate(browser_view, runtime_style, width, height, tit, chrome));
}

void DoWindowClose(std::string lab) {
  CEF_REQUIRE_UI_THREAD();
  CefRefPtr<CefBrowser> browser;
  {
    std::lock_guard<std::mutex> lock(g_mu);
    browser = FindLocked(lab);
  }
  if (browser) {
    browser->GetHost()->CloseBrowser(false);
  }
}

void DoWindowFocus(std::string lab) {
  CEF_REQUIRE_UI_THREAD();
  CefRefPtr<CefBrowser> browser;
  {
    std::lock_guard<std::mutex> lock(g_mu);
    browser = FindLocked(lab);
  }
  if (!browser) {
    return;
  }
  if (auto view = CefBrowserView::GetForBrowser(browser)) {
    if (auto window = view->GetWindow()) {
      window->Show();
      window->Activate();
    }
  }
  browser->GetHost()->SetFocus(true);
}

void DoEmitTo(std::string lab, std::string json) {
  CEF_REQUIRE_UI_THREAD();
  CefRefPtr<CefBrowser> browser;
  {
    std::lock_guard<std::mutex> lock(g_mu);
    browser = FindLocked(lab);
  }
  EmitOnUI(browser, json);
}

void DoEmitAll(std::string json) {
  CEF_REQUIRE_UI_THREAD();
  std::map<std::string, CefRefPtr<CefBrowser>> copy;
  {
    std::lock_guard<std::mutex> lock(g_mu);
    copy = g_by_label;
  }
  for (const auto& kv : copy) {
    EmitOnUI(kv.second, json);
  }
}

int WindowCreate(const char* label,
                 const char* title,
                 const char* url,
                 int width,
                 int height,
                 const char* chrome_text) {
  std::string lab = label ? label : "main";
  std::string tit = title ? title : "";
  std::string u = url && url[0] ? url : "about:blank";
  WindowChrome chrome = ChromeOrDefault(chrome_text ? chrome_text : "");
  if (CefCurrentlyOn(TID_UI)) {
    DoWindowCreate(lab, tit, u, width, height, chrome);
  } else {
    CefPostTask(TID_UI, base::BindOnce(&DoWindowCreate, lab, tit, u, width, height, chrome));
  }
  return 0;
}

int WindowClose(const char* label) {
  std::string lab = label && label[0] ? label : "main";
  if (CefCurrentlyOn(TID_UI)) {
    DoWindowClose(lab);
  } else {
    CefPostTask(TID_UI, base::BindOnce(&DoWindowClose, lab));
  }
  return 0;
}

int WindowFocus(const char* label) {
  std::string lab = label && label[0] ? label : "main";
  if (CefCurrentlyOn(TID_UI)) {
    DoWindowFocus(lab);
  } else {
    CefPostTask(TID_UI, base::BindOnce(&DoWindowFocus, lab));
  }
  return 0;
}

int WindowEmitTo(const char* label, const char* json_utf8) {
  if (!json_utf8) {
    return -1;
  }
  std::string lab = label && label[0] ? label : "main";
  std::string json = json_utf8;
  if (CefCurrentlyOn(TID_UI)) {
    DoEmitTo(lab, json);
  } else {
    CefPostTask(TID_UI, base::BindOnce(&DoEmitTo, lab, json));
  }
  return 0;
}

int WindowEmitAll(const char* json_utf8) {
  if (!json_utf8) {
    return -1;
  }
  std::string json = json_utf8;
  if (CefCurrentlyOn(TID_UI)) {
    DoEmitAll(json);
  } else {
    CefPostTask(TID_UI, base::BindOnce(&DoEmitAll, json));
  }
  return 0;
}

}  // namespace xgoef

extern "C" void xgoef_window_set_pending_label(const char* label) {
  xgoef::WindowSetPendingLabel(label ? label : "main");
}

extern "C" int xgoef_window_create(const char* label,
                                   const char* title,
                                   const char* url,
                                   int width,
                                   int height,
                                   const char* chrome) {
  return xgoef::WindowCreate(label, title, url, width, height, chrome);
}

extern "C" int xgoef_window_close(const char* label) {
  return xgoef::WindowClose(label);
}

extern "C" int xgoef_window_focus(const char* label) {
  return xgoef::WindowFocus(label);
}

extern "C" int xgoef_ipc_emit_to(const char* label, const char* json_utf8) {
  return xgoef::WindowEmitTo(label, json_utf8);
}

extern "C" int xgoef_ipc_emit_all(const char* json_utf8) {
  return xgoef::WindowEmitAll(json_utf8);
}
