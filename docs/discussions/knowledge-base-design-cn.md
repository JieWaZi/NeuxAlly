# 知识库与 Wiki Mesh 联合详细设计

> 状态：待评审设计，不是当前已实现能力。本文给出可实施的首版方案；产品需求以[领域与行为规格](knowledge-base-specs-cn.md)为准。
> 本文按 NeuxAlly 与 Wiki Mesh 的职责组织，不采用 ZDdiv3 的设计模板。实现方案由本次设计提出，未经实现和真实环境验证的部分不标为已通过。

## 已确认的产品定案

| 维度 | 首版行为 |
| --- | --- |
| 资源归属 | 知识库是系统资源，伙伴是使用者，不按伙伴挂载 |
| 内容类型 | 通用文档类和项目软件类；项目类仅在关联项目范围内可用 |
| 导航 | 主题树为主，页面类型作为筛选；两类库共用骨架、分别维护知识规则 |
| 整理入口 | 页面上传只存 raw，再手动整理；聊天明确要求整理附件时沿用当前 CLI 和模型 |
| 工作台 | 左侧目录与内容，右侧维护聊天；续改保持知识库和聊天身份 |
| 存取 | 默认本地；可连接用户部署的远程 Wiki Mesh 文件服务 |
| 知识处理 | 当前智能体读取原件并按 Skill 整理；客户端 CLI 负责查询、编辑、校验、保存与关键词索引 |
| 远程服务 | 仅提供文件工具、导出目录限制、写入 IP 白名单和必要并发保护 |
| 维护触发 | 手动与可选的现有定时任务；代码变化不自动触发模型工作 |
| 原件阅读 | 使用当前智能体已有能力读取文本型 PDF、DOCX、Markdown；不新增专门格式转换层 |
| 来源移除 | 默认归档，保留原件和历史引用；彻底删除另设操作 |
| 检索 | 目录、别名和关键词，不引入 embedding、向量模型或远程搜索服务 |

首版不增加独立调度框架、远程模型执行、组织成员体系、离线编辑双向合并、第三方云文档同步或旧 Demo 自动迁移。

## 两个项目如何装配

### 能力边界

NeuxAlly 负责界面、项目关联、执行授权、Chat、Execution、通知和自动任务。Wiki Mesh 客户端负责知识内容模型、Skill、文件存取、编辑校验和本机索引；原件阅读使用当前智能体既有能力。远程程序只是共享文件后端，不认识伙伴、项目、概念页或实现说明。

~~~mermaid
flowchart LR
  UI["知识库界面"] --> Core["NeuxAlly Core"]
  Agent["当前聊天智能体"] --> CLI["内置 wikimesh CLI"]
  CLI -->|"受管能力调用"| Core
  Core --> Engine["Wiki Mesh 公开知识引擎"]
  Engine --> Rules["内容规则、编辑与校验"]
  Engine --> Index["本机关键词索引"]
  Engine --> Store["统一文件存取"]
  Store --> Local["本地目录"]
  Store --> Remote["用户部署的轻量文件服务"]
  Other["其他客户端"] --> Remote
~~~

本设计选择公开 Go 引擎与 CLI 共用实现：NeuxAlly Core 调用新增的 Wiki Mesh 公开包，避免为了每个页面请求反复启动进程。随应用交付 wikimesh 二进制和 Skill，供智能体及独立命令使用。产品上的“内置插件”不要求新增任意第三方插件框架。

受管 wikimesh CLI 复用 NeuxAlly 已有的能力身份和回执协议，经 Core 校验后进入公开引擎。Core 不再反向调用同一个受管 CLI，避免递归。独立使用 Wiki Mesh 时，CLI 直接使用同一引擎和自己的明确配置，不读取 NeuxAlly 私有数据。

### 现有入口与拟增加职责

| 仓库位置 | 当前依据 | 本次变化 |
| --- | --- | --- |
| NeuxAlly server/internal/agentcapability/contract | 已有命令发现、Schema、执行授权 | 增加 knowledge 操作及精确资源绑定 |
| NeuxAlly server/internal/agentcli | 已有受管 CLI 客户端 | 公开可供 wikimesh 受管模式复用的最小调用契约，不能跨模块导入 internal |
| NeuxAlly server/apis/workflows/send_creation_message.go | 已有真实创作 Chat 与接纳收据 | 复用接纳和恢复机制，不复制工作流业务或 DSL |
| NeuxAlly server/repositories/models/workflow_creation.go | 已有会话与最后保存回执关系 | 知识库独立维护会话关联，消息仍归 Chat |
| NeuxAlly server/repositories/models/automatic_task.go | 已有伙伴目标、配置冻结、运行和调度 | 复用伙伴目标，增加结构化知识维护绑定 |
| NeuxAlly server/internal/knowledge/ | 拟新增领域适配目录 | 范围计算、引擎适配、受管文件与知识会话辅助能力 |
| NeuxAlly server/apis/knowledge/ | 拟新增 API 目录 | 资源、资料、页面、维护会话和连接入口 |
| NeuxAlly server/services/knowledge/ | 拟新增复用业务 | 只提取确由 API、能力调用、自动任务共同使用的流程 |
| NeuxAlly desktop/src/modules/knowledge/ | 拟新增前端模块 | 查询 Hook、页面和工作台装配，复用共享控件及聊天 |
| Wiki Mesh internal/app/wikiapp | 当前页面解析、来源配置依赖本地及类型目录 | 将可复用知识能力整理到公开边界，CLI 保留输入输出编排 |
| Wiki Mesh pkg/wiki | 拟新增公开引擎 | 库、主题、页面、原件引用、变更、查询、校验及结果类型 |
| Wiki Mesh pkg/filestore | 拟新增公开文件契约 | 本地与远程适配、文件版本、原子提交与回执 |
| Wiki Mesh pkg/qmd | 现有关键词检索 SDK | 仅通过公开接口建立本机索引，显式禁用向量与模型 |
| Wiki Mesh internal/app/fileserver | 拟新增服务入口 | 轻量文件 HTTP 传输和部署配置，复用 filestore 的存储实现 |
| Wiki Mesh skills | 当前仅有 DevWiki Demo | 形成共同规则、文档类和软件类任务规则；旧模板不作为冻结约束 |

具体文件按共同变化的职责组织，不机械拆成一方法一文件；NeuxAlly 不导入 Wiki Mesh 的 internal 包。前端不直接接触服务器路径、远程凭据或 Wiki Mesh 数据库。

## Wiki Mesh 接入前需要重设计哪些部分

这次不是只把现有 Skill 打包进应用。现有 Demo 的 Skill、类型目录、局部文件编辑和来源配置，尚不足以承担系统级两类知识及远程共享。

| 部分 | 处理方式 | 接入前需要完成的变化 |
| --- | --- | --- |
| 知识内容模型 | 重设计 | 增加 general／software、稳定知识／页面／来源身份、原件版本、来源关系、内容历史；主题导航替代按 topics／workflows 固定分目录 |
| 页面读写与校验 | 重设计 | 从 topic／workflow 的硬编码目录和字段扩展为规则集；知识校验在客户端，文件版本校验在存取端 |
| Skill 与 references | 重设计 | 共同的查询、读取 raw、增量摄入、编辑、核查和保存规则，加两类内容规则；调整固定目录、重复确认和强制 embedding 步骤 |
| Skill 写入方法 | 重设计 | 从直接编辑正式文件改成 prepare／validate／commit；每轮普通整理在现有授权内直接保存，破坏性动作单独确认 |
| CLI | 保留基础设施、扩展契约 | 复用 Cobra 与集中 i18n，增加 kb 命令族、JSON 输出、主题／页面身份、候选和真实回执；不是只有查询 Skill |
| i18n 与内容语言 | 补齐跨层契约 | 前端复用 i18next，CLI 扩展现有 Catalog 的语言选择，Core 复用 status_error；界面语言、Wiki 编写语言及原件语言分别表达 |
| 知识公开 SDK | 新增稳定边界 | 从 internal/app/wikiapp 提炼公开 pkg/wiki，Core 与独立 CLI 共用，避免 NeuxAlly 导入另一个模块的 internal |
| 本地配置与项目映射 | 重设计 | Wiki Mesh Demo 的 project_slug 不再同时承担 NeuxAlly 项目、知识库和本地路径身份；连接、共享目录和项目关联分别表达 |
| 文件存取与并发 | 新增／统一 | 本地目录与远程工具共用 filestore；完整版本检查、短锁、候选提交、幂等和崩溃恢复 |
| 远程程序 | 新增轻量能力 | 独立文件服务，与现有 agent/session daemon 分开；只传文件，不做知识查询、原件转换或模型整理 |
| qmd 关键词索引 | 优先复用 | 只索引生成后的 Wiki Markdown；调整主题、别名、页面身份和快照映射，缓存每客户端独立 |
| 向量与模型后端 | 首版不接入 | 显式禁用 embedding／rerank／query expansion，不分发 GGUF 或 llama 运行资产；若当前包依赖会引入加载行为，需要拆成可选构建能力并验证 |
| PDF／Word 转换器 | 不新增 | raw 保存原件，交给当前智能体已有阅读能力，最终只编写 Wiki Markdown |
| agent run／daemon | 不接入 NeuxAlly 执行链 | Demo 可独立保留；应用内继续复用已有 Execution、Runtime、Chat 和自动任务 |
| init、repo、check | 随新模型调整 | 初始化生成新元数据与主题骨架；repo 配置表达存取端；check 区分共同结构及两类知识规则，不静默迁移旧 Demo |
| 打包与版本 | 新增受管交付 | 将公开包、CLI、两类 Skill 与内容格式版本一起固定交付；远端文件协议独立版本化 |

