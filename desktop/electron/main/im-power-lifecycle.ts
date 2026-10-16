import type { PowerSaveBlocker } from "electron";
import type { CoreEventClient } from "../shared/core-event-client";

/** 系统运行策略由 Core 推送；窗口隐藏和页面卸载不释放运行需求。 */
export function installIMPowerLifecycle(
  blocker: Pick<PowerSaveBlocker, "start" | "stop" | "isStarted">,
  events: Pick<CoreEventClient, "subscribe">,
) {
  let blockerID: number | undefined;
  const stop = events.subscribe(
    "system",
    { kind: "global" },
    {
      onEvent: ({ payload }) => {
        if (
          typeof payload !== "object" ||
          payload === null ||
          !("keepAwake" in payload) ||
          typeof payload.keepAwake !== "boolean"
        )
          return;
        if (
          payload.keepAwake &&
          (blockerID === undefined || !blocker.isStarted(blockerID))
        )
          blockerID = blocker.start("prevent-app-suspension");
        else if (!payload.keepAwake && blockerID !== undefined) {
          blocker.stop(blockerID);
          blockerID = undefined;
        }
      },
    },
  );
  return () => {
    stop();
    if (blockerID !== undefined) blocker.stop(blockerID);
    blockerID = undefined;
  };
}
