#include "xgoef_window_state.h"

#include <iostream>
#include <string>

namespace {

int g_saw_min = -1;

void OnNote(const char*, int, int, int, int, int, int minimized) {
  g_saw_min = minimized;
}

}  // namespace

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
  if (items[0].label != "aux" || !items[0].positioned || items[0].maximized || items[0].minimized) {
    std::cerr << "aux fields\n";
    return 1;
  }
  if (items[1].label != "main" || items[1].width != 800 || !items[1].positioned ||
      !items[1].maximized || items[1].minimized) {
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
      items[0].positioned || !items[0].maximized || items[0].minimized) {
    std::cerr << "maximize-only\n";
    return 1;
  }
  if (!xgoef::ParseWindowState("main\t10\t20\t800\t600\tpmn\n", &items, &err) || items.size() != 1 ||
      !items[0].positioned || !items[0].maximized || !items[0].minimized || items[0].width != 800) {
    std::cerr << "pmn\n";
    return 1;
  }
  if (!xgoef::ParseWindowState("main\t0\t0\t0\t0\tn\n", &items, &err) || items.size() != 1 ||
      items[0].positioned || items[0].maximized || !items[0].minimized) {
    std::cerr << "minimize-only\n";
    return 1;
  }
  if (!xgoef::ParseWindowState("main\t1\t2\t3\t4\tppmmnn\n", &items, &err) || items.size() != 1 ||
      !items[0].positioned || !items[0].maximized || !items[0].minimized) {
    std::cerr << "duplicate flags\n";
    return 1;
  }
  if (xgoef::ParseWindowState("main\t10\t20\t800\t600\tz\n", &items, &err)) {
    std::cerr << "flag z accepted\n";
    return 1;
  }
  if (xgoef::ParseWindowState("main\t10\t20\t800\t600\tq\n", &items, &err)) {
    std::cerr << "flag q accepted\n";
    return 1;
  }
  if (xgoef::ShowRestore(true, true, 0) != 2 || xgoef::ShowRestore(true, false, 1) != 2) {
    std::cerr << "minimized wins\n";
    return 1;
  }
  if (xgoef::ShowRestore(false, true, 2) != 1) {
    std::cerr << "maximized wins over fallback\n";
    return 1;
  }
  if (xgoef::ShowRestore(false, false, 1) != 1 || xgoef::ShowRestore(false, false, 2) != 2 ||
      xgoef::ShowRestore(false, false, 0) != 0) {
    std::cerr << "fallback\n";
    return 1;
  }
  if (xgoef::ShowRestore(false, false, 9) != 0 || xgoef::ShowRestore(false, false, -3) != 0) {
    std::cerr << "bad fallback\n";
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
  g_saw_min = -1;
  xgoef_window_state_set_note(&OnNote);
  xgoef::WindowStateNote("main", 0, 0, 0, 0, 0, 1);
  if (g_saw_min != 1) {
    std::cerr << "note minimized\n";
    return 1;
  }
  xgoef_window_state_set_note(nullptr);
  std::cout << "ok\n";
  return 0;
}
