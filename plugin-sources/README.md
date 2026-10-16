# 固定插件源

`neuxally-plugins.bundle` 是独立 Git 仓库的完整可恢复快照，固定提交和包来源在 `source.lock.json`。开发者可用 `git clone plugin-sources/neuxally-plugins.bundle` 在应用仓库之外恢复源仓库，审阅或维护 `catalog.json`、标准插件包和来源记录。应用运行时不访问 Git 或插件服务器。

`pnpm plugins:prepare` 在构建时从该提交生成本地目录，并按目标平台获取已固定 URL 和 SHA-256 的飞书 CLI 发布资产与 GitHub CLI 源码。`pnpm plugins:check` 仅离线检查源快照和目录，不下载或执行 CLI。已经缓存原始发布资产后，可使用 `node scripts/plugin-resources.mjs --offline` 完成离线准备。

更新流程：在恢复的源仓库修改包并提交，重新导出 bundle，核对并更新来源锁，再执行生成和检查。远端插件仓库可在确定托管位置后作为维护入口；应用仍只消费审核过的固定提交。

飞书包保留官方固定提交的完整 `skills/` 与相对引用、许可证和 provenance；首版主验收路径为文档、多维表格、日程和任务。包维护者与上游作者分别记录。原生程序只进入构建产物，不提交大型程序文件，也不安装到全局 PATH。

GitHub 包使用官方远程 MCP 及固定 GitHub CLI 2.102.0 的隔离构建（2.102.0-neuxally.1）。源码归档、Go 1.27.1 工具链及 patches/ 中底层钥匙串命名空间补丁固定；构建会运行上游真实登录编排的隔离回归。原始发布程序即使使用 --insecure-storage 仍会访问其他 gh 钥匙串，因此不能直接作为本应用的登录程序。离线构建还需预先缓存该工具链和 go.sum 固定的模块依赖。清单与认证适配由 NeuxAlly contributors 编写，操作能力来自官方 MCP；MCP 是远端服务，不随本地包复制或固定服务实现版本。两个提供者程序和许可证均按目标平台进入构建产物。
