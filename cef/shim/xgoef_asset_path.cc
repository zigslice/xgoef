// Path mapping for xgoef://app/<path>. Kept free of CEF headers so it can be
// tested with a plain C++ compiler. Rules match package asset (asset/root.go).

#include "xgoef_asset_path.h"

#include <sys/stat.h>

#include <cstdlib>
#include <string>
#include <vector>

namespace xgoef {
namespace {

bool StartsWith(const std::string& s, const std::string& prefix) {
  return s.size() >= prefix.size() && s.compare(0, prefix.size(), prefix) == 0;
}

bool Decode(const std::string& in, std::string* out) {
  out->clear();
  for (size_t i = 0; i < in.size(); ++i) {
    if (in[i] == '%' && i + 2 < in.size()) {
      int hi = -1, lo = -1;
      auto hex = [](char c) -> int {
        if (c >= '0' && c <= '9') return c - '0';
        if (c >= 'a' && c <= 'f') return c - 'a' + 10;
        if (c >= 'A' && c <= 'F') return c - 'A' + 10;
        return -1;
      };
      hi = hex(in[i + 1]);
      lo = hex(in[i + 2]);
      if (hi < 0 || lo < 0) {
        return false;
      }
      char ch = static_cast<char>((hi << 4) | lo);
      if (ch == '\0') {
        return false;
      }
      out->push_back(ch);
      i += 2;
      continue;
    }
    out->push_back(in[i]);
  }
  return true;
}

// Collapse a URL path the way path.Clean("/" + p) does, without leaving "/".
bool CleanURLPath(const std::string& decoded, std::string* rel, std::string* err) {
  if (decoded.find('\\') != std::string::npos ||
      decoded.find('\0') != std::string::npos) {
    *err = "outside";
    return false;
  }
  std::vector<std::string> parts;
  std::string cur;
  auto flush = [&]() {
    if (cur.empty() || cur == ".") {
      cur.clear();
      return;
    }
    if (cur == "..") {
      if (!parts.empty()) {
        parts.pop_back();
      }
      cur.clear();
      return;
    }
    parts.push_back(cur);
    cur.clear();
  };
  for (char c : decoded) {
    if (c == '/') {
      flush();
    } else {
      cur.push_back(c);
    }
  }
  flush();
  rel->clear();
  for (size_t i = 0; i < parts.size(); ++i) {
    if (i) rel->push_back('/');
    rel->append(parts[i]);
  }
  return true;
}

bool Within(const std::string& path, const std::string& root) {
  if (path == root) return true;
  std::string prefix = root;
  if (prefix.empty() || prefix.back() != '/') prefix.push_back('/');
  return StartsWith(path, prefix);
}

std::string Real(const std::string& path) {
  char* got = realpath(path.c_str(), nullptr);
  if (!got) return std::string();
  std::string out(got);
  free(got);
  return out;
}

}  // namespace

bool ResolveAssetPath(const std::string& root,
                      const std::string& url,
                      std::string* out_path,
                      std::string* err) {
  if (err) err->clear();
  auto fail = [&](const std::string& msg) {
    if (err) *err = msg;
    return false;
  };
  if (root.empty()) return fail("root");
  const std::string prefix = "xgoef://app";
  if (url.size() < prefix.size() ||
      url.compare(0, prefix.size(), prefix) != 0) {
    return fail("scheme");
  }
  if (url.size() > prefix.size() && url[prefix.size()] != '/' &&
      url[prefix.size()] != '?' && url[prefix.size()] != '#') {
    return fail("scheme");
  }
  std::string rest = url.substr(prefix.size());
  if (!rest.empty() && rest[0] == '/') rest.erase(rest.begin());
  auto cut = rest.find_first_of("?#");
  if (cut != std::string::npos) rest.resize(cut);

  std::string decoded;
  if (!Decode(rest, &decoded)) return fail("decode");
  std::string rel;
  std::string clean_err;
  if (!CleanURLPath(decoded, &rel, &clean_err)) return fail("outside");
  if (rel.empty()) rel = "index.html";

  std::string root_real = Real(root);
  if (root_real.empty()) return fail("root");

  std::string joined = root_real + "/" + rel;
  struct stat st;
  if (stat(joined.c_str(), &st) == 0 && S_ISDIR(st.st_mode)) {
    joined += "/index.html";
  }
  std::string real = Real(joined);
  if (real.empty()) return fail("missing");
  if (stat(real.c_str(), &st) == 0 && S_ISDIR(st.st_mode)) {
    real = Real(real + "/index.html");
    if (real.empty()) return fail("missing");
  }
  if (!Within(real, root_real)) return fail("outside");
  if (out_path) *out_path = real;
  return true;
}

}  // namespace xgoef
