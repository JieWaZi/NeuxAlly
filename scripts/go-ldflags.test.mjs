import assert from "node:assert/strict";
import test from "node:test";
import { allyCoreLdflags, applicationVersionSymbol } from "./go-ldflags.mjs";

test("本地构建只去掉符号和调试信息", () => {
  assert.equal(allyCoreLdflags(""), "-s -w");
});

test("发布构建把语义版本写入 Core", () => {
  assert.equal(
    allyCoreLdflags("0.1.0"),
    `-s -w -X=${applicationVersionSymbol}=0.1.0`,
  );
});

test("拒绝 tag 前缀和预发布号", () => {
  for (const version of ["v0.1.0", "0.1", "0.1.0-beta", "0.1.0 ", "dev"]) {
    assert.throws(() => allyCoreLdflags(version), /ALLY_VERSION/);
  }
});
