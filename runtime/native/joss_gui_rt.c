//go:build ignore
/*
 * joss_gui_rt.c — Joss Native Desktop GUI Runtime (C11, zero external DLLs)
 *
 * Platforms:
 *   - Windows: Win32 + DWM Dark Mode / Mica + PerMonitorV2 HiDPI + WebView2 COM
 *              (locates system Edge WebView2 Runtime via Registry — zero WebView2Loader.dll)
 *   - macOS:   Cocoa NSWindow + WKWebView via Objective-C runtime (<objc/message.h>)
 *   - Linux:   GTK3 + WebKitGTK via dlopen (zero compile-time GTK headers, automatic headless fallback)
 *   - Headless/CI: deterministic full-lifecycle mode when JOSS_GUI_HEADLESS=1 or no display server
 */

#if defined(__APPLE__) && !defined(_DARWIN_C_SOURCE)
#define _DARWIN_C_SOURCE
#elif defined(__unix__)
#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif
#ifndef _POSIX_C_SOURCE
#define _POSIX_C_SOURCE 200809L
#endif
#endif

#include "joss_rt.h"

#include <ctype.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#if defined(_WIN32)
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#ifndef UNICODE
#define UNICODE
#endif
#include <windows.h>
#include <dwmapi.h>
#include <objbase.h>
#elif defined(__APPLE__)
#include <objc/message.h>
#include <objc/runtime.h>
#else
#include <dlfcn.h>
#endif

extern const char* joss_strval(int64_t v);
extern int64_t     joss_is_null(int64_t v);
extern int64_t     joss_boolval(int64_t v);

#define JOSS_WINDOW_MAGIC        0x4A4F535357494E44ULL
#define JOSS_WINDOW_MAX_BINDINGS 128
#define JOSS_WINDOW_MAX_HANDLERS 64

typedef struct {
    char*   name;
    int64_t callable;
} JossWinCallback;

typedef struct JossWindow {
    uint64_t        magic;
    char*           title;
    int64_t         width;
    int64_t         height;
    int64_t         min_width;
    int64_t         min_height;
    bool            visible;
    bool            resizable;
    bool            frameless;
    bool            always_on_top;
    bool            maximized;
    bool            minimized;
    bool            headless;
    char*           theme; /* "system", "dark", "light" */
    char*           url;
    char*           html;
    JossWinCallback bindings[JOSS_WINDOW_MAX_BINDINGS];
    int             binding_count;
    JossWinCallback handlers[JOSS_WINDOW_MAX_HANDLERS];
    int             handler_count;
#if defined(_WIN32)
    HWND            hwnd;
    void*           webview_env;
    void*           webview_controller;
    void*           webview;
    void*           wv2_env_handler;
    void*           wv2_ctrl_handler;
    HMODULE         wv2_dll;
#elif defined(__APPLE__)
    void*           ns_window;
    void*           wk_webview;
#else
    void*           gtk_window;
    void*           wk_webview;
    void*           gtk_lib;
    void*           wk_lib;
#endif
} JossWindow;

static char* joss_gui_strdup(const char* s) {
    if (!s) s = "";
    size_t n = strlen(s);
    char* out = (char*)malloc(n + 1U);
    if (!out) return NULL;
    memcpy(out, s, n + 1U);
    return out;
}

static const char* joss_gui_cstr(const void* p) {
    if (!p) return "";
    int64_t v = (int64_t)(uintptr_t)p;
    if (v == 0 || joss_is_null(v)) return "";
    if (joss_is_boxed_ptr(v)) {
        const char* s = joss_strval(v);
        return s ? s : "";
    }
    uintptr_t u = (uintptr_t)p;
    if ((uint64_t)u < JOSS_MIN_PTR_ADDR || ((uint64_t)u >> 47) != 0) {
        const char* s = joss_strval(v);
        return s ? s : "";
    }
    return (const char*)p;
}

static JossWindow* joss_as_window(void* p) {
    int64_t u = joss_unbox_value((int64_t)(uintptr_t)p);
    if (u == 0 || (uint64_t)u < JOSS_MIN_PTR_ADDR || ((uint64_t)u >> 47) != 0) return NULL;
    JossWindow* w = (JossWindow*)(uintptr_t)u;
    return (w->magic == JOSS_WINDOW_MAGIC) ? w : NULL;
}

static bool joss_gui_is_headless(void) {
    const char* hl = getenv("JOSS_GUI_HEADLESS");
    if (hl && (strcmp(hl, "1") == 0 || strcmp(hl, "true") == 0 || strcmp(hl, "TRUE") == 0)) {
        return true;
    }
#if !defined(_WIN32) && !defined(__APPLE__)
    const char* disp = getenv("DISPLAY");
    const char* wayland = getenv("WAYLAND_DISPLAY");
    if ((!disp || !*disp) && (!wayland || !*wayland)) {
        return true;
    }
#endif
    return false;
}

