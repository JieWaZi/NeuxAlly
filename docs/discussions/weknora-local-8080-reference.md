# 本地 WeKnora（127.0.0.1:8080）协议参考

本文记录对当前可连通的本地 WeKnora 实例的 HTTP 探测结果，供 NeuxAlly WeKnora-lite 字段与路由对齐。完整 OpenAPI 以运行实例为准，不在此复制全量路径表。

## 探测结论摘要

| 项 | 观测值 |
| --- | --- |
| 根地址 | `http://127.0.0.1:8080/` 返回 WeKnora SPA（`200`，`Content-Type: text/html`） |
| 管理 UI 路由 | 前端 bundle 含 `/platform/knowledge-bases`（知识库列表） |
| API 前缀 | Swagger 声明 `basePath: /api/v1`（实际请求形如 `http://127.0.0.1:8080/api/v1/...`） |
| 机器可读契约 | `GET /swagger/doc.json` 无需认证（约 317 个 path）；`GET /swagger/index.html` 为 Swagger UI |
| 未认证 API | `GET /api/v1` → `401`，正文 `{"error":"Unauthorized: missing authentication"}` |
| 公开配置 | `GET /api/v1/auth/config` → `200`，示例：`{"complex_password_enabled":false,"registration_mode":"self_serve","success":true}` |

探测时间对应用户确认实例可达的环境；未使用真实账号，因而不包含带 JWT 的实网响应样本。下列 JSON 字段名、枚举与语义来自该实例暴露的 Swagger 2.0 定义。

## 认证

Swagger `securityDefinitions`：

| 方式 | Header | 说明 |
| --- | --- | --- |
| Bearer | `Authorization: Bearer {token}` | 用户登录 JWT |
| ApiKeyAuth | `X-API-Key` | 空间 Key 固定所属空间；平台 Key 调空间接口时需同时传 `X-Tenant-ID` |

登录：`POST /api/v1/auth/login`，body 为 `email`、`password`（`password` 最少 6 字符）。成功响应含 `token`、`refresh_token`、`user`、`active_tenant`、`memberships` 等（定义名 `LoginResponse`）。

模型与知识库读写类接口均标注需 Bearer 或 ApiKeyAuth。

## 知识库 REST 入口

