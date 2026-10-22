# NeuxAlly UI 标准

## 体系与组件

Tailwind CSS 4 是页面样式体系，shared/ui 持有基础组件，Radix 提供交互原语，Lucide 提供默认图标。业务页面复用共享组件和变体，不重复实现基础控件或引入第二套样式系统。

界面保持紧凑桌面密度。浅色、暗色及跟随系统使用相同布局；主题由 ThemeProvider.resolvedTheme 与根 data-theme 决定。普通 Card 以语义背景、排版、留白分层，无常驻边框和阴影；选中、键盘焦点和错误可使用强调轮廓。

## 排版与尺寸

系统 UI 字体承载界面，系统等宽字体承载代码与终端，不加载远程字体。排版使用 type-_，text-_ 仅承载颜色。

| Token                   | 字号／行高 |
| ----------------------- | ---------- |
| caption                 | 11 / 16px  |
| ui                      | 12 / 16px  |
| body                    | 13 / 20px  |
| label                   | 14 / 20px  |
| title                   | 16 / 24px  |
| page-title              | 20 / 28px  |
| display                 | 24 / 32px  |
| conversation            | 15 / 26px  |
| conversation-title      | 24 / 32px  |
| conversation-heading    | 20 / 30px  |
| conversation-subheading | 17 / 26px  |
| conversation-code       | 14 / 22px  |

常规字重使用 400、500、600；不使用低于 11px 的文字。文字按钮通过 size 使用 28 / 32 / 36px 阶梯，表单控件使用 control-sm / md / lg 的 28 / 36 / 44px 阶梯。独立操作提供至少 36×36px 真实命中区及可访问名称。

图标 icon-xs / sm / md / lg / xl 为 12 / 14 / 16 / 20 / 24px，默认描边 1.5。头像 avatar-xs / sm / md / lg / xl / hero / docked 为 24 / 32 / 40 / 56 / 80 / 128 / 160px，保持完整身体与透明边缘。

间距使用 4px 网格，紧凑控件允许既有 2px、6px 半步。radius-xs / sm / md / lg / xl / 2xl / 3xl / 4xl 为 4 / 6 / 8 / 12 / 16 / 20 / 24 / 32px。文字按钮和 Scene Tab 使用 radius-control，圆形和胶囊使用明确变体。

尺寸与颜色的实际权威定义在 shared/styles、shared/ui 及其变体；禁止通过页面 className 重写基础组件的高度、字号、圆角、图标与阴影。

## 表面、布局与表单

- Scene 使用统一页面背景和 pt-shell-content-top。相同层级优先共享表面，不反复嵌套 Card、标题与分隔线。
- 列表复用 ResourcePageSurface、Header、Toolbar、SearchField 和 CardGrid。工具栏控件达到可读下限后才换行。
- 响应式以所属 Scene 或 pane 的实际容器宽度判断；空间不足时换行或调整结构，保留字体与命中区。
- 表单控件填满所属列；名称、路径、URL 和多行字段使用整列。筛选、搜索、日期范围使用共享语义宽度。
- 单选、短枚举和搜索多选分别复用 Select、ResourceFilterSelect、SearchableMultiSelect。浮层匹配触发框并限制于视口。
- Popover 筛选触发使用 FieldControlTrigger，不用操作 Button 模拟输入字段。
- 字段提供可见标签、示例及必要说明。错误就近显示，说明与错误分别表达；不将数据模型全部展开为默认表单。
- 设置使用唯一 Settings Scene，分类导航与内容独立滚动。设置行随容器换行，不增加无业务意义的全局确定操作。
- 主面板、分组和浮层各有明确层级。常驻阴影只用于有空间语义的浮层，不作为普通内容分组装饰。

## 动效与状态

使用 instant、quick、standard、slow、deliberate 及现有共享 Presence。只过渡实际变化属性，不使用 transition-all 或缩放至零的弹入。监听 prefers-reduced-motion，隐藏、离屏与卸载时暂停或释放动画。

Shell、输入区及可用操作先挂载，加载与错误在所属区域展示。内容首次加载使用与实际布局对应的 Skeleton 骨架屏；知识库卡片、文档列表、Wiki 目录及正文均遵守这一约束，不用转圈或加载文字替代内容骨架。首次加载、空、无结果、后台刷新、离线、错误、取消与禁用分别表达；后台刷新保留已有数据。

统一 Toast 与通知按通知模块契约工作。提示消失不代表阅读或业务处理；当前错误仅在对应操作恢复后清除。权限请求和问答保持原领域入口。

## 工作台与可访问性

Scene 和 pane 组合聊天、文件、浏览器、终端等内容。切换布局保留文档、原生视图和终端身份。工作流会话与画布位于主容器，其他文件进入右侧工作台。关闭内容时释放其资源。

图标操作有 Tooltip 与访问名称；菜单、表单、分屏和弹层支持键盘及焦点恢复。状态同时使用文字和语义色。普通文字对比度至少 4.5:1，重要图标与交互边界至少 3:1。

运行时坐标、图表或编辑器高度、原生窗口适配允许必要布局值，重复使用时提升为共享 Token。任何例外必须解释真实需要，不通过修改检查规则绕过生产约束。

## 验证

执行前端完整检查及 UI 策略检查。验证浅暗主题、窄容器、长中文、键盘、焦点、加载与错误状态。真实页面与原生视图的视觉验收需用户明确授权，并与自动化结果分别说明。
