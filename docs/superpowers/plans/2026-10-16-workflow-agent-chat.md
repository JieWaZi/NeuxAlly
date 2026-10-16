# 聊天调用工作流实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 普通聊天选择一个工作流，由 Agent 使用既有 CLI 启动和等待，在原消息交付结果与文件，并能查看及停止本次运行。

**Architecture:** 复用单一系统 Skill 和 `allycli`，Agent 拥有业务调用编排。Core 补齐权威消息引用、准确运行关联、停止传播和受管文件交付；前端通过 Chat 公开贡献槽和统一事件展示 Workflow 原运行，不新增调度器。

**Tech Stack:** Go、ginx、GORM／SQLite、React、TypeScript、assistant-ui、TipTap、TanStack Query、现有 Electron Bridge。

**Spec:** [聊天调用工作流 SPEC](../specs/2026-10-16-workflow-agent-chat-design.md)。

## Global Constraints

* 首版一条消息最多选择一个工作流；内部节点、多个 Agent 和子流程沿用既有工作流规则。
* 仅为普通伙伴聊天新增入口。团队对话和工作流创作／调整的发送契约保持既有行为，不显示无法传递的引用入口。
* 任务理解、参数组织、启动、重复等待和总结由 Agent 完成。应用不新增自动启动、参数推断或等待调度器，不为各工作流生成独立技能或连接器。
* 复用系统 `skills/ally`、`allycli`、现有工作流服务、消息日志、共享组件和统一资源事件；不新增前端永久轮询或业务连接。
* 每个真实运行只有一份权威状态；UI、聊天和工作看板读取原 Workflow 数据。
* 仅在独立 worktree 的功能分支修改并提交，不合并 main、不推送。保留原目录的讨论稿。
* 所有新增和修改 Go 结构体的全部字段具有中文注释；公开 API 字段使用简短业务标签。生成文件只运行生成命令更新。
* SQL 默认单表查询，既有关联通过索引条件和有界批量读取完成，不为展示字段新增 JOIN。
* 未获明确授权不打开、控制或截图运行中的应用或页面。自动化验收与真实模型、视觉验收分别报告。

## Review Focus

1. 接纳、停止与授权撤销并发：用户停止后不得遗留取消窗口之外的新运行；由 Task 2 的屏障并发测试覆盖。
2. 消息／命令事实到达顺序及回执重放：引用和附件只能从 Core 事实建立，重放同一命令不重复文件；由 Task 1、2 的事件重放测试覆盖。
3. 文件路径及同名输出：未声明、越界软链接和同名文件不能误交付或覆盖；由 Task 2 的真实临时文件测试覆盖。
4. 输入法、纯引用与发送失败：工作流选择必须与正文一起发送和恢复，第二个替换第一个；由 Task 3 的真实输入组件与请求测试覆盖。
5. Agent 等待超时、失败及用户停止：继续等原 Run、失败不重跑、用户停止不替换方案；由 Task 4 的受控 Agent 行为场景覆盖，并报告真实 Runtime 验收边界。

## 文件和任务边界

| Task | 主要文件与职责 |
| --- | --- |
| 1 | `server/internal/workinput/types.go`、`server/internal/chat/eventlog/types.go`、`server/internal/chat/messageview/{types,message_view,agent_command}.go`、`server/services/execution/{work,agent}.go`、`server/services/workflow/{capabilities,runs}.go`、聊天 API 与对应测试：消息引用及受信运行关联 |
| 2 | `server/services/workflow/{agent,capabilities,runs}.go`、`server/repositories/{models,controllers}/workflow*`、`server/apis/chats/cancel_chat_execution.go`、`server/apis/workflows/cancel_run.go`、CLI 契约和对应测试：取消及正式文件交付 |
| 3 | `desktop/src/modules/chat/{pages,components,runtime}`、`desktop/src/modules/workflow`、`desktop/src/app/providers.tsx`、i18n 和对应测试：单选引用、消息执行项、原运行右侧过程及会话文件 |
| 4 | `skills/ally/references/workflow-execution.md`、关联架构和指南、必要跨层回归测试：Agent 指引、集成验收及证据报告 |

新增文件只用于有独立职责的业务组件／集成测试，不为单个私有 helper 拆文件。Task 1、2 修改公开类型后运行 `pnpm api:generate`，生成目录不手改。

---

### Task 1: 聊天工作流引用与受信运行事实

