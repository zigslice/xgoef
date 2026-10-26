#include "xgoef_csp.h"

#include <iostream>
#include <string>

namespace {

int fails = 0;

void Expect(bool cond, const std::string& msg) {
  if (!cond) {
    std::cerr << "FAIL " << msg << "\n";
    ++fails;
  }
}

// Same text as conf.DefaultCSP. Go is the source of that default.
const char kGoDefault[] =
    "default-src 'self'; script-src 'self' 'unsafe-inline'; "
    "style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; "
    "font-src 'self' data:; connect-src 'self'";

}  // namespace

int main() {
  using xgoef::CSPHeaderValue;
  std::string out = "stale";

  Expect(std::string(xgoef::kCSPSwitch) == "xgoef-csp", "csp switch");
  Expect(std::string(xgoef::kNoCSPSwitch) == "xgoef-no-csp", "no-csp switch");
  Expect(xgoef::kCSPMaxBytes == 2048, "max bytes");

  Expect(CSPHeaderValue(false, kGoDefault, &out) && out == kGoDefault, "default accepted");

  const std::string custom = "default-src 'none'; img-src 'self' data:";
  Expect(CSPHeaderValue(false, custom, &out) && out == custom, "custom accepted");

  Expect(!CSPHeaderValue(true, kGoDefault, &out) && out.empty(), "disabled wins");
  Expect(!CSPHeaderValue(true, "", &out) && out.empty(), "disabled and empty");
  Expect(!CSPHeaderValue(false, "", &out) && out.empty(), "empty configured");

  std::string nul = "default-src '";
  nul.push_back('\0');
  nul += "self'";
  const std::string bad[] = {
      "default-src 'self'\n",
      "default-src 'self'\r\nX-Injected: 1",
      nul,
      "default-src\t'self'",
      std::string(2049, 'a'),
  };
  for (const std::string& policy : bad) {
    out = "stale";
    Expect(!CSPHeaderValue(false, policy, &out) && out.empty(), "reject unsafe policy");
  }

  const std::string maxed(2048, 'b');
  Expect(CSPHeaderValue(false, maxed, &out) && out == maxed, "2048 accepted");

  Expect(CSPHeaderValue(false, custom, nullptr), "null out");
  Expect(!CSPHeaderValue(true, custom, nullptr), "null out disabled");

  if (fails != 0) {
    std::cerr << fails << " failed\n";
    return 1;
  }
  return 0;
}
