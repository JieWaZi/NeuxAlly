import { describe, expect, it } from "vitest";
import {
  displayFilePath,
  localFileURL,
  parseReference,
  referenceHref,
} from "./file-reference";

describe("统一文件引用语法", () => {
  it.each([
    ["/any root/无后缀", "/any root/无后缀"],
    ["~/Desktop/share/a.txt", "~/Desktop/share/a.txt"],
    ["./目录/a.custom", "./目录/a.custom"],
    ["../a.txt", "../a.txt"],
    [".gitignore", ".gitignore"],
    ["C:\\work\\a.txt", "C:\\work\\a.txt"],
  ])("不同入口共享路径识别：%s", (text, path) => {
    for (const source of ["inline", "link", "path"] as const)
      expect(parseReference(text, source)).toMatchObject({
        kind: "file",
        path,
      });
  });
  it("字面路径不按 URL 解码，URL 解码一次并保留行号", () => {
    expect(parseReference("/tmp/100%20.txt", "inline")).toMatchObject({
      path: "/tmp/100%20.txt",
    });
    expect(parseReference("file:///tmp/100%2520.txt#L12-L15")).toMatchObject({
      path: "/tmp/100%20.txt",
      line: 12,
      endLine: 15,
    });
    expect(parseReference("./src/a.ts:12:3", "inline")).toMatchObject({
      path: "./src/a.ts",
      line: 12,
      column: 3,
    });
  });
  it("不把变量、命令、锚点和不支持的协议转换为文件", () => {
    for (const value of [
      "a",
      "python3 -m http.server 8000",
      "#section",
      "javascript:alert(1)",
    ])
      expect(parseReference(value, "inline")).toBeNull();
  });
  it("引用传递和本地 URL 编码不丢失特殊字符", () => {
    const file = parseReference("/tmp/a #1%25.txt", "path");
    if (!file) throw new Error("预期文件");
    expect(parseReference(referenceHref(file))).toMatchObject({
      path: "/tmp/a #1%25.txt",
    });
    expect(parseReference(localFileURL("/tmp/a #1%25.txt"))).toMatchObject({
      path: "/tmp/a #1%25.txt",
    });
  });
});
describe("真实路径展示", () => {
  it("项目根目录展示真实目录名，同名来源保留完整路径", () => {
    expect(displayFilePath("/work/share", ["/work/share"])).toBe("share");
    expect(displayFilePath("/work/share", ["/work/share", "/work/other"])).toBe(
      "share",
    );
    expect(
      displayFilePath("/work/share", ["/work/share", "/other/share"]),
    ).toBe("/work/share");
  });
  it("相对项目展示，多来源明确区分，无法相对化保留真实完整路径", () => {
    expect(displayFilePath("/work/share/src/a.txt", ["/work/share"])).toBe(
      "src/a.txt",
    );
    expect(
      displayFilePath("/work/share/src/a.txt", ["/work/share", "/work/other"]),
    ).toBe("share/src/a.txt");
    expect(
      displayFilePath("/work/share/src/a.txt", ["/work/share", "/other/share"]),
    ).toBe("/work/share/src/a.txt");
    expect(displayFilePath("/outside/a.txt", ["/work/share"])).toBe(
      "/outside/a.txt",
    );
  });
});