static const char* joss_gui_invoke_binding(JossWindow* w, const char* name, const char* payload) {
    if (!w || !name || !*name) return "";
    for (int i = 0; i < w->binding_count; i++) {
        if (w->bindings[i].name && strcmp(w->bindings[i].name, name) == 0) {
            void* fn_raw = joss_closure_prepare_call((void*)(uintptr_t)w->bindings[i].callable);
            if (!fn_raw) return "";
            typedef const char* (*JossIPCFn)(const char*);
            JossIPCFn fn = (JossIPCFn)fn_raw;
            const char* res = fn(payload ? payload : "");
            return joss_gui_cstr(res);
        }
    }
    return "";
}

static void joss_gui_emit_event(JossWindow* w, const char* evt) {
    if (!w || !evt || !*evt) return;
    for (int i = 0; i < w->handler_count; i++) {
        if (w->handlers[i].name && strcmp(w->handlers[i].name, evt) == 0) {
            void* fn_raw = joss_closure_prepare_call((void*)(uintptr_t)w->handlers[i].callable);
            if (!fn_raw) return;
            typedef void (*JossEvtFn)(void);
            JossEvtFn fn = (JossEvtFn)fn_raw;
            fn();
            return;
        }
    }
}

/* =========================================================================
 * Platform Backend: Windows (Win32 + DWM Dark/Mica + HiDPI + WebView2 COM)
 * ========================================================================= */
#if defined(_WIN32)

/* Minimal WebView2 COM vtable definitions (zero external WebView2Loader.dll) */
typedef struct JossWV2CoreVtbl {
    HRESULT (STDMETHODCALLTYPE *QueryInterface)(void*, const GUID*, void**);
    ULONG   (STDMETHODCALLTYPE *AddRef)(void*);
    ULONG   (STDMETHODCALLTYPE *Release)(void*);
    HRESULT (STDMETHODCALLTYPE *GetSettings)(void*, void**);
    HRESULT (STDMETHODCALLTYPE *GetSource)(void*, LPWSTR*);
    HRESULT (STDMETHODCALLTYPE *Navigate)(void*, LPCWSTR);
    HRESULT (STDMETHODCALLTYPE *NavigateToString)(void*, LPCWSTR);
    void*   _pad7_to_26[20];
    HRESULT (STDMETHODCALLTYPE *AddScriptToExecuteOnDocumentCreated)(void*, LPCWSTR, void*);
    void*   _pad28;
    HRESULT (STDMETHODCALLTYPE *ExecuteScript)(void*, LPCWSTR, void*);
} JossWV2CoreVtbl;

typedef struct {
    JossWV2CoreVtbl* lpVtbl;
} JossWV2Core;

typedef struct JossWV2ControllerVtbl {
    HRESULT (STDMETHODCALLTYPE *QueryInterface)(void*, const GUID*, void**);
    ULONG   (STDMETHODCALLTYPE *AddRef)(void*);
    ULONG   (STDMETHODCALLTYPE *Release)(void*);
    HRESULT (STDMETHODCALLTYPE *GetIsVisible)(void*, BOOL*);
    HRESULT (STDMETHODCALLTYPE *PutIsVisible)(void*, BOOL);
    HRESULT (STDMETHODCALLTYPE *GetBounds)(void*, RECT*);
    HRESULT (STDMETHODCALLTYPE *PutBounds)(void*, RECT);
    void*   _pad7_to_22[16];
    HRESULT (STDMETHODCALLTYPE *NotifyParentWindowPositionChanged)(void*);
    HRESULT (STDMETHODCALLTYPE *Close)(void*);
    HRESULT (STDMETHODCALLTYPE *GetCoreWebView2)(void*, JossWV2Core**);
} JossWV2ControllerVtbl;

typedef struct {
    JossWV2ControllerVtbl* lpVtbl;
} JossWV2Controller;

typedef struct JossWV2EnvironmentVtbl {
    HRESULT (STDMETHODCALLTYPE *QueryInterface)(void*, const GUID*, void**);
    ULONG   (STDMETHODCALLTYPE *AddRef)(void*);
    ULONG   (STDMETHODCALLTYPE *Release)(void*);
    HRESULT (STDMETHODCALLTYPE *CreateCoreWebView2Controller)(void*, HWND, void*);
} JossWV2EnvironmentVtbl;

typedef struct {
    JossWV2EnvironmentVtbl* lpVtbl;
} JossWV2Environment;

typedef struct JossWV2CompletedHandlerVtbl {
    HRESULT (STDMETHODCALLTYPE *QueryInterface)(void*, const GUID*, void**);
    ULONG   (STDMETHODCALLTYPE *AddRef)(void*);
    ULONG   (STDMETHODCALLTYPE *Release)(void*);
    HRESULT (STDMETHODCALLTYPE *Invoke)(void*, HRESULT, void*);
} JossWV2CompletedHandlerVtbl;

typedef struct {
    JossWV2CompletedHandlerVtbl* lpVtbl;
    JossWindow*                  win;
    volatile LONG                ref_count;
} JossWV2Handler;

