# 聊天调用工作流验收

## 范围与结论

实际验收日期为 **2026-10-17**，文件名保留设计开始日。范围依据[聊天调用工作流 SPEC](../specs/2026-10-16-workflow-agent-chat-design.md)和[实施计划](../plans/2026-10-16-workflow-agent-chat.md)，在独立分支 `codex/workflow-agent-chat` 验证。未合并或推送，未打开、控制或截图运行中的桌面与浏览器。

Task 1、2、3 均在修复独立评审发现后通过各自复审；Task 4 当前完成补充集成与文档自审，**整个功能分支的最终独立评审和分支交付仍待主控执行**。以下自动化结果不能替代真实账号、真实模型和桌面视觉验收。

## SPEC 逐项证据

| 验收点 | 实际证据 | 边界 |
| --- | --- | --- |
| 1. 单选、替换、删除、队列及失败恢复 | Task 3 真实 TipTap、ChatPage 与队列测试；第二次选择替换、纯引用发送、按对应队列项恢复正文节点与工作流，甲乙入队后拒绝甲并重试甲 | jsdom 验证交互和身份，不证明像素布局 |
| 2. 首发续发限制一个与授权范围 | Task 1 `TestChatWorkflowInput`、`TestConsumerBindsOneWorkflowReferenceAndInstructions`、首发/续发重命名幂等；Task 3 过滤无当前版本项及特殊入口；新增 CLI 集成拒绝未选定义、其他 Chat 的 get/deliver/receipt | 没有依据已知 ID 扩大授权 |
| 3. 实际 CLI 契约、启动、同 Run 等待与内容 | 新增 `TestConsumerCLIExecutesAndDeliversThroughRealCore` 通过真实 Execute、HTTP Registry、生产 BindConsumer/Execution/Workflow、SQLite、编译器及 Runner，超时后恢复调度并等待原 Run 的 succeeded；内容精确为 `REVIEW THIS CHANGE` | Runtime 协议与环境探测为替身；无真实模型 |
| 4. 自动准备业务输入与必要追问 | 更新前 7 场景、新增 deliver 发现基线，以及更新后 fresh 消费者 9 场景均作出预期决策 | 是独立消费者实际决策，不是静态文本搜索，也不是实际账号执行 |
| 5. 原 Run 右侧与状态/释放 | Task 3 ChatThread、workflow-call、WorkflowRunView 测试覆盖原 runId、错误身份、404/500、断线陈旧提示、总结状态、关闭/隐藏释放及特殊 entry 排除 | 复用实际组件与 HTTP 客户端测试，未打开运行应用 |
| 6. 两入口停止及竞态 | Task 2 持久撤销屏障、跨页取消、精确 Execution 路径与响应失败后的原身份重试；终态保持、重复停止、Go race；新增 CLI 读到 `completed:true`、cancelled 及 `actor:user/source:run/requestedAt` | Runner 真正收敛终态，不手动翻数据库状态 |
| 7. 混合交付与文件边界 | 新增 CLI 集成先读 files 不产生附件，再 deliver 固化文件；真实 Runtime 文本事件和附件在原助手终态消息共存。Task 2 普通字符串、越界软链接、缺失文件、同名不同 Run、非终态拒绝及原大小字节回归 | 只交付输出 Schema 的最终 file；部分失败结果不推断成全部节点文件 |
| 8. 登记顺序、失败恢复与重放 | Task 1 fact-first/message-first；Task 2 真实 Journal 迟到事实、原执行身份、固化后回执失败且源文件删除恢复、HTTP receipt 登记失败恢复；新增 CLI 同 ID replay/receipt、删原产物后读附件字节、原消息只有一份附件 | 不用重跑工作流补偿消息登记；桌面文件打开另有 Electron 自动化，未做视觉操作 |
| 9. 全仓检查、race、SQL | 本次完整检查结果见下节；复用 Task 2 实际 SQL 与已通过 race；新增跨层测试单独 race | 不修改检查豁免，不重复无变化的 SQL 基准 |
| 10. 独立审查与平台边界 | 各任务评审和返工记录见下节；全分支审查待主控 | 未预称全部评审或真实平台通过 |

