import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { test } from "node:test";
import { loadModule } from "./load-module.mjs";

const { installAutomaticTaskPowerLifecycle } = loadModule(
  "main/automatic-task-power-lifecycle.ts",
);
const flush = () => new Promise((resolve) => setImmediate(resolve));

test("按事件顺序发送休眠和唤醒，并在退出时取消请求和释放监听", async (t) => {
  const originalFetch = globalThis.fetch;
  t.after(() => {
    globalThis.fetch = originalFetch;
  });
  const monitor = new EventEmitter();
  const requests = [];
  let finishFirst;
  globalThis.fetch = async (url, options) => {
    requests.push({ url, ...options });
    if (requests.length === 1)
      await new Promise((resolve) => {
        finishFirst = resolve;
      });
    return { ok: true };
  };
  const logs = [];
  const dispose = installAutomaticTaskPowerLifecycle(
    monitor,
    async () => ({
      baseUrl: "http://127.0.0.1:8999",
      token: "local-test-token",
    }),
    async (message) => {
      logs.push(message);
    },
  );
  monitor.emit("suspend");
  monitor.emit("resume");
  await flush();
  assert.equal(requests.length, 1);
  assert.equal(JSON.parse(requests[0].body).suspended, true);
  assert.equal(requests[0].headers.Authorization, "Bearer local-test-token");
  finishFirst();
  await flush();
  assert.equal(requests.length, 2);
  assert.equal(JSON.parse(requests[1].body).suspended, false);
  dispose();
  assert.equal(requests[1].signal.aborted, true);
  assert.equal(monitor.listenerCount("suspend"), 0);
  assert.equal(monitor.listenerCount("resume"), 0);
  monitor.emit("resume");
  await flush();
  assert.equal(requests.length, 2);
  assert.deepEqual(logs, []);
});
