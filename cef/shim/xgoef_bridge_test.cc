#include "xgoef_bridge.h"

#include <unistd.h>

#include <cstdlib>
#include <fstream>
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

std::string TempDir() {
  char buf[] = "/tmp/xgoef-bridge-XXXXXX";
  char* dir = mkdtemp(buf);
  if (!dir) {
    std::perror("mkdtemp");
    std::exit(1);
  }
  return dir;
}

void WriteFile(const std::string& path, const std::string& body) {
  std::ofstream out(path, std::ios::binary);
  out << body;
  if (!out) {
    std::cerr << "write " << path << "\n";
    std::exit(1);
  }
}

}  // namespace

int main() {
  Expect(xgoef::OriginOf("https://Example.COM:443/a") == "https://example.com", "default https");
  Expect(xgoef::OriginOf("http://example.com:80/a") == "http://example.com", "default http");
  Expect(xgoef::OriginOf("http://example.com:0080/a") == "http://example.com", "padded port");
  Expect(xgoef::OriginOf("http://example.com:8080/a") == "http://example.com:8080", "custom port");
  Expect(xgoef::OriginOf("https://example.com:8443") == "https://example.com:8443", "no path");
  Expect(xgoef::OriginOf("http://user:pass@example.com/a") == "http://example.com", "userinfo");
  Expect(xgoef::OriginOf("http://[::1]:80/a") == "http://[::1]", "ipv6 default");
  Expect(xgoef::OriginOf("http://[::A]:8080/a") == "http://[::a]:8080", "ipv6 port");
  Expect(xgoef::OriginOf("xgoef://App/index.html") == "xgoef://app", "xgoef");
  Expect(xgoef::OriginOf("xgoef://app:80/index.html") == "xgoef://app:80", "xgoef port");
  Expect(xgoef::OriginOf("file:///tmp/index.html") == "file://", "file");
  Expect(xgoef::OriginOf("about:blank").empty(), "about");
  Expect(xgoef::OriginOf("devtools://devtools/bundled/inspector.html").empty(), "devtools");
  Expect(xgoef::OriginOf("chrome://settings").empty(), "chrome");
  Expect(xgoef::OriginOf("http://example.com:65536/").empty(), "port range");
  Expect(xgoef::OriginOf("http://example.com:/").empty(), "empty port");
  Expect(xgoef::OriginOf("http://[::1]:/").empty(), "empty ipv6 port");
  Expect(xgoef::OriginOf("").empty(), "empty url");

  const std::string dev = "http://127.0.0.1:5173";
  const std::string example = "https://example.com";
  Expect(xgoef::ShouldInjectBridge(false, true, "xgoef://app/index.html", "", ""), "main asset");
  Expect(xgoef::ShouldInjectBridge(false, true, "http://127.0.0.1:5173/", "", dev), "main dev");
  Expect(!xgoef::ShouldInjectBridge(false, true, "http://127.0.0.1:5173/", "", ""), "main dev denied");
  Expect(xgoef::ShouldInjectBridge(false, true, "about:blank", "", ""), "main blank");
  Expect(xgoef::ShouldInjectBridge(false, true, "", "", ""), "main empty");
  Expect(!xgoef::ShouldInjectBridge(true, true, "xgoef://app/index.html", "", dev), "disabled");
  Expect(!xgoef::ShouldInjectBridge(false, true, "devtools://devtools/bundled/inspector.html", "", ""),
         "devtools main");
  Expect(!xgoef::ShouldInjectBridge(false, true, "chrome-extension://abc/page.html", "", ""),
         "extension main");
  Expect(xgoef::ShouldInjectBridge(false, false, "xgoef://app/embed.html", "xgoef://app/index.html", ""),
         "same asset frame");
  Expect(xgoef::ShouldInjectBridge(false, false, "http://127.0.0.1:5173/child.html",
                                   "http://127.0.0.1:5173/", dev),
         "same dev frame");
  Expect(xgoef::ShouldInjectBridge(false, false, "https://example.com:443/a", "https://example.com/b",
                                   example),
         "default port frame");
  Expect(!xgoef::ShouldInjectBridge(false, false, "https://cdn.example/a.js", "xgoef://app/index.html",
                                    example),
         "foreign frame");
  Expect(!xgoef::ShouldInjectBridge(false, false, "http://127.0.0.1:5174/", "http://127.0.0.1:5173/",
                                    dev),
         "other port");
  Expect(!xgoef::ShouldInjectBridge(false, false, "about:blank", "xgoef://app/index.html", ""),
         "blank frame");
  Expect(!xgoef::ShouldInjectBridge(true, false, "xgoef://app/embed.html", "xgoef://app/index.html", ""),
         "disabled frame");

  Expect(!xgoef::RemoteNavigationBlocked("xgoef://app/index.html", ""), "asset navigation");
  Expect(!xgoef::RemoteNavigationBlocked("about:blank", ""), "blank navigation");
  Expect(xgoef::RemoteNavigationBlocked("https://example.com/a", ""), "https denied");
  Expect(!xgoef::RemoteNavigationBlocked("https://example.com/a", example), "https allowed");
  Expect(xgoef::RemoteNavigationBlocked("http://example.com/a", example), "scheme mismatch");
  Expect(!xgoef::RemoteNavigationBlocked("http://user:pass@example.com/a", "http://example.com"),
         "userinfo origin");
  Expect(!xgoef::RemoteNavigationBlocked("https://example.com/a", "https://evil.test,https://example.com"),
         "second origin");
  Expect(xgoef::RemoteNavigationBlocked("https://example.com/a", "https://example.com,"),
         "trailing comma");
  Expect(xgoef::RemoteNavigationBlocked("https://example.com/a", "https://example.com, "),
         "space in list");
  Expect(xgoef::RemoteNavigationBlocked("https://example.com/\n", example), "control in url");

  Expect(xgoef::BridgeScriptPath("/tmp/web") == "/tmp/web/xgoef.js", "join");
  Expect(xgoef::BridgeScriptPath("/tmp/web/") == "/tmp/web/xgoef.js", "slash");
  Expect(xgoef::BridgeScriptPath("/tmp/web\\") == "/tmp/web/xgoef.js", "backslash");
  Expect(xgoef::BridgeScriptPath("").empty(), "empty root");
  Expect(xgoef::BridgeScriptPath(std::string("/tmp/\0web", 9)).empty(), "nul root");

  const std::string dir = TempDir();
  const std::string file = dir + "/xgoef.js";
  WriteFile(file, "window.__XGOEF__ = {};\n");
  std::string body;
  std::string err;
  Expect(xgoef::ReadBridgeScript(file, &body, &err) && body == "window.__XGOEF__ = {};\n" && err.empty(),
         "read");

  WriteFile(file, std::string("\xEF\xBB\xBF") + "var x = 1;\n");
  Expect(xgoef::ReadBridgeScript(file, &body, &err) && body == "var x = 1;\n", "bom");

  WriteFile(file, "");
  Expect(!xgoef::ReadBridgeScript(file, &body, &err) && err == "empty" && body.empty(), "empty file");

  WriteFile(file, std::string("\xEF\xBB\xBF"));
  Expect(!xgoef::ReadBridgeScript(file, &body, &err) && err == "empty", "bom only");

  WriteFile(file, std::string("ab\0c", 4));
  Expect(!xgoef::ReadBridgeScript(file, &body, &err) && err == "nul", "nul file");

  Expect(!xgoef::ReadBridgeScript(dir + "/missing.js", &body, &err) && err == "missing", "missing");
  Expect(!xgoef::ReadBridgeScript(dir, &body, &err) && err == "notfile", "directory");
  Expect(!xgoef::ReadBridgeScript("", &body, &err) && err == "path", "empty path");
  Expect(!xgoef::ReadBridgeScript(std::string("a\0b", 3), &body, &err) && err == "path", "nul path");
  Expect(!xgoef::ReadBridgeScript(file, nullptr, &err) && err == "path", "null out");

  WriteFile(file, std::string(static_cast<size_t>(xgoef::kBridgeScriptMaxBytes) + 1, 'a'));
  Expect(!xgoef::ReadBridgeScript(file, &body, &err) && err == "large", "large");

  WriteFile(file, std::string(static_cast<size_t>(xgoef::kBridgeScriptMaxBytes), 'b'));
  Expect(xgoef::ReadBridgeScript(file, &body, &err) &&
             body.size() == static_cast<size_t>(xgoef::kBridgeScriptMaxBytes),
         "max size");

  Expect(xgoef::BridgeScriptPath(dir) == file, "path matches file");

  if (fails != 0) {
    std::cerr << fails << " failed\n";
    return 1;
  }
  return 0;
}
