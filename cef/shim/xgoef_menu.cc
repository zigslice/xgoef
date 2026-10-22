#include "xgoef_menu.h"

#include "xgoef_menu_model.h"
#include "xgoef_menu_views.h"
#include "xgoef_window.h"

#include <map>
#include <mutex>
#include <string>
#include <utility>
#include <vector>

#include "include/base/cef_callback.h"
#include "include/cef_menu_model.h"
#include "include/cef_menu_model_delegate.h"
#include "include/views/cef_box_layout.h"
#include "include/views/cef_menu_button.h"
#include "include/views/cef_menu_button_delegate.h"
#include "include/views/cef_panel.h"
#include "include/views/cef_window.h"
#include "include/wrapper/cef_closure_task.h"
#include "include/wrapper/cef_helpers.h"

extern "C" void xgoef_tray_apply(const char* tooltip, const char* icon, const char* flat);
extern "C" void xgoef_tray_note_click(xgoef_menu_click_fn fn);

namespace xgoef {

std::mutex g_mu;
std::string g_menu_flat;
std::string g_tray_flat;
xgoef_menu_click_fn g_click = nullptr;

void Notify(const std::string& window, const std::string& kind, const std::string& id) {
  xgoef_menu_click_fn fn = nullptr;
  {
    std::lock_guard<std::mutex> lock(g_mu);
    fn = g_click;
  }
  if (fn) {
    fn(window.c_str(), kind.c_str(), id.c_str());
  }
}

int NextID(std::map<int, std::string>* ids) {
  return static_cast<int>(ids->size()) + 1;
}

void FillModel(CefRefPtr<CefMenuModel> model,
               const std::vector<MenuEntry>& entries,
               const std::string& parent,
               std::map<int, std::string>* ids) {
  for (const MenuEntry& e : entries) {
    if (e.parent != parent) {
      continue;
    }
    if (e.separator()) {
      model->AddSeparator();
      continue;
    }
    const int cmd = NextID(ids);
    (*ids)[cmd] = e.id;
    bool has_child = false;
    for (const MenuEntry& c : entries) {
      if (c.parent == e.id) {
        has_child = true;
        break;
      }
    }
    if (has_child) {
      CefRefPtr<CefMenuModel> sub = model->AddSubMenu(cmd, e.text);
      if (sub) {
        FillModel(sub, entries, e.id, ids);
      }
    } else {
      model->AddItem(cmd, e.text);
    }
    if (!e.enabled()) {
      model->SetEnabled(cmd, false);
    }
  }
}

class MenuController : public CefMenuButtonDelegate, public CefMenuModelDelegate {
 public:
  explicit MenuController(std::string kind) : kind_(std::move(kind)) {}

  void ExecuteCommand(CefRefPtr<CefMenuModel> menu_model,
                      int command_id,
                      cef_event_flags_t event_flags) override {
    (void)menu_model;
    (void)event_flags;
    CEF_REQUIRE_UI_THREAD();
    std::string id;
    {
      std::lock_guard<std::mutex> lock(mu_);
      auto it = ids_.find(command_id);
      if (it == ids_.end()) {
        return;
      }
      id = it->second;
    }
    Notify(WindowLabel(), kind_, id);
  }

  void OnButtonPressed(CefRefPtr<CefButton> button) override { (void)button; }

  void OnMenuButtonPressed(CefRefPtr<CefMenuButton> menu_button,
                           const CefPoint& screen_point,
                           CefRefPtr<CefMenuButtonPressedLock> button_pressed_lock) override {
    (void)button_pressed_lock;
    CEF_REQUIRE_UI_THREAD();
    const int cmd = menu_button->GetID();
    CefRefPtr<CefMenuModel> sub;
    std::string leaf;
    {
      std::lock_guard<std::mutex> lock(mu_);
      auto sub_it = subs_.find(cmd);
      if (sub_it != subs_.end()) {
        sub = sub_it->second;
      } else {
        auto id_it = ids_.find(cmd);
        if (id_it != ids_.end()) {
          leaf = id_it->second;
        }
      }
    }
    if (sub) {
      menu_button->ShowMenu(sub, screen_point, CEF_MENU_ANCHOR_TOPLEFT);
      return;
    }
    if (!leaf.empty()) {
      Notify(WindowLabel(), kind_, leaf);
    }
  }

  CefRefPtr<CefView> BuildBar(const std::vector<MenuEntry>& entries,
                              CefRefPtr<CefBrowserView> browser_view) {
    std::map<int, std::string> ids;
    std::map<int, CefRefPtr<CefMenuModel>> subs;
    CefRefPtr<CefPanel> bar = CefPanel::CreatePanel(nullptr);
    CefBoxLayoutSettings row;
    row.horizontal = 1;
    bar->SetToBoxLayout(row);

    bool any = false;
    for (const MenuEntry& e : entries) {
      if (!e.parent.empty() || e.separator()) {
        continue;
      }
      const int cmd = NextID(&ids);
      ids[cmd] = e.id;
      CefRefPtr<CefMenuButton> button = CefMenuButton::CreateMenuButton(this, e.text);
      button->SetID(cmd);
      bool has_child = false;
      for (const MenuEntry& c : entries) {
        if (c.parent == e.id) {
          has_child = true;
          break;
        }
      }
      if (has_child) {
        CefRefPtr<CefMenuModel> sub = CefMenuModel::CreateMenuModel(this);
        FillModel(sub, entries, e.id, &ids);
        subs[cmd] = sub;
      }
      if (!e.enabled()) {
        button->SetEnabled(false);
      }
      bar->AddChildView(button);
      any = true;
    }
    if (!any) {
      return browser_view;
    }
    {
      std::lock_guard<std::mutex> lock(mu_);
      ids_ = std::move(ids);
      subs_ = std::move(subs);
      browser_view_ = browser_view;
    }

    CefRefPtr<CefPanel> column = CefPanel::CreatePanel(nullptr);
    CefBoxLayoutSettings col;
    col.horizontal = 0;
    CefRefPtr<CefBoxLayout> layout = column->SetToBoxLayout(col);
    column->AddChildView(bar);
    column->AddChildView(browser_view);
    if (layout) {
      layout->SetFlexForView(browser_view, 1);
    }
    return column;
  }

