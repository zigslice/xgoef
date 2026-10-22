/**
 * xgoef frontend bridge (Tauri-like invoke / listen / emit / channel).
 *
 * Transport: CEF MessageRouter `window.cefQuery` (injected by the host
 * renderer shim). Falls back to `__xgoefPost` when present.
 */
(function (global) {
  const pending = new Map();
  let nextId = 1;
  const listeners = new Map();
  const channels = new Map();

  class XgoefError extends Error {
    constructor(code, message) {
      super(message || "invoke failed");
      this.name = "XgoefError";
      this.code = code || "command.failed";
    }
  }

  function asError(body) {
    if (body && typeof body === "object") {
      return new XgoefError(body.code, body.message);
    }
    if (typeof body === "string" && body) {
      return new XgoefError("command.failed", body);
    }
    return new XgoefError("command.failed", "invoke failed");
  }

  function post(msg) {
    const body = JSON.stringify(msg);

    if (typeof global.cefQuery === "function") {
      global.cefQuery({
        request: body,
        persistent: false,
        onSuccess: function (response) {
          try {
            dispatch(JSON.parse(response));
          } catch (e) {
            dispatch({
              type: "response",
              id: msg.id,
              ok: false,
              error: {
                code: "ipc.bad_response",
                message: "invalid host response: " + (e && e.message ? e.message : e),
              },
            });
          }
        },
        onFailure: function (code, message) {
          dispatch({
            type: "response",
            id: msg.id,
            ok: false,
            error: {
              code: "ipc.transport",
              message: message || "cefQuery failed (" + code + ")",
            },
          });
        },
      });
      return;
    }

    if (typeof global.__xgoefPost === "function") {
      global.__xgoefPost(body);
      return;
    }

    queueMicrotask(function () {
      dispatch({
        type: "response",
        id: msg.id,
        ok: false,
        error: { code: "ipc.disconnected", message: "xgoef host bridge not connected" },
      });
    });
  }

  function dispatch(msg) {
    if (msg.type === "response" && pending.has(msg.id)) {
      const { resolve, reject } = pending.get(msg.id);
      pending.delete(msg.id);
      if (msg.ok) resolve(msg.payload);
      else reject(asError(msg.error));
      return;
    }
    if (msg.type === "event") {
      const list = listeners.get(msg.event) || [];
      list.forEach(function (fn) {
        try {
          fn(msg.payload);
        } catch (_) {}
      });
      return;
    }
    if (msg.type === "channel") {
      acceptChannel(msg.channel, msg.index, msg.payload);
    }
  }

  function acceptChannel(id, index, payload) {
    const state = channels.get(id);
    if (!state || typeof index !== "number" || index < state.expected || index > state.expected + 64) {
      return;
    }
    state.buf.set(index, payload);
    while (state.buf.has(state.expected)) {
      const body = state.buf.get(state.expected);
      state.buf.delete(state.expected);
      state.expected++;
      state.handlers.forEach(function (fn) {
        try {
          fn(body);
        } catch (_) {}
      });
    }
  }

  global.__xgoefDispatch = dispatch;

  global.__XGOEF__ = {
    invoke: function (cmd, args, opts) {
      const id = nextId++;
      opts = opts || {};
      let cancelPosted = false;
      function cancel() {
        if (cancelPosted) return;
        cancelPosted = true;
        post({ type: "cancel", invoke: id });
      }
      const promise = new Promise(function (resolve, reject) {
        pending.set(id, { resolve: resolve, reject: reject });
        const msg = { type: "invoke", id: id, cmd: cmd, args: args || {} };
        if (typeof opts.timeout === "number" && opts.timeout !== 0) {
          msg.timeout = opts.timeout;
        }
        post(msg);
      });
      promise.cancel = cancel;
      if (opts.signal && typeof opts.signal.addEventListener === "function") {
        if (opts.signal.aborted) {
          cancel();
        } else {
          opts.signal.addEventListener("abort", cancel);
        }
      }
      return promise;
    },
    listen: function (event, handler) {
      if (!listeners.has(event)) listeners.set(event, []);
      listeners.get(event).push(handler);
      return function () {
        const list = listeners.get(event) || [];
        listeners.set(
          event,
          list.filter(function (h) {
            return h !== handler;
          })
        );
      };
    },
    emit: function (event, payload) {
      post({ type: "event", event: event, payload: payload });
    },
    XgoefError: XgoefError,
    channel: function () {
      const id = nextId++;
      const state = { expected: 0, handlers: [], buf: new Map() };
      channels.set(id, state);
      return {
        id: id,
        toJSON: function () {
          return { __channel: id };
        },
        onmessage: function (handler) {
          state.handlers.push(handler);
          return function () {
            state.handlers = state.handlers.filter(function (h) {
              return h !== handler;
            });
          };
        },
        close: function () {
          channels.delete(id);
          post({ type: "channel-close", channel: id });
        },
      };
    },
    plugins: {
      fs: {
        readText: function (path) {
          return global.__XGOEF__.invoke("fs:read_text", { path: path });
        },
        writeText: function (path, contents) {
          return global.__XGOEF__.invoke("fs:write_text", { path: path, contents: contents });
        },
        exists: function (path) {
          return global.__XGOEF__.invoke("fs:exists", { path: path });
        },
        list: function (path) {
          return global.__XGOEF__.invoke("fs:list", { path: path });
        },
      },
      os: {
        platform: function () {
          return global.__XGOEF__.invoke("os:platform", {});
        },
        arch: function () {
          return global.__XGOEF__.invoke("os:arch", {});
        },
      },
      shell: {
        open: function (target) {
          const value = String(target || "");
          if (value.indexOf("://") >= 0) {
            return global.__XGOEF__.invoke("shell:open", { url: value });
          }
          return global.__XGOEF__.invoke("shell:open", { path: value });
        },
      },
      clipboard: {
        readText: function () {
          return global.__XGOEF__.invoke("clipboard:read_text", {});
        },
        writeText: function (text) {
          return global.__XGOEF__.invoke("clipboard:write_text", { text: text });
        },
      },
      dialog: {
        open: function (opts) {
          return global.__XGOEF__.invoke("dialog:open", opts || {});
        },
        save: function (opts) {
          return global.__XGOEF__.invoke("dialog:save", opts || {});
        },
      },
      updater: {
        check: function () {
          return global.__XGOEF__.invoke("updater:check", {});
        },
        install: function () {
          return global.__XGOEF__.invoke("updater:install", {});
        },
      },
      process: {
        exit: function (code) {
          const args = {};
          if (typeof code === "number") {
            args.code = code;
          }
          return global.__XGOEF__.invoke("process:exit", args);
        },
        relaunch: function () {
          return global.__XGOEF__.invoke("process:relaunch", {});
        },
      },
      notification: {
        isPermissionGranted: function () {
          return global.__XGOEF__.invoke("notification:is_permission_granted", {});
        },
        requestPermission: function () {
          return global.__XGOEF__.invoke("notification:request_permission", {});
        },
        show: function (opts) {
          if (typeof opts === "string") {
            return global.__XGOEF__.invoke("notification:show", { title: opts });
          }
          return global.__XGOEF__.invoke("notification:show", opts || {});
        },
      },
      "global-shortcut": {
        register: function (shortcut) {
          return global.__XGOEF__.invoke("global-shortcut:register", { shortcut: shortcut });
        },
        unregister: function (shortcut) {
          return global.__XGOEF__.invoke("global-shortcut:unregister", { shortcut: shortcut });
        },
        unregisterAll: function () {
          return global.__XGOEF__.invoke("global-shortcut:unregister_all", {});
        },
        isRegistered: function (shortcut) {
          return global.__XGOEF__.invoke("global-shortcut:is_registered", { shortcut: shortcut });
        },
      },
      deeplink: {
        getCurrent: function () {
          return global.__XGOEF__.invoke("deeplink:get_current", {});
        },
      },
    },
  };
  global.__XGOEF__.listen("dev-reload", function () {
    if (global.location && typeof global.location.reload === "function") {
      global.location.reload();
    }
  });
})(typeof window !== "undefined" ? window : globalThis);