可保留的实现需要经适配测试验证，不是全部重写。特别是 pkg/qmd 的关键词能力、Markdown 基础解析、集中 CLI 文案及命令框架，都应先复用；原件阅读直接复用当前智能体能力。

接入验收需要同时证明：独立 CLI 能使用本地及远端库；NeuxAlly 受管调用不越界；普通聊天和维护页读写同一批 Markdown；没有依赖旧目录或旧 Skill 才能成功的隐藏路径。重设计的协议与模型属于本轮方案，是否保留旧独立命令作为 Demo 不影响新库格式，也不建设自动兼容层。

## i18n 与 Wiki 编写语言

本节是新增设计建议。当前 NeuxAlly 使用 i18next，初始化语言和 fallback 均为 zh-CN，源码中也有部分 en-US 资源；这不代表整个应用已经交付完整英文界面。Wiki Mesh 的 internal/ui 已集中保存中文 Catalog，但 Messages 仍返回单一目录，不能据此宣称已有语言切换。

### 三种语言分别处理

| 语言 | 所属事实 | 首版建议 |
| --- | --- | --- |
| displayLocale | 本次界面或终端的展示设置 | 默认 zh-CN；新增知识模块文案提供对应 zh-CN、en-US 资源，复用宿主语言入口，不增加知识库专属界面语言开关 |
| wikiLanguage | 共享知识库的默认编写语言 | 创建时可选，默认取当时界面语言，独立 CLI 未指定时用 zh-CN；持久写入 knowledge.json，不跟随各客户端展示语言变化 |
| 原件语言 | 用户提供的原始内容 | 原样保存，允许同库混合中文和英文资料；来源摘录保留实际原文，不因界面切换重写 raw |

wikiLanguage 使用 BCP 47 语言标签，首版建议提供 zh-CN／en-US 两个编写选项，其他值明确报告当前不支持，后续按能力扩展。它属于智能体编写指令，不能由 CLI 保证模型一定能生成该语言。未知展示语言 fallback 到 zh-CN，不把未知内容语言静默改成另一种 Wiki 编写语言；原件不因内容语言选项而限制为这两种语言。

新建页面的标题、摘要、正文及新增主题默认使用 wikiLanguage；更新旧页面时沿用页面 language，避免同一篇文章因换客户端而反复翻译。用户明确要求翻译时按普通候选变更保存，必要时更新该页 language；修改库默认语言只影响后续新建内容，不批量翻译已有页面。聊天回答使用本轮对话语言，引用的页面身份和原文不因此变化。

### 界面、CLI 与错误

| 边界 | 设计要求 |
| --- | --- |
| 前端资源 | 拟新增 shared/i18n/knowledge.ts，提供 knowledgeZh、knowledgeEn，在 app 的统一 i18n 入口注册 knowledge namespace；语义 Key 具有类型约束，两份新增资源键集合一致 |
| 文案范围 | 列表、创建、连接、上传、主题建议、类型筛选、空／加载／陈旧／失败状态、整理及核查回执、归档及永久删除提示、可访问名称全部进入资源目录 |
| 动态数据 | 库名、已保存主题名、页面标题、原件名和用户正文是内容，按原值展示；不得拿用户字符串当作翻译 Key。pageType、status 等稳定枚举由客户端映射展示标签 |
| 数字与时间 | 日期、相对时间、数字、文件大小按 locale 使用 Intl；数量文案使用 i18next 的 count 规则，不拼接中英文句子 |
| Core | 新增业务错误通过 constants/status_error 的 @errZH／@errEN 定义并按现有生成流程更新；引擎和文件端返回稳定 code、结构化 details，不把底层路径和英文日志直接作为用户提示 |
| CLI | 在现有 internal/ui Catalog 内补语言目录及选择入口，帮助、交互和可读错误不散落到命令；受管模式采用宿主冻结的 displayLocale，独立模式按显式 locale 参数、本机设置、系统语言、zh-CN 的顺序解析 |
| 机器协议 | JSON 字段、枚举、命令、错误码、requestId、身份、文件路径及摘要保持语言无关。可读消息可以本地化，但不能用消息字符串判错；展示语言不参与文件提交摘要或幂等身份 |
| 远端 | 只返回文件操作的稳定错误和参数；各客户端自行展示，不在共享文件中保存某个用户的界面翻译或安装语言包 |

软件主题建议的名称先在创建界面按 displayLocale 展示；真正创建时按 wikiLanguage 写入实际 title，之后按用户内容管理。切换界面语言不自动改主题名、主题 ID 或路径。内置建议使用稳定 presetKey 查找文案，presetKey 不是已保存主题的身份。

### Skill 与跨语言查询

共同语言规则随插件打包，首版保持一份权威 Skill 正文，按任务传入 wikiLanguage、page.language 和对话语言，不复制两套会漂移的中文／英文知识规则。Skill 名称、命令和版本仍为稳定标识；将来翻译说明文本也必须对应同一规则版本。

原件中的标准缩写、代码符号和专有名词保留原文。智能体可为已确认的同一概念补充中英文 aliases，但不能把意思相近的词自动认定为同义词。关键词搜索先使用原查询，必要时使用已保存别名和原文术语再查；首版不承诺无需别名即可跨语言语义召回，也不为此引入 embedding。

## 主题、页面与来源的内容模型

### 稳定身份

所有业务身份采用 32 位无连字符 UUID，时间为 UTC Unix 毫秒；文件摘要采用 SHA-256 小写十六进制。标题、主题名和文件名均不承担身份职责。

| 身份 | 作用域与用途 |
| --- | --- |
| libraryId | NeuxAlly 本机接入记录；用于 UI 和执行授权 |
| knowledgeId | 随知识文件保存的共享库身份，跨客户端保持相同 |
| connectionId / exportId | 本机连接及远程导出根；不等于知识库身份 |
| themeId / pageId | 库内主题和页面身份；移动、改名不改变身份 |
| sourceId / sourceVersionId | 原始资料及不可变原件版本 |
| changeId | 一次知识变更；共享历史不记录私人 Chat 正文 |
| requestId | 一次写请求的幂等身份；重试保持相同值 |
| rootRevision | 文件存取端的单库提交代次；用于防止丢失更新 |
| chatId / executionId | NeuxAlly 真实聊天及执行，不复制成远端执行对象 |

同一 knowledgeId 从不同位置接入时，若是相同存储位置则复用本机记录；若位置不同，返回身份冲突并要求明确处理，不静默当作镜像。普通接入不迁移本地库到远端。

### 主题导航

主题保存 id、title、parentId、order。根主题的 parentId 为空；同父主题名称去首尾空白后唯一，禁止环。主题分组可以没有正文。页面具有一个主主题和零到多个关联主题。

软件类按实际能力和工程关注点组织，例如“能力与业务 → 身份与会话”；通用类按资料中的研究主题组织。初始未分类知识可进入“待归类”，整理时由 Skill 根据已有主题归属，避免每份资料都新建根主题。raw 上传时不要求先选择 Wiki 主题，也不因上传就创建页面。

导航展示实际发布的主题与页面，按 order、title、id 稳定排序。页面类型筛选只影响当前阅读视图，不移动文件、不改变主题关系，也不缩小本次执行的授权集合。

### 软件类主题建议与资料归属

以下是设计者建议的覆盖清单，不是强制生成的一级目录。首版创建软件库时只建立“待归类”，并在空状态提供可选主题建议；用户可以采用、改名或忽略。整理时仅为实际有知识的关注点建立主题，优先复用用户已有结构，不一次生成整棵空树。通用类不套用这份软件主题清单。

