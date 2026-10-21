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
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
