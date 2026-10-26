#include "xgoef_eval.h"

#include <iostream>
#include <string>
#include <utility>
#include <vector>

static int fail(const char* why) {
  std::cerr << why << "\n";
  return 1;
}

int main() {
  const std::pair<const char*, const char*> ok[] = {
      {"xgoef://app/index.html", "xgoef://app/index.html"},
      {"xgoef://app/other.html", "xgoef://app/index.html"},
      {"XGOEF://APP/a#frag", "xgoef://app/b?q=1"},
      {"https://example.test/a", "https://example.test/b"},
      {"https://Example.TEST/a", "https://example.test/b?x=1#y"},
      {"https://example.test/a", "https://example.test:443/b"},
      {"http://example.test:80/a", "http://example.test/b"},
      {"http://127.0.0.1:5173/a", "http://127.0.0.1:5173/"},
      {"http://[::1]/a", "http://[::1]/b"},
      {"http://[::1]:8080/a", "http://[::1]:8080/b"},
  };
  for (const auto& pair : ok) {
    if (!xgoef::OriginAllowed(pair.first, pair.second)) {
      std::cerr << "want allow " << pair.first << " vs " << pair.second << "\n";
      return 1;
    }
  }
  const std::pair<const char*, const char*> bad[] = {
      {"https://evil.test/", "xgoef://app/index.html"},
      {"xgoef://app.evil/a", "xgoef://app/a"},
      {"xgoef://other/a", "xgoef://other/a"},
      {"xgoef://app:80/a", "xgoef://app/a"},
      {"http://127.0.0.1:5173/", "http://127.0.0.1:5174/"},
      {"http://example.test/", "https://example.test/"},
      {"http://example.test.evil/", "http://example.test/"},
      {"http://user:pass@example.test/", "http://example.test/"},
      {"http://example.test\\@evil.test/", "http://example.test/"},
      {"javascript:alert(1)", "javascript:alert(1)"},
      {"data:text/html,hi", "data:text/html,hi"},
      {"file:///tmp/a", "file:///tmp/a"},
      {"about:blank", "about:blank"},
      {"", "xgoef://app/"},
      {"xgoef://app/", ""},
      {"http://[::1]/", "http://127.0.0.1/"},
      {"http://[::1]:8080/", "http://[::1]/"},
      {"https://example.test/a b", "https://example.test/"},
  };
  for (const auto& pair : bad) {
    if (xgoef::OriginAllowed(pair.first, pair.second)) {
      std::cerr << "want deny " << pair.first << " vs " << pair.second << "\n";
      return 1;
    }
  }

  if (xgoef::ScriptAccepted("") || xgoef::ScriptAccepted(" \n\t") || xgoef::ScriptAccepted(std::string("ok\0", 3))) {
    return fail("script");
  }
  if (!xgoef::ScriptAccepted("window.ok = 1;")) {
    return fail("accept");
  }

  std::vector<std::string> scripts = {"window.ok = 1;", "a\nb\nc"};
  std::string text = xgoef::FormatScriptTable(scripts);
  std::vector<std::string> got;
  std::string err;
  if (!xgoef::ParseScriptTable(text, &got, &err) || got.size() != 2 || got[0] != scripts[0] ||
      got[1] != scripts[1]) {
    return fail("round trip");
  }
  if (xgoef::ParseScriptTable(text + "1", &got, &err)) {
    return fail("trailing");
  }
  if (!xgoef::ParseScriptTable("0\n", &got, &err) || !got.empty()) {
    return fail("empty table");
  }

  xgoef::EvalReset();
  if (xgoef_eval_set_init("main", "javascript:alert(1)", text.c_str()) == 0) {
    return fail("bad pin stored");
  }
  if (xgoef_eval_set_init("main", "https://example.test/index.html", text.c_str()) != 0) {
    return fail("set");
  }
  std::string allowed;
  if (!xgoef::EvalLookup("main", &allowed, &got) || allowed != "https://example.test/index.html" ||
      got.size() != 2) {
    return fail("lookup");
  }
  if (xgoef_eval_set_init("main", "https://example.test/", "0\n") != 0) {
    return fail("clear");
  }
  if (xgoef::EvalLookup("main", &allowed, &got)) {
    return fail("cleared");
  }
  if (xgoef_eval_set_init("", "xgoef://app/index.html", text.c_str()) != 0) {
    return fail("default label");
  }
  if (!xgoef::EvalLookup("main", &allowed, &got) || got.size() != 2) {
    return fail("main label");
  }

  std::cout << "ok\n";
  return 0;
}
