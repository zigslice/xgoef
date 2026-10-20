// C ABI for the app menu and tray. The flat text is menu.Menu.Encode.
#ifndef XGOEF_MENU_H_
#define XGOEF_MENU_H_

#ifdef __cplusplus
extern "C" {
#endif

typedef void (*xgoef_menu_click_fn)(const char* window_label,
                                    const char* kind,
                                    const char* item_id);

void xgoef_menu_set(const char* encoded);
void xgoef_tray_set(const char* tooltip, const char* icon, const char* encoded);
int xgoef_menu_popup(const char* window_label);
int xgoef_tray_popup(const char* window_label);
void xgoef_menu_set_click(xgoef_menu_click_fn fn);

#ifdef __cplusplus
}
#endif

#endif  // XGOEF_MENU_H_
