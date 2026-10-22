import { defaultLibraryChunkingConfig } from "./knowledge-model";
import { describe, expect, it } from "vitest";
import { buildCreateLibraryBody } from "./knowledge-library-write-body";
import {
  readLibraryIndexingStrategy,
  readLibraryWikiSettings,
} from "./knowledge-library-config";
import type { GithubComJieWaZiNeuxAllyServerApisKnowledgeLibrary as LibraryDTO } from "@/shared/api/generated/types.gen";

describe("knowledge-library-config", () => {
  it("reads nested wikiConfig and indexingStrategy from library DTO", () => {
    const dto = {
      id: "lib",
      purpose: "用途",
      wikiConfig: {
        extraction_granularity: "exhaustive",
        content_instructions: "内容要求",
        extraction_instructions: "提取重点",
      },
      indexingStrategy: {
        wikiEnabled: true,
        vectorEnabled: true,
        keywordEnabled: false,
      },
      embeddingModelId: "embed-1",
    } as LibraryDTO;
    expect(readLibraryWikiSettings(dto)).toEqual({
      extractionGranularity: "exhaustive",
      contentInstructions: "内容要求",
      extractionInstructions: "提取重点",
    });
    expect(readLibraryIndexingStrategy(dto)).toEqual({
      wikiEnabled: true,
      vectorEnabled: true,
      keywordEnabled: false,
    });
  });

  it("builds create body for general libraries with wiki and indexing", () => {
    const body = buildCreateLibraryBody("req", {
      chunkingConfig: defaultLibraryChunkingConfig(),
      name: "库名",
      description: "用途",
      kind: "general",
      language: "zh-CN",
      wiki: {
        extractionGranularity: "focused",
        contentInstructions: "a",
        extractionInstructions: "b",
      },
      indexing: {
        wikiEnabled: true,
        vectorEnabled: true,
        keywordEnabled: true,
      },
      embeddingModelId: "text-embedding-3-small",
    });
    expect(body.wikiConfig).toEqual({
      extraction_granularity: "focused",
      content_instructions: "a",
      extraction_instructions: "b",
    });
    expect(body.indexingStrategy).toMatchObject({
      wikiEnabled: true,
      vectorEnabled: true,
    });
    expect(body.embeddingModelId).toBe("text-embedding-3-small");
  });
});