## 真实 CLI 集成路径

测试位于 [agent_cli_integration_test.go](../../../server/apis/workflows/agent_cli_integration_test.go)，复用 [core_integration_test.go](../../../server/apis/workflows/core_integration_test.go) 的 `openCoreWorkflowPorts`。既有 fixture 补齐真实 Consumer 仓储及与生产 `global/config.go` 相同的 `WithEventPublisher` 数据库事件绑定。测试运行的是 `agentcli.Execute` 参数解析及真实 loopback HTTP，不是模拟整个 Workflow 服务或 HTTP binding。

真实接纳产生 Runtime 请求及受管连接文件，CLI 从该请求的环境读取连接身份。真实生产 `agentCommandFacts.record` 写入 Chat Journal、JSONL 和查询索引。为了确定性覆盖短观察超时，测试暂时停止实际 Runner，接纳并观察原 Run，随后重新启动同一个服务及协调者；运行结果由真实编译结果计算、真实 Runner 提交，没有直接改数据库终态。

测试验证固定 Revision、稳定启动 request-id、同 CommandID 和 RunID 的重放/收据、重复启动和观察阶段仅一个已接纳运行、`completed:false` 观察不取消、最终内容及唯一声明文件、get/wait 不登记文件、deliver 正式附件、重复收据与命令不重复文件、删除原文件后正式附件字节仍准确、其他定义和聊天不可越权，以及实际用户停止原因查询。模型替身最后通过 Runtime 文本事件提交总结，真实终态消息同时保留总结、附件和独立运行引用。

正式附件读取从 Chat 查询得到工作区，从 deliver 回执及消息得到稳定相对路径，再读取精确字节 `verified report bytes`，并非仅检查文件存在。当前 Chat 文件读取入口在 Electron 的 `workspace-files.ts`，没有对应 Go Chat 文件读取服务；本测试不声称执行了 Electron 打开、预览或下载。已有 Electron 文件边界测试纳入全仓检查。

## Skill 消费者证据

旧指南的充分上下文自动准备参数、缺少目标追问、观察超时继续同 Run、失败不重启、用户停止不继续、交付能力缺失准确报告、启动响应不确定查原 receipt 共 7 场景均通过。新增 deliver 已实现后，另一 fresh 消费者读取旧指南及实际发现契约，也正确决定显式 deliver 并在响应丢失后查询原 receipt。**不存在可归因于旧指南的已观察决策失败，不记为 RED。**

本次仅在 [工作流执行参考](../../../skills/ally/references/workflow-execution.md) 补实际取消字段路径、deliver 与附件字段、原请求回执恢复参考，原已验证规则保持。纯参考改动的纪律措辞 micro-test 为 N/A。

更新后 fresh 消费者只读取系统 Skill 与该参考，未读取 SPEC、计划、实现或其他报告；实际 9 场景决策均符合预期：充分上下文自动组织 task；缺仓库/代码版本一次追问；超时继续同 Run；STEP_FAILED 不重跑；用户停止后结束；成功且 files.available 仍先 deliver；交付响应丢失查同 ID receipt；启动响应不确定且源文件已删仍查原 receipt；声明文件不可用时分别报告运行成功与文件未交付。取消夹具使用真实 `get.run` / `wait.resource.run` 嵌套，附件为 `name/path/mediaType/managed`。

消费者原始记录分别为本地执行账本的 `skill-baseline.md` 与 `skill-acceptance.md`。这些记录不代表消费者执行过 CLI；实际 HTTP、运行与文件由上节集成测试证明。

## 验证命令与结果

所有 Go 命令在 `server` 执行，原始日志保存在临时目录，文件名如下。

