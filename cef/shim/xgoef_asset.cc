// CEF scheme handler for xgoef://app/. The browser process registers the
// factory; every process must still register the scheme name (see App).

#include "xgoef_asset_path.h"

#include "include/cef_parser.h"
#include "include/cef_scheme.h"
#include "include/wrapper/cef_stream_resource_handler.h"

namespace xgoef {
namespace {

class AssetFactory : public CefSchemeHandlerFactory {
 public:
  explicit AssetFactory(std::string root) : root_(std::move(root)) {}

  CefRefPtr<CefResourceHandler> Create(CefRefPtr<CefBrowser> browser,
                                       CefRefPtr<CefFrame> frame,
                                       const CefString& scheme_name,
                                       CefRefPtr<CefRequest> request) override {
    (void)browser;
    (void)frame;
    (void)scheme_name;
    std::string path, err;
    if (!ResolveAssetPath(root_, request->GetURL().ToString(), &path, &err)) {
      return nullptr;
    }
    CefRefPtr<CefStreamReader> stream = CefStreamReader::CreateForFile(path);
    if (!stream) {
      return nullptr;
    }
    std::string ext;
    auto dot = path.find_last_of('.');
    if (dot != std::string::npos) {
      ext = path.substr(dot + 1);
    }
    CefString mime = CefGetMimeType(ext);
    if (mime.empty()) {
      mime = "application/octet-stream";
    }
    return new CefStreamResourceHandler(mime, stream);
  }

 private:
  std::string root_;
  IMPLEMENT_REFCOUNTING(AssetFactory);
};

}  // namespace

void RegisterAssetScheme(const std::string& root) {
  if (root.empty()) {
    return;
  }
  CefRegisterSchemeHandlerFactory("xgoef", "app", new AssetFactory(root));
}

}  // namespace xgoef
