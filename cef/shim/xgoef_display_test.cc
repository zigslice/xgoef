#include "xgoef_display.h"

#include <iostream>
#include <string>

static int fail(const char* why) {
  std::cerr << why << "\n";
  return 1;
}

int main() {
  xgoef::DisplayTable table;
  table.has_window = true;
  table.window_id = 10;
  table.window_scale = 1.5;
  xgoef::DisplayInfo primary;
  primary.id = 10;
  primary.scale = 1.5;
  primary.width = 1280;
  primary.height = 800;
  primary.work_y = 32;
  primary.work_width = 1280;
  primary.work_height = 768;
  primary.pixel_width = 1920;
  primary.pixel_height = 1200;
  primary.work_pixel_y = 48;
  primary.work_pixel_width = 1920;
  primary.work_pixel_height = 1152;
  primary.primary = true;
  xgoef::DisplayInfo side;
  side.id = 11;
  side.scale = 1;
  side.x = 1280;
  side.width = 1920;
  side.height = 1080;
  side.work_x = 1280;
  side.work_width = 1920;
  side.work_height = 1040;
  side.pixel_x = 1280;
  side.pixel_width = 1920;
  side.pixel_height = 1080;
  side.work_pixel_x = 1280;
  side.work_pixel_width = 1920;
  side.work_pixel_height = 1040;
  side.rotation = 90;
  table.displays.push_back(primary);
  table.displays.push_back(side);

  std::string text = xgoef::FormatDisplayTable(table);
  xgoef::DisplayTable got;
  std::string err;
  if (!xgoef::ParseDisplayTable(text, &got, &err) || !got.has_window || got.window_id != 10 ||
      got.window_scale != 1.5 || got.displays.size() != 2 || !got.displays[0].primary ||
      got.displays[1].rotation != 90 || got.displays[1].x != 1280) {
    return fail("round trip");
  }
  if (xgoef::FormatDisplayTable(got) != text) {
    return fail("stable");
  }

  xgoef::DisplayTable none;
  text = xgoef::FormatDisplayTable(none);
  if (text != "window - 0.000000\n" || !xgoef::ParseDisplayTable(text, &got, &err) ||
      got.has_window || !got.displays.empty()) {
    return fail("empty list");
  }
  if (xgoef::ParseDisplayTable("", &got, &err) || err.empty()) {
    return fail("blank");
  }
  if (xgoef::ParseDisplayTable(
          "window 1 1.000000\n"
          "1 1.000000 0 0 1 1 0 0 1 1 0 0 1 1 0 0 1 1 45 1\n",
          &got, &err)) {
    return fail("rotation");
  }
  if (xgoef::ParseDisplayTable(
          "window 1 1.000000\n"
          "1 1.000000 0 0 1 1 0 0 1 1 0 0 1 1 0 0 1 1 0 1\n"
          "2 1.000000 2 0 1 1 2 0 1 1 2 0 1 1 2 0 1 1 0 1\n",
          &got, &err)) {
    return fail("two primaries");
  }
  std::cout << "ok\n";
  return 0;
}