| 命令 | 结果 | 日志 |
| --- | --- | --- |
| `pnpm skills:generate`、`pnpm skills:check` | 退出 0，内嵌资源由生成器更新 | 本次命令输出 |
| `gofmt -w apis/workflows/agent_cli_integration_test.go apis/workflows/core_integration_test.go` | 已格式化 | 本次命令输出 |
| `go test ./apis/workflows -run TestConsumerCLIExecutesAndDeliversThroughRealCore -count=1` | 退出 0；实际完整链路通过 | `neux-task4-cli-final.log` |
| `go test -race ./services/workflow ./apis/workflows -run 'TestAgentWaitCompletionMatchesReturnedRunSnapshot\|TestConsumerCLIExecutesAndDeliversThroughRealCore' -count=1` | 修复后两包退出 0，无 data-race 报告 | `neux-task4-snapshot-race.log` |
| `go test -race ./services/workflow -count=1` | 退出 0，无 data-race 报告 | `neux-task4-workflow-race.log` |
| `NODE_OPTIONS=--no-experimental-webstorage pnpm check` | 退出 0；252 个前端测试文件 / 1612 项测试、71 项 Electron 测试及全部 Go 包通过 | `neux-task4-check.log` |

最终完整检查在生产源码和测试固定后运行。文档、根脚本、资源、Pi bridge、内嵌技能、工作流 SDK/展示、API 生成一致性、前端类型/Lint/UI/格式、全部测试、生产构建、Electron 构建、Go vet 与 Go test 均通过；没有失败测试或未处理异常。启动画面静态 JS 487822/500000 bytes，工作台启动 JS 4343772/4500000 bytes，均在预算内。输出保留基线已有 jsdom Canvas、Vite 大 chunk 与 OpenAPI 函数类型提示，未新增豁免。检查后只填写验收结果与计划状态，并再次运行 docs:check 和 diff 检查。

新增跨层测试最初覆盖已有生产行为，没有通过删除已有功能制造 RED。其后真实 race 运行发现并修复一处生产快照竞态，见下节。搭建期间的失败均如实归为测试装配问题：缺少输入 Actor、CLI 误用 `--timeout-ms`（实际为 `--timeout`）、fixture 未绑定数据库事件发布、混合消息断言遗漏后来取消测试创建的第二个运行引用。均修正装配或与实际场景一致的断言，未放宽文件和身份校验。

复用 Task 2 SQL 证据：真实 SQLite 共 800 条运行，其中 257 条为该消费者的活跃运行；沿 `idx_workflow_consumer_runs (consumer_lease_id, id)` 实际执行 3 次查询，每次最多 128 行，无 JOIN、无临时排序。另有 130 条跨页停止，首项读取失败时其他 129 条仍进入 cancelling，重复停止恢复首项。日志为 `neux-task2-sql.log`、`neux-task2-sql-race-barrier.log`。

已完成的相关 race：Task 2 最后修复版 `go test -race ./apis/chats ./services/workflow ./internal/chat/messageview -count=1` 通过，日志 `neux-task2-fix1-race.log`。本次新增快照一致性修复后重新运行 Workflow race；查询索引未变，不重复同一 SQL 基准。

## 独立评审与返工

Task 1 评审发现名称快照参与幂等摘要，重命名破坏原请求重放；修复为稳定 ID 并保留首次展示快照，首发、续发、绑定均补回归。修复提交 `2d3ee82`，复审通过。

Task 2 评审发现停止失败后的重试可能读取新活跃执行，以及交付将数据库错误吞作文件不可用。修复为明确 `executions/{executionId}/cancel`、三个前端入口保留原参数重试，并仅将明确文件不可用转成描述；真实基础设施错误继续失败。修复提交 `697c626`，复审通过。返工包含接口生成、前端调用链和真实 RED/GREEN，另有两次 lint/format 夹具问题导致完整检查重跑。

Task 3 评审发现甲乙入队后甲拒绝会从全局编辑器快照恢复乙，导致引用错误。修复为对应项的类型化编辑器快照传递；真实 TipTap 甲乙输入、甲拒绝、恢复甲、API 重试甲通过。修复提交 `629e0eb`，复审通过。前期还通过真实空正文附件集成发现 SDK 只接受用户 attachments，改为助手 metadata 适配；组件 fixture role、FocusScope 清理和格式问题分别修正后完整验证。

