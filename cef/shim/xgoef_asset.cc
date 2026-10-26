// CEF scheme handler for xgoef://app/. The browser process registers the
// factory; every process must still register the scheme name (see App).

#include "xgoef_asset_path.h"
#include "xgoef_csp.h"

#include "include/cef_parser.h"
#include "include/cef_scheme.h"
#include "include/wrapper/cef_stream_resource_handler.h"

namespace xgoef {
namespace {

class AssetFactory : public CefSchemeHandlerFactory {
 public:
  AssetFactory(std::string root, std::string csp)
      : root_(std::move(root)), csp_(std::move(csp)) {}

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
    if (csp_.empty()) {
      return new CefStreamResourceHandler(mime, stream);
    }
    CefResponse::HeaderMap headers;
    headers.insert(std::make_pair(CefString("Content-Security-Policy"), CefString(csp_)));
    return new CefStreamResourceHandler(200, "OK", mime, headers, stream);
  }

 private:
  std::string root_;
  std::string csp_;
  IMPLEMENT_REFCOUNTING(AssetFactory);
};

}  // namespace

void RegisterAssetScheme(const std::string& root, bool csp_disabled, const std::string& csp) {
  if (root.empty()) {
    return;
  }
  std::string header;
  CSPHeaderValue(csp_disabled, csp, &header);
  CefRegisterSchemeHandlerFactory("xgoef", "app", new AssetFactory(root, header));
}

}  // namespace xgoef
