"use strict";

const assert = require("assert");
const fs = require("fs");
const path = require("path");
const vm = require("vm");

const source = fs.readFileSync(path.join(__dirname, "xgoef.js"), "utf8");

function load() {
  const sandbox = {
    queueMicrotask,
    Map,
    Promise,
    Error,
    JSON,
    String,
    Object,
  };
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);
  return sandbox;
}

async function rejected(job) {
  try {
    await job;
  } catch (err) {
    return err;
  }
  throw new Error("invoke resolved");
}

async function main() {
  {
    const sandbox = load();
    const posted = [];
    sandbox.__xgoefPost = (body) => posted.push(JSON.parse(body));
    const job = sandbox.__XGOEF__.invoke("greet", { name: "x" });
    assert.equal(posted.length, 1);
    assert.equal(posted[0].cmd, "greet");
    sandbox.__xgoefDispatch({
      type: "response",
      id: posted[0].id,
      ok: false,
      error: { code: "acl.denied", message: "nope" },
    });
    const err = await rejected(job);
    assert.equal(err.code, "acl.denied");
    assert.equal(err.message, "nope");
    assert.equal(err.name, "XgoefError");
    assert.ok(err instanceof sandbox.__XGOEF__.XgoefError);
    assert.ok(err instanceof Error);
  }

  {
    const sandbox = load();
    let posted;
    sandbox.__xgoefPost = (body) => {
      posted = JSON.parse(body);
    };
    const job = sandbox.__XGOEF__.invoke("greet", {});
    sandbox.__xgoefDispatch({
      type: "response",
      id: posted.id,
      ok: true,
      payload: "hi",
    });
    assert.equal(await job, "hi");
  }

  {
    const sandbox = load();
    let posted;
    sandbox.__xgoefPost = (body) => {
      posted = JSON.parse(body);
    };
    const job = sandbox.__XGOEF__.invoke("greet", {});
    sandbox.__xgoefDispatch({
      type: "response",
      id: posted.id,
      ok: false,
      error: "old",
    });
    const err = await rejected(job);
    assert.equal(err.code, "command.failed");
    assert.equal(err.message, "old");
  }

  {
    const sandbox = load();
    const err = await rejected(sandbox.__XGOEF__.invoke("greet", {}));
    assert.equal(err.code, "ipc.disconnected");
    assert.match(err.message, /not connected/);
  }

  {
    const sandbox = load();
    sandbox.cefQuery = (query) => query.onSuccess("not-json");
    const err = await rejected(sandbox.__XGOEF__.invoke("greet", {}));
    assert.equal(err.code, "ipc.bad_response");
    assert.match(err.message, /invalid host response/);
  }

  {
    const sandbox = load();
    sandbox.cefQuery = (query) => query.onFailure(7, "nope");
    const err = await rejected(sandbox.__XGOEF__.invoke("greet", {}));
    assert.equal(err.code, "ipc.transport");
    assert.equal(err.message, "nope");
  }

  {
    const sandbox = load();
    const posted = [];
    sandbox.__xgoefPost = (body) => posted.push(JSON.parse(body));
    sandbox.__XGOEF__.plugins.process.exit();
    sandbox.__XGOEF__.plugins.process.exit(7);
    sandbox.__XGOEF__.plugins.process.relaunch();
    assert.deepEqual(posted.map((msg) => msg.cmd), ["process:exit", "process:exit", "process:relaunch"]);
    assert.deepEqual(posted[0].args, {});
    assert.deepEqual(posted[1].args, { code: 7 });
    assert.deepEqual(posted[2].args, {});
  }

  {
    const sandbox = load();
    const posted = [];
    sandbox.__xgoefPost = (body) => posted.push(JSON.parse(body));
    sandbox.__XGOEF__.plugins.notification.requestPermission();
    sandbox.__XGOEF__.plugins.notification.show("hello");
    sandbox.__XGOEF__.plugins.notification.show({ title: "xgoef", body: "hi", icon: "icon.png" });
    assert.deepEqual(
      posted.map((msg) => msg.cmd),
      ["notification:request_permission", "notification:show", "notification:show"]
    );
    assert.deepEqual(posted[1].args, { title: "hello" });
    assert.deepEqual(posted[2].args, { title: "xgoef", body: "hi", icon: "icon.png" });
  }

  {
    const sandbox = load();
    const posted = [];
    sandbox.__xgoefPost = (body) => posted.push(JSON.parse(body));
    const shortcut = sandbox.__XGOEF__.plugins["global-shortcut"];
    shortcut.register("Control+Shift+X");
    shortcut.isRegistered("Control+Shift+X");
    shortcut.unregister("Control+Shift+X");
    shortcut.unregisterAll();
    assert.deepEqual(
      posted.map((msg) => msg.cmd),
      [
        "global-shortcut:register",
        "global-shortcut:is_registered",
        "global-shortcut:unregister",
        "global-shortcut:unregister_all",
      ]
    );
    assert.deepEqual(posted[0].args, { shortcut: "Control+Shift+X" });
    assert.deepEqual(posted[3].args, {});
  }

  {
    const sandbox = load();
    const posted = [];
    sandbox.__xgoefPost = (body) => posted.push(JSON.parse(body));
    const view = sandbox.__XGOEF__.plugins.webview;
    view.print();
    view.isDevToolsOpen();
    view.openDevTools();
    view.closeDevTools();
    assert.deepEqual(
      posted.map((msg) => msg.cmd),
      ["webview:print", "webview:is_dev_tools_open", "webview:open_dev_tools", "webview:close_dev_tools"]
    );
    assert.deepEqual(posted[0].args, {});
  }

  {
    const sandbox = load();
    const posted = [];
    sandbox.__xgoefPost = (body) => posted.push(JSON.parse(body));
    const win = sandbox.__XGOEF__.plugins.window;
    win.minimize();
    win.maximize();
    win.unminimize();
    win.unmaximize();
    win.toggleMaximize();
    win.show();
    win.hide();
    win.center();
    win.setTitle("hello");
    win.setAlwaysOnTop(true);
    win.setFullscreen(false);
    win.setSize(800, 600);
    win.isMinimized();
    win.isMaximized();
    win.isFullscreen();
    win.isAlwaysOnTop();
    win.isVisible();
    assert.deepEqual(
      posted.map((msg) => msg.cmd),
      [
        "window:minimize",
        "window:maximize",
        "window:unminimize",
        "window:unmaximize",
        "window:toggle_maximize",
        "window:show",
        "window:hide",
        "window:center",
        "window:set_title",
        "window:set_always_on_top",
        "window:set_fullscreen",
        "window:set_size",
        "window:is_minimized",
        "window:is_maximized",
        "window:is_fullscreen",
        "window:is_always_on_top",
        "window:is_visible",
      ]
    );
    assert.deepEqual(posted[0].args, {});
    assert.deepEqual(posted[8].args, { title: "hello" });
    assert.deepEqual(posted[9].args, { alwaysOnTop: true });
    assert.deepEqual(posted[10].args, { fullscreen: false });
    assert.deepEqual(posted[11].args, { width: 800, height: 600 });
    win.setResizable(false);
    win.isResizable();
    win.setMinimizable(true);
    win.isMinimizable();
    win.setMaximizable(false);
    win.isMaximizable();
    win.setClosable(true);
    win.isClosable();
    win.setMinSize(320, 240);
    win.setMaxSize(0, 0);
    win.minSize();
    win.maxSize();
    assert.deepEqual(posted[17].cmd, "window:set_resizable");
    assert.deepEqual(posted[17].args, { resizable: false });
    assert.deepEqual(posted[25].args, { width: 320, height: 240 });
    assert.deepEqual(posted[26].args, { width: 0, height: 0 });
    assert.deepEqual(posted[27].cmd, "window:min_size");
    assert.deepEqual(posted[28].cmd, "window:max_size");
  }

  {
    const sandbox = load();
    let seen = 0;
    const bridge = sandbox.__XGOEF__;
    const dispatch = sandbox.__xgoefDispatch;
    bridge.listen("tick", function () {
      seen += 1;
    });
    vm.runInContext(source, sandbox);
    assert.strictEqual(sandbox.__XGOEF__, bridge);
    assert.strictEqual(sandbox.__xgoefDispatch, dispatch);
    sandbox.__xgoefDispatch({ type: "event", event: "tick", payload: 1 });
    assert.equal(seen, 1);
  }

  {
    const sandbox = load();
    const posted = [];
    sandbox.__xgoefPost = (body) => posted.push(JSON.parse(body));
    sandbox.__XGOEF__.plugins.eval.run("mark");
    assert.deepEqual(posted.map((msg) => msg.cmd), ["eval:run"]);
    assert.deepEqual(posted[0].args, { name: "mark" });
    assert.equal(posted[0].args.script, undefined);
  }

  {
    const sandbox = load();
    const posted = [];
    sandbox.__xgoefPost = (body) => posted.push(JSON.parse(body));
    const auto = sandbox.__XGOEF__.plugins.autostart;
    auto.isEnabled();
    auto.enable();
    auto.disable();
    assert.deepEqual(
      posted.map((msg) => msg.cmd),
      ["autostart:is_enabled", "autostart:enable", "autostart:disable"]
    );
    assert.deepEqual(posted[0].args, {});
    assert.deepEqual(posted[1].args, {});
    assert.equal(posted[1].args.args, undefined);
  }

  {
    const sandbox = load();
    const posted = [];
    sandbox.__xgoefPost = (body) => posted.push(JSON.parse(body));
    const log = sandbox.__XGOEF__.plugins.log;
    log.trace("t");
    log.debug("d");
    log.info("i");
    log.warn("w");
    log.error("e");
    assert.deepEqual(
      posted.map((msg) => msg.cmd),
      ["log:write", "log:write", "log:write", "log:write", "log:write"]
    );
    assert.deepEqual(posted[2].args, { level: "info", message: "i" });
    assert.equal(posted[2].args.path, undefined);
    assert.equal(posted[2].args.crash, undefined);
    assert.deepEqual(posted.map((msg) => msg.args.level), ["trace", "debug", "info", "warn", "error"]);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