| 建议主题 | 主要知识与可生长的子主题 | 常见 raw 依据 |
| --- | --- | --- |
| 能力与业务 | 业务对象、用户能力、规则、边界、状态；如身份与会话、资料管理 | 需求说明、用户故事、业务流程、产品说明、验收条件 |
| 架构与组件 | 系统边界、模块职责、执行链、事件与依赖；如桌面生命周期、任务执行 | 架构图、方案设计、概要／详细设计、模块说明、经核对的代码 |
| 数据与存储 | 数据模型、权威来源、事务、一致性、缓存、备份与迁移 | 数据字典、Schema、存储设计、迁移说明、持久化代码 |
| 接口与集成 | API、CLI、SDK、事件协议、外部系统及兼容约束 | OpenAPI、接口说明、协议、对接记录、第三方 SDK 资料 |
| 安全与权限 | 认证、授权、工作范围、密钥处理、数据保护及威胁边界 | 权限设计、安全方案、威胁分析、已核验安全修复说明 |
| 运行与交付 | 环境配置、构建、发布、部署、监控、运维及故障恢复 | 部署手册、配置说明、CI 配置、监控说明、故障记录、发布资料 |
| 质量与性能 | 测试策略、质量边界、可访问性、容量及性能机制 | 测试设计、验收用例、基准记录、性能报告、兼容性说明 |
| 开发与协作 | 工程规范、本地调试、代码组织、文档约定及贡献方式 | 开发指南、AGENTS、编码约定、贡献说明、本地环境操作资料 |

这些主题也可以直接采用项目已有的领域名称，不要求保留同名外层分组。依赖选择通常归入使用它的组件或集成主题；形成全项目约束时再建立“依赖与兼容”等实际主题。国际化、可访问性等跨领域关注点同样可按知识量独立成主题。决策、术语和故障属于知识表达形式，不统一堆到“会议纪要”“ADR”“术语表”“故障记录”这种来源或类型目录。

判断归属时按以下顺序：用户明确指定的合理主题 → 已有页面的职责和主主题 → 当前知识主要回答的业务或技术问题 → 必要时新增最小主题。一个页面有一个主主题，可关联多个主题；不把同一段正文复制到多个主题。创建、改名或移动普通主题在本次明确维护授权内提交；删除主题或无法判断的重大边界冲突另行处理。

**进入主题的是整理后的知识，原始文件始终在 raw。** 一份文档可以支持多个主题；多份文档也可以共同维护同一页面。资料类型、来源章节与 Wiki pageType 没有一对一映射，具体示例如下：

| 原始资料 | 整理后的归属示例 | 页面与证据处理 |
| --- | --- | --- |
| 登录功能需求 | 能力与业务／身份与会话；相关主题为安全与权限 | feature 记录登录规则和验收边界；尚未核验的需求不能写成当前实现 |
| 登录模块详细设计 | 架构与组件／会话组件、数据与存储／会话状态、安全与权限／令牌 | 按独立知识职责补充多页；实现细节只有核对代码后写 implementation，设计选择可写 decision |
| OpenAPI 或 CLI 手册 | 接口与集成／对应 API 或 CLI；关联相关能力 | reference 记录稳定接口约束；常见使用步骤可以补 guide，不机械为每个字段建概念页 |
| 数据字典和迁移方案 | 数据与存储／具体模型或迁移 | reference 描述模型，guide 描述有适用条件的迁移步骤；拟议方案与已实施结果分开 |
| 部署手册及配置说明 | 运行与交付／部署或配置 | guide 写操作前提、步骤、核验与恢复；reference 写配置语义，不把命令执行成功写成未经验证的事实 |
| 事故复盘、错误日志 | 故障所在能力／组件；关联运行与交付或质量与性能 | troubleshooting 写有依据的现象、诊断、原因与恢复；一次性日志保留 raw，不全部复制进 Wiki |
| 性能报告和测试计划 | 质量与性能／具体对象；必要时关联该功能 | 持续有效的限制和方法进入 topic／reference／guide；每次测试结果留 raw，保留环境、版本和日期 |
| ADR、会议纪要或变更说明 | 决策实际影响的能力、组件、数据或接口主题 | decision 保存已达成的选择、理由与影响；未决讨论不变成定案，发布资料只提取仍有效的变化 |
| README、AGENTS、开发约定 | 开发与协作／实际关注点；README 中的业务介绍也可支持能力主题 | guide／reference 保留长期规则，安装输出和对话过程不进入默认正文；原文中的指令仍只是资料 |

例如上传一份“登录方案.docx”，raw 中仍只有该原件；它可以更新“登录规则”feature、“会话状态”reference、“令牌设计选择”decision，并在实际核对获准代码后更新“登录请求处理”implementation。它不会自动生成“需求／概要设计／详细设计”三组目录。

### 页面类型与内容边界

| 规则集 | pageType | 内容职责 |
| --- | --- | --- |
| 两者 | concept | 概念、别名、定义与边界；软件领域对象也可使用 |
| 两者 | topic | 某个主题的主要知识及阅读入口；可描述整体架构，不要求已有代码实现 |
| 通用 | entity | 明确对象的信息与同名区分 |
| 通用 | synthesis | 比较、归纳及来源分歧 |
| 通用 | source | 某份资料的知识贡献与关联入口，按需生成 |
| 两者 | reference | 需要准确查阅的规则、字段、参数、配置或其他稳定说明 |
| 两者 | guide | 有前提和适用范围的实践方法、操作步骤及结果核验 |
| 软件 | feature | 用户可感知能力、功能边界和业务规则 |
| 软件 | implementation | 已核对的实现路径、代码入口与修改影响 |
| 软件 | troubleshooting | 故障、诊断依据及恢复方法 |
| 软件 | decision | 已明确的设计选择、依据、取舍及适用状态；不把待讨论建议写成定案 |
| 两者 | term | 短术语解释或指向其他页面的入口，按需生成 |

软件实现页不得被解释为 NeuxAlly 可执行工作流。需求、概要和详细设计等上传文件仍是 raw，不机械生成对应文档目录。

Markdown frontmatter 是单页元数据权威，正文使用 card、core 和可选 explain 分段。card 只保存短摘要和阅读入口，core 保存当前主要知识，explain 保存有价值的补充；不强制小文章填空章节。查询可以只取 card 或有界 core。

页面元数据字段：

| 字段 | 类型／默认 | 约束 |
| --- | --- | --- |
| id, title | string，必填 | id 稳定；title 1—200 字符，不要求全库同名唯一 |
| pageType | string，必填 | 属于当前库规则集 |
| language | string，必填 | 页面实际主要编写语言；新页写入库 wikiLanguage，更新旧页沿用；原文摘录可保持来源语言 |
| themeId | string，必填 | 主主题存在 |
| relatedThemeIds, aliases | string[]，默认空 | 去重；别名用于检索，不自动等同为事实 |
| summary | string，默认空 | 最长 1000 字符，保存后参与导航摘要 |
| status | active / archived | 默认 active；历史正文独立保留 |
| relatedPageIds | string[]，默认空 | 库内实际存在，删除或合并时同步处理 |
| sourceRefs | SourceRef[]，默认空 | 绑定资料版本和定位；缺少依据的推断必须明确标注 |
| codeRefs | CodeRef[]，默认空 | 软件实现证据；包含核对的代码版本和相对路径 |
| updatedAt, lastChangeId | 时间与身份 | 客户端提交生成，不能从助手文字推断 |

SourceRef 包含 sourceId、sourceVersionId、locator、excerpt、excerptHash、claimKind。locator 包含 type（page、heading、paragraph、excerpt 或 unknown）及 value，只保存阅读工具实际确认的位置。claimKind 为 source_statement、inference 或 disagreement。CLI 确定性检查资料版本、字段、摘录摘要及关系；摘录是否支持陈述由当前智能体核查，不能声称文件存在就证明语义正确。来源位置不足时不伪造页码、章节或精确引文。

CodeRef 包含 repoBindingId、commitId、path、symbol、startLine、endLine、verifiedAt；没有实际核对则不填 verifiedAt 或声称当前代码一致。相对路径只作为证据，不能扩展执行可访问目录。

内部链接使用稳定 pageId，例如 [[页面身份|显示标题]]；来源链接使用 source 身份与定位标识。标题改名不会破坏关系，渲染器将引用转换为当前库内受管跳转，不执行自定义 URL。

## 文件布局与数据权威

### 库根目录

首版明确一个 exportId 对应一个完整知识库文件根，拥有独立 head、锁、事务和回执；不支持在同一导出根内按子目录再划分多个库。一个服务器可以导出多个独立 exportId。每个文件存储根有 data 和 control 两个同级目录。data 是可经文件工具访问的知识内容；control 是存取端私有事务控制，不向智能体或普通文件工具开放。

~~~text
<root>/
  data/
    knowledge.json
    wiki/
      navigation.json
      pages/<pageId>.md
      catalog.json
    sources/<sourceId>.json
    raw/<sourceId>/<sourceVersionId>/<originalName>
    processing/<sourceVersionId>.json
    history/<changeId>/
      change.json
      before/<pageId>.md
      after/<pageId>.md
  control/
    head.json
    receipts/<request-key-hash>.json
    uploads/<uploadId>
    transactions/<transactionId>/
