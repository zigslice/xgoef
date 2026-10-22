#include "xgoef_window_chrome.h"

namespace xgoef {

bool ParseWindowChrome(const std::string& text, WindowChrome* out, std::string* err) {
  auto fail = [&](const char* why) {
    if (err) {
      *err = std::string("window chrome ") + why;
    }
    return false;
  };

  WindowChrome chrome;
  if (!text.empty()) {
    std::string fields[3];
    int commas = 0;
    for (char c : text) {
      if (c == ',') {
        if (commas >= 2) {
          return fail("fields");
        }
        ++commas;
        continue;
      }
      fields[commas].push_back(c);
    }
    if (commas != 2) {
      return fail("fields");
    }
    if (fields[0] == "1") {
      chrome.decorations = true;
    } else if (fields[0] == "0") {
      chrome.decorations = false;
    } else {
      return fail("decorations");
    }
    if (fields[1] == "system" || fields[1] == "light" || fields[1] == "dark") {
      chrome.theme = fields[1];
    } else {
      return fail("theme");
    }
    if (fields[2] == "1") {
      chrome.transparent = true;
    } else if (fields[2] == "0") {
      chrome.transparent = false;
    } else {
      return fail("transparent");
    }
  }
  if (out) {
    *out = chrome;
  }
  if (err) {
    err->clear();
  }
  return true;
}

}  // namespace xgoef
