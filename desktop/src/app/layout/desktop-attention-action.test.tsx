import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TooltipProvider } from "@/shared/ui/tooltip";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ApplicationUpdateState } from "@/shared/desktop/application-update";
import { DesktopAttentionAction } from "./desktop-attention-action";
import "@/shared/i18n";

const updates = vi.hoisted(() => ({
  state: undefined as ApplicationUpdateState | undefined,
  busy: false,
  download: vi.fn(),
  openInstaller: vi.fn(),
}));
vi.mock("@/shared/desktop/application-update", () => ({
  useApplicationUpdate: () => updates,
}));
vi.mock("@/modules/notification", () => ({
  NotificationBell: ({ onOpen }: { onOpen: () => void }) => (
    <button onClick={onOpen}>通知</button>
  ),
}));

function setup() {
  const onOpen = vi.fn();
  const onOpenUpdates = vi.fn();
  render(
    <TooltipProvider>
      <DesktopAttentionAction
        active={false}
        connected
        onOpen={onOpen}
        onOpenUpdates={onOpenUpdates}
      />
    </TooltipProvider>,
  );
  return { onOpen, onOpenUpdates, user: userEvent.setup() };
}

beforeEach(() => {
  updates.state = undefined;
  updates.busy = false;
  vi.clearAllMocks();
});

describe("软件更新占用铃铛入口", () => {
  it("没有可用更新时保持原通知入口", async () => {
    const { onOpen, user } = setup();
    await user.click(screen.getByRole("button", { name: "通知" }));
    expect(onOpen).toHaveBeenCalledOnce();
  });
  for (const status of ["available", "downloading", "downloaded"] as const) {
    it(`${status} 状态定位关于软件，并按状态下载或打开安装包`, async () => {
      updates.state = {
        revision: 1,
        currentVersion: "0.1.0",
        platform: "darwin",
        arch: "arm64",
        status,
        downloadedBytes: 50,
        release: {
          version: "0.2.0",
          url: "https://github.com/JieWaZi/NeuxAlly/releases/tag/v0.2.0",
          notes: "更新",
          publishedAt: "2026-10-16T00:00:00Z",
          downloadSize: 100,
        },
      };
      const { onOpenUpdates, user } = setup();
      expect(
        screen.queryByRole("button", { name: "通知" }),
      ).not.toBeInTheDocument();
      await user.click(
        screen.getByRole("button", {
          name:
            status === "available"
              ? "下载新版本 v0.2.0"
              : status === "downloaded"
                ? "打开 v0.2.0 安装包"
                : "新版本下载中 50%，查看进度",
        }),
      );
      expect(onOpenUpdates).toHaveBeenCalledOnce();
      expect(updates.download).toHaveBeenCalledTimes(
        status === "available" ? 1 : 0,
      );
      expect(updates.openInstaller).toHaveBeenCalledTimes(
        status === "downloaded" ? 1 : 0,
      );
    });
  }
});
