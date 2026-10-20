#include "xgoef_asset_path.h"

#include <sys/stat.h>
#include <unistd.h>

#include <cstdlib>
#include <fstream>
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

std::string TempDir() {
  char buf[] = "/tmp/xgoef-asset-XXXXXX";
  char* dir = mkdtemp(buf);
  if (!dir) {
    std::perror("mkdtemp");
    std::exit(1);
  }
  return dir;
}

void Write(const std::string& path, const std::string& body) {
  std::ofstream out(path);
  out << body;
}

}  // namespace

int main() {
  std::string base = TempDir();
  std::string web = base + "/web";
  mkdir(web.c_str(), 0755);
  mkdir((web + "/css").c_str(), 0755);
  Write(web + "/index.html", "hi");
  Write(web + "/css/app.css", "x");
  Write(base + "/secret.txt", "no");
  if (symlink((base + "/secret.txt").c_str(), (web + "/link.txt").c_str()) != 0) {
    std::perror("symlink");
    return 1;
  }

  std::string out, err;
  Expect(xgoef::ResolveAssetPath(web, "xgoef://app/index.html", &out, &err), "index");
  Expect(out.size() >= 10 && out.find("/index.html") != std::string::npos, out);
  Expect(xgoef::ResolveAssetPath(web, "xgoef://app/", &out, &err), "slash");
  Expect(xgoef::ResolveAssetPath(web, "xgoef://app/css/../index.html?x=1", &out, &err),
         "clean");
  Expect(!xgoef::ResolveAssetPath(web, "xgoef://app/link.txt", &out, &err) && err == "outside",
         "symlink " + err);
  Expect(!xgoef::ResolveAssetPath(web, "xgoef://other/index.html", &out, &err), "host");
  Expect(!xgoef::ResolveAssetPath(web, "xgoef://app/../../secret.txt", &out, &err) &&
             err == "missing",
         "dots " + err);
  Expect(!xgoef::ResolveAssetPath(web, "file:///etc/passwd", &out, &err), "file");

  if (fails) {
    std::cerr << fails << " failed\n";
    return 1;
  }
  std::cout << "ok\n";
  return 0;
}
