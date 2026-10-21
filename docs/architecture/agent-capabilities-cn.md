# 系统技能与软件能力

## 职责

系统 ally 技能教授任务方法，allycli 负责发现、参数和调用，Core 负责身份、授权、领域规则与真实回执。技能与命令名称是当前技术契约，产品名称为 NeuxAlly。

server/internal/agentcapability/contract 是命令 Schema 与解析的共同来源，agentcapability 负责执行授权和有界观察，agentcli 提供客户端。记忆、IM、团队与工作流业务状态仍由各自领域拥有。

## 授权与请求

受管执行得到绑定当前 Execution 的精确能力上下文。能力发现只报告当前可用范围，不能因知道其他命令而扩大授权。上下文失效、身份不匹配或范围不足时拒绝调用。

写命令使用稳定 request-id，保存内容摘要和领域回执。同键同内容重放原回执，同键异内容冲突；响应不确定时查询收据，避免生成新请求重复副作用。

返回区分同步完成、耐久接纳与候选提交。查询或有限 wait 的超时不改变业务状态；真正完成以所属资源终态为准。

普通聊天的工作流能力由所选定义和原 Execution 绑定；启动回执保留定义、不可变版本与运行身份。`workflow.runs.get` 与 `workflow.runs.wait` 读取原运行，`completed` 只代表终态，结果仍需按真实状态和 `cancellationReason` 区分。

`workflow.runs.deliver` 显式交付已声明的最终文件，返回可重放的受管附件引用。生产 Execution 将真实启动与交付回执写入原消息的可信命令事实；查询同操作、同请求身份的回执可补回消息登记。登记失败时 commands 与 receipts 均返回 `ok:false`、原 `commandId` 和 `query_receipt` 指引；`error.details.nextStep` 固定原 operation/requestId，`runId` 保留原运行身份。领域结果与文件仍已提交，只有原消息登记成功才返回正式 attachments，调用者不能换键启动或重跑补偿。完整文件边界见[工作流](workflows-cn.md)，消息归属见[聊天与执行](chat-and-execution-cn.md)。

## 知识分发

skills/ally 是产品技能源，与个人开发技能安装环境分别维护。Core 按内容摘要准备系统目录，每次受管执行获得当前构建携带的资源。普通用户技能不覆盖系统保留身份。

修改资源运行 pnpm skills:generate 与 pnpm skills:check。直接 Go 命令使用已提交 bundle；重新构建并启动 Core 后，新执行才使用新打包内容。

指南按场景解释决策与验证，不静态重复所有命令 Schema。工作流 SDK、参数及结果 Schema 优先通过实时发现获取。