与 NeuxAlly 远端适配相关的主路径（均相对于 `/api/v1`）：

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/knowledge-bases` | 列表；可选 query `agent_id`、`creator=mine\|others` |
| POST | `/knowledge-bases` | 创建；body 为完整 `KnowledgeBase` 对象 |
| GET | `/knowledge-bases/{id}` | 详情 |
| PUT | `/knowledge-bases/{id}` | 更新名称、描述与嵌套 `config` |
| PATCH | `/knowledge-bases/{id}` | 部分更新（索引开关、分块、模型、wiki_config 等） |
| DELETE | `/knowledge-bases/{id}` | 删除库及全部内容 |

列表与详情成功时包装为：

```json
{
  "success": true,
  "data": { }
}
```

`data` 元素类型为 `KnowledgeBaseView`（与持久化 `KnowledgeBase` 字段基本一致，另含 `capabilities`、`my_permission` 等视图字段）。

### 知识库类型 `type`

| 值 | 含义 |
| --- | --- |
| `document` | 文档库（省略 `type` 时默认为 document） |
| `faq` | FAQ 库 |
| `wiki` | Wiki 型库 |

### 与索引、模型相关的主要字段（KnowledgeBase / KnowledgeBaseView）

| JSON 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | 知识库 ID |
| `name` | string | 名称（必填） |
| `description` | string | 用户描述 |
| `embedding_model_id` | string | 嵌入模型 ID，来自 `GET /models`；**已有文档后不可修改** |
| `summary_model_id` | string | 摘要模型 ID，亦作自动标签、AI 描述等默认 LLM |
| `vector_store_id` | string | 向量库绑定，**仅创建时可设，之后不可改** |
| `storage_backend_id` | string | 文件存储实例 |
| `indexing_strategy` | object | 见下节 |
| `wiki_config` | object | Wiki 生成配置（见下节） |
| `chunking_config` | object | 分块策略 |
| `faq_config` | object | 仅 FAQ 库 |
| `extract_config` | object | 知识图谱抽取 |
| `capabilities` | object | 只读能力摘要：`vector`、`keyword`、`wiki`、`graph`、`faq` 布尔值 |

创建时的 agent 提示（Swagger `x-agent-hint`）：省略 `type` 即 document；**省略索引开关时默认启用 vector 与 keyword**；`embedding_model_id`、`summary_model_id` 应从 `model.list`（即 `GET /models`）选取；未出现的字段保留服务端默认。

## `indexing_strategy`（RAG 与 Wiki 等管线开关）

对象类型 `IndexingStrategy`，四个独立布尔开关，JSON 字段名如下：

| 字段 | 含义 |
| --- | --- |
| `vector_enabled` | 语义向量嵌入与向量检索 |
| `keyword_enabled` | BM25 / 关键词检索 |
| `wiki_enabled` | 从文档自动生成 Wiki 页面 |
| `graph_enabled` | 知识图谱实体与关系抽取 |

语义（Swagger 描述）：控制该库激活哪些索引管线——向量检索、关键词检索、Wiki 生成、图谱抽取。

**更新语义**

- 在 `PUT` 的 `config.indexing_strategy` 或 `PATCH` 顶层 `indexing_strategy` 中：**`null` / 省略表示不修改**（保留现有策略）。
- 在 `PATCH` 中：**单个布尔字段省略则保持原值**；仅发送需要变更的开关。

**与 RAG / Wiki 的关系（对齐理解）**

- **RAG 侧**：通常对应 `vector_enabled` 与/或 `keyword_enabled`；混合召回调试见 `GET|POST /knowledge-bases/{id}/hybrid-search`（原始向量+关键词，无 rerank）。
- **Wiki 侧**：`wiki_enabled` 打开 Wiki 管线；页面读写走 **`/knowledgebase/{kb_id}/wiki/...`** 前缀（与 `knowledge-bases` 不同，见下文）。
- **图谱**：`graph_enabled` 与 `extract_config` 配合；`capabilities.graph` 反映是否启用。

**嵌入模型何时必需**

- `vector_enabled` 为 true 时，需要有效 `embedding_model_id` 及可用 Embedding 模型配置，否则向量索引与向量检索无法工作。
- `keyword_enabled` 单独为 true 时，理论上可仅依赖 BM25（`capabilities.keyword`），Swagger 未要求 keyword 必须绑定 embedding。
- `GET /knowledge-bases/{id}/move-targets` 说明：可迁移目标库需 **相同 type 且相同 embedding 模型**，暗示向量索引库对 embedding 有强绑定。
- 创建文档库时 agent 提示要求显式设置 `embedding_model_id`（即使默认打开 vector），联调时应配置至少一个 `type: "Embedding"` 的模型。

## `wiki_config`

类型 `WikiConfig`，用于 Wiki 生成与 ingest 并发（文档库在开启 `wiki_enabled` 时同样使用该结构；`type: wiki` 库专用存储）。

| JSON 字段 | 类型 | 枚举 / 约束 | 说明 |
| --- | --- | --- | --- |
| `extraction_granularity` | string | `focused`、`standard`、`exhaustive` | 抽取粒度；**空值等价于 `standard`** |
| `content_instructions` | string | 建议 ≤4000 字节 | 生成 Wiki 正文的语气与侧重；引用格式固定 |
| `extraction_instructions` | string | 建议 ≤4000 字节 | 页面抽取保留哪些概念；输出结构固定 |
| `synthesis_model_id` | string | — | 写 Wiki 页的模型；空则使用 `summary_model_id` |
| `max_pages_per_ingest` | integer | 0–1000，0 表示不限制 | 单次 ingest 创建/更新页面上限 |
| `ingest_batch_size` | integer | 0 用默认 5 | 单批 claim 的 pending 操作数 |
| `ingest_max_inflight` | integer | 0 用默认 4 | 该 KB 并发 ingest 批上限 |
| `ingest_map_parallel` | integer | 0 用默认 10 | Map 阶段并发 |
| `ingest_reduce_parallel` | integer | 0 用默认 10 | Reduce 阶段并发 |

粒度枚举含义（Swagger + 与 NeuxAlly 核心域模型一致）：

- `focused`：更少页面、更紧主题。
- `standard`：默认平衡。
- `exhaustive`：更多 slug、更细覆盖。

## 嵌入与模型配置

### 模型列表

- `GET /api/v1/models`：当前空间全部模型（agent id `model.list`）。
- 响应 `data[]` 为 `ModelResponse`，关键字段：`id`、`name`、`display_name`、`type`、`status`、`parameters`、`capabilities` 等。

### 模型类型 `type`（枚举）

`Embedding`、`Rerank`、`KnowledgeQA`、`VLLM`、`ASR`

知识库应引用 **`Embedding`** 类型记录的 `id` 作为 `embedding_model_id`，**`KnowledgeQA`**（或策略允许的 LLM 类型）作为 `summary_model_id` / `wiki_config.synthesis_model_id`。

### 模型参数中的嵌入项

创建模型 `POST /models` 时 body 含 `parameters`（`ModelParameters`）。与嵌入相关的嵌套字段：

```json
{
  "embedding_parameters": {
    "dimension": 0,
    "supports_dimension_override": false,
    "truncate_prompt_tokens": 0
  },
  "base_url": "https://embedding.example/v1",
  "provider": "openai-compatible",
  "interface_type": "openai"
}
```

（`api_key` 等敏感字段在 Swagger 创建体中存在；示例不含密钥。）

### 连接测试

- `POST /api/v1/initialization/embedding/test`：验证 Embedding 接口并返回向量维度。
- Body 类型 `ModelTestRequest`，必填 `modelName`；可选 `modelId`（从已存模型补全密钥）、`baseUrl`、`provider`、`dimension`、`supportsDimensionOverride` 等。

## Wiki HTTP 前缀（与知识库路径区分）

文档库 / 资料：`/api/v1/knowledge-bases/...`、`/api/v1/knowledge/...`

Wiki（注意 **`knowledgebase` 单数、无连字符**）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/knowledgebase/{kb_id}/wiki/pages` | 页面列表 |
| GET/PUT/DELETE | `/knowledgebase/{kb_id}/wiki/pages/{slug}` | 按 slug 读写；slug 可含 `/` |
| GET | `/knowledgebase/{kb_id}/wiki/revisions/{slug}` | 版本历史 |
| POST | `/knowledgebase/{kb_id}/wiki/revert` | 恢复 |
| GET | `/knowledgebase/{kb_id}/wiki/graph` | 关系图 |
| GET | `/knowledgebase/{kb_id}/wiki/search` | Wiki 搜索 |

