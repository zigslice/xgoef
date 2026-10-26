// Script table and origin check. No CEF headers.
// A script may run only when the frame URL is still on the pinned origin.
#ifndef XGOEF_EVAL_H_
#define XGOEF_EVAL_H_

#include <string>
#include <vector>

namespace xgoef {

bool ScriptAccepted(const std::string& source);
bool ParseScriptTable(const std::string& text, std::vector<std::string>* out, std::string* err);
std::string FormatScriptTable(const std::vector<std::string>& scripts);

// frame and allowed are absolute URLs. Paths are ignored.
// xgoef://app and http(s) origins match. Every other scheme is denied.
bool OriginAllowed(const std::string& frame, const std::string& allowed);

// Replace the init scripts for a label. An empty list clears it.
// allowed is the window's pinned URL. The caller holds no CEF lock.
void EvalSetInit(const std::string& label, const std::string& allowed,
                 const std::vector<std::string>& scripts);
bool EvalLookup(const std::string& label, std::string* allowed, std::vector<std::string>* scripts);
void EvalReset();

}  // namespace xgoef

#ifdef __cplusplus
extern "C" {
#endif

/* Replace init scripts for label. text is FormatScriptTable.
 * Empty text clears the label. Returns 0, or -1 when the table or the
 * pinned URL is rejected. A rejection leaves the previous scripts in place. */
int xgoef_eval_set_init(const char* label, const char* allowed_url, const char* text);

/* Run one trusted script on the main frame. 0 ran, -1 no browser or timeout,
 * -2 origin denied (nothing ran), -3 script rejected (nothing ran). */
int xgoef_eval_run(const char* label, const char* allowed_url, const char* script);

#ifdef __cplusplus
}
#endif

#endif  // XGOEF_EVAL_H_
