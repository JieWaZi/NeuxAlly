import { createHash } from "node:crypto";
import { readFile, realpath } from "node:fs/promises";
import path from "node:path";
import { within } from "./workspace-files";

export async function rendererFile(dist: string, url: string): Promise<string> {
  const parsed = new URL(url);
  if (parsed.protocol !== "ally:" || parsed.hostname !== "app")
    throw new Error("界面资源地址无效");
  const root = await realpath(dist);
  const target = await realpath(
    path.join(root, decodeURIComponent(parsed.pathname)),
  );
  if (!within(root, target)) throw new Error("界面资源超出根目录");
  return target;
}
/** 首帧主题脚本按构建内容授权，页面不能执行额外注入的内联脚本。 */
export async function rendererCSP(file: string): Promise<string> {
  const hashes: string[] = [];
  if (path.extname(file) === ".html") {
    const html = await readFile(file, "utf8");
    for (const match of html.matchAll(
      /<script\b([^>]*)>([\s\S]*?)<\/script>/gi,
    )) {
      if (!/\bsrc\s*=/i.test(match[1] ?? "") && match[2]?.trim())
        hashes.push(
          `'sha256-${createHash("sha256").update(match[2]).digest("base64")}'`,
        );
    }
  }
  return `default-src 'self'; script-src 'self' ${hashes.join(" ")}; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: http://127.0.0.1:*; font-src 'self' data:; connect-src 'self' http://127.0.0.1:*; worker-src 'self' blob:; object-src 'none'; frame-src blob: http: https:`;
}
