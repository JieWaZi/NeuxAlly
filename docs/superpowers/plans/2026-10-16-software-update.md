# 桌面软件升级首轮实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans 完成核对与修复，使用 superpowers:requesting-code-review 启动独立审查。步骤以复选框记录实际状态。

**Goal:** 交付未签名版本的检查、下载与手动覆盖升级能力，审查通过后推送功能分支并向 main 提交 PR。

**Architecture:** Electron 主进程持有唯一更新服务，经受限 Bridge 和 revision 快照向共享 Query Cache 交付状态。设置及 Shell 复用状态，系统安装器完成用户手动安装。

**Tech Stack:** Electron 44.2.0、React 19、TypeScript、TanStack Query、Node fetch、Zod、现有 electron-builder DMG/NSIS 与 GitHub Releases。

**Spec:** [桌面软件升级首轮 SPEC](../specs/2026-10-16-software-update-design.md)。

## 全局约束

- 固定仓库 JieWaZi/NeuxAlly；平台为 macOS arm64、macOS x64、Windows x64；保持应用标识与数据目录。
- 本轮未做正式签名，安装由用户手动完成，不新增自动替换、镜像、差分、自动回滚、账号或付费服务。
- 只在独立 worktree 的 codex/software-update 分支工作；用户已选择独立子代理审查、修复后推送并向 main 提交 PR，无需再次选择执行或集成方式。
- 项目文档与业务文案使用中文；英文 README 按项目约定同步。修改前遵守 AGENTS.md 阅读路由，保留无关修改。
- 不打开或控制运行中的页面与桌面应用；自动化与真实平台验收分别报告。

## 审查重点

1. 铃铛被更新操作替换后，审批、问答及工作看板是否仍有可达入口；既有快照不能因更新而改变业务已读。
2. 检查失败、用户重试、已有安装包及新发行撤回等状态下，界面操作是否仍能恢复，不被缓存锁死。
3. 恶意或损坏的元数据、重定向、安装包路径与实际文件大小是否突破主进程信任边界。
4. 慢速响应、流中断、磁盘写入失败及退出期间，句柄、临时文件、订阅与定时器是否完整释放。
5. 快照与事件竞态、多个消费者及 Renderer 重新加载是否导致旧状态覆盖、重复下载或无限订阅。

## 执行说明

本轮开始前已有未提交实现和测试。以下先核对已有代码，再复验，不补写或声称不存在的测试先失败历史。独立审查发现的功能缺陷使用新增回归测试先复现，再修复；修复后更新本文及 SPEC。实现状态与验证状态分开记录。

### 任务 1：主进程升级服务与 Bridge

**Files:**
- 新增：`desktop/electron/main/application-update.ts`、`desktop/electron/shared/application-update.ts`。
- 修改：`desktop/electron/main/index.ts`、`desktop-services.ts`、`desktop/electron/preload/index.ts`、`desktop/electron/shared/contracts.ts`、`validation.ts`。
- 测试：`desktop/electron/tests/application-update.test.mjs`、`bridge.test.mjs`。

**Interfaces:** 消费已打包版本、平台、架构、独立缓存目录、fetch 及系统打开文件能力；提供 `snapshot()`、`subscribe()`、`start()`、`check()`、`download()`、`openInstaller()`、`dispose()` 及 SPEC 中的四个白名单命令。

- [x] 核对已有实现的版本筛选、下载校验、缓存、退出与来源边界，禁止扩展为自动替换。
- [x] 运行 `node --test desktop/electron/tests/application-update.test.mjs desktop/electron/tests/bridge.test.mjs`，21/21 通过。
- [ ] 把确认的问题加入回归测试，观察失败后修复，再复验上述测试；没有缺陷则保留已有实现。
- [x] 主进程、Bridge 及相应测试已提交为 `17acee0`；21/21 定向测试及全量检查通过。

### 任务 2：关于软件、Shell 入口及状态同步

**Files:**
- 新增：`desktop/src/shared/desktop/application-update.ts`、`desktop/src/modules/settings/components/about-software-section.tsx`、`desktop/src/app/layout/desktop-attention-action.tsx` 及各自测试。
- 修改：`desktop/src/app/layout/app-shell.tsx`、`desktop/src/modules/settings/pages/settings-scene.tsx`、`settings-navigation.ts`、`desktop/src/shared/i18n/index.ts`。