单页 `GET` 成功时直接返回 `WikiPageDetail` 对象（**不一定**包在 `{success,data}` 内），与列表接口解码方式不同；NeuxAlly 官方适配层已按此区分。

## 示例请求与响应（虚构数据）

以下 ID 均为占位，仅说明 JSON 形态。

### 创建文档库并打开向量、关键词与 Wiki

```http
POST /api/v1/knowledge-bases HTTP/1.1
Host: 127.0.0.1:8080
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "name": "示例产品文档",
  "description": "用于联调的虚构库",
  "type": "document",
  "embedding_model_id": "mdl_embed_001",
  "summary_model_id": "mdl_chat_001",
  "indexing_strategy": {
    "vector_enabled": true,
    "keyword_enabled": true,
    "wiki_enabled": true,
    "graph_enabled": false
  },
  "wiki_config": {
    "extraction_granularity": "standard",
    "content_instructions": "面向实施工程师，强调操作步骤。",
    "extraction_instructions": "保留接口名、配置项与约束条件。"
  }
}
```

```json
{
  "success": true,
  "data": {
    "id": "kb_abc123",
    "name": "示例产品文档",
    "type": "document",
    "embedding_model_id": "mdl_embed_001",
    "summary_model_id": "mdl_chat_001",
    "indexing_strategy": {
      "vector_enabled": true,
      "keyword_enabled": true,
      "wiki_enabled": true,
      "graph_enabled": false
    },
    "wiki_config": {
      "extraction_granularity": "standard",
      "content_instructions": "面向实施工程师，强调操作步骤。",
      "extraction_instructions": "保留接口名、配置项与约束条件。"
    },
    "capabilities": {
      "vector": true,
      "keyword": true,
      "wiki": true,
      "graph": false,
      "faq": false
    },
    "knowledge_count": 0,
    "chunk_count": 0,
    "processing_count": 0
  }
}
```

