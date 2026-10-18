import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { it, expect, vi, afterEach } from "vitest";
import "@/shared/i18n";
import { PluginCard } from "./plugin-card";
import type { PluginCatalogEntry, PluginDetail } from "../model/plugin-view";

afterEach(cleanup);
it("keeps the saved card focused on description, partner usage and management", () => {
  render(
    <MemoryRouter>
      <PluginCard
        detail={{
          plugin: { id: "p", name: "飞书办公" },
          account: { available: true },
          installations: [{ wakerId: "w1", versionId: "v1" }],
          versions: [
            {
              id: "v1",
              version: "1.0.98",
              package: {
                manifest: {
                  name: "feishu",
                  author: { name: "Developer" },
                },
              },
            },
          ],
        }}
        onInstall={vi.fn()}
      />
    </MemoryRouter>,
  );
  expect(screen.getByText("1 个伙伴使用")).toBeVisible();
  const more = screen.getByRole("button", { name: "飞书办公更多操作" });
  fireEvent.pointerDown(more, { button: 0, ctrlKey: false });
  expect(screen.getByRole("menuitem", { name: "查看详情" })).toHaveAttribute(
    "href",
    "/resources/plugins/p",
  );
  expect(screen.queryByText("已授权")).not.toBeInTheDocument();
  expect(screen.queryByText("Developer")).not.toBeInTheDocument();
  expect(screen.queryByText(/1.0.98/)).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: /安装/ }),
  ).not.toBeInTheDocument();
});

it.each([
  [false, false, "安装 飞书办公"],
  [true, false, "授权 飞书办公"],
  [true, true, "安装 飞书办公"],
])(
  "catalog action follows actual installation and shared authorization (%s, %s)",
  (installed, available, label) => {
    const pkg: NonNullable<PluginCatalogEntry["package"]> = {
      manifest: { name: "office" },
      presentation: { displayName: "飞书办公" },
    };
    Object.assign(pkg.manifest ?? {}, {
      extensions: { "com.ally": { provider: "feishu" } },
    });
    const detail: PluginDetail = {
      installations: installed
        ? [{ wakerId: "already-installed-partner" }]
        : [],
      account: { available },
      versions: [{ package: pkg }],
    };
    render(
      <MemoryRouter>
        <PluginCard
          catalog={{ key: "office", package: pkg }}
          detail={detail}
          onInstall={vi.fn()}
        />
      </MemoryRouter>,
    );
    expect(screen.getByRole("button", { name: label })).toBeVisible();
    if (available || !installed)
      expect(screen.queryByText("授权")).not.toBeInTheDocument();
  },
);
