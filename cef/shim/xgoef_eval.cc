#include "xgoef_eval.h"

#include <map>
#include <mutex>

namespace xgoef {
namespace {

constexpr int kMaxScriptBytes = 16 * 1024;
constexpr int kMaxInitScripts = 8;

struct Origin {
  std::string scheme;
  std::string host;
  int port = 0;
};

bool AsciiLower(const std::string& in, std::string* out) {
  out->assign(in);
  for (char& c : *out) {
    unsigned char u = static_cast<unsigned char>(c);
    if (u >= 128) {
      return false;
    }
    if (u >= 'A' && u <= 'Z') {
      c = static_cast<char>(u - 'A' + 'a');
    }
  }
  return true;
}

bool SchemeOK(const std::string& scheme) {
  if (scheme.empty() || scheme[0] < 'a' || scheme[0] > 'z') {
    return false;
  }
  for (size_t i = 1; i < scheme.size(); ++i) {
    char c = scheme[i];
    if ((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.') {
      continue;
    }
    return false;
  }
  return true;
}

bool ParseDecimal(const std::string& text, int max_value, int* out) {
  if (text.empty() || text.size() > 6 || (text.size() > 1 && text[0] == '0')) {
    return false;
  }
  int n = 0;
  for (char c : text) {
    if (c < '0' || c > '9') {
      return false;
    }
    n = n * 10 + (c - '0');
    if (n > max_value) {
      return false;
    }
  }
  *out = n;
  return true;
}

bool SplitHost(const std::string& authority, std::string* host, std::string* port) {
  if (authority.empty()) {
    return false;
  }
  if (authority[0] == '[') {
    size_t end = authority.find(']');
    if (end == std::string::npos || end < 2) {
      return false;
    }
    *host = authority.substr(1, end - 1);
    if (host->empty() || host->find('[') != std::string::npos) {
      return false;
    }
    if (end + 1 == authority.size()) {
      port->clear();
      return true;
    }
    if (authority[end + 1] != ':' || end + 2 == authority.size()) {
      return false;
    }
    *port = authority.substr(end + 2);
    return !port->empty();
  }
  if (authority.find(':') != authority.rfind(':')) {
    return false;
  }
  size_t colon = authority.find(':');
  if (colon == std::string::npos) {
    *host = authority;
    port->clear();
    return true;
  }
  *host = authority.substr(0, colon);
  *port = authority.substr(colon + 1);
  return !host->empty() && !port->empty();
}

bool NormalizePort(const std::string& scheme, const std::string& port_text, int* out) {
  if (port_text.empty()) {
    if (scheme == "http") {
      *out = 80;
      return true;
    }
    if (scheme == "https") {
      *out = 443;
      return true;
    }
    if (scheme == "xgoef") {
      *out = -1;
      return true;
    }
    return false;
  }
  return ParseDecimal(port_text, 65535, out) && *out >= 1;
}

bool ParseOrigin(const std::string& raw, Origin* out) {
  if (raw.empty() || raw.size() > 2048) {
    return false;
  }
  for (char c : raw) {
    switch (c) {
      case '\0':
      case ' ':
      case '\t':
      case '\r':
      case '\n':
      case '\\':
      case '<':
      case '>':
      case '"':
      case '\'':
        return false;
      default:
        break;
    }
  }
  size_t sep = raw.find("://");
  if (sep == std::string::npos || sep == 0 || sep + 3 >= raw.size()) {
    return false;
  }
  std::string scheme;
  if (!AsciiLower(raw.substr(0, sep), &scheme) || !SchemeOK(scheme)) {
    return false;
  }
  std::string rest = raw.substr(sep + 3);
  size_t hash = rest.find('#');
  if (hash != std::string::npos) {
    rest.resize(hash);
  }
  size_t query = rest.find('?');
  if (query != std::string::npos) {
    rest.resize(query);
  }
  size_t slash = rest.find('/');
  if (slash != std::string::npos) {
    rest.resize(slash);
  }
  if (rest.empty() || rest.find('@') != std::string::npos) {
    return false;
  }
  std::string host;
  std::string port_text;
  if (!SplitHost(rest, &host, &port_text)) {
    return false;
  }
  if (!AsciiLower(host, &host) || host.empty()) {
    return false;
  }
  int port = 0;
  if (!NormalizePort(scheme, port_text, &port)) {
    return false;
  }
  out->scheme = scheme;
  out->host = host;
  out->port = port;
  return true;
}

bool TakeLine(const std::string& text, size_t* pos, std::string* line) {
  if (*pos >= text.size()) {
    return false;
  }
  size_t nl = text.find('\n', *pos);
  if (nl == std::string::npos) {
    return false;
  }
  line->assign(text, *pos, nl - *pos);
  *pos = nl + 1;
  return true;
}

struct InitRecord {
  std::string allowed;
  std::vector<std::string> scripts;
};

std::mutex g_mu;
std::map<std::string, InitRecord> g_init;

}  // namespace

bool ScriptAccepted(const std::string& source) {
  if (source.empty() || source.size() > static_cast<size_t>(kMaxScriptBytes)) {
    return false;
  }
  if (source.find('\0') != std::string::npos) {
    return false;
  }
  for (char c : source) {
    if (c != ' ' && c != '\t' && c != '\r' && c != '\n' && c != '\v' && c != '\f') {
      return true;
    }
  }
  return false;
}

bool ParseScriptTable(const std::string& text, std::vector<std::string>* out, std::string* err) {
  if (text.find('\0') != std::string::npos) {
    if (err) {
      *err = "nul";
    }
    return false;
  }
  size_t pos = 0;
  std::string line;
  if (!TakeLine(text, &pos, &line)) {
    if (err) {
      *err = "table";
    }
    return false;
  }
  int count = 0;
  if (!ParseDecimal(line, kMaxInitScripts, &count)) {
    if (err) {
      *err = "count";
    }
    return false;
  }
  std::vector<std::string> scripts;
  scripts.reserve(static_cast<size_t>(count));
  for (int i = 0; i < count; ++i) {
    if (!TakeLine(text, &pos, &line)) {
      if (err) {
        *err = "table";
      }
      return false;
    }
    int n = 0;
    if (!ParseDecimal(line, kMaxScriptBytes, &n) || n < 1) {
      if (err) {
        *err = "length";
      }
      return false;
    }
    if (pos + static_cast<size_t>(n) > text.size()) {
      if (err) {
        *err = "table";
      }
      return false;
    }
    std::string source = text.substr(pos, static_cast<size_t>(n));
    pos += static_cast<size_t>(n);
    if (!ScriptAccepted(source)) {
      if (err) {
        *err = "script";
      }
      return false;
    }
    scripts.push_back(source);
  }
  if (pos != text.size()) {
    if (err) {
      *err = "table";
    }
    return false;
  }
  if (out) {
    *out = scripts;
  }
  return true;
}

std::string FormatScriptTable(const std::vector<std::string>& scripts) {
  std::string out = std::to_string(scripts.size()) + "\n";
  for (const std::string& source : scripts) {
    out += std::to_string(source.size()) + "\n";
    out += source;
  }
  return out;
}

bool OriginAllowed(const std::string& frame, const std::string& allowed) {
  Origin f;
  Origin a;
  if (!ParseOrigin(frame, &f) || !ParseOrigin(allowed, &a)) {
    return false;
  }
  if (f.scheme != a.scheme || f.host != a.host || f.port != a.port) {
    return false;
  }
  if (a.scheme == "xgoef") {
    return a.host == "app";
  }
  return a.scheme == "http" || a.scheme == "https";
}

void EvalSetInit(const std::string& label, const std::string& allowed,
                 const std::vector<std::string>& scripts) {
  std::lock_guard<std::mutex> lock(g_mu);
  std::string lab = label.empty() ? "main" : label;
  if (scripts.empty()) {
    g_init.erase(lab);
    return;
  }
  g_init[lab] = InitRecord{allowed, scripts};
}

bool EvalLookup(const std::string& label, std::string* allowed, std::vector<std::string>* scripts) {
  std::lock_guard<std::mutex> lock(g_mu);
  auto it = g_init.find(label.empty() ? "main" : label);
  if (it == g_init.end()) {
    return false;
  }
  if (allowed) {
    *allowed = it->second.allowed;
  }
  if (scripts) {
    *scripts = it->second.scripts;
  }
  return true;
}

void EvalReset() {
  std::lock_guard<std::mutex> lock(g_mu);
  g_init.clear();
}

}  // namespace xgoef

extern "C" int xgoef_eval_set_init(const char* label, const char* allowed_url, const char* text) {
  std::string body = text ? text : "";
  std::vector<std::string> scripts;
  if (!body.empty()) {
    std::string err;
    if (!xgoef::ParseScriptTable(body, &scripts, &err)) {
      return -1;
    }
  }
  std::string allowed = allowed_url ? allowed_url : "";
  if (!scripts.empty() && !xgoef::OriginAllowed(allowed, allowed)) {
    return -1;
  }
  xgoef::EvalSetInit(label ? label : "", allowed, scripts);
  return 0;
}