~~~

主题树是逻辑导航，与物理 pages 目录分离，避免调整主题时搬动大量文件。原件文件名只用于展示及保留原始扩展名；路径段中的来源和版本身份防止重名覆盖。

knowledge.json 保存 schemaVersion=1、knowledgeId、kind、name、description、wikiLanguage、createdAt、updatedAt、rulesVersion、archived。kind 为 general 或 software。navigation.json 保存主题；页面元数据是页面 frontmatter；catalog.json 是客户端派生导航索引，包含页面身份、路径、摘要、类型、language 和内容摘要，可重建。

sources 文件保存资料及版本清单；processing 文件保存某个原件版本的真实阅读与整理结论。原件和 Wiki 是两层正文，不增加必需的转换全文、parserVersion 或标准化位置映射目录。智能体自行使用阅读工具产生的临时文本只是执行材料，不默认发布或索引。

history/change.json 保存 changeId、requestId、previousRevision、时间、修改文件与页面、sourceVersionIds、处理结论和客户端公开身份。修改前后页面及发生变化的导航元数据分别放入 before 和 after，支持读取准确历史与差异；不可变 raw 不再复制进每次历史。私人 Chat、执行凭据、账号与完整项目路径不写入共享历史。

### 本机目录

NeuxAlly 在既有受管文件根下增加：

| 路径 | 职责 |
| --- | --- |
| knowledge/libraries/<knowledgeId>/ | 默认本地库根，包含 data 和 control |
| knowledge/cache/<libraryId>/ | 远程文件快照、已知 revision 和本机索引 |
| knowledge/candidates/<executionId>/<changeId>/ | 候选变更与临时阅读材料，不进入正式检索 |
| knowledge/sessions/<sessionId>/ | 维护执行工作区，消息仍归原 Chat 日志 |

缓存、候选和账号文件不能从普通知识下载 API 获取。应用数据目录保存本机数据库及私有连接凭据，受管文件目录不混入凭据。连接凭据使用独立私有文件、限制本机文件权限，API 只报告是否配置，不回传明文。

默认本地库使用同一文件事务机制，普通读写无需启动服务器。外部编辑器不能绕过客户端提交直接覆盖受管发布文件；需要人工编辑时从 UI 或 CLI 创建候选并保存。独立 Demo 文件接入另走显式导入，不自动“升级”已有目录。

## 统一文件提交：本地与远端共用

本设计选择“短锁、暂存及可恢复日志”，不在首版引入 Git 服务或分布式锁。锁是文件存储根级的进程间锁，不能仅用某个 Core 的内存 mutex；服务器一个导出根只允许一个服务进程打开，启动时持有实例锁。

### 文件契约

公开 filestore 契约提供 Info、List、Read、Stage、Commit、Receipt。每次操作均有 Context 和导出根身份；Read 返回文件内容、sha256、size、rootRevision。List 返回当前根版本及稳定分页，不返回 control 内容。

CommitInput 包含 requestId、expectedRevision、operations 和 payloadDigest，永久删除还可带 purgeTargets。每个普通操作为 write 或 delete，目标是 data 内规范相对路径；write 引用已经 Stage 的 uploadId。新建文件的 expectedHash 为 missing，替换或删除须带原文件摘要。重命名由同一批中的写新路径和删旧路径表达。

rootRevision 是单调递增整数，每次成功 Commit 加一，暂存上传不改变它。所有写入都经过 Commit，包括原件登记、页面、导航及处理结论。并发原件上传也可能使旧候选冲突，首版允许保守冲突，优先保证不会覆盖已有内容。

### 提交顺序

1. 获得存储根排他锁，重新检查请求来源及写入权限。
2. 先按 requestId 读取回执。同键同摘要返回原结果；同键异摘要报 REQUEST_CONFLICT，不能覆盖。
3. 检查 expectedRevision 及所有目标文件摘要、路径、暂存文件摘要和数量限制。
4. 在 control/transactions 中准备完整新文件、旧文件备份和操作清单，刷盘后将日志标记为 prepared。
5. prepared 后不因 HTTP 连接取消而中断存储提交。按清单替换文件、处理删除，写入新 head，保存 committed 回执。
6. 完成必要刷盘后释放锁。异步清理事务暂存不影响已保存历史，回执保留以支持不确定结果重试。

读操作使用同一根的读锁取得一致文件快照。大文件先形成受控读取快照再释放锁，网络流式发送期间不占用发布锁。携带 atRevision 的读取若不再是当前版本，返回 REVISION_CONFLICT，由客户端重新选择快照。

多文件的“原子”是通过文件协议的一致可见性及恢复实现，不声称多次文件系统 rename 天然具有磁盘原子性。服务进程重启后先恢复 prepared 日志，再开放根目录读写；恢复失败将该导出根标为不可用，不能读出半批文件。

### 通用永久文件清理

普通 delete 会保留可恢复的事务材料，不能被报告为彻底删除。文件协议的 CommitInput 可带 purgeTargets，每项绑定目标 path、被删除文件摘要及其上传身份；清空整个导出根使用 purgeAllData。客户端仍负责生成来源 tombstone、引用状态及历史元数据，文件服务只处理文件和残留副本。

该能力声明为 purge-copies，源文件的删除与知识元数据修改先按同一根事务保存，再由持久 purge 清单清理属于目标的暂存上传、已完成事务旧文件备份和受控读取快照。服务端记录暂存、读取快照与提交目标的路径、摘要和上传身份，不能只从正式目录判断是否还有残留。

清理期间关闭目标文件新的读取快照，等待已有受控读取结束或取消；其他客户端以前已经下载的文件不在此服务的管理范围。仍被其他正式文件引用的相同字节不擅自删除，在回执中说明残留属于其他活跃文件；目标范围不扩大成删除其他来源。

回执分别表达文件变更已提交和 purgeState=pending/complete。只有被管理的目标副本清理完成才报告永久删除完成；失败或重启从 purge 清单继续，不能回报普通 delete 成功就结束。事务 prepared 中的材料必须先恢复，不能由回收过程破坏。

purgeAllData 对该导出根加持久清空状态，拒绝其他写入并清理全部 data 与相应内容残留；控制回执可保留不含原件字节的身份和摘要。导出配置由部署者维护，清空内容不会递归删除服务器的其他根。数据量超过普通批次时按清单有界推进，仍属于通用文件清理，不需要知识实体或模型任务。

不支持 purge-copies 的存取端隐藏永久删除能力并返回明确能力不足，归档和普通读写仍可用。本机 filestore 使用同一清理机制，Core 另清理自己管理的缓存和候选。

### 冲突与不确定结果

智能体阅读原件和生成候选可并行。版本冲突后保留原候选，由客户端重新读取当前主题、页面和来源，重新合并及校验；不能只换成最新 revision 后原样覆盖旧候选。无法消解的内容分歧在原聊天追问。

提交超时不等于失败。客户端使用相同 requestId 查询 Receipt；已提交则恢复本机索引，未提交且可证明不存在 prepared 日志时才重试同一请求。UNKNOWN 或恢复中的结果继续显示待对账，不生成新 requestId。

本地和远程都需通过同一竞争测试，保证“两个候选基于同一版本，最多一个首次提交成功”。仅客户端先读版本再普通写文件不能满足此要求。

## 远程端的最小文件协议

首版选择轻量 HTTP JSON 加流式文件上传，不增加标准 MCP SDK 和会话管理。这里采用用户要求的 MCP 式工具边界；未来需要标准 MCP 时可包装同一 filestore 契约，客户端知识模型不随传输变化。

远端配置只包含监听地址、导出根映射、可选访问令牌、写入 IP 白名单、可信代理和资源上限。不配置知识分类、模型、qmd 或 NeuxAlly 项目。读写身份可使用部署者配置的共享令牌；不建设账户和角色管理。

### 工具入口

以下是拟新增协议，所有路径相对文件服务基址：

| 方法与路径 | 输入 | 响应 |
| --- | --- | --- |
| GET /v1/info | 无 | protocolVersion=1、能力、限制及可访问 exportId，不返回真实目录 |
| GET /v1/exports/:exportId/files | prefix、cursor、limit、atRevision | items、nextCursor、rootRevision；条目含 path、size、sha256 |
| GET /v1/exports/:exportId/content | path、可选 atRevision；可用 Range | 原始字节及摘要、版本响应头 |
| POST /v1/exports/:exportId/uploads | 流式字节、内容摘要、大小 | uploadId、sha256、size、expiresAt；只是暂存 |
| POST /v1/exports/:exportId/commits | CommitInput | requestId、rootRevision、changedPaths、committedAt |
| GET /v1/exports/:exportId/receipts/:requestId | 身份 | committed、recovering、absent 及原回执；永久删除还返回 purgeState |

