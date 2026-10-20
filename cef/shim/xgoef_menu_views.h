#ifndef XGOEF_MENU_VIEWS_H_
#define XGOEF_MENU_VIEWS_H_

#include "include/views/cef_browser_view.h"
#include "include/views/cef_window.h"

namespace xgoef {

// When an app menu has been set, wrap the browser in a column whose first
// child is a menu-button bar. Otherwise return the browser view unchanged.
CefRefPtr<CefView> WrapWithAppMenu(CefRefPtr<CefWindow> window,
                                   CefRefPtr<CefBrowserView> browser_view);

}  // namespace xgoef

#endif  // XGOEF_MENU_VIEWS_H_