**Files:**
* Modify: 上表 Task 1 文件，必要的 `server/services/chat/journal.go`、`server/internal/chat/eventstream/live_adapter.go`、`server/apis/chats/{create_chat,send_message,message_input}.go`。
* Test: `server/services/workflow/consumer_test.go`、`server/internal/chat/messageview/agent_command_test.go`、`server/services/execution/work_test.go`、`server/services/execution/runtime_events_test.go`，以及聊天 API 相邻测试。

**Interfaces:**
* Produces: `eventlog.MessageWorkflowReference{ID, Name string}`，公开 JSON `workflows`；用户输入经 `workinput.Input` 和消息日志保存，仅由 `BindConsumer` 从真实定义补齐。
* Produces: `eventlog.WorkflowRunReference{WorkflowID, RunID, RevisionID, Name string}`，公开 JSON `workflowRuns`；`eventlog.AgentCommandData.WorkflowRun` 保存服务端确认的引用，`messageview.Message.WorkflowRuns` 投影到所属助手执行消息。
* Produces: 启动命令回执含真实 `runId/definitionId/revisionId/name`；`execution.agentCommandFacts.record` 只从受信 Core 结果提取引用，原生工具 stdout 不产生授权事实。
* Consumes: 既有 `workflowIds`、`BindConsumer`、`AppendExecutionFacts`、`workflow.runs.start` 回执。为 Task 2 预留 `AgentCommandData.Attachments []eventlog.AttachmentReference`，按命令身份／稳定路径去重投影到助手消息附件。

- [ ] **Step 1: 先写失败测试。** `TestConsumerBindsOneWorkflowReferenceAndInstructions`：选中已保存定义得到真实 ID／名称及指导 Agent 读契约再用 CLI 的上下文；传两个定义拒绝。`TestWorkflowCommandFactsReplayOnAssistantMessage`：先后不同顺序的执行消息和启动事实最终投影同一运行，伪造 stdout 无 Core 事实时无引用。
- [ ] **Step 2: 验证 RED。** 在 `server` 运行 `go test ./services/workflow ./internal/chat/messageview ./services/execution -run 'Workflow|Consumer' -count=1`，记录缺失行为导致的断言失败；编译缺字段不是充分 RED 证据。
- [ ] **Step 3: 最小实现。** 补齐真实选择快照、输入深拷贝／摘要／持久化／失败恢复所需传递；首版聊天最多一个。启动后将 Core 确认的运行身份写入既有命令事实并让 live／durable 两条投影一致。保留原生运行日志，业务执行项不以任意工具文本建立。
- [ ] **Step 4: 验证 GREEN。** 重跑聚焦测试，再 `gofmt -w` 修改的 Go 文件，运行 `pnpm api:generate`、`pnpm api:check`，在 `server` 运行 `go vet ./...`、`go test ./...`。预计全部通过；已有失败按名称报告。
- [ ] **Step 5: 自审并提交。** 仅提交本任务文件，中文提交信息说明消息引用、运行事实及重放保证。报告 RED／GREEN 命令、输出、接口和提交；不触碰 main。

### Task 2: 用户停止传播与最终文件交付

**Files:**
* Modify: Task 2 文件，`server/internal/agentcapability/contract/{catalog,output,display}.go`、`server/services/execution/agent.go`，必要的已有文件安全能力。
* Test: `server/services/workflow/{consumer,agent,retry,terminal_receipt}_test.go`、`server/apis/workflows/files_test.go`、`server/internal/chat/messageview/agent_command_test.go`、Repository 相邻真实 SQLite 测试。

**Interfaces:**
* Consumes: Task 1 的运行引用及 `AgentCommandData.Attachments`，原消费者与 Chat 关联。
* Produces: `Service.CancelConsumerExecution(ctx context.Context, chatID, executionID string) error`，聊天取消 API 在停止 Agent 前调用；持久关闭本次消费者并取消已接纳关联运行。已关闭授权在 `StartRun` 最终接纳锁／事务内重核，防止并发遗漏。
* Produces: `models.WorkflowRun.ConsumerLeaseID string` 作为可索引调用关联，替代 JSON Context 中同一事实；不重复保存两个权威关联。`WorkflowConsumerLease` 增加持久撤销时间，Controller 提供撤销和索引有界运行读取，游标遍历不漏活跃运行。
* Produces: Run 的可空取消原因结构，至少含 `actor/source/requestedAt`；通过新 `CancelRunWithReason` 或等效既有职责方法接入，旧 `CancelRun` 调用者保持既有语义。常量集中，公开字段中文标签，终态不得被停止改写。
* Produces: 新增现有命令域内的 `workflow.runs.deliver`（参数 `runId`，需要稳定 request-id，同步写操作），只允许本聊天获准且已终态的运行。返回 `runId/definitionId/revisionId/name/attachments/files`；把成功运行声明的最终文件固化为 Chat 附件，`files` 保留不可用文件的准确状态。
* Produces: `workflow.runs.get/wait` 返回最终文件描述和明确取消原因，等待仍用原事件观察；不新增等待循环后台任务。

