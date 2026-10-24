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
    page,
    wakerId,
  }: {
    search: string;
    page: number;
    wakerId: string;
  }) => (
    <section>
      技能内容 {search} {page} {wakerId}
    </section>
  ),
}));
vi.mock("@/modules/connector", () => ({
  ConnectorResourceCollection: () => <section>连接器内容</section>,
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
it("分类、搜索、视图和分页由 URL 恢复，切换时立即展示对应分类", () => {
  render(
    <MemoryRouter
      initialEntries={["/resources?category=skills&search=review&page=2"]}
    >
      <HistoryControls />
      <Routes>
        <Route path="resources" element={<ResourcesPage />} />
      </Routes>
    </MemoryRouter>,
  );
  expect(screen.getByText(/技能内容 review 2/)).toBeVisible();
  fireEvent.click(screen.getByRole("radio", { name: "插件" }));
  expect(screen.queryByText(/技能内容/)).not.toBeInTheDocument();
  expect(screen.getByText(/插件内容 catalog/)).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "我的插件" }));
  expect(screen.getByText(/插件内容 mine/)).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "历史返回" }));
  fireEvent.click(screen.getByRole("button", { name: "历史返回" }));
  expect(screen.getByText(/技能内容 review 2/)).toBeVisible();
  expect(screen.getByRole("status")).toHaveTextContent(
    "category=skills&search=review&page=2",
  );
});
