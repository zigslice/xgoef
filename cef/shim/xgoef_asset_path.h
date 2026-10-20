#ifndef XGOEF_ASSET_PATH_H_
#define XGOEF_ASSET_PATH_H_

#include <string>

namespace xgoef {

// Map xgoef://app/<path> onto a file inside root.
// On failure returns false and sets err to "outside", "missing", "scheme",
// "decode", or "root". out_path is absolute and symlink-resolved.
bool ResolveAssetPath(const std::string& root,
                      const std::string& url,
                      std::string* out_path,
                      std::string* err);

}  // namespace xgoef

#endif  // XGOEF_ASSET_PATH_H_