- [ ] **Step 1: 先写失败测试。** `TestConsumerCancellationClosesAdmissionRace` 用屏障覆盖取消与最终接纳交错；`TestConsumerCancellationDoesNotRewriteTerminalRun` 覆盖重复停止和已终态。`TestAgentDeliversDeclaredFilesToAssistantMessage` 使用真实临时文件验证声明文件、同名不同 Run、重复回执和消息登记重放；`TestAgentDeliveryRejectsEscapingFiles` 覆盖越界软链接及普通字符串路径。
- [ ] **Step 2: 验证 RED。** `go test ./services/workflow ./internal/chat/messageview ./repositories/controllers -run 'Consumer|AgentDeliver|WorkflowCommand' -count=1`，记录所需行为缺失；接口新增时先通过现有可调用入口验证 RED，不以编译失败代替行为测试。
- [ ] **Step 3: 最小实现。** 在 Workflow 原接纳／取消锁和统一事务内实现撤销与原因，聊天取消捕获准确执行身份。复用文件校验及 Chat 受管目录，使用 Run＋输出字段稳定身份避免覆盖，先安全固化文件再提交可重放登记；登记失败允许查询原交付回执恢复。文件交付不在 GET／wait 隐式写入，由 Agent 显式调用 deliver。
- [ ] **Step 4: 验证 GREEN。** 聚焦测试通过后，运行 `go test -race ./services/workflow ./internal/chat/messageview`；对新增消费者查询记录真实 `EXPLAIN QUERY PLAN` 和有界查询次数证据。运行 `gofmt`、`pnpm api:generate`、`pnpm api:check`、`go vet ./...`、`go test ./...`。
- [ ] **Step 5: 自审并提交。** 中文提交说明双向停止、接纳竞态和最终文件交付；报告无遗漏运行与文件安全的证据，列出 Task 3 消费的最终生成类型和命令契约。

### Task 3: 单工作流输入、消息执行项与右侧运行过程

**Files:**
* Modify: `desktop/src/modules/chat/components/{chat-composer-input,chat-thread,chat-workspace-surface}.tsx`、`desktop/src/modules/chat/pages/chat-page.tsx`、`desktop/src/modules/chat/runtime/{chat-queue-store,use-chat-runtime,chat-projection}.ts*`、`desktop/src/app/providers.tsx`、`desktop/src/shared/i18n/{chat,workflow}.ts`。
* Create: `desktop/src/modules/chat/components/chat-workflow-reference.tsx`：引用类型、消息标签；`desktop/src/modules/workflow/components/workflow-call.tsx`：受信运行执行项与右侧过程内容，必要相邻测试。
* Modify: `desktop/src/modules/workflow/index.ts`、Workflow 公开导出、既有运行组件和 Chat 贡献槽；跨模块通过公开入口及 app 装配。
* Test: `desktop/src/modules/chat/runtime/chat-composer.integration.test.tsx`、`desktop/src/modules/chat/runtime/chat-projection.test.ts`、`desktop/src/modules/chat/pages` 和 Workflow 过程相邻集成测试。

**Interfaces:**
* Consumes: 生成 API 中 `workflows`、`workflowRuns`、助手消息 `attachments` 和原 Run 查询／停止接口。
* Produces: `ChatWorkflowOption{id,name,description}`；输入 props `workflows/selectedWorkflowId/onWorkflowSelectionChange`，可根据实际异步搜索需求增加语义明确回调；`ChatSendSelection.workflowIds` 长度最多一，`selectedWorkflows` 为乐观消息快照。
* Produces: 普通 `ChatPageContent` 搜索并装配工作流，特殊 `entry` 不启用；服务端名称搜索与游标读取覆盖首屏之外的定义，不能只在首屏本地过滤。
* Produces: 运行内容引用 `{type:'workflow-run', ids:{workflowId,runId}}`，经 `useOpenWorkbenchContent` 打开右侧；Workflow 公共贡献组件按该引用读取原运行并复用画布、详情与停止，不导航走原聊天。

