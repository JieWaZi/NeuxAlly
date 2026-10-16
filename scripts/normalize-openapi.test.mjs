import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

test("空枚举与真实 unknown 同时存在时保留各自的契约", () => {
  const directory = mkdtempSync(path.join(tmpdir(), "ally-openapi-"));
  try {
    const file = path.join(directory, "spec.json");
    writeFileSync(
      file,
      JSON.stringify({
        components: {
          schemas: {
            Content: { properties: { description: { type: "string" } } },
            State: {
              type: "string",
              enum: ["unknown", "pending", "unknown"],
              description:
                "* `unknown` - StateUnspecified 空值\n* `pending` - 等待\n* `unknown` - StateUnknown 不明\n",
              "x-enum-labels": {
                pending: "等待",
                unknown: "StateUnknown 不明",
              },
            },
          },
        },
      }),
    );
    execFileSync(process.execPath, [
      fileURLToPath(new URL("./normalize-openapi.mjs", import.meta.url)),
      file,
    ]);
    const schema = JSON.parse(readFileSync(file, "utf8")).components.schemas
      .State;
    assert.deepEqual(schema.enum, ["", "pending", "unknown"]);
    assert.equal(schema["x-enum-labels"][""], "StateUnspecified 空值");
    assert.equal(schema["x-enum-labels"].unknown, "StateUnknown 不明");
    assert.match(schema.description, /\* `` - StateUnspecified/);
    assert.match(schema.description, /\* `unknown` - StateUnknown/);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});
