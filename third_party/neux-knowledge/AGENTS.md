# neux-knowledge 开发约定

- 独立 Go 知识核心，SQLite 是唯一权威，公开入口为 core；禁止依赖 NeuxAlly internal。
- Go 1.25.8；默认 SQLite，不设置 WAL、busy_timeout、foreign_keys 或连接池参数；默认单表查询。
- 原件保持原字节，上传不启动模型；模型原文、阶段诊断与可信页面发布分别持久化；不创建聊天或通用执行。
- 所有写入显式 requestId 与期望版本；内容、历史、来源结论和回执原子发布；取消、旧 attempt、权限及结构冲突不得发布。
- 中文注释及文档，每个 Go 结构体字段有准确中文注释；上游版权原文保留。
- WeKnora 固定 8776a2d92d11ad3e76865d51cf3192e37e2a9976；真实迁移登记 upstream/manifest.json，保留完整许可；不迁 WikiMesh 代码。
- 不提交或推送；不修改其他任务独占目录。
- 修改前先写实际失败测试；交付运行 gofmt、go test ./...、go vet ./...、go test -race ./...。
