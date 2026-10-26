#include "xgoef_taskbar.h"

#include <iostream>
#include <string>

int main() {
  const xgoef::SkipTaskbarMessage hide = xgoef::DescribeSkip(true);
  if (std::string(hide.type) != "_NET_WM_STATE" || hide.action != 1 ||
      std::string(hide.atom) != "_NET_WM_STATE_SKIP_TASKBAR" || hide.source != 1) {
    std::cerr << "skip add\n";
    return 1;
  }
  const xgoef::SkipTaskbarMessage show = xgoef::DescribeSkip(false);
  if (show.action != 0 || std::string(show.atom) != hide.atom) {
    std::cerr << "skip remove\n";
    return 1;
  }

  xgoef::ProgressProperty prop;
  if (!xgoef::DescribeProgress(40, &prop) || prop.remove || prop.value != 40 ||
      std::string(prop.name) != "_NET_WM_XAPP_PROGRESS") {
    std::cerr << "progress 40\n";
    return 1;
  }
  if (!xgoef::DescribeProgress(0, &prop) || prop.remove || prop.value != 0) {
    std::cerr << "progress 0\n";
    return 1;
  }
  if (!xgoef::DescribeProgress(-1, &prop) || !prop.remove) {
    std::cerr << "progress clear\n";
    return 1;
  }
  if (xgoef::DescribeProgress(101, &prop) || xgoef::DescribeProgress(-2, &prop) ||
      xgoef::DescribeProgress(40, nullptr)) {
    std::cerr << "progress rejected\n";
    return 1;
  }

  bool skip = true;
  if (xgoef::TaskbarTakeExpected("main", &skip)) {
    std::cerr << "missing expect\n";
    return 1;
  }
  xgoef::TaskbarExpect("", true);
  if (xgoef::TaskbarTakeExpected("", &skip)) {
    std::cerr << "empty label\n";
    return 1;
  }
  xgoef::TaskbarExpect("main", true);
  xgoef::TaskbarExpect("main", false);
  if (!xgoef::TaskbarTakeExpected("main", &skip) || skip) {
    std::cerr << "expect replace\n";
    return 1;
  }
  if (xgoef::TaskbarTakeExpected("main", &skip)) {
    std::cerr << "expect consumed\n";
    return 1;
  }
  xgoef::TaskbarExpect("aux", true);
  if (!xgoef::TaskbarTakeExpected("aux", &skip) || !skip) {
    std::cerr << "aux expect\n";
    return 1;
  }
  if (xgoef::TaskbarTakeExpected("main", nullptr)) {
    std::cerr << "null out\n";
    return 1;
  }

  std::cout << "ok\n";
  return 0;
}
