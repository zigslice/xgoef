// Reads CefDisplay on the UI thread and moves a Views window in DIP.

#include "xgoef_display.h"
#include "xgoef_window.h"

#include <cstdlib>
#include <cstring>
#include <string>
#include <vector>

#include "include/base/cef_callback.h"
#include "include/cef_display.h"
#include "include/cef_waitable_event.h"
#include "include/wrapper/cef_helpers.h"
#include "include/views/cef_browser_view.h"
#include "include/views/cef_window.h"
#include "include/wrapper/cef_closure_task.h"

namespace xgoef {
namespace {

int Span(int origin, int far) {
  int delta = far - origin;
  if (delta < 0) {
    delta = -delta;
  }
  if (delta == 0) {
    delta = 1;
  }
  return delta;
}

void ToPixels(CefRefPtr<CefDisplay> display, int x, int y, int width, int height, int* out_x,
              int* out_y, int* out_width, int* out_height) {
  CefPoint origin(x, y);
  CefPoint far(x + width, y + height);
  display->ConvertPointToPixels(origin);
  display->ConvertPointToPixels(far);
  *out_x = origin.x;
  *out_y = origin.y;
  *out_width = Span(origin.x, far.x);
  *out_height = Span(origin.y, far.y);
}

DisplayInfo FromDisplay(CefRefPtr<CefDisplay> display, int64_t primary_id) {
  DisplayInfo info;
  info.id = display->GetID();
  info.scale = display->GetDeviceScaleFactor();
  CefRect bounds = display->GetBounds();
  CefRect work = display->GetWorkArea();
  info.x = bounds.x;
  info.y = bounds.y;
  info.width = bounds.width > 0 ? bounds.width : 1;
  info.height = bounds.height > 0 ? bounds.height : 1;
  info.work_x = work.x;
  info.work_y = work.y;
  info.work_width = work.width > 0 ? work.width : 1;
  info.work_height = work.height > 0 ? work.height : 1;
  ToPixels(display, info.x, info.y, info.width, info.height, &info.pixel_x, &info.pixel_y,
           &info.pixel_width, &info.pixel_height);
  ToPixels(display, info.work_x, info.work_y, info.work_width, info.work_height, &info.work_pixel_x,
           &info.work_pixel_y, &info.work_pixel_width, &info.work_pixel_height);
  int rotation = display->GetRotation();
  if (rotation != 90 && rotation != 180 && rotation != 270) {
    rotation = 0;
  }
  info.rotation = rotation;
  info.primary = info.id == primary_id;
  return info;
}

std::string BuildDisplayTable(const std::string& label) {
  CEF_REQUIRE_UI_THREAD();
  DisplayTable table;
  std::vector<CefRefPtr<CefDisplay>> displays;
  CefDisplay::GetAllDisplays(displays);
  int64_t primary_id = 0;
  if (CefRefPtr<CefDisplay> primary = CefDisplay::GetPrimaryDisplay()) {
    primary_id = primary->GetID();
  }
  bool saw_primary = false;
  for (const CefRefPtr<CefDisplay>& display : displays) {
    if (!display) {
      continue;
    }
    DisplayInfo info = FromDisplay(display, primary_id);
    if (info.primary) {
      saw_primary = true;
    }
    table.displays.push_back(info);
  }
  if (!table.displays.empty() && !saw_primary) {
    table.displays[0].primary = true;
  }
  std::string lab = label.empty() ? "main" : label;
  CefRefPtr<CefBrowser> browser = WindowBrowser(lab);
  if (browser) {
    if (CefRefPtr<CefBrowserView> view = CefBrowserView::GetForBrowser(browser)) {
      if (CefRefPtr<CefWindow> window = view->GetWindow()) {
        if (CefRefPtr<CefDisplay> current = window->GetDisplay()) {
          table.has_window = true;
          table.window_id = current->GetID();
          table.window_scale = current->GetDeviceScaleFactor();
        }
      }
    }
  }
  return FormatDisplayTable(table);
}

int MoveOnUI(const std::string& label, int x, int y) {
  CEF_REQUIRE_UI_THREAD();
  std::string lab = label.empty() ? "main" : label;
  CefRefPtr<CefBrowser> browser = WindowBrowser(lab);
  if (!browser) {
    return -1;
  }
  CefRefPtr<CefBrowserView> view = CefBrowserView::GetForBrowser(browser);
  if (!view) {
    return -1;
  }
  CefRefPtr<CefWindow> window = view->GetWindow();
  if (!window) {
    return -1;
  }
  CefRect bounds = window->GetBounds();
  bounds.x = x;
  bounds.y = y;
  window->SetBounds(bounds);
  return 0;
}

class DisplayCall : public CefBaseRefCounted {
 public:
  std::string text;
  int rc = -1;
  CefRefPtr<CefWaitableEvent> event = CefWaitableEvent::CreateWaitableEvent(false, false);

 private:
  IMPLEMENT_REFCOUNTING(DisplayCall);
};

void QueryOnUI(std::string label, CefRefPtr<DisplayCall> call) {
  call->text = BuildDisplayTable(label);
  call->event->Signal();
}

void MoveTask(std::string label, int x, int y, CefRefPtr<DisplayCall> call) {
  call->rc = MoveOnUI(label, x, y);
  call->event->Signal();
}

}  // namespace

std::string QueryDisplays(const std::string& label) {
  if (CefCurrentlyOn(TID_UI)) {
    return BuildDisplayTable(label);
  }
  CefRefPtr<DisplayCall> call = new DisplayCall();
  CefPostTask(TID_UI, base::BindOnce(&QueryOnUI, label, call));
  if (!call->event->TimedWait(2000)) {
    return "";
  }
  return call->text;
}

int QueryMove(const std::string& label, int x, int y) {
  if (CefCurrentlyOn(TID_UI)) {
    return MoveOnUI(label, x, y);
  }
  CefRefPtr<DisplayCall> call = new DisplayCall();
  CefPostTask(TID_UI, base::BindOnce(&MoveTask, label, x, y, call));
  if (!call->event->TimedWait(2000)) {
    return -1;
  }
  return call->rc;
}

}  // namespace xgoef

extern "C" char* xgoef_displays(const char* window_label) {
  std::string text = xgoef::QueryDisplays(window_label ? window_label : "");
  if (text.empty()) {
    return nullptr;
  }
  char* out = static_cast<char*>(std::malloc(text.size() + 1));
  if (!out) {
    return nullptr;
  }
  std::memcpy(out, text.data(), text.size() + 1);
  return out;
}

extern "C" void xgoef_displays_free(char* text) { std::free(text); }

extern "C" int xgoef_window_move(const char* label, int x, int y) {
  return xgoef::QueryMove(label ? label : "", x, y);
}