static wchar_t* joss_win_utf8_to_wide(const char* s) {
    if (!s) s = "";
    int needed = MultiByteToWideChar(CP_UTF8, 0, s, -1, NULL, 0);
    if (needed <= 0) needed = 1;
    wchar_t* out = (wchar_t*)calloc((size_t)needed, sizeof(wchar_t));
    if (!out) return NULL;
    MultiByteToWideChar(CP_UTF8, 0, s, -1, out, needed);
    return out;
}

static void joss_win_wv2_resize(JossWindow* w) {
    if (!w || !w->hwnd || !w->webview_controller) return;
    RECT rc;
    if (GetClientRect(w->hwnd, &rc)) {
        JossWV2Controller* ctrl = (JossWV2Controller*)w->webview_controller;
        ctrl->lpVtbl->PutBounds(ctrl, rc);
    }
}

static void joss_win_wv2_navigate(JossWindow* w, const char* url) {
    if (!w || !w->webview || !url || !*url) return;
    wchar_t* wurl = joss_win_utf8_to_wide(url);
    if (!wurl) return;
    JossWV2Core* wv = (JossWV2Core*)w->webview;
    wv->lpVtbl->Navigate(wv, wurl);
    free(wurl);
}

static void joss_win_wv2_set_html(JossWindow* w, const char* html) {
    if (!w || !w->webview || !html || !*html) return;
    wchar_t* whtml = joss_win_utf8_to_wide(html);
    if (!whtml) return;
    JossWV2Core* wv = (JossWV2Core*)w->webview;
    wv->lpVtbl->NavigateToString(wv, whtml);
    free(whtml);
}

static void joss_win_wv2_eval(JossWindow* w, const char* script) {
    if (!w || !w->webview || !script || !*script) return;
    wchar_t* wscript = joss_win_utf8_to_wide(script);
    if (!wscript) return;
    JossWV2Core* wv = (JossWV2Core*)w->webview;
    wv->lpVtbl->ExecuteScript(wv, wscript, NULL);
    free(wscript);
}

static HRESULT STDMETHODCALLTYPE joss_wv2_handler_qi(void* self, const GUID* riid, void** ppv) {
    (void)riid;
    if (!ppv) return E_POINTER;
    *ppv = self;
    JossWV2Handler* h = (JossWV2Handler*)self;
    InterlockedIncrement(&h->ref_count);
    return S_OK;
}

static ULONG STDMETHODCALLTYPE joss_wv2_handler_addref(void* self) {
    JossWV2Handler* h = (JossWV2Handler*)self;
    return (ULONG)InterlockedIncrement(&h->ref_count);
}

static ULONG STDMETHODCALLTYPE joss_wv2_handler_release(void* self) {
    JossWV2Handler* h = (JossWV2Handler*)self;
    LONG rc = InterlockedDecrement(&h->ref_count);
    if (rc <= 0) {
        free(h);
        return 0;
    }
    return (ULONG)rc;
}

static HRESULT STDMETHODCALLTYPE joss_wv2_ctrl_completed(void* self, HRESULT hr, void* ctrl_raw) {
    JossWV2Handler* h = (JossWV2Handler*)self;
    JossWindow* w = h ? h->win : NULL;
    if (FAILED(hr) || !w || !ctrl_raw) return S_OK;

    JossWV2Controller* ctrl = (JossWV2Controller*)ctrl_raw;
    ctrl->lpVtbl->AddRef(ctrl);
    w->webview_controller = ctrl;

    JossWV2Core* wv = NULL;
    if (SUCCEEDED(ctrl->lpVtbl->GetCoreWebView2(ctrl, &wv)) && wv) {
        wv->lpVtbl->AddRef(wv);
        w->webview = wv;
    }

    joss_win_wv2_resize(w);
    ctrl->lpVtbl->PutIsVisible(ctrl, w->visible ? TRUE : FALSE);

    if (w->url && w->url[0]) {
        joss_win_wv2_navigate(w, w->url);
    } else if (w->html && w->html[0]) {
        joss_win_wv2_set_html(w, w->html);
    }
    return S_OK;
}

static JossWV2CompletedHandlerVtbl g_joss_wv2_ctrl_vtbl = {
    joss_wv2_handler_qi,
    joss_wv2_handler_addref,
    joss_wv2_handler_release,
    joss_wv2_ctrl_completed
};

static HRESULT STDMETHODCALLTYPE joss_wv2_env_completed(void* self, HRESULT hr, void* env_raw) {
    JossWV2Handler* h = (JossWV2Handler*)self;
    JossWindow* w = h ? h->win : NULL;
    if (FAILED(hr) || !w || !env_raw || !w->hwnd) return S_OK;

    JossWV2Environment* env = (JossWV2Environment*)env_raw;
    env->lpVtbl->AddRef(env);
    w->webview_env = env;

    JossWV2Handler* ctrl_h = (JossWV2Handler*)calloc(1, sizeof(JossWV2Handler));
    if (!ctrl_h) return E_OUTOFMEMORY;
    ctrl_h->lpVtbl = &g_joss_wv2_ctrl_vtbl;
    ctrl_h->win = w;
    ctrl_h->ref_count = 1;
    w->wv2_ctrl_handler = ctrl_h;

    env->lpVtbl->CreateCoreWebView2Controller(env, w->hwnd, ctrl_h);
    return S_OK;
}

