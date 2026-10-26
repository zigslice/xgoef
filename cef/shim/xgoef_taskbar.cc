#include "xgoef_taskbar.h"

#include <mutex>
#include <string>
#include <unordered_map>

namespace xgoef {
namespace {

std::mutex g_mu;
std::unordered_map<std::string, bool> g_expected;

}  // namespace

void TaskbarExpect(const std::string& label, bool skip) {
  if (label.empty()) {
    return;
  }
  std::lock_guard<std::mutex> lock(g_mu);
  g_expected[label] = skip;
}

bool TaskbarTakeExpected(const std::string& label, bool* skip) {
  if (label.empty() || skip == nullptr) {
    return false;
  }
  std::lock_guard<std::mutex> lock(g_mu);
  const auto it = g_expected.find(label);
  if (it == g_expected.end()) {
    return false;
  }
  *skip = it->second;
  g_expected.erase(it);
  return true;
}

}  // namespace xgoef

extern "C" void xgoef_taskbar_expect(const char* label, int skip) {
  xgoef::TaskbarExpect(label ? label : "", skip != 0);
}
