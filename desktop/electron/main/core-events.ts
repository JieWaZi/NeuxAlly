import type { CoreConnection } from "../shared/contracts";
import { CoreEventClient } from "../shared/core-event-client";

/** 应用根生命周期持有；窗口关闭、隐藏、重载都不重建主连接。 */
export function createCoreEvents(
  connection: () => Promise<CoreConnection>,
  log: (message: string) => Promise<void>,
): CoreEventClient {
  return new CoreEventClient({
    connection,
    log: (message) => {
      void log(message);
    },
  });
}
