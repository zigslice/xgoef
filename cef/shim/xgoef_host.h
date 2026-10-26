/* xgoef_host.h — C ABI surface for the Go/LLGo Host implementation.
 *
 * Lifecycle mirrors cefsimple (C++) and docs/05-lifecycle.md.
 * Startup URL is taken from argv `--url=` (same as cefsimple).
 * IPC uses CefMessageRouter; see xgoef_ipc_*.
 */
#ifndef XGOEF_HOST_H_
#define XGOEF_HOST_H_

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Call before any other CEF function (Linux/Windows). */
void xgoef_cef_api_hash(void);

/* Returns >=0 when this process is a CEF helper and should exit with that code.
 * Returns -1 when this is the browser process. */
int xgoef_cef_execute_process(int argc, char** argv);

/* Initialize browser process. root_cache_path / subprocess_path may be NULL.
 * no_sandbox non-zero disables the sandbox (learning builds).
 * When subprocess_path is set, CEF helpers use that native binary instead of
 * re-entering the Go host executable (required for reliable GPU/renderer).
 * Returns 0 on success, non-zero on failure (see cef_get_exit_code). */
int xgoef_cef_initialize(int argc,
                         char** argv,
                         const char* root_cache_path,
                         int no_sandbox,
                         const char* subprocess_path);

void xgoef_cef_run_loop(void);
void xgoef_cef_shutdown(void);

/* ---- IPC (CefMessageRouter bridge) ---- */

/* Called on the CEF UI thread when the page issues cefQuery with a JSON
 * request body. |window_label| is the bound browser label (default "main").
 * The host must eventually call xgoef_ipc_respond. */
typedef void (*xgoef_ipc_query_fn)(int64_t query_id,
                                   const char* window_label,
                                   const char* request_utf8);

/* Register the Go/LLGo invoke handler. Pass NULL to clear. */
void xgoef_ipc_set_handler(xgoef_ipc_query_fn fn);

/* Called on the CEF UI thread after a query is dropped (navigation or
 * cefQueryCancel). The pending callback is already gone. */
typedef void (*xgoef_ipc_cancel_fn)(int64_t query_id);
void xgoef_ipc_set_cancel_handler(xgoef_ipc_cancel_fn fn);

/* Complete a query. ok!=0 → Success(result_utf8); else Failure(-2, result_utf8).
 * Safe to call from any thread (posted to UI if needed). */
void xgoef_ipc_respond(int64_t query_id, int ok, const char* result_utf8);

/* Host → renderer events: |json_utf8| is a full event envelope, e.g.
 * {"type":"event","event":"tick","payload":{...}} */
int xgoef_ipc_emit_to(const char* window_label, const char* json_utf8);
int xgoef_ipc_emit_all(const char* json_utf8);

/* Window label registry (P1). Pending label binds the next OnAfterCreated. */
void xgoef_window_set_pending_label(const char* label);
/* chrome is decorations,theme,transparent from cef.WindowChrome.Encode.
 * NULL or "" is decorated, system theme, opaque. A bad string is ignored. */
int xgoef_window_create(const char* label,
                        const char* title,
                        const char* url,
                        int width,
                        int height,
                        const char* chrome);
int xgoef_window_close(const char* label);
int xgoef_window_focus(const char* label);
/* Verb for the Views window bound to |label|. Codes are xgoef::kWindowOp*.
 * Returns 0, -1 when that label has no Views window or the UI thread does
 * not answer, or -2 when the verb, title, or size is rejected. */
int xgoef_window_op(const char* label, int op, int a, int b, const char* text);
/* 1, 0, or -1 when that label has no Views window or the UI thread does
 * not answer. Query codes are xgoef::kWindowQuery*. */
int xgoef_window_query(const char* label, int query);
/* Store resize limits before the window exists. |text| is the eight-field
 * form from cef.WindowLimits.Encode. Returns 0, or -2 when rejected. */
int xgoef_window_limits_seed(const char* label, const char* text);
/* Change one limit field on a live Views window. Field codes are
 * xgoef::kWindowLimit*. Returns 0, -1 when that label has no Views window,
 * or -2 when the field or size is rejected. */
int xgoef_window_limit(const char* label, int field, int a, int b);
/* (a << 16) | b on success. -1 when that label has no Views window.
 * -2 when the field is unknown. */
int xgoef_window_limit_get(const char* label, int field);

/* Monitor table for |window_label|. The text is xgoef::FormatDisplayTable.
 * NULL when CEF does not answer. Caller frees with xgoef_displays_free. */
char* xgoef_displays(const char* window_label);
void xgoef_displays_free(char* text);
/* Move a Views window's origin to DIP x, y. Returns 0, or -1 when that
 * label has no Views window. */
int xgoef_window_move(const char* label, int x, int y);

/* System print dialog for |label|. Returns 0, or -1 when that label has no
 * browser or the UI thread does not answer. */
int xgoef_window_print(const char* label);
/* Open or close developer tools. Already open / already closed is 0.
 * Returns -1 when that label has no browser or the UI thread does not answer. */
int xgoef_devtools_open(const char* label);
int xgoef_devtools_close(const char* label);
/* 1 open, 0 closed, -1 when that label has no browser or the UI thread
 * does not answer. */
int xgoef_devtools_is_open(const char* label);

/* Replace init scripts for |label|. |text| is the length-prefixed table.
 * |allowed_url| is the origin those scripts may run on. Empty text clears.
 * Returns 0, or -1 when the table or the URL is rejected. */
int xgoef_eval_set_init(const char* label, const char* allowed_url, const char* text);
/* Run one trusted script now. 0 ran, -1 unavailable, -2 origin denied,
 * -3 script rejected. -2 and -3 do not call ExecuteJavaScript. */
int xgoef_eval_run(const char* label, const char* allowed_url, const char* script);

#ifdef __cplusplus
}
#endif

#endif /* XGOEF_HOST_H_ */
