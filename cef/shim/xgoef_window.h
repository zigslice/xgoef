// Window label registry + host→renderer event delivery (ExecuteJavaScript).
#ifndef XGOEF_WINDOW_H_
#define XGOEF_WINDOW_H_

#include <string>

#include "include/cef_browser.h"
#include "include/views/cef_browser_view.h"
#include "include/views/cef_window.h"

#include "include/cef_request_context.h"

#include "xgoef_host.h"
#include "xgoef_window_chrome.h"

namespace xgoef {

// Next browser created will be bound to this label (cleared after take/bind).
void WindowSetPendingLabel(const std::string& label);
std::string WindowTakePendingLabel();
// Raw pending label. Empty means no window create is in progress.
std::string WindowPeekPendingLabel();

// Bind / unbind browsers for EmitTo / Close / Focus.
void WindowBind(const std::string& label, CefRefPtr<CefBrowser> browser);
CefRefPtr<CefBrowser> WindowBrowser(const std::string& label);
void WindowUnbind(CefRefPtr<CefBrowser> browser);
std::string WindowLabelFor(CefRefPtr<CefBrowser> browser);
bool WindowBoundLabel(CefRefPtr<CefBrowser> browser, std::string* out);

// Label for a window that is being created: the bound browser, else the
// pending label. Empty when neither is set.
std::string WindowCreatingLabel(CefRefPtr<CefBrowserView> view);

// Conf / saved geometry. --xgoef-size=W,H overrides the 960x640 fallback.
void DefaultWindowSize(int* width, int* height);
bool SavedSize(const std::string& label, int* width, int* height);
bool SavedMaximized(const std::string& label);
void ApplySavedPlace(CefRefPtr<CefWindow> window, const std::string& label);
void NoteWindowGeometry(CefRefPtr<CefWindow> window, CefRefPtr<CefBrowserView> view);

// Empty text is the default. A bad string is logged and replaced with the default.
WindowChrome ChromeOrDefault(const std::string& text);

// light/dark get a request context that shares storage with the global one
// and does not change the global color scheme. system returns nullptr.
CefRefPtr<CefRequestContext> ChromeContext(const WindowChrome& chrome);

// transparent sets background_color to 0 (windowed CEF then paints opaque white).
// dark and light set an opaque color. system leaves the CEF default.
void ApplyBrowserBackground(CefBrowserSettings* settings, const WindowChrome& chrome);

// Create another Views window (must run on UI thread or is posted).
// Returns 0 on success. chrome may be NULL.
int WindowCreate(const char* label,
                 const char* title,
                 const char* url,
                 int width,
                 int height,
                 const char* chrome);

int WindowClose(const char* label);
int WindowFocus(const char* label);

// Deliver JSON event envelope to one / all window main frames.
int WindowEmitTo(const char* label, const char* json_utf8);
int WindowEmitAll(const char* json_utf8);

}  // namespace xgoef

#endif  // XGOEF_WINDOW_H_