目录分页按 UTF-8 路径字节顺序，limit 默认 100、最大 500，cursor 绑定 prefix 和 rootRevision。翻页期间发生提交返回冲突，客户端重新列举，不能拼接不同版本的“完整清单”。

上传与提交都检查写入 IP。文件路径只接受正斜线相对路径，拒绝绝对路径、空段、父引用、盘符及 control 名称；使用受限目录句柄阻止软链接和路径检查后的替换竞争。文件工具不解释 Markdown 及其 frontmatter。

### 提交示例

~~~json
{
  "requestId": "change-001",
  "expectedRevision": 18,
  "payloadDigest": "sha256-of-normalized-request",
  "operations": [
    {"op": "write", "path": "wiki/pages/0123456789abcdef0123456789abcdef.md", "expectedHash": "old-sha256", "uploadId": "staged-page"},
    {"op": "write", "path": "wiki/catalog.json", "expectedHash": "old-catalog-sha256", "uploadId": "staged-catalog"}
  ]
}
~~~

摘要由规范化后的操作和内容摘要计算，操作路径唯一且稳定排序，拒绝重复 JSON 字段和重复目标路径。版本、路径或摘要检查失败均不改变任何正式文件。

### IP 白名单

白名单关闭时采用部署的普通文件访问范围；开启后，只有命中的来源 IP 或 CIDR 可以执行 Stage、Commit 及其他写动作。开启且为空则拒绝全部远程写入。查询和读取不受“写入白名单”额外限制，仍遵守导出目录及可选访问令牌。

来源采用真实 TCP 对端；仅当对端属于部署配置的可信代理时才解析代理转发链。普通客户端携带任意 IP 请求头不能绕过名单。白名单首版由部署配置管理，不增加远程成员管理页面。

### 文件服务错误

| 状态／错误 | 处理 |
| --- | --- |
| 400 INVALID_PATH / INVALID_REQUEST | 客户端修正参数 |
| 401 AUTH_REQUIRED | 连接凭据需要配置或更新 |
| 403 WRITE_IP_DENIED / EXPORT_DENIED | 保留已读内容，禁用本次写入并显示原因 |
| 404 FILE_NOT_FOUND / EXPORT_NOT_FOUND | 区分资料缺失与整个连接不可用 |
| 409 REVISION_CONFLICT / REQUEST_CONFLICT | 分别重新读取合并或停止复用错误请求身份 |
| 413 LIMIT_EXCEEDED | 保留已保存原件，明确本次限制 |
| 422 HASH_MISMATCH | 暂存内容校验失败，不提交 |
| 503 ROOT_RECOVERING / ROOT_UNAVAILABLE | 等待对账或部署者修复，不能降级为空知识库 |

以上是远端协议码；NeuxAlly 将其映射为本项目 status_error，不将跨项目原始 error 直接透传到界面。

## 客户端查询、缓存与索引

每个客户端维护自己的 qmd 数据库；共享的是知识文件，不是索引数据库。Core 以 libraryId 复用本机引擎实例，同一客户端的伙伴使用同一份库索引。

查询流程：

1. Core 校验当前执行可用 libraryId 和操作范围。
2. Wiki Mesh 读取当前 head，比较本机已物化 revision。需要更新时按一致目录清单下载变更的 Wiki、导航和必要元数据；不为查询整库下载 raw。
3. 在临时缓存快照内验证摘要并更新关键词索引，成功后整体切换本机 activeRevision；失败保留旧快照及明确陈旧状态。
4. 先用主题、别名、标题和 card 定位候选，必要时进入 qmd Search；禁止调用需要 embedding 的 VSearch 或模型重排。
5. 返回 pageId、标题、主题、摘要、实际文件 hash、rootRevision、命中片段和来源入口。需要原件时再按 sourceVersionId 读取。
6. 远端在查询期间再次变化时，返回本次实际快照版本；需要最新结果则重试同步，不把多个代次混在一份结果里。

远端不可用时，普通查询返回明确错误；UI 可以保留已有阅读快照并标记未刷新。CLI 只有显式 allow-stale 时才查询旧缓存，结果携带 stale=true 和最后成功时间。

连续快照同步最多重试三次；写入过于频繁时返回明确冲突或仍在刷新，保留上一份完整快照，不无限重试或发布半份索引。

向量 provider 显式为 none，reranker 和 query expansion 不初始化；索引只包含 active Wiki 正文和有价值的导航元数据。raw、history、control、候选及维护报告默认排除。

主题或别名变更会更新相应本机索引；归档页面移出默认查询。来源归档不自动删除引用它的有效 Wiki，阅读时可见来源已归档但仍能回溯。

不为远程服务建立永久轮询：进入页面、恢复前台、执行查询、提交前后及用户刷新时检查 head；外部客户端的更新在这些检查点呈现。首版不承诺远端编辑即时推送。

## 原件阅读与 LLM Wiki 整理

### 原件登记

页面上传支持 PDF、DOCX、MD，保存原始字节和来源元数据后返回，不启动模型或专门格式转换。使用 requestId 与附件内容身份防止重复入库，不能只凭文件名去重。同名新上传默认是新来源；显式替换才为原 sourceId 新增版本。

来源文件包含 id、displayName、mimeType、status、currentVersionId、versions。版本项包含 id、sha256、size、originalName、createdAt、originKind；远程 origin 只表达 upload 或 chat-import，不共享 Chat 身份。本机另保存真实附件到来源的回执。

来源 status 为 active、archived、purged。每个版本的 processing 结论为 pending、processed、no-change、failed 或 cancelled；进行中的状态从真实 Execution 获取，不能把运行曾启动视为处理成功。

点击整理冻结本批 sourceVersionIds。执行期间新增原件留到下一批；聊天整理只处理用户指定附件，不顺带处理其他 pending 来源。

### 直接阅读原件

已确认流程是 raw 原件 → 当前智能体阅读 → LLM Wiki Markdown。不是 raw → 固定转换器生成全文 Markdown → 再整理 Wiki。

Wiki Mesh 的 sources read 返回受管文件引用、原件身份、版本、摘要和媒体类型。远程原件先按摘要下载到当前客户端的受管资料缓存，再为本次 Execution 注册只读文件引用；模型看不到远端凭据或可任意扩展的文件路径。

NeuxAlly 将原件引用按当前 Runtime 已有的附件／文件能力交付给智能体。PDF 和 Word 的阅读使用该智能体已有工具及其可用的文档 Skill；Wiki Mesh 自身不重复实现 PDF 阅读器、DOCX 解析器或转换服务。

能力检查区分“当前智能体支持”“明确不支持”和“需要执行后才能确认”。明确不支持时整理前给出缺失能力；未知时允许在当前执行中尝试，真实失败后保留原件和原因。不能假定每个 CLI 对所有格式都支持，也不能隐式更换用户选择的智能体、账号或模型。

智能体可以在当前执行中按其阅读工具需要产生临时材料，但这些不成为知识库第三层权威正文。阅读记录至少绑定实际 sourceVersionId，引用尽量使用工具确认的页码、章节或必要摘录。只有标题或元数据时不能声称已经读完原件。

首版资料范围仍为文本型 PDF、DOCX 和 Markdown，不增加 OCR 或复杂图表理解能力。归档后的原件继续按权限可读；purged 原件返回明确 unavailable。

### 整理规则

Skill 先读现有主题、别名和相关页面，再决定补充、独立新增、合并或保留分歧。通用类提取概念、对象及主题关系；软件类区分需求、已决设计及当前实现，涉及实现结论时验证获准代码和实际版本，并按本设计的主题归属规则组织知识。

processing 成功结论必须与本批页面和导航变更在同一个文件提交中保存。no-change 也写入明确处理结果；失败不写 processed。部分资料成功可单独形成已发布 change，回执列出逐来源结果，未完成部分保持待处理。

智能体只有完成实际阅读与维护后才保存 processed 或 no-change 结论，不因下载成功就判定已经理解。相同原件由多个客户端同时整理时，第二次提交会因库代次变化重新读取，先检查第一轮结果再判断是否还需新增。

### 一轮原件整理的真实保存链路

