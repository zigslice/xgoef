// Display table shared by the CEF query and the Go parser.
// No CEF headers. One header line, then one line per display.
//
//   window <id|-> <scale>
//   <id> <scale> <x> <y> <width> <height> <workX> <workY> <workWidth> <workHeight>
//       <pixelX> <pixelY> <pixelWidth> <pixelHeight>
//       <workPixelX> <workPixelY> <workPixelWidth> <workPixelHeight>
//       <rotation> <primary>
//
// Positions and sizes before the pixel fields are DIP. Scale is six digits
// after the decimal. rotation is 0, 90, 180, or 270. Exactly one primary
// when the list is not empty.
#ifndef XGOEF_DISPLAY_H_
#define XGOEF_DISPLAY_H_

#include <cstdint>
#include <string>
#include <vector>

namespace xgoef {

struct DisplayInfo {
  int64_t id = 0;
  double scale = 1;
  int x = 0;
  int y = 0;
  int width = 0;
  int height = 0;
  int work_x = 0;
  int work_y = 0;
  int work_width = 0;
  int work_height = 0;
  int pixel_x = 0;
  int pixel_y = 0;
  int pixel_width = 0;
  int pixel_height = 0;
  int work_pixel_x = 0;
  int work_pixel_y = 0;
  int work_pixel_width = 0;
  int work_pixel_height = 0;
  int rotation = 0;
  bool primary = false;
};

struct DisplayTable {
  bool has_window = false;
  int64_t window_id = 0;
  double window_scale = 0;
  std::vector<DisplayInfo> displays;
};

std::string FormatDisplayTable(const DisplayTable& table);
bool ParseDisplayTable(const std::string& text, DisplayTable* out, std::string* err);

}  // namespace xgoef

#endif  // XGOEF_DISPLAY_H_