Task 4 的真实 CLI race 运行出现 `completed:true` 与同回执 `resource.run.status:cancelling` 不一致（`neux-task4-cli-race.log`）。定位到 wait 先读取资源，再次读取 Run 判断终态，两读之间 Runner 已提交终态。这是逻辑快照竞态，不是 Go data-race 告警。确定性回归 `TestAgentWaitCompletionMatchesReturnedRunSnapshot` 在第一份 queued 快照读出后驱动真实协调者完成，原实现返回 completed:true，取得真实 RED（`neux-task4-wait-red.log`）；最小修复从已返回的 `models.WorkflowRun` 快照判断终态，删除第二次读取。回归 GREEN（`neux-task4-wait-green.log`）及两包聚焦 race 通过（`neux-task4-snapshot-race.log`）。没有新增接口、状态源或等待循环。

Task 4 的自审、聚焦集成及全仓检查属于本次实施证据，不能代替即将由主控组织的全分支独立评审。

## 主控裁定及误判成本

以下保留本地 `progress.md` 全部 Ruling 的实际决策及判断有误时的返工边界；其中已发生的接口、队列恢复与测试布局修正也在上节如实记录。

* Task 1: Ruling: 纳入 intent.go 的唯一输入转换路径；发送失败恢复限定为未接纳输入的引用恢复，已启动回合保护普通原生重试，不新增能力重签 — 符合既有事实及用户不重复运行的要求 — 判断有误时需扩展人工重新运行入口。

* Task 2: Ruling: 允许在既有 Execution 取消职责中增加按捕获身份取消的最小入口 — chatID 取消可能误伤新一轮，必须先绑定本次身份再撤销消费者／停止 — 判断有误时需调整既有取消接口及调用测试。文件固化复用 workspace 的已安全打开 reader，codec 仅独立命令事实显式身份；都仍归原职责。

* Task 2: fix round 1/5 (two Important, reviewer final). Ruling: 停止请求须显式保留原 Execution 身份并允许必要的最小前端调用链修改 — 单在API内部读取current不能覆盖响应失败后跨回合重试，也无法满足真实用户停止行为 — 判断有误时需重调停止接口和所有既有调用者。文件交付只将明确不可用文件转描述，不吞基础设施错误；真实读取故障恢复复用同request-id。

* Task 3: Ruling: 助手正式附件在assistant-ui适配层使用typed metadata承载，再复用ChatMessageAttachments — 实际空正文文件集成RED证明当前SDK只支持用户attachments并会抛错，正式Chat API仍为唯一事实 — 判断有误时需调整适配和附件组件消费，不能以路径文本替代交付。

* Task 3: Ruling: 允许统一useQueryEvents可选onState透传 — 原Hook仅失效cache且既有RunView未反馈断线，SPEC要求就近离线/陈旧状态，必须复用现有订阅源 — 判断有误时需调整所有受影响共享Hook调用并复测，不能另建连接或状态源。

* Task 3: Ruling: 项级编辑器快照通过显式typed拒绝恢复对象/输入props传递，接受约6处窄pass-through，不用assistant-ui.__internal_getRuntime — SDK getState是React缓存，setRunConfig后立即恢复仍读旧空文档；已知项数据应直接送恢复消费者 — 判断有误时需调整恢复参数接口和调用测试，避免升级时依赖SDK私有实现。

* Task 4: Ruling: CLI跨层集成改放execution包并调用真实私有agentCommandFacts.record，不在测试桥接中重写生产事实映射 — 重写映射会绕过本功能关键文件登记路径，SPEC要求仅Runtime/model替身；无需仅为测试公开生产接口或启动全部Runtime Service — 判断有误时需增加测试装配成本或调整跨层测试布局。后续实际验证execution→workflow→execution导入环，改复用apis/workflows现有openCoreWorkflowPorts完整Core fixture，新增该包agent_cli_integration_test.go，真实request.Env连接自然走私有生产Record；无需生产重构。

## 未执行的验收

未使用真实账号模型执行自动补参、持续等待、最终总结和交付；受控消费者是决策测试，Runtime 替身是协议边界测试，两者均不替代该项。未启动或操作桌面、浏览器，未取得真实渲染、浅暗主题像素、文件预览或下载的视觉验收。全分支独立审查、最终交付仍待主控，不自动合并、推送或清理工作树。