~~~mermaid
sequenceDiagram
  actor User as 用户
  participant Core as NeuxAlly Core
  participant Agent as 当前智能体
  participant CLI as 受管 Wiki Mesh CLI
  participant Engine as 客户端知识引擎
  participant Store as 本地或远程文件端
  User->>Core: 上传原件
  Core->>Engine: 登记 raw
  Engine->>Store: 保存原始文件与元数据
  Store-->>Core: 经引擎返回原件保存回执
  User->>Core: 手动整理或发送明确聊天要求
  Core->>Agent: 通过真实 Execution 接纳任务
  Agent->>CLI: 请求原件与已有 Wiki
  CLI->>Core: 校验本次库与操作范围
  Core->>Engine: 读取获准资料
  Engine->>Store: 获取原件及实际版本
  Engine-->>Agent: 经受管入口返回只读文件引用
  Agent->>Agent: 使用已有能力阅读并编写 Markdown Wiki
  Agent->>CLI: 提交候选 Markdown 与来源关系
  CLI->>Core: 校验保存权限及请求身份
  Core->>Engine: 校验候选及基础版本
  Engine->>Store: 条件提交关联文件
  alt 基础版本冲突
    Store-->>Agent: 经客户端回传冲突
    Agent->>Agent: 重新阅读与合并后再提交
  else 保存成功
    Store-->>Engine: 文件回执
    Engine-->>Core: 内容版本与本机索引状态
    Core-->>User: 本轮修改结果，继续同一聊天
  end
~~~

图中返回箭头为逻辑结果归属；实际受管调用按 Core、CLI 和 Execution 的既有响应路径返回。读取文件成功与智能体真正完成阅读分开，写回执与模型回复分开。

## CLI、Skill 与受管权限

### 命令设计

下列是目标契约，尚不是现成命令；具体 Schema 由共同契约生成，帮助文案继续放在 Wiki Mesh internal/ui。

| CLI 操作 | 用途与写权限 |
| --- | --- |
| wikimesh kb list / inspect | 只发现当前获准库及当前快照 |
| wikimesh kb search / read / sources read | 有界查询、页面分段与来源定位 |
| wikimesh kb sources import | 导入明确获准附件；写入限定目标库 |
| wikimesh kb changes prepare | 生成候选目录、固定基础 revision 与资料范围 |
| wikimesh kb changes validate | 本机校验页面、来源、关系与完整性 |
| wikimesh kb changes commit / receipt | 保存和查询真实回执；同一请求幂等 |
| wikimesh kb check | 确定性检查；语义核查仍由当前智能体完成 |
| wikimesh kb sources archive / restore / purge | 分开表达归档、恢复及明确永久删除 |
| wikimesh serve-files | 独立远程文件服务，与现有 agent daemon 分开 |

受管环境不接受用户级配置覆盖当前库根、远程地址或凭据。未知 libraryId、越界路径或失效执行均拒绝。独立 CLI 的本地／远程配置与 NeuxAlly 接入记录分别维护，不从同名 project slug 推导授权。

所有返回为版本化 JSON，包含 schemaVersion、requestId、libraryId、knowledgeId、rootRevision 和 operation-specific result。查询追加 stale 与实际范围；写入追加 changedPageIds、sourceResults 和索引状态。正文分块返回，默认不整库输出。

候选目录仅供本次执行写入，不能直接编辑正式根。prepare 绑定目标库、基础 revision、允许操作和源版本；commit 不允许在同一候选中替换目标库或扩大资料读取范围。

### Skill 组织

共同 Skill 解释资料层、证据、主题导航、增量整理、变化保存、语言和回执；general 规则解释文档知识组织；software 规则解释工程主题、设计状态、代码依据及排障。具体任务规则为 query、ingest、edit、check，按任务加载，不复制每个库一套 Skill。

[Skill 详细设计与通用正文草案](knowledge-base-skills-cn.md)定义入口分工、加载条件、四类任务的输入输出、通用页面模板和场景验收。正文是待实施草案，不是已经安装或通过真实智能体验证的 Skill。

代码层校验路径、版本、身份、允许类型和引用存在；Skill 负责判断知识是否有价值、来源是否支持结论及如何表达分歧。普通保存通过现有授权后直接发布，不在每轮结果后增加独立审批。

受管调用走既有 Execution 身份和精确操作上下文。常规项目聊天默认只有知识读取；用户明确要求维护时，只增加本次目标和必要写操作，不给所有库授予修改权。

## NeuxAlly 本机记录与项目接入

以下为拟新增模型，全部沿用现有 GORM 注册、UUID、毫秒时间、status_error 和事务约定。API／Service 不接触原生 SQL；查询默认单表，不增加 JOIN。

### 接入与连接记录

| 模型／字段 | 类型／默认 | 约束与用途 |
| --- | --- | --- |
| KnowledgeConnection.ID | string | 本机主键 |
| Name, BaseURL, ExportID | string | 名称 1—80；规范基址；远端导出根，不含真实服务器路径 |
| CredentialRef | string，默认空 | 私有凭据引用，JSON 不回传 |
| CreatedAt, UpdatedAt | int64 | 本机连接生命周期 |
| KnowledgeLibrary.ID | string | 本机 libraryId |
| KnowledgeID | string | 唯一索引；共享库身份 |
| StorageKind | local / remote | 默认 local |
| LocalRoot | string，默认空 | 本机真实受管根，远程时为空；不向模型输出 |
| ConnectionID | string，默认空 | 远端连接已唯一绑定 exportId；本地时为空 |
| Snapshot | JSON 对象 | name、kind、brief、wikiLanguage、revision、counts、checkedAt 的只读投影；只有引擎回执或读取能更新 |
| CreatedAt, UpdatedAt | int64 | 本机接入记录生命周期 |

表名分别为 knowledge_connections、knowledge_libraries、knowledge_project_bindings、knowledge_repository_bindings 和 knowledge_maintenance_sessions，由各模型显式 TableName 并通过既有 RegisterTable 注册。外部网络或文件等待不得包含在 AutoMigrate 或业务 SQL 事务内。

一个连接记录表示 baseURL、exportId 及凭据这一存取端点；同服务器多个库使用不同 exportId。KnowledgeLibrary 不再提供 RelativeRoot，根修订代次不跨库共用。

新增或编辑名称、用途、主题必须先提交知识文件，不能只改 Snapshot。列表使用已接入库的本机投影，不代表远端全部共享目录的实时清单。

KnowledgeProjectBinding 使用 projectId 和 libraryId 作为复合主键，保存 createdAt。项目删除只解除关联，不删除知识内容。

同一默认代码目录的项目必须共用一套默认项目 Wiki：KnowledgeRepositoryBinding 以现有 ProjectSourceID 为唯一主键，关联 libraryId。创建第二个项目库时先在短事务中检查该映射，已存在则返回已有库供绑定，不能并发创建两套。此映射用于仓库知识去重，项目授权仍由 KnowledgeProjectBinding 决定。

不同 clone 或不同路径不因 Git remote 相同就自动合并；用户可显式关联已有库。非默认的重叠目录不自动扩展共用关系。若选择某远程项目库作为默认库，则明确更新仓库映射，不复制内容。

### 维护会话与回执

KnowledgeMaintenanceSession 保存 id、libraryId、projectId、chatId、wakerId、activeExecutionId、acceptedExecutionId、requestId、defaults、lastChangeId、createdAt、updatedAt。defaults 复用现有 Runtime、Account、Model、Reasoning、PermissionLevel 结构，凭据不进入响应。

通用库维护默认复用现有内置私人助理作为执行身份，不为每个库新建伙伴；这与现有工作流创建默认执行身份的代码保持一致，知识库仍是系统级资源。项目库维护使用本次选定关联项目的实际伙伴与项目身份，不借用其他伙伴拥有的 ProjectID；同一共享库可以有多个明确项目上下文的维护会话。

普通聊天维护不创建另一份聊天。新增 knowledge_change_links 保存本机 libraryId、changeId、chatId、executionId、requestId、createdAt；主键为独立 UUID，libraryId 与 changeId 组合唯一，记录一份已发布知识变更的本机执行来源。人工编辑的 chatId 和 executionId 可为空；远端其他客户端的变更不会伪造本机 Chat。此表不是第二份任务或内容历史。已有 requestidempotency 只复用摘要、稳定身份及重放判断，不把它当作持久恢复日志。知识文件回执是内容提交事实，私人 Chat 关联的恢复依赖下面的本机操作锚点。

网络、模型与文件操作不得放在 SQLite 事务中。创建库采取先保存本机 provisioning 锚点、再初始化知识文件、最后确认接入记录的顺序；失败留下可重试身份，不把半创建目录当作已可用。接入记录删除不等同于远端删除。

### 本机持久操作锚点

所有可能产生知识文件副作用的入口，包括创建库、附件导入、普通聊天保存、页面编辑和来源归档，先在既有受管 provisioning 根下写入 knowledge-operations/<operationId>.json，原子替换并刷盘后才能调用文件存取。它是文件与本机数据库之间的恢复日志，不新增任务框架或复制 Chat 正文。

锚点包含 operationId、requestId、payloadDigest、operationKind、libraryId、knowledgeId、baseRevision、changeId、派生来源及版本身份、chatId、executionId、真实附件引用、授权范围摘要、暂存上传身份、文件回执和恢复阶段。私人路径只在本机保存，远端只获得文件操作和公开 requestId。

