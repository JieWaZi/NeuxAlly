import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { expect, it, vi, afterEach } from "vitest";
import "@/shared/i18n";
import { PluginOverview } from "./plugin-overview";
import type { PluginCatalogEntry } from "../model/plugin-view";
vi.mock("./plugin-skill-dialog", () => ({
  PluginSkillDialog: ({ name }: { name: string }) => (
    <section aria-label="技能正文">{name}</section>
  ),
}));
afterEach(cleanup);
const pkg: PluginCatalogEntry["package"] = {
  manifest: {
    name: "github",
    version: "1.0.1",
    author: { name: "GitHub" },
    license: "MIT",
  },
  presentation: {
    displayName: "GitHub",
    developerName: "GitHub",
    shortDescription: "Manage repositories",
  },
  skills: Array.from({ length: 7 }, (_, i) => ({
    key: `skill-${i}`,
    name: `skill-${i}`,
    description: `Skill ${i}`,
  })),
  servers: [
    { key: "github", type: "http", url: "https://api.githubcopilot.com/mcp/" },
  ],
};
it("shows plugin metadata and installed partners, and opens an included skill", () => {
  render(
    <MemoryRouter>
      <PluginOverview
        pkg={pkg}
        skillContext={{ catalogKey: "github" }}
        detail={{
          installations: [{ wakerId: "w1", enabled: true, versionId: "v1" }],
          partnerNames: { w1: "开发伙伴" },
          versions: [{ id: "v1", version: "1.0.1" }],
        }}
      />
    </MemoryRouter>,
  );
  expect(screen.getByText("GitHub")).toBeVisible();
  expect(screen.getByText("MIT")).toBeVisible();
  expect(screen.getByRole("heading", { name: "已安装 1" })).toBeVisible();
  expect(screen.getByRole("link", { name: /开发伙伴/ })).toHaveAttribute(
    "href",
    "/wakers/w1/resources",
  );
  expect(
    screen.queryByRole("button", { name: "查看技能 skill-6" }),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "查看全部 7 项技能" }));
  fireEvent.click(screen.getByRole("button", { name: "查看技能 skill-6" }));
  expect(screen.getByLabelText("技能正文")).toHaveTextContent("skill-6");
});
it("keeps unknown installation state distinct from an empty installation list", () => {
  render(
    <MemoryRouter>
      <PluginOverview
        pkg={pkg}
        installationStatus={<p role="status">正在读取安装伙伴</p>}
      />
    </MemoryRouter>,
  );
  expect(screen.getByRole("status")).toHaveTextContent("正在读取安装伙伴");
  expect(screen.getByRole("heading", { name: "已安装" })).toBeVisible();
  expect(screen.queryByText("尚未安装到伙伴")).not.toBeInTheDocument();
});

it("hides absent capabilities and keeps link values as icons", () => {
  render(
    <MemoryRouter>
      <PluginOverview
        pkg={{
          manifest: {
            name: "empty",
            homepage: "https://example.com",
            repository: "https://example.com/repo",
          },
          skills: [],
          servers: [],
        }}
      />
    </MemoryRouter>,
  );
  expect(
    screen.queryByRole("heading", { name: /技能/ }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("heading", { name: /连接器/ }),
  ).not.toBeInTheDocument();
  expect(screen.getAllByText("官方网站")).toHaveLength(1);
  expect(screen.getAllByText("源代码")).toHaveLength(1);
  expect(screen.getByRole("button", { name: "官方网站" })).toHaveTextContent(
    "",
  );
});

it("shows the same plugin metadata in partner scope without other partner installations", () => {
  render(
    <MemoryRouter>
      <PluginOverview pkg={pkg} scope="partner" />
    </MemoryRouter>,
  );
  expect(screen.getByRole("heading", { name: "插件信息" })).toBeVisible();
  expect(screen.getByText("GitHub")).toBeVisible();
  expect(screen.getByText("1.0.1")).toBeVisible();
  expect(screen.getByText("MIT")).toBeVisible();
  expect(screen.getByText("Manage repositories")).toBeVisible();
  expect(
    screen.queryByRole("heading", { name: /^已安装/ }),
  ).not.toBeInTheDocument();
});

it("groups the explanation and plugin information before capabilities", () => {
  render(
    <MemoryRouter>
      <PluginOverview pkg={{ ...pkg, readme: "说明正文" }} />
    </MemoryRouter>,
  );
  expect(
    screen
      .getAllByRole("heading", { level: 2 })
      .map((heading) => heading.textContent),
  ).toEqual(["说明", "插件信息", "技能 7", "连接器 1", "已安装 0"]);
  expect(
    screen.queryByRole("heading", { name: "简介" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("heading", { name: "插件说明" }),
  ).not.toBeInTheDocument();
  expect(screen.getByText("说明正文")).toBeVisible();
});
