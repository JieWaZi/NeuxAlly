import { test } from "node:test";
import assert from "node:assert/strict";
import { EventEmitter } from "node:events";

import { loadModule } from "./load-module.mjs";

test("网页访客视图只接受已准备的隔离会话和网页地址", async () => {
  const sessions = new Map();
  const fromPartition = (name) => {
    if (!sessions.has(name)) {
      const value = Object.assign(new EventEmitter(), {
        protocol: {
          handle(scheme, handler) {
            assert.equal(scheme, "file");
            value.fileHandler = handler;
          },
          unhandle() {},
        },
        setPermissionRequestHandler(handler) {
          value.permissionRequest = handler;
        },
        setPermissionCheckHandler(handler) {
          value.permissionCheck = handler;
        },
      });
      sessions.set(name, value);
    }
    return sessions.get(name);
  };
  const { BrowserWebviewPolicy } = loadModule(
    "main/browser-webview-policy.ts",
    {
      electron: { session: { fromPartition } },
    },
  );
  const contents = Object.assign(new EventEmitter(), { id: 9 });
  const policy = new BrowserWebviewPolicy({ webContents: contents });
  const blocked = {
    prevented: false,
    preventDefault() {
      this.prevented = true;
    },
  };
  contents.emit(
    "will-attach-webview",
    blocked,
    {},
    {
      partition: "unknown",
      src: "https://example.com/",
    },
  );
  assert.equal(blocked.prevented, true);

  const partition = policy.prepare("https://example.com/", "/project");
  assert.equal(policy.prepare("https://example.com/", "/project"), partition);
  assert.equal(sessions.size, 1);
  const isolated = sessions.get(partition);
  assert.equal((await isolated.fileHandler()).status, 403);
  assert.equal(isolated.permissionCheck(), false);
  isolated.permissionRequest(null, "camera", (allowed) =>
    assert.equal(allowed, false),
  );

  const preferences = { preload: "/unsafe.js", nodeIntegration: true };
  const allowed = {
    prevented: false,
    preventDefault() {
      this.prevented = true;
    },
  };
  contents.emit("will-attach-webview", allowed, preferences, {
    partition,
    src: "https://example.com/",
  });
  assert.equal(allowed.prevented, false);
  assert.equal(preferences.preload, undefined);
  assert.equal(preferences.session, isolated);
  assert.equal(preferences.nodeIntegration, false);
  assert.equal(preferences.sandbox, true);
  assert.equal(preferences.webSecurity, true);

  const local = {
    prevented: false,
    preventDefault() {
      this.prevented = true;
    },
  };
  contents.emit(
    "will-attach-webview",
    local,
    {},
    {
      partition,
      src: "file:///etc/passwd",
    },
  );
  assert.equal(local.prevented, true);
  assert.throws(() => policy.prepare("file:///etc/passwd"), /允许范围/);

  const guest = Object.assign(new EventEmitter(), {
    setWindowOpenHandler(handler) {
      this.openWindow = handler;
    },
    isDestroyed: () => false,
    loaded: [],
    async loadURL(url) {
      this.loaded.push(url);
    },
  });
  contents.emit("did-attach-webview", {}, guest);
  const navigation = {
    prevented: false,
    preventDefault() {
      this.prevented = true;
    },
  };
  guest.emit("will-navigate", navigation, "file:///etc/passwd");
  assert.equal(navigation.prevented, true);
  assert.deepEqual(guest.openWindow({ url: "file:///etc/passwd" }), {
    action: "deny",
  });
  assert.deepEqual(guest.openWindow({ url: "https://example.com/next" }), {
    action: "deny",
  });
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(guest.loaded, ["https://example.com/next"]);
  policy.dispose();
  assert.equal(contents.listenerCount("will-attach-webview"), 0);
  assert.equal(contents.listenerCount("did-attach-webview"), 0);
});