阶段为 prepared → staged → commit-sent → files-committed → projected → complete；无文件副作用的失败单独记录。文件回执取得后先写回锚点，再在短数据库事务中更新 Snapshot、knowledge_change_links 和会话 lastChangeId。派生 sourceId、sourceVersionId 和 changeId 在第一次锚点中固定，重试不能重新生成。

上传原件和后续 Wiki 整理是不同子操作，子 requestId 从父操作及动作身份稳定派生，分别保存回执。进程在文件已提交、数据库关联未完成时退出，重启根据锚点查询同一文件回执并补齐关联，不靠共享历史猜测私人 Chat。

prepared 或 commit-sent 状态先查询文件回执，recovering 继续对账，明确 absent 后才重放同一请求；无法确认时不启动另一次整理。恢复必须重新校验当前资源和执行范围，不能让已撤销授权继续产生新副作用。

已完成锚点首版保留轻量结果以支持请求重放；原文仍归 Chat 或知识文件，不复制进锚点。启动扫描采用有界批次，其他库和已完成任务不因某个远端离线而阻塞。

## Core API 和界面数据流

所有路径相对既有 Core 认证基址，沿用 ginx Operator、OpenAPI 生成及稳定错误封装。以下是拟新增接口，不手改生成文件。

### API 集合

| 方法与路径 | 主要参数 | 结果与语义 |
| --- | --- | --- |
| GET /knowledge/libraries | search、kind、storageKind、cursor、limit | 已接入库、nextCursor、total |
| POST /knowledge/libraries | requestId、kind、name、description、wikiLanguage、可选 projectId | 新 libraryId，默认创建本地库 |
| POST /knowledge/libraries/connect | requestId、connectionId | 读取远程 knowledge.json 后接入，返回 libraryId |
| GET /knowledge/libraries/:id | 身份 | 库投影、真实可用操作、revision 及连接状态 |
| PUT /knowledge/libraries/:id/metadata | requestId、baseRevision、name、description、wikiLanguage | 写文件后刷新投影；修改语言不批量重写旧页 |
| POST /knowledge/connections | name、baseURL、exportId、可选 token | connectionId；token 只入私有存储 |
| PUT /knowledge/connections/:id | name、baseURL、exportId、凭据更新语义 | 更新连接后失效已有可用性结果，绑定位置变化先验证 |
| DELETE /knowledge/connections/:id | 身份 | 仍有接入库时返回 CONNECTION_IN_USE，不隐式删除库 |
| POST /knowledge/connections/:id/test | 身份 | 实际协议、导出能力和当前可读／可写状态 |
| DELETE /knowledge/libraries/:id/access | 身份 | 仅解除本机接入，保留远端文件 |
| GET /knowledge/libraries/:id/navigation | revision、pageType | 主题树及有界页面窗口 |
| GET /knowledge/libraries/:id/pages | themeId、pageType、query、cursor、limit | 当前快照页面列表 |
| GET /knowledge/libraries/:id/pages/:pageId | view、offset、limit、可选 revision | 正文、元数据和实际版本 |
| GET /knowledge/libraries/:id/sources | status、processingState、cursor、limit | 来源与版本，状态各自表达 |
| POST /knowledge/libraries/:id/sources | requestId、multipart 文件 | 资料身份和原件保存回执，不启动整理 |
| GET /knowledge/libraries/:id/sources/:sourceId/content | versionId、可选来源定位 | 受管原件读取或定位内容 |
| POST /knowledge/libraries/:id/sources/:sourceId/archive | requestId、baseRevision | 默认移除来源 |
| POST /knowledge/libraries/:id/sources/:sourceId/restore | requestId、baseRevision | 恢复为 active，不重跑模型 |
| POST /knowledge/libraries/:id/sources/:sourceId/purge | requestId、baseRevision、confirmation | 独立永久删除，先展示引用影响 |
| POST /knowledge/libraries/:id/changes/prepare | requestId、baseRevision、允许修改范围 | UI 人工编辑或受管任务的候选身份 |
| PUT /knowledge/libraries/:id/changes/:changeId | 页面、主题及关系修改 | 只写候选，主题新增／改名／移动／删除均在此验证 |
| POST /knowledge/libraries/:id/changes/:changeId/validate | 候选身份 | 确定性检查报告 |
| POST /knowledge/libraries/:id/changes/:changeId/commit | requestId、baseRevision | 真实保存回执，人工编辑不必调用模型 |
| POST /knowledge/libraries/:id/maintenance/messages | sessionId、requestId、content、projectId、sourceVersionIds、defaults | 返回真实 Chat、Session 与 accepted Execution |
| GET /knowledge/libraries/:id/history | cursor、limit | 共享 change 列表，不含私人消息 |
| GET /knowledge/libraries/:id/history/:changeId | 身份 | 修改范围、版本与页面差异 |
| POST /knowledge/libraries/:id/history/:changeId/restore | requestId、baseRevision、页面范围 | 历史内容形成新候选，校验后发布新版本，不回滚当前文件代次 |
| POST /knowledge/libraries/:id/archive | requestId、baseRevision | 库元数据归档；原件、Wiki 与历史保留 |
| POST /knowledge/libraries/:id/purge | requestId、baseRevision、confirmation | 独立全库永久删除；不等同于断开接入 |
| PUT /projects/:projectId/knowledge-bindings | libraryIds、默认关联选择 | 验证项目与软件类库后修改本机关联 |

维护消息 sourceVersionIds 为空且首次点击“整理新增”时，由 Core 固定当前待整理版本；续改仅修改本轮明确范围，不自动扩大为全量重新摄入。普通聊天用能力入口导入获准附件，不让前端伪造任意绝对路径。

库和来源列表 limit 默认 50、最大 100。游标绑定筛选及稳定的 updatedAt、id；总量按同一筛选窗口计算。文件导航及历史的游标另绑定 rootRevision，避免分页期间混合版本。正文默认读取 card 或 core，单次最多 64 KiB，继续阅读使用偏移。

写操作默认返回项目标准成功响应；创建、上传和维护接纳因下一步必须使用身份或回执而返回最小结果。缺失字段表示不修改；空 description 表示清空；不接受客户端用 null 清空身份、类型或存取根。首版不支持原地改变知识类型或切换存取位置。

### 前端装配

系统级知识库列表复用 ResourcePageSurface、Header、Toolbar、SearchField 和 CardGrid。列表展示通用／项目、本地／远程、待整理数及连接状态；创建默认本地，提供 Wiki 编写语言及说明，远程接入为独立入口。软件类的空状态主题建议是按需采用的选项，创建时不自动填满工程目录。

详情工作台左侧为主题树和内容，右侧为维护聊天。顶部在原始资料、Wiki、关联和历史之间切换，页面类型筛选作用于 Wiki；主题、pageId、view、筛选保存在 Scene 路由，维护 Chat 和组件身份不随内容标签重建。

聊天复用现有消息渲染、输入、智能体配置、审批、停止与问答；知识模块只提供受控上下文和保存回执。停止的是当前 Execution，不能把停止伪装成原件读取失败或整批成功。

原型的类型一级目录需按本次主题导航决策调整。当前图册仍是设计参考，不作为新导航、远程接入或真实运行的视觉验收证据。

### 事件与恢复

注册知识库资源变更到现有 Core 统一事件通道，最小事件包含 libraryId、rootRevision、changeId、changedKinds。前端精确失效相关 Query，不新增 WS 或永久页面轮询，不逐 Token 更新库计数。

远程其他客户端不会直接向本机发事件；本机在规定检查点发现新 revision 后再发送本机资源变化。维护结果和失败沿真实 Chat／Execution 接入通知与看板，知识库不另存一套关注计数。

## 维护会话和定时任务

页面首次发送前保持草稿；接纳后返回稳定 sessionId、chatId 和 executionId，后续发送复用 Chat。保存回执 lastChangeId 来源于真正 Commit，助手“完成”文本不产生内容版本。

同一会话同时只有一个活跃发送；另一个会话或客户端可并行生成候选，最终由文件事务解决竞争。关闭页面不会取消已接纳工作，重开后恢复真实 Chat 和当前库版本。

项目库维护必须选定一个当前关联项目作为代码范围。普通聊天本来就在项目中时继承该项目；全局入口缺少唯一项目时先选择项目，不能把共享库关联的所有目录一起给智能体。

定时维护使用现有 AutomaticTask，仍以已有伙伴目标执行，不新增 knowledge 调度器或远程任务平台。通用库默认由现有内置私人助理执行，项目库使用明确的关联项目伙伴。任务显示在现有自动任务管理中，并从知识库提供入口。

在 AutomaticTaskConfiguration 增加可空的结构化 knowledgeMaintenance 绑定：libraryId、operation（ingest-pending 或 check）、projectId。配置由 Core 验证并在机会领取时冻结；不是通过识别指令字符串授予知识写权限。运行关联复用 AutomaticRun 和真实 Execution。

