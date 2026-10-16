import path from "node:path";

type ApplicationIconTarget = {
  readonly isPackaged: boolean;
  readonly dock?: {
    setIcon(iconPath: string): void;
  };
  getAppPath(): string;
};

export function configureApplicationIcon(
  application: ApplicationIconTarget,
): void {
  // 正式包由 Info.plist 声明图标；开发态直接运行 Electron.app，需要单独设置。
  if (application.isPackaged || !application.dock) return;
  application.dock.setIcon(
    path.join(application.getAppPath(), "resources/app-icon-source.png"),
  );
}
