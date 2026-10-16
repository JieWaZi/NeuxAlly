import type { DesktopBridge } from "../../../electron/shared/contracts";

declare global {
  interface Window {
    allyDesktop?: DesktopBridge;
  }
}

export function desktopBridge(): DesktopBridge {
  if (!window.allyDesktop) throw new Error("桌面桥接不可用");
  return window.allyDesktop;
}