static JossWV2CompletedHandlerVtbl g_joss_wv2_env_vtbl = {
    joss_wv2_handler_qi,
    joss_wv2_handler_addref,
    joss_wv2_handler_release,
    joss_wv2_env_completed
};

static bool joss_win_locate_webview2_dll(wchar_t* out_path, size_t out_cap) {
    static const struct {
        HKEY           root;
        const wchar_t* subkey;
    } candidates[] = {
        { HKEY_LOCAL_MACHINE, L"SOFTWARE\\WOW6432Node\\Microsoft\\EdgeUpdate\\Clients\\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" },
        { HKEY_CURRENT_USER,  L"Software\\Microsoft\\EdgeUpdate\\Clients\\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" },
        { HKEY_LOCAL_MACHINE, L"SOFTWARE\\Microsoft\\EdgeUpdate\\Clients\\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" }
    };
#if defined(_M_ARM64) || defined(__aarch64__)
    const wchar_t* arch_dir = L"arm64";
#elif defined(_M_X64) || defined(__x86_64__)
    const wchar_t* arch_dir = L"x64";
#else
    const wchar_t* arch_dir = L"x86";
#endif

    for (size_t i = 0; i < sizeof(candidates) / sizeof(candidates[0]); i++) {
        HKEY hkey = NULL;
        if (RegOpenKeyExW(candidates[i].root, candidates[i].subkey, 0, KEY_READ, &hkey) != ERROR_SUCCESS) {
            continue;
        }
        wchar_t loc[MAX_PATH] = {0};
        wchar_t pv[128] = {0};
        DWORD loc_sz = (DWORD)sizeof(loc);
        DWORD pv_sz = (DWORD)sizeof(pv);
        LONG r1 = RegQueryValueExW(hkey, L"location", NULL, NULL, (LPBYTE)loc, &loc_sz);
        LONG r2 = RegQueryValueExW(hkey, L"pv", NULL, NULL, (LPBYTE)pv, &pv_sz);
        RegCloseKey(hkey);
        if (r1 == ERROR_SUCCESS && r2 == ERROR_SUCCESS && loc[0] && pv[0]) {
            swprintf(out_path, out_cap, L"%ls\\%ls\\EBWebView\\%ls\\EmbeddedBrowserWebView.dll", loc, pv, arch_dir);
            if (GetFileAttributesW(out_path) != INVALID_FILE_ATTRIBUTES) {
                return true;
            }
        }
    }
    return false;
}

static void joss_win_init_webview2(JossWindow* w) {
    if (!w || !w->hwnd || w->headless) return;

    wchar_t temp_dir[MAX_PATH] = {0};
    wchar_t user_data_dir[MAX_PATH] = {0};
    if (GetTempPathW(MAX_PATH, temp_dir) > 0) {
        swprintf(user_data_dir, MAX_PATH, L"%lsjoss_webview2", temp_dir);
        CreateDirectoryW(user_data_dir, NULL);
    }

    JossWV2Handler* env_h = (JossWV2Handler*)calloc(1, sizeof(JossWV2Handler));
    if (!env_h) return;
    env_h->lpVtbl = &g_joss_wv2_env_vtbl;
    env_h->win = w;
    env_h->ref_count = 1;
    w->wv2_env_handler = env_h;

    wchar_t dll_path[MAX_PATH * 2] = {0};
    if (joss_win_locate_webview2_dll(dll_path, sizeof(dll_path) / sizeof(dll_path[0]))) {
        HMODULE dll = LoadLibraryW(dll_path);
        if (dll) {
            typedef HRESULT (STDAPICALLTYPE *CreateInternalFn)(BOOL, int, LPCWSTR, void*, void*);
            CreateInternalFn fn = NULL;
            *(void**)(&fn) = (void*)(uintptr_t)GetProcAddress(dll, "CreateWebViewEnvironmentWithOptionsInternal");
            if (fn) {
                w->wv2_dll = dll;
                HRESULT hr = fn(TRUE, 0, user_data_dir[0] ? user_data_dir : NULL, NULL, env_h);
                if (SUCCEEDED(hr)) {
                    return;
                }
            }
        }
    }

    HMODULE loader = LoadLibraryW(L"WebView2Loader.dll");
    if (loader) {
        typedef HRESULT (STDAPICALLTYPE *CreateLoaderFn)(LPCWSTR, LPCWSTR, void*, void*);
        CreateLoaderFn fn = NULL;
        *(void**)(&fn) = (void*)(uintptr_t)GetProcAddress(loader, "CreateCoreWebView2EnvironmentWithOptions");
        if (fn) {
            w->wv2_dll = loader;
            (void)fn(NULL, user_data_dir[0] ? user_data_dir : NULL, NULL, env_h);
        }
    }
}

