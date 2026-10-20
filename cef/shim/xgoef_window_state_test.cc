#include "xgoef_window_state.h"

#include <iostream>
#include <string>

int main() {
  const std::string flat =
      "aux\t1\t2\t3\t4\tp\n"
      "main\t10\t20\t800\t600\tpm\n";
  std::vector<xgoef::WindowPlace> items;
  std::string err;
  if (!xgoef::ParseWindowState(flat, &items, &err) || items.size() != 2) {
    std::cerr << "parse " << err << " size " << items.size() << "\n";
    return 1;
  }
  if (items[0].label != "aux" || !items[0].positioned || items[0].maximized) {
    std::cerr << "aux fields\n";
    return 1;
  }
  if (items[1].label != "main" || items[1].width != 800 || !items[1].positioned ||
      !items[1].maximized) {
    std::cerr << "main fields\n";
    return 1;
  }
  if (xgoef::ParseWindowState("nope\n", &items, &err)) {
    std::cerr << "bad line accepted\n";
    return 1;
  }
  if (xgoef::ParseWindowState("main\t0\t0\t0\t0\tp\n", &items, &err)) {
    std::cerr << "empty size accepted\n";
    return 1;
  }
  if (!xgoef::ParseWindowState("main\t0\t0\t0\t0\tm\n", &items, &err) || items.size() != 1 ||
      items[0].positioned || !items[0].maximized) {
    std::cerr << "maximize-only\n";
    return 1;
  }
  xgoef::WindowStateSet(flat);
  xgoef::WindowPlace found;
  if (!xgoef::WindowStateFind("main", &found) || found.height != 600) {
    std::cerr << "find\n";
    return 1;
  }
  xgoef::WindowStateSet("bad\n");
  if (xgoef::WindowStateFind("main", &found)) {
    std::cerr << "bad set kept old rows\n";
    return 1;
  }
  std::cout << "ok\n";
  return 0;
}