- [ ] **Step 1: 先写失败测试。** 在真实 TipTap 输入中 `/` 可选择工作流，第二个替换第一个；中文输入法回车不误发，删除、队列和失败恢复保留准确引用。发送 payload 是选中真实 ID。投影的助手消息含运行与文件；执行项打开确切右侧引用，渲染运行中、失败和已停止；特殊 entry 不出现工作流入口。
- [ ] **Step 2: 验证 RED。** `pnpm --dir desktop exec vitest run src/modules/chat/runtime/chat-composer.integration.test.tsx src/modules/chat/runtime/chat-projection.test.ts`，针对新增集成文件另运行同目录测试，记录实际 UI 行为缺失。
- [ ] **Step 3: 最小实现。** 复用已有 `/` 提示和引用节点，普通聊天一个选择；保持技能等既有语义。新增工作流业务执行项放在执行过程区，统一事件按原 Run 更新；右侧内容复用组件并保持关闭释放订阅。正式文件复用会话附件组件，不只显示任意路径文本。普通卡片无常驻边框阴影，使用现有语义 Token。
- [ ] **Step 4: 验证 GREEN。** 聚焦集成测试通过后运行 `pnpm --dir desktop check`、`pnpm api:check`；禁止为省事修改检查例外。未授权不启动／控制桌面或浏览器。
- [ ] **Step 5: 自审并提交。** 中文提交记录输入、消息和右侧交互。报告浅暗主题和窄容器自动化覆盖及仍需真实视觉验收的范围。

### Task 4: Skill 行为、文档与整体验收

**Files:**
* Modify: `skills/ally/references/workflow-execution.md`，必要的 `skills/ally/SKILL.md` 引用；通过 `pnpm skills:generate` 更新内嵌资源。
* Modify: `docs/architecture/{workflows,agent-capabilities,chat-and-execution,notifications-and-work-board}-cn.md` 与 `docs/guide/chat-and-workbench-cn.md` 中实际实现的相关章节。
* Create: `docs/superpowers/acceptance/2026-10-16-workflow-agent-chat.md`：需求逐项证据及真实平台边界。
* Test: 必要的跨层 CLI／文件／停止回归放回所属 Task 1、2、3 的测试文件，避免仅检查 Skill 包含某段文字。

**Interfaces:**
* Consumes: 前三任务最终契约和完整用户流程；Skill 从 live discovery 获取真实参数，不复制完整 Schema。
* Produces: Agent 执行指南明确自动组织业务输入、固定 Revision、稳定 request-id、wait 超时继续原 Run、终态原因区分、显式 deliver 后把返回文件交付消息、失败／用户停止不自动重跑。
* Produces: 验收报告列出测试、命令、退出码、独立审查、SQL 查询证据与未执行的真实模型／视觉验收，不把模拟行为测试说成真实账号执行。

- [ ] **Step 1: 读取 `writing-skills` 与验证技能。** 使用受控场景先记录旧指南消费行为缺口：已有充分代码评审上下文、缺少必要目标、一次 wait 超时、运行失败、用户主动停止及最终文件交付。测试的是消费者决策，不 grep 指令文本。
- [ ] **Step 2: 更新指南并复测。** 明确每种场景下一步动作，运行 `pnpm skills:generate`、`pnpm skills:check`，以同样受控场景验证新指南行为并记录限度。
- [ ] **Step 3: 更新当前能力文档并整体验收。** 在 worktree 运行 `NODE_OPTIONS=--no-experimental-webstorage pnpm check`，必要时追加已识别竞态的 race 测试；确认每个 SPEC 验收点都有证据或明确未验证边界。
- [ ] **Step 4: 自审并提交。** 提交指南、生成资源、文档与验收报告；主控生成整个功能分支的独立评审包，修复确认的重要问题后重新运行相关测试和完整检查。
- [ ] **Step 5: 分支交付。** 按 `finishing-a-development-branch` 保留独立 worktree／分支和提交，报告 SPEC、计划、验收路径与检查结果；不自动合并、推送或清理仍待用户查看的 worktree。
