// X11 taskbar hints. No CEF headers.
#include "xgoef_taskbar.h"

#include <X11/Xatom.h>
#include <X11/Xlib.h>

#include <cstring>

namespace xgoef {

void SendSkipTaskbar(Display* display, Window window, bool skip) {
  if (display == nullptr || window == 0) {
    return;
  }
  const SkipTaskbarMessage msg = DescribeSkip(skip);
  const Atom type = XInternAtom(display, msg.type, False);
  const Atom atom = XInternAtom(display, msg.atom, False);
  XEvent ev;
  std::memset(&ev, 0, sizeof(ev));
  ev.type = ClientMessage;
  ev.xclient.window = window;
  ev.xclient.message_type = type;
  ev.xclient.format = 32;
  ev.xclient.data.l[0] = msg.action;
  ev.xclient.data.l[1] = static_cast<long>(atom);
  ev.xclient.data.l[2] = 0;
  ev.xclient.data.l[3] = msg.source;
  ev.xclient.data.l[4] = 0;
  XSendEvent(display, XDefaultRootWindow(display), False,
             SubstructureRedirectMask | SubstructureNotifyMask, &ev);
  XFlush(display);
}

void SetTaskbarProgress(Display* display, Window window, int progress) {
  if (display == nullptr || window == 0) {
    return;
  }
  ProgressProperty prop;
  if (!DescribeProgress(progress, &prop)) {
    return;
  }
  const Atom name = XInternAtom(display, prop.name, False);
  if (prop.remove) {
    XDeleteProperty(display, window, name);
  } else {
    unsigned long value = prop.value;
    XChangeProperty(display, window, name, XA_CARDINAL, 32, PropModeReplace,
                    reinterpret_cast<unsigned char*>(&value), 1);
  }
  XFlush(display);
}

}  // namespace xgoef
