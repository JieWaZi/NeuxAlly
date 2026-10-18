import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import {
  MemoryRouter,
  Routes,
  Route,
  useNavigate,
  useLocation,
} from "react-router-dom";
import { afterEach, it, expect, vi } from "vitest";
import "@/shared/i18n";
import type * as PluginModule from "@/modules/plugin";
import { ResourcesPage } from "./resources-page";
vi.mock("@/modules/plugin", async (original) => ({
  ...(await original<typeof PluginModule>()),
  usePluginComponents: () => ({ items: [] }),
  PluginComponentCollection: () => <section>插件能力</section>,
  PluginResourceCollection: ({
    view,
    onViewChange,
  }: {
    view: string;
    onViewChange: (value: string) => void;
  }) => (
    <section>
      插件内容 {view}
      <button onClick={() => onViewChange("mine")}>我的插件</button>
    </section>
  ),
}));
vi.mock("@/modules/skill", () => ({
  SkillResourceCollection: ({
    search,
    source,
    wakerId,
    adding,
  }: {
    search: string;
    source: string;
    wakerId: string;
    adding: boolean;
  }) => (
    <section>
      技能内容 {search} {source} {wakerId}
      {adding && <p>技能导入</p>}
    </section>
  ),
}));
vi.mock("@/modules/connector", () => ({
  ConnectorResourceCollection: ({ adding }: { adding: boolean }) => (
    <section>连接器内容{adding && <p>连接器创建</p>}</section>
  ),
}));
afterEach(cleanup);
function HistoryControls() {
  const navigate = useNavigate();
  const location = useLocation();
  return (
    <>
      <output>{location.search}</output>
      <button onClick={() => navigate(-1)}>历史返回</button>
    </>
  );
}
it("分类、搜索、视图和来源由 URL 恢复，切换时立即展示对应分类", () => {
  render(
    <MemoryRouter
      initialEntries={[
        "/resources?category=skills&search=review&source=independent",
      ]}
    >
      <HistoryControls />
      <Routes>
        <Route path="resources" element={<ResourcesPage />} />
      </Routes>
    </MemoryRouter>,
  );
  expect(screen.getByText(/技能内容 review independent/)).toBeVisible();
  fireEvent.click(screen.getByRole("radio", { name: "插件" }));
  expect(screen.queryByText(/技能内容/)).not.toBeInTheDocument();
  expect(screen.getByText(/插件内容 catalog/)).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "我的插件" }));
  expect(screen.getByText(/插件内容 mine/)).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "历史返回" }));
  fireEvent.click(screen.getByRole("button", { name: "历史返回" }));
  expect(screen.getByText(/技能内容 review independent/)).toBeVisible();
  expect(screen.getByRole("status")).toHaveTextContent(
    "category=skills&search=review&source=independent",
  );
});

it.each([
  ["/resources?category=skills", "添加技能", "技能导入"],
  ["/resources?category=connectors", "添加连接器", "连接器创建"],
  ["/wakers/w1/resources?category=skills", "添加技能", "技能导入"],
])("%s 的添加操作位于页首，并打开对应流程", (entry, label, content) => {
  render(
    <MemoryRouter initialEntries={[entry]}>
      <Routes>
        <Route path="resources" element={<ResourcesPage />} />
        <Route path="wakers/:wakerId/resources" element={<ResourcesPage />} />
      </Routes>
    </MemoryRouter>,
  );
  const button = screen.getByRole("button", { name: label });
  expect(button.closest("header")).not.toBeNull();
  expect(button).toHaveClass("bg-button-primary");
  expect(
    screen.queryByRole("button", { name: "安装" }),
  ).not.toBeInTheDocument();
  fireEvent.click(button);
  expect(screen.getByText(content)).toBeVisible();
});