static void joss_win_apply_theme(JossWindow* w) {
    if (!w || !w->hwnd) return;
    BOOL dark = (w->theme && strcmp(w->theme, "dark") == 0) ? TRUE : FALSE;
    /* DWMWA_USE_IMMERSIVE_DARK_MODE = 20 */
    DwmSetWindowAttribute(w->hwnd, 20, &dark, sizeof(dark));
    /* DWMWA_SYSTEMBACKDROP_TYPE = 38, DWMSBT_MAINWINDOW (Mica) = 2 */
    int backdrop = 2;
    DwmSetWindowAttribute(w->hwnd, 38, &backdrop, sizeof(backdrop));
}

static LRESULT CALLBACK joss_win_wndproc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam) {
    JossWindow* w = (JossWindow*)(uintptr_t)GetWindowLongPtrW(hwnd, GWLP_USERDATA);
    switch (msg) {
        case WM_SIZE:
            if (w) {
                joss_win_wv2_resize(w);
            }
            return 0;
        case WM_MOVE:
        case WM_MOVING:
            if (w && w->webview_controller) {
                JossWV2Controller* ctrl = (JossWV2Controller*)w->webview_controller;
                ctrl->lpVtbl->NotifyParentWindowPositionChanged(ctrl);
            }
            break;
        case WM_GETMINMAXINFO:
            if (w && (w->min_width > 0 || w->min_height > 0)) {
                MINMAXINFO* mmi = (MINMAXINFO*)lParam;
                if (w->min_width > 0) mmi->ptMinTrackSize.x = (LONG)w->min_width;
                if (w->min_height > 0) mmi->ptMinTrackSize.y = (LONG)w->min_height;
                return 0;
            }
            break;
        case WM_CLOSE:
            if (w) {
                w->visible = false;
                if (w->webview_controller) {
                    JossWV2Controller* ctrl = (JossWV2Controller*)w->webview_controller;
                    ctrl->lpVtbl->Close(ctrl);
                    ctrl->lpVtbl->Release(ctrl);
                    w->webview_controller = NULL;
                }
                if (w->webview) {
                    JossWV2Core* wv = (JossWV2Core*)w->webview;
                    wv->lpVtbl->Release(wv);
                    w->webview = NULL;
                }
                joss_gui_emit_event(w, "close");
            }
            DestroyWindow(hwnd);
            return 0;
        case WM_DESTROY:
            PostQuitMessage(0);
            return 0;
        default:
            break;
    }
    return DefWindowProcW(hwnd, msg, wParam, lParam);
}

static void joss_win_enable_hidpi(void) {
    HMODULE u32 = GetModuleHandleW(L"user32.dll");
    if (!u32) return;
    typedef BOOL(WINAPI* SetDpiCtxFn)(HANDLE);
    SetDpiCtxFn fn = NULL;
    *(void**)(&fn) = (void*)(uintptr_t)GetProcAddress(u32, "SetProcessDpiAwarenessContext");
    if (fn) {
        /* DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = (HANDLE)-4 */
        fn((HANDLE)(intptr_t)-4);
    }
}

static void joss_platform_window_create(JossWindow* w) {
    if (!w || w->headless) return;
    joss_win_enable_hidpi();
    CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);

    WNDCLASSEXW wc;
    memset(&wc, 0, sizeof(wc));
    wc.cbSize = sizeof(wc);
    wc.lpfnWndProc = joss_win_wndproc;
    wc.hInstance = GetModuleHandleW(NULL);
    wc.hCursor = LoadCursorW(NULL, IDC_ARROW);
    wc.hbrBackground = (HBRUSH)(uintptr_t)(COLOR_WINDOW + 1);
    wc.lpszClassName = L"JossNativeWindowClass";
    RegisterClassExW(&wc);

    DWORD style = w->frameless ? WS_POPUP : WS_OVERLAPPEDWINDOW;
    if (!w->resizable && !w->frameless) {
        style &= ~(DWORD)(WS_THICKFRAME | WS_MAXIMIZEBOX);
    }
    DWORD ex_style = w->always_on_top ? WS_EX_TOPMOST : 0;
    wchar_t* wtitle = joss_win_utf8_to_wide(w->title);
    w->hwnd = CreateWindowExW(
        ex_style, L"JossNativeWindowClass", wtitle ? wtitle : L"Joss Application",
        style, CW_USEDEFAULT, CW_USEDEFAULT,
        (int)(w->width > 0 ? w->width : 1024),
        (int)(w->height > 0 ? w->height : 768),
        NULL, NULL, wc.hInstance, NULL
    );
    free(wtitle);
    if (w->hwnd) {
        SetWindowLongPtrW(w->hwnd, GWLP_USERDATA, (LONG_PTR)(uintptr_t)w);
        joss_win_apply_theme(w);
        joss_win_init_webview2(w);
    }
}

static void joss_platform_window_run(JossWindow* w) {
    if (!w || w->headless || !w->hwnd) return;
    ShowWindow(w->hwnd, w->maximized ? SW_SHOWMAXIMIZED : (w->minimized ? SW_SHOWMINIMIZED : SW_SHOW));
    UpdateWindow(w->hwnd);
    MSG msg;
    while (GetMessageW(&msg, NULL, 0, 0) > 0) {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }
}

/* =========================================================================
 * Platform Backend: macOS (Cocoa NSWindow + WKWebView via objc_msgSend)
 * ========================================================================= */
