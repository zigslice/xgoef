// Decide which frames receive xgoef.js, and read that file from the asset root.
// No CEF headers, so this file can be compiled on its own.
// The renderer evals the script in OnContextCreated. The switch names match
// the flags runtime.App adds to the browser command line.
#ifndef XGOEF_BRIDGE_H_
#define XGOEF_BRIDGE_H_

#include <string>

namespace xgoef {

inline constexpr const char kAssetRootSwitch[] = "xgoef-asset-root";
inline constexpr const char kNoBridgeSwitch[] = "xgoef-no-bridge";
inline constexpr const char kRemoteSwitch[] = "xgoef-remote";
inline constexpr int kBridgeScriptMaxBytes = 1024 * 1024;

// Origin of a hierarchical URL, with the default port removed.
// Empty when the URL has no host (about, data, chrome, and similar).
// file URLs share the origin "file://".
std::string OriginOf(const std::string& url);

// True when url is http(s) and its origin is one comma-separated entry of
// allowlist. allowlist entries are canonical origins (OriginOf form).
// An empty entry, a space, or a byte below 0x20 rejects the whole list.
bool RemoteOriginAllowed(const std::string& url, const std::string& allowlist);

// True when an http(s) navigation must be cancelled.
// Other schemes are left to the existing navigation rules.
bool RemoteNavigationBlocked(const std::string& url, const std::string& allowlist);

// True when this frame should run the bridge script.
// disabled is --xgoef-no-bridge. is_main is CefFrame::IsMain().
// allowlist is --xgoef-remote. An http(s) frame is included only when its
// origin is in that list. A main frame of another scheme is included unless
// it is an internal browser page. A subframe is included only when its
// origin equals the main frame origin.
bool ShouldInjectBridge(bool disabled,
                        bool is_main,
                        const std::string& url,
                        const std::string& main_url,
                        const std::string& allowlist);

// Absolute or relative path of the bridge file inside the asset root.
// Empty when root is empty or contains a NUL.
std::string BridgeScriptPath(const std::string& asset_root);

// Read a bridge script. On success out is the UTF-8 source with a leading
// BOM removed. On failure err is one of: path, missing, notfile, large,
// empty, read, nul.
bool ReadBridgeScript(const std::string& path, std::string* out, std::string* err);

}  // namespace xgoef

#endif  // XGOEF_BRIDGE_H_
