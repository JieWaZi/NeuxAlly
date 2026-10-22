# 知识能力与模型调用边界

普通聊天的知识查询、附件入库及整理发起规则见[知识使用规则](../../skills/ally/references/knowledge.md)。该 Skill 只负责确认目标和调用公开知识能力；任务接纳后由独立 Lite 管线完成整理。

整理模型不读取该 Skill，不领取阶段、不写候选文件，也不通过智能体工具提交结果。模型消息与解析由核心管理，实际 CLI 调用由 Runtime 适配。当前职责与约束统一见[知识库架构](../architecture/knowledge-cn.md#lite-管线与模型接口)。