#elif defined(__APPLE__)

static void joss_platform_window_create(JossWindow* w) {
    if (!w || w->headless) return;
    Class nsAppCls = objc_getClass("NSApplication");
    if (!nsAppCls) return;
    id (*msg_id)(id, SEL) = (id (*)(id, SEL))objc_msgSend;
    id app = msg_id((id)nsAppCls, sel_registerName("sharedApplication"));
    void (*msg_setPolicy)(id, SEL, long) = (void (*)(id, SEL, long))objc_msgSend;
    msg_setPolicy(app, sel_registerName("setActivationPolicy:"), 0); /* NSApplicationActivationPolicyRegular */
    w->ns_window = (void*)app;
}

static void joss_platform_window_run(JossWindow* w) {
    if (!w || w->headless || !w->ns_window) return;
    void (*msg_void)(id, SEL) = (void (*)(id, SEL))objc_msgSend;
    msg_void((id)w->ns_window, sel_registerName("run"));
}

/* =========================================================================
 * Platform Backend: Linux (GTK3 + WebKitGTK via dlopen, zero header deps)
 * ========================================================================= */
#else

static void joss_platform_window_create(JossWindow* w) {
    if (!w || w->headless) return;
    void* gtk = dlopen("libgtk-3.so.0", RTLD_LAZY);
    if (!gtk) {
        w->headless = true;
        return;
    }
    w->gtk_lib = gtk;
    void* wk = dlopen("libwebkit2gtk-4.1.so.0", RTLD_LAZY);
    if (!wk) wk = dlopen("libwebkit2gtk-4.0.so.37", RTLD_LAZY);
    w->wk_lib = wk;

    typedef int (*GtkInitCheckFn)(int*, char***);
    GtkInitCheckFn gtk_init_check_fn = NULL;
    *(void**)(&gtk_init_check_fn) = dlsym(gtk, "gtk_init_check");
    if (!gtk_init_check_fn || !gtk_init_check_fn(NULL, NULL)) {
        w->headless = true;
        return;
    }
    typedef void* (*GtkWindowNewFn)(int);
    GtkWindowNewFn gtk_window_new_fn = NULL;
    *(void**)(&gtk_window_new_fn) = dlsym(gtk, "gtk_window_new");
    if (gtk_window_new_fn) {
        w->gtk_window = gtk_window_new_fn(0 /* GTK_WINDOW_TOPLEVEL */);
    }
    if (w->gtk_window) {
        typedef void (*GtkSetTitleFn)(void*, const char*);
        typedef void (*GtkSetDefaultSizeFn)(void*, int, int);
        GtkSetTitleFn set_title_fn = NULL;
        GtkSetDefaultSizeFn set_size_fn = NULL;
        *(void**)(&set_title_fn) = dlsym(gtk, "gtk_window_set_title");
        *(void**)(&set_size_fn) = dlsym(gtk, "gtk_window_set_default_size");
        if (set_title_fn && w->title) set_title_fn(w->gtk_window, w->title);
        if (set_size_fn) set_size_fn(w->gtk_window, (int)w->width, (int)w->height);

        typedef unsigned long (*GSignalConnectDataFn)(void*, const char*, void (*)(void), void*, void*, int);
        GSignalConnectDataFn sig_fn = NULL;
        void (*quit_fn)(void) = NULL;
        *(void**)(&sig_fn) = dlsym(gtk, "g_signal_connect_data");
        *(void**)(&quit_fn) = dlsym(gtk, "gtk_main_quit");
        if (sig_fn && quit_fn) {
            sig_fn(w->gtk_window, "destroy", quit_fn, NULL, NULL, 0);
        }
    }
    if (w->gtk_window && w->wk_lib) {
        typedef void* (*WkNewFn)(void);
        typedef void  (*GtkContainerAddFn)(void*, void*);
        WkNewFn wk_new_fn = NULL;
        GtkContainerAddFn add_fn = NULL;
        *(void**)(&wk_new_fn) = dlsym(w->wk_lib, "webkit_web_view_new");
        *(void**)(&add_fn) = dlsym(gtk, "gtk_container_add");
        if (wk_new_fn && add_fn) {
            w->wk_webview = wk_new_fn();
            if (w->wk_webview) add_fn(w->gtk_window, w->wk_webview);
        }
    }
}

static void joss_platform_window_run(JossWindow* w) {
    if (!w || w->headless || !w->gtk_lib || !w->gtk_window) return;
    typedef void (*GtkShowAllFn)(void*);
    typedef void (*GtkMainFn)(void);
    GtkShowAllFn show_all_fn = NULL;
    GtkMainFn gtk_main_fn = NULL;
    *(void**)(&show_all_fn) = dlsym(w->gtk_lib, "gtk_widget_show_all");
    *(void**)(&gtk_main_fn) = dlsym(w->gtk_lib, "gtk_main");
    if (show_all_fn) show_all_fn(w->gtk_window);
    if (gtk_main_fn) gtk_main_fn();
}

#endif

/* =========================================================================
 * Public Joss Native Window API (@fn_Window_*)
 * ========================================================================= */

