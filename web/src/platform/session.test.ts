import { afterEach, describe, expect, it, vi } from "vitest";
import { requestSession, SessionError } from "./session";

afterEach(() => vi.unstubAllGlobals());
describe("本机 Web 会话", () => {
  it("配对凭据只进入 POST 正文，Cookie 使用同源策略", async () => {
    const session = {
      protocolVersion: 1,
      csrfToken: "csrf",
      expiresAt: 123,
      capabilities: { workspace: false, git: false, terminal: false },
    };
    const fetch = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify(session), { status: 200 }),
      );
    vi.stubGlobal("fetch", fetch);
    expect(await requestSession("one-time-code")).toEqual(session);
    const [url, init] = fetch.mock.calls[0]!;
    expect(url).toBe("/api/web/session");
    expect(init.method).toBe("POST");
    expect(init.credentials).toBe("same-origin");
    expect(init.body).toBe(JSON.stringify({ code: "one-time-code" }));
    expect(init.headers.Authorization).toBeUndefined();
  });
  it("刷新复用 Cookie，401 保持认证失效语义", async () => {
    const fetch = vi
      .fn()
      .mockResolvedValue(new Response(null, { status: 401 }));
    vi.stubGlobal("fetch", fetch);
    await expect(requestSession()).rejects.toEqual(new SessionError(401));
    expect(fetch.mock.calls[0]![1].method).toBe("GET");
    expect(fetch.mock.calls[0]![1].body).toBeUndefined();
  });
  it("拒绝不匹配的网页协议", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            protocolVersion: 2,
            csrfToken: "csrf",
            expiresAt: 123,
            capabilities: {},
          }),
        ),
      ),
    );
    await expect(requestSession()).rejects.toEqual(new SessionError(409));
  });
});