### 仅关闭向量索引（PATCH 局部更新）

```http
PATCH /api/v1/knowledge-bases/kb_abc123 HTTP/1.1
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "indexing_strategy": {
    "vector_enabled": false
  }
}
```

未出现的 `keyword_enabled`、`wiki_enabled` 等 **保持原值**。

### 获取模型列表（选取 embedding_model_id）

```http
GET /api/v1/models HTTP/1.1
Authorization: Bearer <access_token>
```

```json
{
  "success": true,
  "data": [
    {
      "id": "mdl_embed_001",
      "name": "text-embedding-example",
      "display_name": "示例 Embedding",
      "type": "Embedding",
      "parameters": {
        "embedding_parameters": {
          "dimension": 1536,
          "supports_dimension_override": false
        }
      }
    }
  ]
}
```

## 与 NeuxAlly WeKnora-lite 对齐要点

1. **双前缀**：业务库 CRUD 用 `knowledge-bases`；Wiki 用 `knowledgebase/{id}/wiki`；lite 路由表须分别登记，不可拼写混用。
2. **字段命名**：索引开关为 snake_case 的 `*_enabled`；Wiki 粒度字段为 `extraction_granularity`，指令为 `content_instructions` / `extraction_instructions`（非 camelCase）。
3. **默认索引**：创建时不传 `indexing_strategy` 时，服务端语义为打开 vector 与 keyword；wiki / graph 需显式开启。
4. **embedding 生命周期**：`embedding_model_id` 在首份文档入库后锁定；lite 若支持“仅关键词”模式，应允许 `vector_enabled: false` 并避免强依赖 embedding。
5. **响应解码**：知识库 JSON 多为 `{success,data}`；部分 Wiki 端点直接返回实体或 204，与 [联合设计中的远端说明](knowledge-base-design-cn.md#远端连接weknora-与-neuxally-web-服务) 一致。
6. **契约来源优先级**：在线实例以 `http://127.0.0.1:8080/swagger/doc.json` 为准；NeuxAlly 仓库内固定 commit 的 adapter 与 `third_party/neux-knowledge/remote/weknora.go` 用于离线 regression，两者版本可能略有漂移，联调应比对 Swagger `info.version` 与部署镜像。

## 未覆盖边界

- 未执行登录，未采集真实 `knowledge-bases` 列表或 Wiki 页面正文。
- 未验证向量库、存储后端、FAQ 导入等扩展路径的实际副作用。
- `POST /auth/auto-setup` 在本实例返回需桌面认证，不能作为无头联调入口。

后续若取得 API Key 或测试账号，应在同文件补充真实（脱敏）响应片段，并运行 `pnpm docs:check` 校验链接。
