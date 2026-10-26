// UI-thread entry for init scripts. The decision to run is in xgoef_eval.h.
#ifndef XGOEF_EVAL_RUN_H_
#define XGOEF_EVAL_RUN_H_

#include <string>

#include "include/cef_frame.h"

namespace xgoef {

void EvalRunInit(const std::string& label, CefRefPtr<CefFrame> frame);

}  // namespace xgoef

#endif  // XGOEF_EVAL_RUN_H_
