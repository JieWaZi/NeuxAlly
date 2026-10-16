import type { PowerMonitor } from "electron";
import type { CoreConnection } from "../shared/contracts";

/** 按事件顺序报告休眠和唤醒，退出时取消请求且释放系统监听。 */
export function installAutomaticTaskPowerLifecycle(
  monitor: Pick<PowerMonitor, "on" | "removeListener">,
  connection: () => Promise<CoreConnection>,
  log: (message: string) => Promise<void>,
) {
  let disposed = false;
  let queue = Promise.resolve();
  const canceled = new AbortController();
  const report = (suspended: boolean) => {
    queue = queue
      .then(async () => {
        if (disposed) return;
        const core = await connection();
        if (disposed) return;
        const response = await fetch(
          `${core.baseUrl}/api/v1/system/automatic-tasks-suspended`,
          {
            method: "POST",
            headers: {
              Authorization: `Bearer ${core.token}`,
              "Content-Type": "application/json",
            },
            body: JSON.stringify({ suspended }),
            signal: AbortSignal.any([
              canceled.signal,
              AbortSignal.timeout(3000),
            ]),
          },
        );
        if (!response.ok)
          await log("自动任务休眠状态暂未同步，将由 Core 时钟检测恢复。");
      })
      .catch(async () => {
        if (!disposed)
          await log("自动任务休眠状态暂未同步，将由 Core 时钟检测恢复。");
      });
  };
  const suspend = () => report(true);
  const resume = () => report(false);
  monitor.on("suspend", suspend);
  monitor.on("resume", resume);
  return () => {
    disposed = true;
    canceled.abort();
    monitor.removeListener("suspend", suspend);
    monitor.removeListener("resume", resume);
  };
}