void* fn_Window_open(const char* title_raw, int64_t width, int64_t height) {
    JossWindow* w = (JossWindow*)calloc(1, sizeof(JossWindow));
    if (!w) return NULL;
    const char* title = joss_gui_cstr(title_raw);
    w->magic = JOSS_WINDOW_MAGIC;
    w->title = joss_gui_strdup((title && *title) ? title : "Joss Application");
    w->width = (width > 0) ? width : 1024;
    w->height = (height > 0) ? height : 768;
    w->visible = true;
    w->resizable = true;
    w->theme = joss_gui_strdup("system");
    w->url = joss_gui_strdup("");
    w->html = joss_gui_strdup("");
    w->headless = joss_gui_is_headless();
    joss_platform_window_create(w);
    return w;
}

void fn_Window_setTitle(void* self_raw, const char* title_raw) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    const char* title = joss_gui_cstr(title_raw);
    free(w->title);
    w->title = joss_gui_strdup(title);
#if defined(_WIN32)
    if (!w->headless && w->hwnd) {
        wchar_t* wt = joss_win_utf8_to_wide(w->title);
        if (wt) { SetWindowTextW(w->hwnd, wt); free(wt); }
    }
#elif !defined(__APPLE__)
    if (!w->headless && w->gtk_lib && w->gtk_window) {
        typedef void (*GtkSetTitleFn)(void*, const char*);
        GtkSetTitleFn fn = NULL;
        *(void**)(&fn) = dlsym(w->gtk_lib, "gtk_window_set_title");
        if (fn) fn(w->gtk_window, w->title);
    }
#endif
}

void fn_Window_setSize(void* self_raw, int64_t width, int64_t height) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    if (width > 0) w->width = width;
    if (height > 0) w->height = height;
#if defined(_WIN32)
    if (!w->headless && w->hwnd) {
        SetWindowPos(w->hwnd, NULL, 0, 0, (int)w->width, (int)w->height, SWP_NOMOVE | SWP_NOZORDER);
    }
#elif !defined(__APPLE__)
    if (!w->headless && w->gtk_lib && w->gtk_window) {
        typedef void (*GtkResizeFn)(void*, int, int);
        GtkResizeFn fn = NULL;
        *(void**)(&fn) = dlsym(w->gtk_lib, "gtk_window_resize");
        if (fn) fn(w->gtk_window, (int)w->width, (int)w->height);
    }
#endif
}

void fn_Window_setMinSize(void* self_raw, int64_t width, int64_t height) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    w->min_width = width;
    w->min_height = height;
}

void fn_Window_setResizable(void* self_raw, int64_t resizable) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    w->resizable = (joss_boolval(resizable) != 0);
}

void fn_Window_setFrameless(void* self_raw, int64_t frameless) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    w->frameless = (joss_boolval(frameless) != 0);
}

void fn_Window_setAlwaysOnTop(void* self_raw, int64_t always_on_top) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    w->always_on_top = (joss_boolval(always_on_top) != 0);
#if defined(_WIN32)
    if (!w->headless && w->hwnd) {
        SetWindowPos(w->hwnd, w->always_on_top ? HWND_TOPMOST : HWND_NOTOPMOST,
                     0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE);
    }
#endif
}

void fn_Window_setTheme(void* self_raw, const char* theme_raw) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    const char* theme = joss_gui_cstr(theme_raw);
    free(w->theme);
    w->theme = joss_gui_strdup((theme && *theme) ? theme : "system");
#if defined(_WIN32)
    joss_win_apply_theme(w);
#endif
}

void fn_Window_navigate(void* self_raw, const char* url_raw) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    const char* url = joss_gui_cstr(url_raw);
    free(w->url);
    w->url = joss_gui_strdup(url);
#if defined(_WIN32)
    if (!w->headless && w->webview) {
        joss_win_wv2_navigate(w, w->url);
    }
#elif !defined(__APPLE__)
    if (!w->headless && w->wk_lib && w->wk_webview) {
        typedef void (*WkLoadUriFn)(void*, const char*);
        WkLoadUriFn fn = NULL;
        *(void**)(&fn) = dlsym(w->wk_lib, "webkit_web_view_load_uri");
        if (fn) fn(w->wk_webview, w->url);
    }
#endif
}

void fn_Window_setHtml(void* self_raw, const char* html_raw) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    const char* html = joss_gui_cstr(html_raw);
    free(w->html);
    w->html = joss_gui_strdup(html);
#if defined(_WIN32)
    if (!w->headless && w->webview) {
        joss_win_wv2_set_html(w, w->html);
    }
#elif !defined(__APPLE__)
    if (!w->headless && w->wk_lib && w->wk_webview) {
        typedef void (*WkLoadHtmlFn)(void*, const char*, const char*);
        WkLoadHtmlFn fn = NULL;
        *(void**)(&fn) = dlsym(w->wk_lib, "webkit_web_view_load_html");
        if (fn) fn(w->wk_webview, w->html, "joss://app/");
    }
#endif
}

