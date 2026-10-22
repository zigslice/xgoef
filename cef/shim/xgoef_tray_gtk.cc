// Optional GTK3 status icon, loaded with dlopen so the shim does not need
// GTK headers. Right-click builds a menu from the flat model; left-click
// reports an empty item id.
#include "xgoef_menu.h"
#include "xgoef_menu_model.h"

#include <dlfcn.h>

#include <list>
#include <mutex>
#include <string>
#include <utility>
#include <vector>

namespace {

using GtkWidget = void;
using GtkStatusIcon = void;

struct GtkFns {
  void* lib = nullptr;
  int (*init_check)(int*, char***) = nullptr;
  GtkStatusIcon* (*icon_new)() = nullptr;
  void (*icon_set_visible)(GtkStatusIcon*, int) = nullptr;
  void (*icon_set_tooltip)(GtkStatusIcon*, const char*) = nullptr;
  void (*icon_set_file)(GtkStatusIcon*, const char*) = nullptr;
  void (*icon_set_name)(GtkStatusIcon*, const char*) = nullptr;
  void (*signal_connect)(void*, const char*, void (*)(), void*, void*, int) = nullptr;
  GtkWidget* (*menu_new)() = nullptr;
  GtkWidget* (*item_new)(const char*) = nullptr;
  GtkWidget* (*separator_new)() = nullptr;
  void (*shell_append)(void*, GtkWidget*) = nullptr;
  void (*item_set_submenu)(GtkWidget*, GtkWidget*) = nullptr;
  void (*set_sensitive)(GtkWidget*, int) = nullptr;
  void (*widget_show)(GtkWidget*) = nullptr;
  void (*menu_popup)(void*, void*, void*, void*, void*, unsigned, unsigned) = nullptr;
  void (*main_loop)() = nullptr;
};

GtkFns g_gtk;
std::mutex g_mu;
std::string g_flat;
std::string g_tip;
std::string g_icon;
xgoef_menu_click_fn g_click = nullptr;
bool g_started = false;
bool g_cef_ready = false;
std::list<std::string> g_ids;

void Click(const char* id) {
  xgoef_menu_click_fn fn = nullptr;
  {
    std::lock_guard<std::mutex> lock(g_mu);
    fn = g_click;
  }
  if (fn) {
    fn("main", "tray", id ? id : "");
  }
}

void OnItem(void*, void* data) {
  auto* id = static_cast<std::string*>(data);
  Click(id ? id->c_str() : "");
}

void OnActivate(void*, void*) { Click(""); }

GtkWidget* BuildGtkMenu(const std::vector<xgoef::MenuEntry>& entries, const std::string& parent) {
  GtkWidget* menu = g_gtk.menu_new();
  for (const xgoef::MenuEntry& e : entries) {
    if (e.parent != parent) {
      continue;
    }
    GtkWidget* item = nullptr;
    if (e.separator()) {
      item = g_gtk.separator_new();
    } else {
      item = g_gtk.item_new(e.text.c_str());
      bool child = false;
      for (const xgoef::MenuEntry& c : entries) {
        if (c.parent == e.id) {
          child = true;
          break;
        }
      }
      if (child) {
        g_gtk.item_set_submenu(item, BuildGtkMenu(entries, e.id));
      } else {
        g_ids.push_back(e.id);
        g_gtk.signal_connect(item, "activate", reinterpret_cast<void (*)()>(OnItem), &g_ids.back(), nullptr, 0);
      }
      if (!e.enabled()) {
        g_gtk.set_sensitive(item, 0);
      }
    }
    g_gtk.shell_append(menu, item);
    g_gtk.widget_show(item);
  }
  g_gtk.widget_show(menu);
  return menu;
}

void OnPopup(void*, unsigned button, unsigned when, void*) {
  std::string flat;
  {
    std::lock_guard<std::mutex> lock(g_mu);
    flat = g_flat;
  }
  std::vector<xgoef::MenuEntry> entries;
  std::string err;
  if (!xgoef::ParseMenu(flat, &entries, &err) || entries.empty()) {
    return;
  }
  GtkWidget* menu = BuildGtkMenu(entries, "");
  g_gtk.menu_popup(menu, nullptr, nullptr, nullptr, nullptr, button, when);
}

bool LoadGtk() {
  if (g_gtk.lib) {
    return g_gtk.menu_new != nullptr;
  }
  g_gtk.lib = dlopen("libgtk-3.so.0", RTLD_LAZY | RTLD_LOCAL);
  if (!g_gtk.lib) {
    return false;
  }
  auto sym = [](const char* name) { return dlsym(g_gtk.lib, name); };
  g_gtk.init_check = reinterpret_cast<int (*)(int*, char***)>(sym("gtk_init_check"));
  g_gtk.icon_new = reinterpret_cast<GtkStatusIcon* (*)()>(sym("gtk_status_icon_new"));
  g_gtk.icon_set_visible = reinterpret_cast<void (*)(GtkStatusIcon*, int)>(sym("gtk_status_icon_set_visible"));
  g_gtk.icon_set_tooltip = reinterpret_cast<void (*)(GtkStatusIcon*, const char*)>(sym("gtk_status_icon_set_tooltip_text"));
  g_gtk.icon_set_file = reinterpret_cast<void (*)(GtkStatusIcon*, const char*)>(sym("gtk_status_icon_set_from_file"));
  g_gtk.icon_set_name = reinterpret_cast<void (*)(GtkStatusIcon*, const char*)>(sym("gtk_status_icon_set_from_icon_name"));
  g_gtk.signal_connect = reinterpret_cast<void (*)(void*, const char*, void (*)(), void*, void*, int)>(sym("g_signal_connect_data"));
  g_gtk.menu_new = reinterpret_cast<GtkWidget* (*)()>(sym("gtk_menu_new"));
  g_gtk.item_new = reinterpret_cast<GtkWidget* (*)(const char*)>(sym("gtk_menu_item_new_with_label"));
  g_gtk.separator_new = reinterpret_cast<GtkWidget* (*)()>(sym("gtk_separator_menu_item_new"));
  g_gtk.shell_append = reinterpret_cast<void (*)(void*, GtkWidget*)>(sym("gtk_menu_shell_append"));
  g_gtk.item_set_submenu = reinterpret_cast<void (*)(GtkWidget*, GtkWidget*)>(sym("gtk_menu_item_set_submenu"));
  g_gtk.set_sensitive = reinterpret_cast<void (*)(GtkWidget*, int)>(sym("gtk_widget_set_sensitive"));
  g_gtk.widget_show = reinterpret_cast<void (*)(GtkWidget*)>(sym("gtk_widget_show"));
  g_gtk.menu_popup = reinterpret_cast<void (*)(void*, void*, void*, void*, void*, unsigned, unsigned)>(sym("gtk_menu_popup"));
  g_gtk.main_loop = reinterpret_cast<void (*)()>(sym("gtk_main"));
  return g_gtk.init_check && g_gtk.icon_new && g_gtk.menu_new && g_gtk.item_new &&
         g_gtk.separator_new && g_gtk.shell_append && g_gtk.widget_show && g_gtk.main_loop &&
         g_gtk.signal_connect && g_gtk.menu_popup && g_gtk.icon_set_visible;
}

// Create the icon on the thread that already initialized GTK. CEF 152 does that
// during CefInitialize and then reads the theme; calling gtk_init earlier, or
// running gtk_main on a side thread, crashes that read.
void RealizeTray() {
  if (!LoadGtk()) {
    return;
  }
  int argc = 0;
  char** argv = nullptr;
  if (!g_gtk.init_check(&argc, &argv)) {
    return;
  }
  GtkStatusIcon* icon = g_gtk.icon_new();
  if (!icon) {
    return;
  }
  std::string tip;
  std::string path;
  {
    std::lock_guard<std::mutex> lock(g_mu);
    tip = g_tip;
    path = g_icon;
  }
  if (!tip.empty() && g_gtk.icon_set_tooltip) {
    g_gtk.icon_set_tooltip(icon, tip.c_str());
  }
  if (!path.empty() && g_gtk.icon_set_file) {
    g_gtk.icon_set_file(icon, path.c_str());
  } else if (g_gtk.icon_set_name) {
    g_gtk.icon_set_name(icon, "applications-internet");
  }
  g_gtk.icon_set_visible(icon, 1);
  g_gtk.signal_connect(icon, "activate", reinterpret_cast<void (*)()>(OnActivate), nullptr, nullptr, 0);
  g_gtk.signal_connect(icon, "popup-menu", reinterpret_cast<void (*)()>(OnPopup), nullptr, nullptr, 0);
}

}  // namespace

extern "C" void xgoef_tray_after_cef_init(void) {
  {
    std::lock_guard<std::mutex> lock(g_mu);
    g_cef_ready = true;
    if (g_started || (g_flat.empty() && g_tip.empty() && g_icon.empty())) {
      return;
    }
    g_started = true;
  }
  RealizeTray();
}

extern "C" void xgoef_tray_note_click(xgoef_menu_click_fn fn) {
  std::lock_guard<std::mutex> lock(g_mu);
  g_click = fn;
}

extern "C" void xgoef_tray_apply(const char* tooltip, const char* icon, const char* flat) {
  {
    std::lock_guard<std::mutex> lock(g_mu);
    g_tip = tooltip ? tooltip : "";
    g_icon = icon ? icon : "";
    g_flat = flat ? flat : "";
    if (!g_cef_ready || g_started || (g_flat.empty() && g_tip.empty() && g_icon.empty())) {
      return;
    }
    g_started = true;
  }
  RealizeTray();
}