  void Show(CefRefPtr<CefBrowser> browser, const std::vector<MenuEntry>& entries) {
    std::map<int, std::string> ids;
    CefRefPtr<CefMenuModel> model = CefMenuModel::CreateMenuModel(this);
    if (!model || !browser) {
      return;
    }
    FillModel(model, entries, "", &ids);
    {
      std::lock_guard<std::mutex> lock(mu_);
      ids_ = std::move(ids);
      browser_view_ = CefBrowserView::GetForBrowser(browser);
    }
    CefRefPtr<CefBrowserView> view = CefBrowserView::GetForBrowser(browser);
    if (!view) {
      return;
    }
    CefRefPtr<CefWindow> window = view->GetWindow();
    if (!window) {
      return;
    }
    const CefRect bounds = window->GetBoundsInScreen();
    window->ShowMenu(model, CefPoint(bounds.x, bounds.y), CEF_MENU_ANCHOR_TOPLEFT);
  }

 private:
  std::string WindowLabel() const {
    CefRefPtr<CefBrowserView> view;
    {
      std::lock_guard<std::mutex> lock(mu_);
      view = browser_view_;
    }
    if (!view) {
      return "main";
    }
    return WindowLabelFor(view->GetBrowser());
  }

  const std::string kind_;
  mutable std::mutex mu_;
  std::map<int, std::string> ids_;
  std::map<int, CefRefPtr<CefMenuModel>> subs_;
  CefRefPtr<CefBrowserView> browser_view_;
  IMPLEMENT_REFCOUNTING(MenuController);
};

MenuController* AppController() {
  static CefRefPtr<MenuController> controller = new MenuController("menu");
  return controller.get();
}

// Popup uses its own controller so Show() does not replace the bar's command ids.
MenuController* MenuPopupController() {
  static CefRefPtr<MenuController> controller = new MenuController("menu");
  return controller.get();
}

MenuController* TrayController() {
  static CefRefPtr<MenuController> controller = new MenuController("tray");
  return controller.get();
}

std::vector<MenuEntry> LoadFlat(const std::string& flat) {
  std::vector<MenuEntry> entries;
  std::string err;
  if (!ParseMenu(flat, &entries, &err)) {
    return {};
  }
  return entries;
}

void PopupOnUI(std::string label, std::string flat, CefRefPtr<MenuController> controller) {
  CEF_REQUIRE_UI_THREAD();
  CefRefPtr<CefBrowser> browser = WindowBrowser(label);
  if (!browser || !controller) {
    return;
  }
  controller->Show(browser, LoadFlat(flat));
}

CefRefPtr<CefView> WrapImpl(CefRefPtr<CefBrowserView> browser_view) {
  std::string flat;
  {
    std::lock_guard<std::mutex> lock(g_mu);
    flat = g_menu_flat;
  }
  if (flat.empty() || !browser_view) {
    return browser_view;
  }
  return AppController()->BuildBar(LoadFlat(flat), browser_view);
}

void MenuSet(const char* encoded) {
  std::lock_guard<std::mutex> lock(g_mu);
  g_menu_flat = encoded ? encoded : "";
}

void TraySet(const char* tooltip, const char* icon, const char* encoded) {
  {
    std::lock_guard<std::mutex> lock(g_mu);
    g_tray_flat = encoded ? encoded : "";
  }
  xgoef_tray_apply(tooltip ? tooltip : "", icon ? icon : "", encoded ? encoded : "");
}

int Popup(const char* window_label, bool tray) {
  std::string flat;
  {
    std::lock_guard<std::mutex> lock(g_mu);
    flat = tray ? g_tray_flat : g_menu_flat;
  }
  if (flat.empty()) {
    return -1;
  }
  std::string label = window_label && window_label[0] ? window_label : "main";
  CefRefPtr<MenuController> controller = tray ? TrayController() : MenuPopupController();
  if (CefCurrentlyOn(TID_UI)) {
    PopupOnUI(label, flat, controller);
  } else {
    CefPostTask(TID_UI, base::BindOnce(&PopupOnUI, label, flat, controller));
  }
  return 0;
}

void SetClick(xgoef_menu_click_fn fn) {
  std::lock_guard<std::mutex> lock(g_mu);
  g_click = fn;
  xgoef_tray_note_click(fn);
}

CefRefPtr<CefView> WrapWithAppMenu(CefRefPtr<CefWindow> window,
                                   CefRefPtr<CefBrowserView> browser_view) {
  (void)window;
  return WrapImpl(browser_view);
}

}  // namespace xgoef

extern "C" void xgoef_menu_set(const char* encoded) { xgoef::MenuSet(encoded); }

extern "C" void xgoef_tray_set(const char* tooltip, const char* icon, const char* encoded) {
  xgoef::TraySet(tooltip, icon, encoded);
}

extern "C" int xgoef_menu_popup(const char* window_label) {
  return xgoef::Popup(window_label, false);
}

extern "C" int xgoef_tray_popup(const char* window_label) {
  return xgoef::Popup(window_label, true);
}

extern "C" void xgoef_menu_set_click(xgoef_menu_click_fn fn) { xgoef::SetClick(fn); }
