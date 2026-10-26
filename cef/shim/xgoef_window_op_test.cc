#include "xgoef_window_op.h"

#include <iostream>
#include <string>

namespace {

int Fail(const char* message) {
  std::cerr << message << "\n";
  return 1;
}

}  // namespace

int main() {
  static_assert(xgoef::kWindowOpMinimize == 1, "minimize");
  static_assert(xgoef::kWindowOpMaximize == 2, "maximize");
  static_assert(xgoef::kWindowOpUnminimize == 3, "unminimize");
  static_assert(xgoef::kWindowOpUnmaximize == 4, "unmaximize");
  static_assert(xgoef::kWindowOpToggleMaximize == 5, "toggle");
  static_assert(xgoef::kWindowOpShow == 6, "show");
  static_assert(xgoef::kWindowOpHide == 7, "hide");
  static_assert(xgoef::kWindowOpCenter == 8, "center");
  static_assert(xgoef::kWindowOpSetTitle == 9, "title");
  static_assert(xgoef::kWindowOpSetAlwaysOnTop == 10, "top");
  static_assert(xgoef::kWindowOpSetFullscreen == 11, "full");
  static_assert(xgoef::kWindowOpSetSize == 12, "size");
  static_assert(xgoef::kWindowQueryMinimized == 1, "qmin");
  static_assert(xgoef::kWindowQueryMaximized == 2, "qmax");
  static_assert(xgoef::kWindowQueryFullscreen == 3, "qfull");
  static_assert(xgoef::kWindowQueryAlwaysOnTop == 4, "qtop");
  static_assert(xgoef::kWindowQueryVisible == 5, "qvis");
  static_assert(xgoef::kWindowTitleMax == 256, "title max");
  static_assert(xgoef::kWindowSizeMin == 1, "size min");
  static_assert(xgoef::kWindowSizeMax == 16384, "size max");

  if (!xgoef::TitleOK("") || !xgoef::TitleOK("hello") || !xgoef::TitleOK("窗口")) {
    return Fail("title accepted");
  }
  if (!xgoef::TitleOK(std::string(256, 'a'))) {
    return Fail("256");
  }
  if (xgoef::TitleOK(std::string(257, 'a'))) {
    return Fail("257");
  }
  if (xgoef::TitleOK("a\nb") || xgoef::TitleOK("a\tb") || xgoef::TitleOK(std::string("a\0b", 3))) {
    return Fail("control");
  }
  if (!xgoef::TitleOK(std::string(1, '\x7f'))) {
    return Fail("del");
  }
  if (xgoef::TitleOK("\xff") || xgoef::TitleOK("\xc0\x80") || xgoef::TitleOK("\xed\xa0\x80")) {
    return Fail("bad utf-8");
  }
  if (!xgoef::TitleOK("\xf0\x9f\x98\x80")) {
    return Fail("emoji");
  }
  if (!xgoef::SizeOK(1, 1) || !xgoef::SizeOK(16384, 16384) || !xgoef::SizeOK(800, 600)) {
    return Fail("size ok");
  }
  if (xgoef::SizeOK(0, 1) || xgoef::SizeOK(1, 0) || xgoef::SizeOK(16385, 1) ||
      xgoef::SizeOK(1, -1)) {
    return Fail("size bad");
  }
  return 0;
}
