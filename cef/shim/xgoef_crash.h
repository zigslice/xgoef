// Decide when the browser process should enable Chromium crashpad.
// No CEF headers, so this file can be compiled on its own.
// Runtime adds --xgoef-crash-dumps=<absolute dir>. OnBeforeCommandLineProcessing
// copies that into crash-dumps-dir and enable-crash-reporter.
#ifndef XGOEF_CRASH_H_
#define XGOEF_CRASH_H_

#include <string>

namespace xgoef {

inline constexpr const char kCrashDumpsSwitch[] = "xgoef-crash-dumps";
inline constexpr const char kCrashDumpsDirSwitch[] = "crash-dumps-dir";
inline constexpr const char kEnableCrashReporterSwitch[] = "enable-crash-reporter";

// CrashSwitches is the pair of Chromium switches to append.
// append_dir and append_enable stay false when the request is rejected
// or the switch is already present. dir is the accepted directory.
struct CrashSwitches {
  bool append_dir = false;
  bool append_enable = false;
  std::string dir;
};

// AbsoluteCrashDir reports whether dir can be passed to crash-dumps-dir.
inline bool AbsoluteCrashDir(const std::string& dir) {
  if (dir.empty() || dir.find('\0') != std::string::npos) {
    return false;
  }
  for (unsigned char c : dir) {
    if (c < 0x20) {
      return false;
    }
  }
  return dir[0] == '/';
}

// CrashSwitchesFor plans crashpad switches for one process command line.
// process_type is empty for the browser process. requested is the value of
// --xgoef-crash-dumps. has_dir and has_enable report switches already present.
// A request is accepted only for the browser process when requested is an
// absolute path and contains no NUL and no byte below 0x20.
inline CrashSwitches CrashSwitchesFor(const std::string& process_type,
                                      const std::string& requested,
                                      bool has_dir,
                                      bool has_enable) {
  CrashSwitches out;
  if (!process_type.empty() || !AbsoluteCrashDir(requested)) {
    return out;
  }
  out.dir = requested;
  out.append_dir = !has_dir;
  out.append_enable = !has_enable;
  return out;
}

}  // namespace xgoef

#endif  // XGOEF_CRASH_H_
