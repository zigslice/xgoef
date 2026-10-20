// Line-oriented menu model shared by the Go encoder and the native shim.
// No CEF headers, so this file can be compiled on its own.
#ifndef XGOEF_MENU_MODEL_H_
#define XGOEF_MENU_MODEL_H_

#include <string>
#include <vector>

namespace xgoef {

struct MenuEntry {
  std::string id;
  std::string text;
  std::string command;
  std::string flags;
  std::string parent;

  bool separator() const { return flags.find('s') != std::string::npos; }
  bool enabled() const { return flags.find('d') == std::string::npos; }
};

// Parse the text produced by menu.Menu.Encode.
// An empty string yields an empty list. A bad line returns false.
bool ParseMenu(const std::string& flat, std::vector<MenuEntry>* out, std::string* err);

}  // namespace xgoef

#endif  // XGOEF_MENU_MODEL_H_
