/** 软件更新仅属于桌面发行，不占用 Core 的通知与工作状态。 */
export type ApplicationUpdateStatus =
  | "unavailable"
  | "idle"
  | "checking"
  | "no-release"
  | "current"
  | "available"
  | "downloading"
  | "downloaded"
  | "error";

export type ApplicationUpdateError =
  | "network"
  | "rate-limited"
  | "invalid-release"
  | "missing-asset"
  | "checksum"
  | "download"
  | "open-installer";

export type ApplicationRelease = {
  version: string;
  url: string;
  notes: string;
  publishedAt: string;
  downloadSize: number;
};

/** 单调版本避免迟到的查询结果覆盖下载进度事件。 */
export type ApplicationUpdateState = {
  revision: number;
  currentVersion: string | null;
  platform: string;
  arch: string;
  status: ApplicationUpdateStatus;
  unavailableReason?: "development" | "platform";
  release?: ApplicationRelease;
  checkedAt?: string;
  downloadedBytes: number;
  error?: ApplicationUpdateError;
};

export const applicationReleasesURL =
  "https://github.com/JieWaZi/NeuxAlly/releases";
