# neux-knowledge

[English](README.md)

基于 Tencent WeKnora 固定提交 `8776a2d92d11ad3e76865d51cf3192e37e2a9976` 适配的独立 SQLite 知识核心。原件字节、Wiki、修订、原始文档索引和知识任务诊断由核心保存，宿主通过[公开 SDK](docs/sdk-contract.md)提供实际 Reader、直接文本模型和可选嵌入能力。

Lite 管线同库批次串行，默认每批5份文档、Map并发10、Reduce并发10、引用并发4。每次模型请求独立上下文。核心解析原始回复，处理有界重试与页改写续写，按页发布成功结果，待文档待办收束后统一维护索引和受影响链接，不创建聊天或通用执行。

- [SDK 与生命周期合同](docs/sdk-contract.md)
- [只读核查与 Markdown 导出](docs/audit-export-cn.md)
- [来源永久清理](docs/source-purge-cn.md)和[整库永久删除](docs/library-delete-cn.md)
- [远端服务](docs/remote-server-cn.md)与[官方 WeKnora 适配](remote/README.md)
- [内容原则](rules/weknora/principles.md)
- [真实来源与适配范围](upstream/pipeline-manifest.json)、[第三方说明](THIRD_PARTY_NOTICES.md)

CLI 暴露已持久业务数据，当前操作通过 `go run ./cmd/neux-knowledge discover` 发现。生成必须显式注入宿主模型；独立 HTTP 服务不自行配置模型。

需要 Go 1.25.8 或更高版本，验证命令为 `go test ./...`、`go vet ./...`、`go test -race ./...`。自动化覆盖可控模型、真实 SQLite 和本地 HTTP 合同，不能替代真实模型语义、服务商和目标平台验收。
