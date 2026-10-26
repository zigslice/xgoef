#include "xgoef_lifecycle.h"

#include <mutex>

namespace {

std::mutex g_mu;
xgoef_page_load_fn g_fn = nullptr;

}  // namespace

extern "C" void xgoef_page_load_set(xgoef_page_load_fn fn) {
  std::lock_guard<std::mutex> lock(g_mu);
  g_fn = fn;
}

extern "C" void xgoef_page_load_note(int is_main,
                                    int is_devtools,
                                    const char* label,
                                    const char* url) {
  const xgoef::PageLoad plan = xgoef::PageLoadFor(
      is_main != 0, is_devtools != 0, label ? label : "", url ? url : "");
  if (!plan.notify) {
    return;
  }
  xgoef_page_load_fn fn = nullptr;
  {
    std::lock_guard<std::mutex> lock(g_mu);
    fn = g_fn;
  }
  if (fn) {
    fn(plan.label.c_str(), plan.url.c_str());
  }
}