定时任务的时区、休眠错过、并发槽、启停和取消语义沿用现有自动任务。没有新增资料时 ingest-pending 返回 no-op，不调用模型；check 按明确任务要求执行，不默认全量重写。远端断线或 IP 无写权限时记录真实失败，不切换本地存储。

## 归档、永久删除与故障恢复

### 默认归档

归档来源只改 source status、导航投影及必要变更记录，保留原件、来源定位、Wiki 和历史；现有引用可读并标记来源已归档。后续默认增量整理不再选取归档来源。

恢复来源不调用模型；原版本若已处理仍保留 processed，未处理版本重新进入可选待整理范围。归档页面与归档来源分别操作，不能因为来源归档就将全部引用页面下架。

### 独立永久删除

永久删除入口先返回影响清单：原件版本数、引用页面、历史与当前客户端缓存。用户明确确认并绑定 baseRevision 后再执行，普通“整理”指令不能获得 purge 权限。

删除某来源的全部原件文件，保留最小来源 tombstone 及引用身份；已有 Wiki 不联动删除，引用状态变为 unavailable。候选、事务备份和本机 qmd 中相应原文按身份清理；有活跃提交或无法确认目录归属时拒绝执行，不盲目清空目录。

彻底删除表示当前知识库内被选中的原件及本机可管理副本清除，不承诺撤回其他客户端以前自行下载的文件。共享知识历史由客户端检查是否包含目标原件副本，存取端的上传暂存、读取快照与控制备份由 purgeTargets 清理。永久删除回执同时要求文件提交和残留清理完成，不能只删除当前 raw 文件就报告完成。

删除整个远端库与解除本机接入分开：普通“断开连接”不发送文件删除请求。全库永久删除必须展示共享影响并单独确认，文件服务仍只执行获准文件操作，不增加组织管理员平台。

### 恢复矩阵

| 故障点 | 可见状态 | 恢复动作 |
| --- | --- | --- |
| 原件暂存后、登记前退出 | 未完成上传 | 重放同 requestId 或清理过期暂存 |
| 原件登记成功、整理未启动 | 待整理 | 保留来源，用户稍后整理 |
| 智能体无法读取或只读到部分内容 | 失败／部分完成 | 保留原件、范围与原因，支持明确重试 |
| 模型取消、候选未保存 | 已停止，未完成部分仍待整理 | 不发布候选，不写 processed |
| 文件 Commit 超时 | 保存结果待确认 | 查询同 requestId 回执 |
| Commit 成功、本机 DB 投影失败 | 内容已保存，接入信息待恢复 | 从本机操作锚点及文件回执修复投影和 Chat 关联，不重跑模型 |
| 目标已删除、残留清理未完成 | 永久删除处理中 | 按持久 purge 清单继续，未完成前不报告彻底删除 |
| Commit 成功、索引失败 | 内容已保存，查询索引待刷新 | 重建本机索引，不重新生成 Wiki |
| 版本冲突 | 保存冲突，候选保留 | 客户端重新读取并合并 |
| 服务端 prepared 后退出 | 导出根恢复中 | 启动前完成日志恢复，再开放读写 |
| IP 白名单变化 | 本次写入被拒绝 | 保留候选，更新部署配置后重新验证 |
| 库元数据损坏 | 库不可用 | 保留文件并给出具体损坏项，不当空库初始化 |

启动恢复从 requestId、文件回执、真实 Chat 与 Execution 对账，不通过临时内存判断成功；既有有效 status_error 原样保留，不把权限、损坏或活跃执行折叠为“不存在”。

## 分发、资源边界与兼容

内置版本固定交付 Wiki Mesh 公开包、CLI 和 Skill，记录内容模型与协议版本。启动只检查兼容性，不在普通任务中自行下载最新 Skill、向量模型或替换用户独立 CLI。

服务端 HTTP protocolVersion=1 与知识 schemaVersion=1 分别演进。页面规则升级不要求文件服务升级；文件服务不支持条件提交时允许明确只读接入，修改按钮报告能力不足，不能退回普通无条件覆盖。

NeuxAlly 新增模型按既有 dbhelper.RegisterTable 和 AutoMigrate 管理，不新增迁移框架。旧应用不自动生成知识库、不扫描目录、不导入 Demo。降级可保留新文件与本机记录，禁止旧版按未知 schema 改写；真正迁移需另有显式操作。

设计默认资源限制如下，属于首轮实现参数，需在样本验收后调整，不代表性能承诺：

| 项目 | 默认 |
| --- | --- |
| 单原件大小 | 100 MiB |
| 一次批量上传 | 20 个文件，逐个回执 |
| 单文件服务提交 | 200 个文件操作，内容先暂存 |
| 目录页 | 默认 100，最大 500 |
| 单次模型读取 | 64 KiB，超出分页 |
| 查询候选 | 默认 20，最大 100 |
| 暂存上传寿命 | 未被 Commit 引用时 24 小时 |
| 正在恢复的导出根 | 暂停该根访问，不阻塞其他独立根 |

不为资源限制建立每库常驻任务。控制回执不因临时上传清理而删除；永久删除及历史压缩需要显式扫描其引用关系。

## 验收场景与设计评审边界

以下是实施后必须验证的行为，本次设计文档没有执行这些功能测试：

| 组别 | 必测场景 |
| --- | --- |
| 主题与页面 | 空主题、中文标题、跨主题页面、类型筛选、主题移动及改名后链接稳定；循环和失效引用被拒绝 |
| 软件归属 | 同份设计跨主题、不同资料合入同页、使用已有主题、不生成全套空目录；需求／定案／实现分开，运行与数据及安全等知识有明确归属 |
| 两类规则 | 文档类概念、方法和多来源分歧；软件类功能与实现分开，代码未核验不写成事实；四类 Skill 任务按需加载 |
| i18n | 新增 zh-CN／en-US Key 对齐，数量与时间格式、长英文及缺失语言 fallback；机器 JSON 语义稳定、状态与错误本地化；切换界面不改主题／正文／摘要，双客户端遵守同库编写语言 |
| 范围 | 无项目不可查询项目库；同仓不同伙伴共用库；不同仓和连接不能靠猜 ID 越界 |
| 原件与读取 | 当前 Runtime 实际读取 PDF、DOCX、MD；明确不支持、仅元数据／部分内容、长中文、同名来源和版本替换；不静默换模型 |
| 增量 | 新增、no-change、部分失败、执行期间再上传、索引失败后重试不重复生成 |
| 聊天 | 页面选择 CLI／模型；聊天继承配置；连续两轮修改；切标签与隐藏重开保持 Chat 身份 |
| 本地竞争 | 两个进程同版本修改、人工候选与模型候选竞争、提交中断后恢复 |
| 远程共享 | 两客户端无模型服务端完成整理；一方保存后另一方刷新缓存和索引 |
| 文件协议 | 一致分页、摘要错误、软链接和越界路径、control 隔离、旧 revision、有范围读取 |
| 幂等与恢复 | 同键重放、同键异内容、响应丢失、prepared 后崩溃、磁盘写失败不暴露半批 |
| IP 写限制 | 开关、空名单、IPv4／IPv6／CIDR、伪造转发头、可信代理、提交前名单改变 |
| 归档删除 | 归档可回溯、恢复不重跑、永久删除影响预览、当前原件及备份清理、断开不删远端 |
| 定时维护 | 明确的库和项目绑定、无新增 no-op、休眠与错过、活跃运行及配置撤销、断线失败 |
| UI | 浅暗主题、窄 Scene、长主题、键盘、焦点、加载／陈旧／失败，以及右侧持续聊天 |
| 升级 | 旧用户零自动副作用；未知模型或协议只读／拒绝写入；Demo 不被静默迁移 |

实施验证遵守本项目的前后端检查、OpenAPI 与生成资源检查，再做真实 CLI、模型、桌面和双客户端远端验收。静态通过不等于模型整理效果或视觉验收通过。

当前产品边界已经收敛，没有需要用户先决定才能编写本设计的事项。尚需工程证据的内容是 各 Runtime 的原件读取与来源定位样本、三平台事务恢复、真实两客户端竞争、资源默认值及最终视觉验收；这些是实施验证门槛，不以猜测写成已通过。

参考：[领域规格](knowledge-base-specs-cn.md)、[前端原型](knowledge-base-ui/README.md)、[Core 软件能力](../architecture/agent-capabilities-cn.md)、[存储边界](../architecture/workspace-and-storage-cn.md)、[自动任务](../architecture/automation-and-im-cn.md)、[工程约定](../development/backend-cn.md)。现有代码依据使用本次核查的 NeuxAlly 4fb33f8 与 Wiki Mesh 67f17c2；后续实现前需重新核对实际基线。
