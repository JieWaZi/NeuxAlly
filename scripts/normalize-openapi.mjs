import { readFile, writeFile } from "node:fs/promises";

const [filePath] = process.argv.slice(2);
if (!filePath) throw new Error("OpenAPI 文件路径不能为空");

const document = JSON.parse(await readFile(filePath, "utf8"));
const schemas = document.components?.schemas;
if (schemas && typeof schemas === "object") {
  for (const [name, schema] of Object.entries(schemas)) {
    if (schema === null) schemas[name] = {};
    if (name === "GithubComGinGonicGinHandlerFunc") schemas[name] = {};
  }
}

function normalizeParameterOrder(value) {
  if (Array.isArray(value)) {
    for (const item of value) normalizeParameterOrder(item);
    return;
  }
  if (!value || typeof value !== "object") return;
  if (Array.isArray(value.parameters)) {
    value.parameters.sort((left, right) =>
      `${left.in ?? ""}:${left.name ?? ""}`.localeCompare(
        `${right.in ?? ""}:${right.name ?? ""}`,
      ),
    );
  }
  for (const child of Object.values(value)) normalizeParameterOrder(child);
}

function normalizeUnspecifiedEnumValues(value) {
  if (Array.isArray(value)) {
    for (const item of value) normalizeUnspecifiedEnumValues(item);
    return;
  }
  if (!value || typeof value !== "object") return;

  const enumLabels = value["x-enum-labels"];
  // toolx 把空字符串暂记为 unknown；同时存在真实 unknown 成员时，标签映射会被覆盖。
  const unspecifiedLine =
    typeof value.description === "string"
      ? value.description
          .split("\n")
          .find(
            (line) =>
              line.startsWith("* `unknown` - ") && line.includes("Unspecified"),
          )
      : undefined;
  const unknownLabel =
    typeof enumLabels?.unknown === "string" &&
    enumLabels.unknown.includes("Unspecified")
      ? enumLabels.unknown
      : unspecifiedLine?.slice("* `unknown` - ".length);
  const unknownIndex = Array.isArray(value.enum)
    ? value.enum.indexOf("unknown")
    : -1;
  if (
    value.type === "string" &&
    unknownIndex >= 0 &&
    typeof unknownLabel === "string" &&
    unknownLabel.includes("Unspecified")
  ) {
    value.enum[unknownIndex] = "";
    value["x-enum-labels"] = Object.fromEntries([
      ["", unknownLabel],
      ...Object.entries(enumLabels ?? {}).filter(
        ([key]) => key !== "unknown" || value.enum.includes("unknown"),
      ),
    ]);
    if (typeof value.description === "string") {
      value.description = value.description.replace(
        `* \`unknown\` - ${unknownLabel}`,
        `* \`\` - ${unknownLabel}`,
      );
    }
  }

  const additionalProperties = value.additionalProperties;
  if (
    additionalProperties &&
    typeof additionalProperties === "object" &&
    !Array.isArray(additionalProperties) &&
    Object.keys(additionalProperties).length === 0
  ) {
    value.additionalProperties = true;
  }

  for (const child of Object.values(value)) {
    normalizeUnspecifiedEnumValues(child);
  }
}

normalizeParameterOrder(document);
normalizeUnspecifiedEnumValues(document);

await writeFile(filePath, `${JSON.stringify(document, null, 2)}\n`);