**Interfaces:** 消费任务 1 的更新快照、命令及事件；`useApplicationUpdate()` 提供共享状态与操作，设置和 Shell 不维护第二份状态。`DesktopAttentionAction` 复用原工作看板导航回调和更新页导航回调。

- [x] 核对已有设置入口、平台安装文案、共享订阅与 revision 合并，下载完成只提供打开安装包。
- [x] 运行 `NODE_OPTIONS=--no-experimental-webstorage pnpm --dir desktop exec vitest run src/shared/desktop/application-update.test.tsx src/modules/settings/components/about-software-section.test.tsx src/app/layout/desktop-attention-action.test.tsx`，3 个文件、9/9 通过。
- [ ] 独立审查时重点核对通知可达性和异常操作恢复；修复按新增失败回归测试、最小实现、复验的顺序执行。
- [x] 设置、Shell 与状态适配及测试已提交为 `9df1181`；前端定向 9/9 及全量检查通过。

### 任务 3：未签名发行、指南与设计文档

**Files:** `.github/workflows/release.yml`、`README.md`、`README.zh-CN.md`、`docs/development/build-and-release-cn.md`、`docs/architecture/desktop-lifecycle-cn.md`、`notifications-and-work-board-cn.md`、`docs/guide/getting-started-cn.md`、相关讨论及本 SPEC、计划。

**Interfaces:** 消费既有 DMG/EXE 文件名及 SHA256SUMS；发布任务准备全部资产后公开 Release，说明当前未签名并链接对应版本 README。

- [x] README 和发行说明明确未签名、macOS 单应用处理、Windows SmartScreen 与 HTTPS 错误区别；不关闭全局防护或证书校验。
- [x] 补齐入门指南的手动升级路径，把讨论中的建议与最终实现边界归入 SPEC，删除已失效的待确认描述。
- [x] 运行 `pnpm docs:check`、`git diff --check` 及发布工作流 5 个多行 Bash 语法检查，全部通过。
- [x] 发行、指南、SPEC 与计划一并提交；不打 tag、不发布安装包。

### 任务 4：完整验证、独立审查与 PR

- [x] 运行 `NODE_OPTIONS=--no-experimental-webstorage pnpm check`，退出码 0；前端 253 个文件、1595 项测试，Electron 84 项测试，生成内容、API、构建、Go vet 与全部 Go 测试通过，完整日志本地保留。
- [x] 已以 base `4fb33f8` 与内容快照 tree `6f8d724` 启动独立子代理；SPEC 无重要缺口，正在按 code-reviewer 模板审查整个变更，审查只读且不继承作者会话。
- [ ] 核实发现并修复 Critical/Important 问题：回归测试先失败再通过，更新 SPEC、计划并复验受影响检查；记录不采纳理由及剩余边界。
- [ ] 确认最终代码完整检查通过、提交干净，更新本文的验证及审查结果。
- [ ] `git push -u origin codex/software-update`，使用中文标题和说明创建目标为 main 的 PR，附到当前任务；不合并 PR。

## 实施与验证结果

| 项目 | 状态与证据 |
| --- | --- |
| 实现提交 | 主进程与 Bridge：`17acee0`；设置、Shell 与共享状态：`9df1181`；发行与文档随本文提交。 |
| 需求与设计 | 已收敛为未正式签名发行、手动覆盖安装，代码核对后补齐本 SPEC 与计划。 |
| 自动化验证 | 定向主进程与 Bridge 21/21、前端 9/9；完整检查退出码 0，前端 253 个文件、1595 项测试、Electron 84 项测试及全部 Go 检查通过；文档和 5 个发布脚本语法通过。 |
| 独立审查 | 独立子代理的 SPEC 检查无重要缺口；固定内容快照的代码审查进行中，尚未获得合入结论。 |
| 真实平台与视觉 | 未执行；缺少运行中应用操作授权，Windows 真机覆盖安装也未验收。 |
| 推送与 PR | 待验证及审查完成。 |

首次定向测试因独立 worktree 未安装依赖而在加载阶段退出；按现有锁文件安装后复验通过，未改依赖版本或锁文件。已有测试未补记为测试先失败开发；审查修复的回归验证另行记录。
