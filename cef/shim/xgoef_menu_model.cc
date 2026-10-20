#include "xgoef_menu_model.h"

#include <sstream>

namespace xgoef {
namespace {

std::vector<std::string> SplitTabs(const std::string& line) {
  std::vector<std::string> cols;
  std::string cur;
  for (char c : line) {
    if (c == '\t') {
      cols.push_back(cur);
      cur.clear();
    } else {
      cur.push_back(c);
    }
  }
  cols.push_back(cur);
  return cols;
}

}  // namespace

bool ParseMenu(const std::string& flat, std::vector<MenuEntry>* out, std::string* err) {
  out->clear();
  std::istringstream in(flat);
  std::string line;
  int n = 0;
  while (std::getline(in, line)) {
    if (!line.empty() && line.back() == '\r') {
      line.pop_back();
    }
    if (line.empty()) {
      continue;
    }
    ++n;
    const std::vector<std::string> cols = SplitTabs(line);
    if (cols.size() != 5 || cols[0].empty()) {
      if (err) {
        *err = "menu line " + std::to_string(n);
      }
      out->clear();
      return false;
    }
    MenuEntry e;
    e.id = cols[0];
    e.text = cols[1];
    e.command = cols[2];
    e.flags = cols[3];
    e.parent = cols[4];
    out->push_back(e);
  }
  for (const MenuEntry& e : *out) {
    if (e.parent.empty()) {
      continue;
    }
    bool found = false;
    for (const MenuEntry& p : *out) {
      if (p.id == e.parent) {
        found = true;
        break;
      }
    }
    if (!found) {
      if (err) {
        *err = "menu parent " + e.parent;
      }
      out->clear();
      return false;
    }
  }
  return true;
}

}  // namespace xgoef
