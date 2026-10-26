#include "xgoef_crash.h"

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

}  // namespace

int main() {
  using xgoef::CrashSwitchesFor;
  const auto both = CrashSwitchesFor("", "/var/lib/xgoef", false, false);
  Expect(both.append_dir && both.append_enable && both.dir == "/var/lib/xgoef", "browser adds both");

  const auto keep_dir = CrashSwitchesFor("", "/var/lib/xgoef", true, false);
  Expect(!keep_dir.append_dir && keep_dir.append_enable && keep_dir.dir == "/var/lib/xgoef",
         "existing dir is kept");

  const auto keep_enable = CrashSwitchesFor("", "/tmp/state", false, true);
  Expect(keep_enable.append_dir && !keep_enable.append_enable, "existing enable is kept");

  const auto present = CrashSwitchesFor("", "/tmp/state", true, true);
  Expect(!present.append_dir && !present.append_enable && present.dir == "/tmp/state",
         "both already present");

  const auto child = CrashSwitchesFor("renderer", "/tmp/state", false, false);
  Expect(!child.append_dir && !child.append_enable && child.dir.empty(), "child process");

  const auto gpu = CrashSwitchesFor("gpu-process", "/tmp/state", false, false);
  Expect(!gpu.append_enable && gpu.dir.empty(), "gpu process");

  const std::string bad_dirs[] = {
      "", "relative", "C:/temp", "/tmp/a\nb", std::string("/tmp/\0b", 7),
  };
  for (const std::string& bad : bad_dirs) {
    const auto plan = CrashSwitchesFor("", bad, false, false);
    Expect(!plan.append_dir && !plan.append_enable && plan.dir.empty(), "reject bad dir");
  }

  const auto spaced = CrashSwitchesFor("", "/tmp/my dir", false, false);
  Expect(spaced.append_dir && spaced.dir == "/tmp/my dir", "space in dir");

  Expect(std::string(xgoef::kCrashDumpsSwitch) == "xgoef-crash-dumps", "request switch");
  Expect(std::string(xgoef::kCrashDumpsDirSwitch) == "crash-dumps-dir", "dir switch");
  Expect(std::string(xgoef::kEnableCrashReporterSwitch) == "enable-crash-reporter", "enable switch");

  if (fails != 0) {
    std::cerr << fails << " failed\n";
    return 1;
  }
  return 0;
}
