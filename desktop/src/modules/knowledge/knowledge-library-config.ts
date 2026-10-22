import type {
  CreateLibraryData,
  UpdateLibraryData,
} from "@/shared/api/generated/types.gen";
import type { GithubComJieWaZiNeuxAllyServerApisKnowledgeLibrary as LibraryDTO } from "@/shared/api/generated/types.gen";
import type {
  ExtractionGranularity,
  LibraryChunkingConfig,
  LibraryIndexingStrategy,
  LibraryWikiSettings,
} from "./knowledge-model";
import { defaultLibraryChunkingConfig, type Library } from "./knowledge-model";

const granularityValues: ExtractionGranularity[] = [
  "focused",
  "standard",
  "exhaustive",
];

function readGranularity(value: string | undefined): ExtractionGranularity {
  return granularityValues.find((item) => item === value) ?? "standard";
}

export function readLibraryWikiSettings(dto: LibraryDTO): LibraryWikiSettings {
  const cfg = dto.wikiConfig;
  return {
    extractionGranularity: readGranularity(cfg?.extraction_granularity),
    contentInstructions: cfg?.content_instructions ?? "",
    extractionInstructions: cfg?.extraction_instructions ?? "",
  };
}

export function readLibraryIndexingStrategy(
  dto: LibraryDTO,
): LibraryIndexingStrategy {
  const strategy = dto.indexingStrategy;
  return {
    wikiEnabled: strategy?.wikiEnabled ?? true,
    vectorEnabled: strategy?.vectorEnabled ?? false,
    keywordEnabled: strategy?.keywordEnabled ?? false,
  };
}

export function readEmbeddingModelId(dto: LibraryDTO): string {
  return dto.embeddingModelId?.trim() ?? "";
}

export function readLibraryChunkingConfig(
  dto: LibraryDTO,
): LibraryChunkingConfig {
  const base = defaultLibraryChunkingConfig();
  const cfg = dto.chunkingConfig;
  return {
    chunk_size: cfg?.chunk_size ?? base.chunk_size,
    chunk_overlap: cfg?.chunk_overlap ?? base.chunk_overlap,
    separators: cfg?.separators ?? base.separators,
    enable_parent_child: cfg?.enable_parent_child ?? base.enable_parent_child,
    parent_chunk_size: cfg?.parent_chunk_size ?? base.parent_chunk_size,
    child_chunk_size: cfg?.child_chunk_size ?? base.child_chunk_size,
    strategy:
      cfg?.strategy === "auto" ||
      cfg?.strategy === "heading" ||
      cfg?.strategy === "heuristic" ||
      cfg?.strategy === "legacy"
        ? cfg.strategy
        : "",
    token_limit: cfg?.token_limit ?? base.token_limit,
    languages: cfg?.languages ?? base.languages,
  };
}

export function buildUpdateLibraryBody(
  requestId: string,
  expectedVersion: number,
  library: Library,
): UpdateLibraryData["body"] {
  const config = buildCreateLibraryBody(requestId, library);
  return {
    requestId,
    expectedVersion,
    name: config.name,
    purpose: config.purpose,
    archived: false,
    policyVersion: library.policyVersion,
    chunkingConfig: config.chunkingConfig,
    wikiConfig: config.wikiConfig,
    indexingStrategy: config.indexingStrategy,
    embeddingModelId: library.embeddingModelId,
  };
}

type CreateLibraryBody = CreateLibraryData["body"] & {
  chunkingConfig: LibraryChunkingConfig;
};

export function buildCreateLibraryBody(
  requestId: string,
  library: {
    name: string;
    description: string;
    kind: string;
    language: string;
    wiki: LibraryWikiSettings;
    indexing: LibraryIndexingStrategy;
    embeddingModelId?: string;
    chunkingConfig: LibraryChunkingConfig;
  },
): CreateLibraryBody {
  const body: CreateLibraryBody = {
    requestId,
    name: library.name,
    wikiLanguage: library.language,
    purpose: library.description,
    chunkingConfig: library.chunkingConfig,
  };
  if (library.kind === "general") {
    body.wikiConfig = {
      extraction_granularity: library.wiki.extractionGranularity,
      content_instructions: library.wiki.contentInstructions,
      extraction_instructions: library.wiki.extractionInstructions,
    };
    body.indexingStrategy = {
      wikiEnabled: library.indexing.wikiEnabled,
      vectorEnabled: library.indexing.vectorEnabled,
      keywordEnabled: library.indexing.keywordEnabled,
      graphEnabled: false,
    };
    if (library.indexing.vectorEnabled && library.embeddingModelId) {
      body.embeddingModelId = library.embeddingModelId;
    }
  }
  return body;
}