void fn_Window_eval(void* self_raw, const char* script_raw) {
    JossWindow* w = joss_as_window(self_raw);
    const char* script = joss_gui_cstr(script_raw);
    if (!w || !script || !*script) return;
#if defined(_WIN32)
    if (!w->headless && w->webview) {
        joss_win_wv2_eval(w, script);
    }
#elif !defined(__APPLE__)
    if (!w->headless && w->wk_lib && w->wk_webview) {
        typedef void (*WkEvalFn)(void*, const char*, void*, void*, void*);
        WkEvalFn fn = NULL;
        *(void**)(&fn) = dlsym(w->wk_lib, "webkit_web_view_run_javascript");
        if (fn) fn(w->wk_webview, script, NULL, NULL, NULL);
    }
#endif
}

void fn_Window_bind(void* self_raw, const char* name_raw, int64_t handler_callable) {
    JossWindow* w = joss_as_window(self_raw);
    const char* name = joss_gui_cstr(name_raw);
    if (!w || !name || !*name) return;
    for (int i = 0; i < w->binding_count; i++) {
        if (w->bindings[i].name && strcmp(w->bindings[i].name, name) == 0) {
            w->bindings[i].callable = handler_callable;
            return;
        }
    }
    if (w->binding_count < JOSS_WINDOW_MAX_BINDINGS) {
        w->bindings[w->binding_count].name = joss_gui_strdup(name);
        w->bindings[w->binding_count].callable = handler_callable;
        w->binding_count++;
    }
}

const char* fn_Window_dispatch(void* self_raw, const char* name_raw, const char* payload_raw) {
    JossWindow* w = joss_as_window(self_raw);
    const char* name = joss_gui_cstr(name_raw);
    const char* payload = joss_gui_cstr(payload_raw);
    return joss_gui_invoke_binding(w, name, payload);
}

void fn_Window_maximize(void* self_raw) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    w->maximized = true;
    w->minimized = false;
#if defined(_WIN32)
    if (!w->headless && w->hwnd) ShowWindow(w->hwnd, SW_MAXIMIZE);
#endif
}

void fn_Window_minimize(void* self_raw) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    w->minimized = true;
    w->maximized = false;
#if defined(_WIN32)
    if (!w->headless && w->hwnd) ShowWindow(w->hwnd, SW_MINIMIZE);
#endif
}

void fn_Window_show(void* self_raw) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    w->visible = true;
#if defined(_WIN32)
    if (!w->headless && w->hwnd) ShowWindow(w->hwnd, SW_SHOW);
#endif
}

void fn_Window_hide(void* self_raw) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    w->visible = false;
#if defined(_WIN32)
    if (!w->headless && w->hwnd) ShowWindow(w->hwnd, SW_HIDE);
#endif
}

void fn_Window_close(void* self_raw) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    w->visible = false;
    joss_gui_emit_event(w, "close");
#if defined(_WIN32)
    if (!w->headless && w->hwnd) {
        DestroyWindow(w->hwnd);
        w->hwnd = NULL;
    }
#endif
}

void fn_Window_run(void* self_raw) {
    JossWindow* w = joss_as_window(self_raw);
    if (!w) return;
    joss_platform_window_run(w);
}

int64_t fn_Window_pollEvents(void* self_raw) {
    (void)self_raw;
    return 0;
}

void fn_Window_render(void* self_raw, const char* view_raw) {
    fn_Window_setHtml(self_raw, view_raw);
}

void fn_Window_on(void* self_raw, const char* evt_raw, int64_t handler_callable) {
    JossWindow* w = joss_as_window(self_raw);
    const char* evt = joss_gui_cstr(evt_raw);
    if (!w || !evt || !*evt) return;
    for (int i = 0; i < w->handler_count; i++) {
        if (w->handlers[i].name && strcmp(w->handlers[i].name, evt) == 0) {
            w->handlers[i].callable = handler_callable;
            return;
        }
    }
    if (w->handler_count < JOSS_WINDOW_MAX_HANDLERS) {
        w->handlers[w->handler_count].name = joss_gui_strdup(evt);
        w->handlers[w->handler_count].callable = handler_callable;
        w->handler_count++;
    }
}

const char* fn_Window_getTitle(void* self_raw) {
    JossWindow* w = joss_as_window(self_raw);
    return (w && w->title) ? w->title : "";
}

int64_t fn_Window_getWidth(void* self_raw) {
    JossWindow* w = joss_as_window(self_raw);
    return w ? w->width : 0;
}

int64_t fn_Window_getHeight(void* self_raw) {
    JossWindow* w = joss_as_window(self_raw);
    return w ? w->height : 0;
}

int64_t fn_Window_isVisible(void* self_raw) {
    JossWindow* w = joss_as_window(self_raw);
    return (w && w->visible) ? 1 : 0;
}

void joss_gui_launch_url(const char* title, const char* url, int64_t width, int64_t height) {
    void* win = fn_Window_open(title, width > 0 ? width : 1120, height > 0 ? height : 760);
    if (!win) return;
    if (url && *url) {
        fn_Window_navigate(win, url);
    }
    fn_Window_run(win);
}
