#include "xgoef_webview.h"

#include <unordered_set>

namespace xgoef {
namespace {

int g_expect = 0;
std::unordered_set<int> g_ids;

}  // namespace

void DevToolsExpect() { ++g_expect; }

bool DevToolsConsume() {
  if (g_expect <= 0) {
    return false;
  }
  --g_expect;
  return true;
}

void DevToolsCancel() {
  if (g_expect > 0) {
    --g_expect;
  }
}

void DevToolsNote(int id) { g_ids.insert(id); }

void DevToolsForget(int id) { g_ids.erase(id); }

bool DevToolsIs(int id) { return g_ids.find(id) != g_ids.end(); }

void DevToolsReset() {
  g_expect = 0;
  g_ids.clear();
}

}  // namespace xgoef
