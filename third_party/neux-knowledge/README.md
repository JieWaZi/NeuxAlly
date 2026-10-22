# neux-knowledge

[简体中文](README.zh-CN.md)

A standalone SQLite knowledge core adapted from Tencent WeKnora at commit `8776a2d92d11ad3e76865d51cf3192e37e2a9976`. Original source bytes, Wiki pages, revisions, document indexes, and job diagnostics stay in the core. A host supplies document readers, a direct text model, and optional embeddings through the [public SDK](docs/sdk-contract.md).

The Lite pipeline processes one batch per library, with upstream defaults of five documents per batch, ten map workers, ten reduce workers, and four citation requests. Each model request has its own context. The core parses raw replies, handles bounded retries and page continuations, publishes successful pages independently, and converges the index and affected links after pending document work settles. It does not create chats or general executions.

- [SDK and lifecycle contract](docs/sdk-contract.md)
- [Audit and Markdown export](docs/audit-export-cn.md)
- [Permanent source cleanup](docs/source-purge-cn.md) and [library deletion](docs/library-delete-cn.md)
- [Remote service](docs/remote-server-cn.md) and [official WeKnora adapter](remote/README.md)
- [Content principles](rules/weknora/principles.md)
- [Source mapping and adaptations](upstream/pipeline-manifest.json), [third-party notices](THIRD_PARTY_NOTICES.md)

The CLI exposes stored business data; discover its current operations with `go run ./cmd/neux-knowledge discover`. Model generation requires an explicitly injected host. The standalone HTTP server does not configure a model by itself.

Use Go 1.25.8 or later and run `go test ./...`, `go vet ./...`, and `go test -race ./...`. Automated checks cover controlled models, real SQLite, and local HTTP contracts; they do not establish live provider quality or platform acceptance.
