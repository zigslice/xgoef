#include "xgoef_bridge.h"

#include <sys/stat.h>

#include <cstdio>
#include <string>

namespace xgoef {
namespace {

std::string Lower(std::string s) {
  for (char& c : s) {
    if (c >= 'A' && c <= 'Z') {
      c = static_cast<char>(c - 'A' + 'a');
    }
  }
  return s;
}

std::string SchemeOf(const std::string& url) {
  const size_t pos = url.find(':');
  if (pos == std::string::npos || pos == 0) {
    return "";
  }
  for (size_t i = 0; i < pos; ++i) {
    const unsigned char c = static_cast<unsigned char>(url[i]);
    const bool ok = (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
                    (c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.';
    if (!ok) {
      return "";
    }
  }
  return Lower(url.substr(0, pos));
}

bool BlockedScheme(const std::string& scheme) {
  return scheme == "devtools" || scheme == "view-source" ||
         scheme.compare(0, 6, "chrome") == 0;
}

void SetErr(std::string* err, const char* code) {
  if (err) {
    *err = code;
  }
}

}  // namespace

std::string OriginOf(const std::string& url) {
  const size_t scheme_end = url.find("://");
  if (scheme_end == std::string::npos || scheme_end == 0) {
    return "";
  }
  const std::string scheme = SchemeOf(url);
  if (scheme.empty() || BlockedScheme(scheme) || scheme == "about" ||
      scheme == "data" || scheme == "blob" || scheme == "javascript") {
    return "";
  }
  if (scheme == "file") {
    return "file://";
  }
  std::string rest = url.substr(scheme_end + 3);
  const size_t cut = rest.find_first_of("/?#");
  if (cut != std::string::npos) {
    rest.resize(cut);
  }
  const size_t at = rest.rfind('@');
  if (at != std::string::npos) {
    rest = rest.substr(at + 1);
  }
  if (rest.empty()) {
    return "";
  }
  std::string host;
  std::string port;
  bool has_port = false;
  if (rest[0] == '[') {
    const size_t br = rest.find(']');
    if (br == std::string::npos) {
      return "";
    }
    host = Lower(rest.substr(0, br + 1));
    if (br + 1 < rest.size()) {
      if (rest[br + 1] != ':') {
        return "";
      }
      has_port = true;
      port = rest.substr(br + 2);
    }
  } else {
    const size_t colon = rest.rfind(':');
    if (colon != std::string::npos) {
      host = Lower(rest.substr(0, colon));
      has_port = true;
      port = rest.substr(colon + 1);
    } else {
      host = Lower(rest);
    }
  }
  if (host.empty() || (host.front() == '[' && host.back() != ']')) {
    return "";
  }
  if (has_port && port.empty()) {
    return "";
  }
  if (!port.empty()) {
    if (port.size() > 5) {
      return "";
    }
    int value = 0;
    for (char c : port) {
      if (c < '0' || c > '9') {
        return "";
      }
      value = value * 10 + (c - '0');
    }
    if (value > 65535) {
      return "";
    }
    int def = -1;
    if (scheme == "http" || scheme == "ws") {
      def = 80;
    } else if (scheme == "https" || scheme == "wss") {
      def = 443;
    }
    if (value == def) {
      port.clear();
    } else {
      port = std::to_string(value);
    }
  }
  if (port.empty()) {
    return scheme + "://" + host;
  }
  return scheme + "://" + host + ":" + port;
}

bool AllowlistContains(const std::string& origin, const std::string& allowlist) {
  if (origin.empty() || allowlist.empty()) {
    return false;
  }
  for (unsigned char c : allowlist) {
    if (c < 0x20 || c == ' ') {
      return false;
    }
  }
  bool found = false;
  size_t i = 0;
  while (i <= allowlist.size()) {
    size_t j = allowlist.find(',', i);
    if (j == std::string::npos) {
      j = allowlist.size();
    }
    if (j == i) {
      return false;
    }
    if (allowlist.compare(i, j - i, origin) == 0) {
      found = true;
    }
    if (j == allowlist.size()) {
      break;
    }
    i = j + 1;
    if (i == allowlist.size()) {
      return false;
    }
  }
  return found;
}

bool RemoteOriginAllowed(const std::string& url, const std::string& allowlist) {
  const std::string scheme = SchemeOf(url);
  if (scheme != "http" && scheme != "https") {
    return false;
  }
  for (unsigned char c : url) {
    if (c < 0x20) {
      return false;
    }
  }
  return AllowlistContains(OriginOf(url), allowlist);
}

bool RemoteNavigationBlocked(const std::string& url, const std::string& allowlist) {
  const std::string scheme = SchemeOf(url);
  if (scheme != "http" && scheme != "https") {
    return false;
  }
  return !RemoteOriginAllowed(url, allowlist);
}

bool ShouldInjectBridge(bool disabled,
                        bool is_main,
                        const std::string& url,
                        const std::string& main_url,
                        const std::string& allowlist) {
  if (disabled) {
    return false;
  }
  const std::string scheme = SchemeOf(url);
  if (BlockedScheme(scheme)) {
    return false;
  }
  if ((scheme == "http" || scheme == "https") && !RemoteOriginAllowed(url, allowlist)) {
    return false;
  }
  if (is_main) {
    return true;
  }
  const std::string origin = OriginOf(url);
  const std::string main_origin = OriginOf(main_url);
  return !origin.empty() && origin == main_origin;
}

std::string BridgeScriptPath(const std::string& asset_root) {
  if (asset_root.empty() || asset_root.find('\0') != std::string::npos) {
    return "";
  }
  std::string root = asset_root;
  while (root.size() > 1 && (root.back() == '/' || root.back() == '\\')) {
    root.pop_back();
  }
  return root + "/xgoef.js";
}

bool ReadBridgeScript(const std::string& path, std::string* out, std::string* err) {
  if (out) {
    out->clear();
  }
  if (!out || path.empty() || path.find('\0') != std::string::npos) {
    SetErr(err, "path");
    return false;
  }
  struct stat st;
  if (stat(path.c_str(), &st) != 0) {
    SetErr(err, "missing");
    return false;
  }
  if (!S_ISREG(st.st_mode)) {
    SetErr(err, "notfile");
    return false;
  }
  if (st.st_size > static_cast<off_t>(kBridgeScriptMaxBytes)) {
    SetErr(err, "large");
    return false;
  }
  if (st.st_size < 0) {
    SetErr(err, "read");
    return false;
  }
  const size_t n = static_cast<size_t>(st.st_size);
  std::string data(n, '\0');
  FILE* file = std::fopen(path.c_str(), "rb");
  if (!file) {
    SetErr(err, "missing");
    return false;
  }
  const size_t got = n == 0 ? 0 : std::fread(data.data(), 1, n, file);
  std::fclose(file);
  if (got != n) {
    SetErr(err, "read");
    return false;
  }
  if (data.size() >= 3 && static_cast<unsigned char>(data[0]) == 0xEF &&
      static_cast<unsigned char>(data[1]) == 0xBB &&
      static_cast<unsigned char>(data[2]) == 0xBF) {
    data.erase(0, 3);
  }
  if (data.empty()) {
    SetErr(err, "empty");
    return false;
  }
  if (data.find('\0') != std::string::npos) {
    SetErr(err, "nul");
    return false;
  }
  *out = data;
  if (err) {
    err->clear();
  }
  return true;
}

}  // namespace xgoef
