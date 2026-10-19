# 插件 v2 验收记录

本文区分源码实现、自动化验证、真实 CLI 加载与真实账号业务结果。架构及协议依据见 [设计](../specs/2026-10-19-plugin-v2-design.md)、[Spec](../specs/2026-10-19-plugin-v2-spec.md) 和 [当前插件架构](../../architecture/plugin-resources-cn.md)。

## 交付位置

- 应用实施基线为 `81e683e`，实现分支为 `codex/plugin-v2-implementation`，生产及生成契约最终提交为 `47a4f69`，没有推送应用仓库。
- 插件公共仓库为 [NeuAally-Plugins](https://github.com/JieWaZi/NeuAally-Plugins)，本轮固定引用提交为 `3474810eb6fc45ec3115aaa8bc87e1a7bea20e66`。
- [v2.0.1 Release](https://github.com/JieWaZi/NeuAally-Plugins/releases/tag/v2.0.1) 已公开发布，包含两个版本为 2.0.1 的系统入口、目录和完整安装 ZIP；六个平台的隔离 Lark CLI 依赖及许可继续固定引用已发布的 [v2.0.0 Release](https://github.com/JieWaZi/NeuAally-Plugins/releases/tag/v2.0.0)，没有覆盖旧资源。
- 插件仓库的 [中文 README](https://github.com/JieWaZi/NeuAally-Plugins/blob/3474810eb6fc45ec3115aaa8bc87e1a7bea20e66/README.zh-CN.md) 说明三类目录、标准包、原生来源引用、HTTP 分发及新增授权桥接，架构图和授权时序图分别标出前端、Core、桥接及供应商职责。

## 本轮三个验收目标

| 目标 | 已观察结果 | 证据边界 |
| --- | --- | --- |
| 上传 Codex/Claude 包，保留来源并按智能体交付 | 隔离 Core 的真实 HTTP 扫描、准备和伙伴安装均成功；保留 hooks/commands 与完整 README。生产适配产物被真实 Codex 与 Claude 加载 | 原生初始化没有向模型发 Prompt；跨 CLI 未知 hook/LSP 事件不宣称语义等价 |
| 对话通过系统 CLI 安装 | 真实 allycli 子进程经 Execution grant、HTTP 和共享安装 Service 写入当前伙伴；同请求重放、异内容冲突、撤销授权及全局配置哨兵均有回归 | 测试使用隔离数据库、受控目录，不代表真实模型已经主动选择安装操作 |
| 从解耦插件库获取内容 | 公开 Release 的目录和两个 ZIP 已通过 HTTP 下载并核对 SHA-256；应用构建只产生来源元数据，不内置插件 ZIP | 用户侧目录获取与安装不依赖 git；维护者构建发布仍使用 Git |

## 包与原生加载

HTTP 验收启动独立 Core、数据库和受管文件根，实际上传两份 ZIP，再调用生产准备及安装接口。Codex 样例正文为 300,005 字符，准确返回可读；Claude 样例正文为 1,100,005 字符，准确返回超限，原文下载仍与上传内容完全相同。目录详情保持只读，不建立系统安装关系。

生产 `PrepareNativeTarget`、`PrepareCodexHome`、`PrepareCodexWorkspace` 的产物分别经本机 CLI 初始化核对：

- Codex CLI `0.161.0`：`plugin/list` 返回 `acceptance@neuxally-managed` 已安装且启用，`skills/list` 返回带相同 pluginId 的 `acceptance:native-probe`。
- Claude Code `2.1.159`：Codex 包派生为 Claude 原生目录，通过 `--plugin-dir` 加载并返回 `native-probe` 命令及插件说明。
- Gemini `0.64.0`：生产 Skill 投射后，真实 `skills list` 返回 `native-probe [Enabled]`，相对资料保留；该发现命令使用私有占位账号与回环地址，没有模型请求。MCP 投射由 Execution/Runtime 定向回归覆盖，没有将技能枚举当作真实 MCP 业务调用。

验证使用私有会话目录，没有调用用户全局插件安装命令。Codex 仍可能发现其默认用户技能；私有 `CODEX_HOME` 不能被描述为所有默认技能的完全隔离。受限执行由接纳层限制交付，不能仅依赖配置目录隔离。

## 公网分发

真实访问 [默认目录](https://github.com/JieWaZi/NeuAally-Plugins/releases/latest/download/catalog.json) 及其固定资源 URL，结果如下：

| 安装包 | 字节数 | SHA-256 |
| --- | ---: | --- |
| feishu-office.zip | 5,767,867 | `618174e70e03f23db15023cfe54424391f83fd572bc1cb1f7b3d19c2fbd59af4` |
| github.zip | 74,868 | `bf2f5ebad2a840b1c21400e1711a81728d4b70176c5d01adbd1837620eb31f78` |

隔离 Core 经默认在线目录准备两个 2.0.1 条目，来源提交与发布清单完全一致。GitHub 已通过生产 HTTP 安装到伙伴；原生 PAT 字段从真实 Claude 清单自动推导，加密保存被接受，公共响应只显示已配置标识。飞书未授权时准确返回未就绪，没有登记成可用账号。

本机公网测试使用已配置的网络代理；代理仅影响验收进程网络环境，未改用户设置。目录缓存、首次失败、陈旧状态、不同来源同名身份、下载中断及摘要错误由定向回归覆盖。

源码中的官方收录只保留来源描述、完整性锁及许可材料，发布构建通过 HTTP 还原完整内容。两个来源的九个原生文件与先前固定上游字节一致；独立空缓存下载、禁止网络的缓存重建和完整 ZIP 索引均通过。GitHub `description` 为“检索 GitHub 仓库、议题和拉取请求，协助代码审查与协作。”，路由策略留在集成声明和开发说明。

## 授权解耦与验证

插件通过 `.neuxally/integration.json` 声明授权入口，供应商协议解释和私有依赖下载位于包内桥接。宿主管理通用会话、身份租约、取消、期限、秘密绑定及界面动作。二维码、链接、验证码必须由当前动态挑战明确给出，普通 URL 不自动生成二维码。公开账号摘要与执行秘密分离。

前端回归覆盖多阶段挑战替换、授权声明发现、仅元数据目录首次安装、取消与 README 版本边界；后端真实子进程协议测试覆盖会话绑定、退出及越范围秘密绑定拒绝。跨仓库验收直接使用生产 Driver、Authorizer 和发布包的真实 Node 桥接，供应商 CLI 使用私有替身；两阶段挑战、状态核验、私密 resolve、退出及取消通过，公开状态未出现四类秘密哨兵，命令解析命中私有固定依赖。该结果证明协议互通，不代表真实账号授权成功。标准 MCP 认证沿目标 CLI 与既有 ACP 能力处理，本轮没有新增独立 OAuth 服务。

## 定向检查

- 插件前端：13 个测试文件、58 项测试通过；TypeScript、定向 ESLint、格式与 UI 规范检查通过。
- 应用插件资源构建：4 项测试通过，确认只生成 HTTP 来源元数据并清理旧内置 ZIP。
- 独立插件仓库：26 项测试通过，覆盖来源 HTTP 物化、完整树锁、缓存、可重现打包、资产下载、授权阶段、取消及私有命令执行。
- 后端：插件解析、目录、通用授权桥接、既有提供者、安装 Service、HTTP API、Execution、Runtime 与能力契约共 10 个相关 Go 包通过；最终入口指纹修复另以安装 Service 定向回归通过。
- 契约与文档：通过项目命令生成 OpenAPI、客户端和系统 Skill bundle；`api:check`、`skills:check`、文档及差异校验通过。
- 独立评审已修复依赖 PATH、超限 README 展示、原生失败降级、重试快照、子进程回收、Claude 直接 MCP 映射、原生配置和授权入口信任等问题，应用 Spec/Quality 及固定来源增量评审通过。

没有运行 `pnpm check`、desktop check、全量前端测试或 `go test ./...`。

## 未执行与适用范围

- 未运行真实飞书账号创建应用、扫码、权限授予和文档写入；GitHub 托管连接与 Claude PAT 的真实业务调用未运行。
- Windows/Linux 依赖已交叉编译并核对分发摘要，未在对应平台执行原生账号与钥匙串流程。
- 未打开、控制或截图运行中的桌面、浏览器与页面，没有桌面视觉验收。
- 外部 CLI 全局插件的后台自动观察与同步、任意远程 marketplace URL 的直接引用解析没有在本轮三个验收目标内实现；本轮采用用户导入和完整收录后的固定来源引用。
- 标准包可导入、文件完整保留与当前账号能授权使用分别表达；不将 OpenAI 目录中的全部插件宣称为已支持。
