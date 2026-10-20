#include "xgoef_menu_model.h"

#include <iostream>
#include <string>

int main() {
  const std::string flat =
      "file\tFile\t\t\t\n"
      "greet\tGreet\tgreet\t\tfile\n"
      "sep-file-1\t\t\tsd\tfile\n"
      "quit\tQuit\tapp:quit\td\tfile\n";
  std::vector<xgoef::MenuEntry> items;
  std::string err;
  if (!xgoef::ParseMenu(flat, &items, &err) || items.size() != 4) {
    std::cerr << "parse " << err << " size " << items.size() << "\n";
    return 1;
  }
  if (items[1].command != "greet" || items[1].parent != "file" || !items[1].enabled()) {
    std::cerr << "greet fields\n";
    return 1;
  }
  if (!items[2].separator() || items[3].enabled()) {
    std::cerr << "flags\n";
    return 1;
  }
  if (xgoef::ParseMenu("nope\n", &items, &err)) {
    std::cerr << "bad line accepted\n";
    return 1;
  }
  if (xgoef::ParseMenu("child\tC\t\t\tmissing\n", &items, &err)) {
    std::cerr << "missing parent accepted\n";
    return 1;
  }
  std::cout << "ok\n";
  return 0;
}
