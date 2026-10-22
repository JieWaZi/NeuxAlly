# WeKnora Lite 代码来源与适配边界

固定上游为 Tencent/WeKnora 提交 `8776a2d92d11ad3e76865d51cf3192e37e2a9976`。整理以该版本 Lite 的方法与提示词为基础，CLI 适配负责独立模型调用。当前架构见[知识核心与宿主接入](../architecture/knowledge-cn.md)，逐文件来源、摘要和必要修改见[迁移清单](../../third_party/neux-knowledge/upstream/manifest.json)。清单记录来源，不代替行为回归。

## 核对入口

| 环节 | 固定上游方法或文件 | 本地职责 |
| --- | --- | --- |
| 批次与局部重试 | `wiki_ingest_batch.go` 的 `ProcessWikiIngest` | SQLite 持久待办、同库串行批次及取消屏障；保留上游批次与阶段并发语义 |
| 文档处理 | `mapOneDocument`、`legacyExtract` | 原件重建、提取回退、文档去重、摘要与引用并发、重解析贡献撤回 |
| 引用 | `wiki_ingest_cite.go` | 原分批、短句柄、引用合并与批次失败隔离 |
| 身份归并 | `wiki_ingest.go` 的 `deduplicateExtractedBatch` | 同批和已有页面身份解析；持久化适配本地 SQLite |
| 目录规划 | `wiki_taxonomy.go` | 原目录规划与 assignment 规则；适配本地目录仓储 |
| 页面合成 | `reduceSlugUpdates` | 原新增与撤回贡献分支、来源上下文、生成失败保留及持久写入失败重排 |
| 索引与链接 | `rebuildIndexPage`、`cleanDeadLinks`、`injectCrossLinks` | 持久收尾范围、索引导言、受影响页的死链和交叉链接处理 |
| 模型模板 | `prompts_wiki.go`、`generate_summary.yaml` | 保留模板及阶段解析，显式构造消息；普通文档摘要与 Wiki 摘要分别处理 |
| 分块与原文重建 | `internal/infrastructure/chunker`、`searchutil.MergeTextChunks` | 复用原算法与偏移语义，避免重叠块拼接重复 |
| 模型传输 | 上游模型 `Chat` 调用 | 核心 `Generator.Generate` → 宿主 `Runtime.GenerateText` → CLI ACP |

上游源文件从[固定提交](https://github.com/Tencent/WeKnora/tree/8776a2d92d11ad3e76865d51cf3192e37e2a9976)读取；不使用上游工作树或后续版本推断。迁入方法的包名、日志、仓储、领域类型与模型端口需要本地适配，不能将此表理解为整个 WeKnora 服务被原样移入。

## 宿主适配

- 本地 SQLite、不可变来源版本、授权、页面版本比较与删除恢复沿知识核心边界实现；不引入上游 Redis、PostgreSQL、多租户服务或对象存储。
- 整理配置只冻结 Runtime、账号与模型，独立于伙伴、Chat 和 Execution；每次模型调用使用独立上下文，不通过 Skill、候选文件或 `next/submit` 交接。
- 核心保留原角色与正文。Codex独立文本路径以原system替换原生基础指令，原user直接发送；仅user时添加已诊断的中性基础指令以满足CLI非空约束。Responses当前把基础指令放在developer消息，不能称为原生system API。assistant续写及其他Runtime仍用文本角色序列化；温度、输出上限和关闭推理是否生效按实际能力记录。
- Codex使用临时原生HOME，只沿用认证/provider，清除全局AGENTS、hooks、MCP、内置skills和模型目录的代码助手指令。真实官方CLI加本地假Responses服务验证最终请求，而非只检查启动参数。Claude使用原生禁工具配置，其他CLI保留现有账号行为；任何观察到的工具或交互均使生成失败。Codex部分原生工具定义和权限/会话元数据仍存在，不能声称完整Chat API等价。
- 受管订阅账号的知识调用等待可取消的账号租约，API key 共享规则不变；Codex 系统 OAuth 的实际种子读取至刷新回写覆盖本进程整次调用。外部 CLI 的并发不由该进程锁或 Lite 的 Map/Reduce 并发配置保证。
- 当前产品沿既有模式调度摘要：开启 Wiki 时生成 Wiki 文档摘要，RAG-only 时生成普通摘要；未增加上游普通摘要与 Wiki 摘要的独立双开关。
- 文件解析保留当前 Markdown、文本 PDF 与 DOCX 能力，未迁入完整 DocReader、OCR 和复杂图表解析服务。关键词和向量检索沿现有本地存储与嵌入接口。
- 页面仅展示资料、Wiki 和图谱；整理结果在来源列表状态中展示，失败原因通过状态提示查看，不提供整理过程或聊天入口。

## 验证口径

对齐验证需要同时核对真实调用顺序、提示词输入、失败分支、来源身份与存储结果。只验证模板字节相同，或只验证某个算法文件存在，都不足以证明端到端一致。自动化与真实环境的验收边界见[验证记录](knowledge-base-acceptance-cn.md)。

本次质量排查中，同一拨测资料的四个知识页来自Pass0实际返回的一实体、三概念，后端未把九个页面丢成四个；引用证据已持久，页面来源与被引用展示另由界面读取。原生账号额外指令与角色扁平化是已查明的模型边界偏差，本轮修复Codex路径；尚未完成同模型、同版本、同配置的真实质量A/B，不能声称生成数量或质量已与参考实例相同。上游生产linkify只对实际页面标题匹配，不新增alias匹配冒称同源修复。Wiki-only无需Embedding；开启当前RAG模式包含向量索引，要求真实Embedding模型。
