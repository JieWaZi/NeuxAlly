import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@/shared/i18n";
import { PluginImportDialog } from "./plugin-import-dialog";

const fixture = vi.hoisted(() => ({
  scan: vi.fn(),
  prepare: vi.fn(),
}));
vi.mock("../api/plugin-queries", () => ({
  scanPluginMutation: () => ({ mutationFn: fixture.scan }),
  preparePluginMutation: () => ({ mutationFn: fixture.prepare }),
  invalidatePluginQueries: async () => undefined,
}));
vi.mock("@/shared/desktop", () => ({
  selectDirectory: vi.fn(),
  selectFiles: vi.fn(),
}));
afterEach(cleanup);
beforeEach(() => {
  fixture.scan.mockReset().mockResolvedValue({
    token: "staged-snapshot",
    package: {
      format: "codex",
      manifest: {
        name: "github",
        version: "1.0.0",
        author: { name: "GitHub" },
        license: "MIT",
      },
      presentation: {
        displayName: "GitHub",
        shortDescription: "Repository tools",
      },
      skills: [
        { key: "skills/review", name: "review", description: "Review code" },
      ],
      servers: [
        {
          key: "github",
          type: "streamable-http",
          url: "https://example.com/mcp",
        },
      ],
      diagnostics: [{ component: "hooks", code: "unsupported_component" }],
    },
  });
  fixture.prepare
    .mockReset()
    .mockResolvedValue({ plugin: { id: "existing-plugin" } });
});
function show() {
  const onPrepared = vi.fn();
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <PluginImportDialog
        targetPluginId="existing-plugin"
        onClose={vi.fn()}
        onPrepared={onPrepared}
      />
    </QueryClientProvider>,
  );
  return onPrepared;
}
describe("plugin package import", () => {
  it("shows original metadata and requires explicit MCP trust before adding a version", async () => {
    const onPrepared = show();
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "/plugins/github.zip" },
    });
    fireEvent.click(screen.getByRole("button", { name: "检查内容" }));
    expect(await screen.findByText("MIT")).toBeInTheDocument();
    expect(screen.getByText("此组件暂不支持，不会启用")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "添加到本机" })).toBeDisabled();
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByRole("button", { name: "添加到本机" }));
    await waitFor(() =>
      expect(fixture.prepare).toHaveBeenCalledWith(
        expect.objectContaining({
          body: {
            token: "staged-snapshot",
            ownerWakerId: "",
            pluginId: "existing-plugin",
            trustConfirmed: true,
          },
        }),
        expect.anything(),
      ),
    );
    await waitFor(() => expect(onPrepared).toHaveBeenCalled());
  });
  it("invalidates the reviewed snapshot when the source changes", async () => {
    show();
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "/plugins/github.zip" },
    });
    fireEvent.click(screen.getByRole("button", { name: "检查内容" }));
    await screen.findByRole("checkbox");
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "/plugins/other.zip" },
    });
    expect(screen.getByRole("button", { name: "添加到本机" })).toBeDisabled();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(fixture.prepare).not.toHaveBeenCalled();
  });
});
