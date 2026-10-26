// DevTools browser accounting. UI thread only.
// A browser created by ShowDevTools must not take a window label.
#ifndef XGOEF_WEBVIEW_H_
#define XGOEF_WEBVIEW_H_

namespace xgoef {

// Expect one OnAfterCreated from the next ShowDevTools.
void DevToolsExpect();
// Returns true once per unmatched Expect. The caller must not WindowBind.
bool DevToolsConsume();
// Drops one unmatched Expect when ShowDevTools did not create a browser.
void DevToolsCancel();
void DevToolsNote(int id);
void DevToolsForget(int id);
bool DevToolsIs(int id);
// Clears the expect count and the id set. Tests use it.
void DevToolsReset();

}  // namespace xgoef

#endif  // XGOEF_WEBVIEW_H_
