#include "xgoef_window_chrome.h"

#include <iostream>
#include <string>

static int fail(const char* why) {
  std::cerr << why << "\n";
  return 1;
}

int main() {
  xgoef::WindowChrome chrome;
  std::string err;
  if (!xgoef::ParseWindowChrome("", &chrome, &err) || !chrome.decorations ||
      chrome.theme != "system" || chrome.transparent || !err.empty()) {
    return fail("empty");
  }
  if (!xgoef::ParseWindowChrome("0,dark,1", &chrome, &err) || chrome.decorations ||
      chrome.theme != "dark" || !chrome.transparent) {
    return fail("dark frameless");
  }
  if (!xgoef::ParseWindowChrome("1,light,0", &chrome, &err) || !chrome.decorations ||
      chrome.theme != "light" || chrome.transparent) {
    return fail("light");
  }
  if (!xgoef::ParseWindowChrome("1,system,0", &chrome, &err) || chrome.theme != "system" ||
      chrome.transparent || !chrome.decorations) {
    return fail("system");
  }
  if (xgoef::ParseWindowChrome("1,blue,0", &chrome, &err) || err.empty()) {
    return fail("bad theme");
  }
  if (xgoef::ParseWindowChrome("2,system,0", &chrome, &err)) {
    return fail("bad decorations");
  }
  if (xgoef::ParseWindowChrome("1,system,2", &chrome, &err)) {
    return fail("bad transparent");
  }
  if (xgoef::ParseWindowChrome("1,system", &chrome, &err)) {
    return fail("short");
  }
  if (xgoef::ParseWindowChrome("1,system,0,1", &chrome, &err)) {
    return fail("long");
  }
  if (xgoef::ParseWindowChrome("1,System,0", &chrome, &err)) {
    return fail("case");
  }
  if (xgoef::ParseWindowChrome("1, system, 0", &chrome, &err)) {
    return fail("spaces");
  }
  std::cout << "ok\n";
  return 0;
}
