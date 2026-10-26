#include "xgoef_webview.h"

#include <iostream>

static int fail(const char* why) {
  std::cerr << why << "\n";
  return 1;
}

int main() {
  xgoef::DevToolsReset();
  if (xgoef::DevToolsConsume()) {
    return fail("consume without expect");
  }
  xgoef::DevToolsCancel();
  if (xgoef::DevToolsConsume()) {
    return fail("cancel without expect");
  }

  xgoef::DevToolsExpect();
  xgoef::DevToolsExpect();
  if (!xgoef::DevToolsConsume() || !xgoef::DevToolsConsume()) {
    return fail("two expects");
  }
  if (xgoef::DevToolsConsume()) {
    return fail("extra consume");
  }

  xgoef::DevToolsExpect();
  xgoef::DevToolsCancel();
  if (xgoef::DevToolsConsume()) {
    return fail("cancel dropped the expect");
  }

  if (xgoef::DevToolsIs(7)) {
    return fail("id before note");
  }
  xgoef::DevToolsNote(7);
  xgoef::DevToolsNote(7);
  xgoef::DevToolsNote(8);
  if (!xgoef::DevToolsIs(7) || !xgoef::DevToolsIs(8)) {
    return fail("noted ids");
  }
  xgoef::DevToolsForget(7);
  if (xgoef::DevToolsIs(7) || !xgoef::DevToolsIs(8)) {
    return fail("forget");
  }
  xgoef::DevToolsForget(7);
  if (xgoef::DevToolsIs(7)) {
    return fail("forget unknown");
  }

  xgoef::DevToolsExpect();
  xgoef::DevToolsNote(9);
  xgoef::DevToolsReset();
  if (xgoef::DevToolsConsume() || xgoef::DevToolsIs(8) || xgoef::DevToolsIs(9)) {
    return fail("reset");
  }

  std::cout << "ok\n";
  return 0;
}
