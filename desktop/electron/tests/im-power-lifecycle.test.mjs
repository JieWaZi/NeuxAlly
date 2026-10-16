import assert from "node:assert/strict";
import { test } from "node:test";
import { loadModule } from "./load-module.mjs";
const { installIMPowerLifecycle } = loadModule("main/im-power-lifecycle.ts");
test("系统策略由共享事件推送，隐藏与断线保持策略，退出释放订阅和防休眠", () => {
  let observer;
  let unsubscribed = false;
  const started = [];
  const stopped = [];
  const dispose = installIMPowerLifecycle(
    {
      start: (kind) => {
        started.push(kind);
        return 7;
      },
      stop: (id) => stopped.push(id),
      isStarted: () => true,
    },
    {
      subscribe: (topic, resource, input) => {
        assert.equal(topic, "system");
        assert.deepEqual(resource, { kind: "global" });
        observer = input;
        return () => {
          unsubscribed = true;
        };
      },
    },
  );
  observer.onEvent({ payload: { keepAwake: true } });
  observer.onEvent({ payload: { keepAwake: true } });
  assert.deepEqual(started, ["prevent-app-suspension"]);
  observer.onEvent({ payload: {} });
  assert.deepEqual(stopped, []);
  observer.onEvent({ payload: { keepAwake: false } });
  assert.deepEqual(stopped, [7]);
  observer.onEvent({ payload: { keepAwake: true } });
  dispose();
  assert.equal(unsubscribed, true);
  assert.deepEqual(stopped, [7, 7]);
});
