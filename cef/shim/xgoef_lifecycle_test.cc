#include "xgoef_lifecycle.h"

#include <iostream>
#include <string>

namespace {

int fails = 0;
int calls = 0;
std::string got_label;
std::string got_url;

void Expect(bool cond, const std::string& msg) {
  if (!cond) {
    std::cerr << "FAIL " << msg << "\n";
    ++fails;
  }
}

void OnLoad(const char* label, const char* url) {
  ++calls;
  got_label = label ? label : "";
  got_url = url ? url : "";
}

}  // namespace

int main() {
  using xgoef::PageLoadFor;
  const auto page = PageLoadFor(true, false, "main", "xgoef://app/index.html");
  Expect(page.notify && page.label == "main" && page.url == "xgoef://app/index.html", "main asset");
  const auto blank = PageLoadFor(true, false, "aux", "about:blank");
  Expect(blank.notify && blank.label == "aux", "about blank");
  const auto dev = PageLoadFor(true, false, "main", "http://127.0.0.1:5173/");
  Expect(dev.notify && dev.url == "http://127.0.0.1:5173/", "dev server");

  Expect(!PageLoadFor(false, false, "main", "xgoef://app/index.html").notify, "subframe");
  Expect(!PageLoadFor(true, true, "main", "devtools://devtools/bundled/inspector.html").notify,
         "devtools");
  Expect(!PageLoadFor(true, false, "", "about:blank").notify, "empty label");
  Expect(!PageLoadFor(true, false, "main", "").notify, "empty url");
  Expect(!PageLoadFor(true, false, "main", "http://example.test/a\nb").notify, "newline");
  Expect(!PageLoadFor(true, false, std::string("ma\0in", 5), "about:blank").notify, "nul label");
  Expect(PageLoadFor(true, false, std::string(64, 'a'), "about:blank").notify, "label max");
  Expect(!PageLoadFor(true, false, std::string(65, 'a'), "about:blank").notify, "label too long");
  Expect(PageLoadFor(true, false, "main", std::string(4096, 'b')).notify, "url max");
  Expect(!PageLoadFor(true, false, "main", std::string(4097, 'b')).notify, "url too long");

  xgoef_page_load_set(OnLoad);
  xgoef_page_load_note(1, 0, "main", "xgoef://app/index.html");
  Expect(calls == 1 && got_label == "main" && got_url == "xgoef://app/index.html", "callback");
  xgoef_page_load_note(0, 0, "main", "xgoef://app/child.html");
  xgoef_page_load_note(1, 1, "main", "devtools://x");
  xgoef_page_load_note(1, 0, "main", "bad\nurl");
  xgoef_page_load_note(1, 0, nullptr, "about:blank");
  Expect(calls == 1, "rejected notes");
  xgoef_page_load_set(nullptr);
  xgoef_page_load_note(1, 0, "main", "about:blank");
  Expect(calls == 1, "cleared callback");

  if (fails != 0) {
    std::cerr << fails << " failed\n";
    return 1;
  }
  return 0;
}
