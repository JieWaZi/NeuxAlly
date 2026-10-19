import React, { useState } from "react";
import { createRoot } from "react-dom/client";
import {
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  Bell,
  BookOpen,
  Bot,
  Check,
  CheckCheck,
  ChevronDown,
  ChevronRight,
  CircleAlert,
  Clock3,
  Code2,
  Columns2,
  Copy,
  File,
  FileCheck2,
  FileCode2,
  FileText,
  Files,
  Folder,
  FolderGit2,
  GitBranch,
  History,
  Inbox,
  Info,
  Layers3,
  Link2,
  ListTree,
  LoaderCircle,
  MessageSquare,
  MoreHorizontal,
  Network,
  PanelLeft,
  PanelRight,
  Paperclip,
  Pencil,
  Plus,
  Search,
  Settings2,
  ShieldCheck,
  Sparkles,
  Square,
  Upload,
  Users,
  Workflow,
  X,
} from "lucide-react";
import { Button } from "@/shared/ui/button";
import { Card, CardFooter } from "@/shared/ui/card";
import { Input } from "@/shared/ui/input";
import { Textarea } from "@/shared/ui/textarea";
import { Badge } from "@/shared/ui/badge";
import {
  PageAction,
  ResourcePageHeader,
  ResourcePageSurface,
  ResourcePageToolbar,
  ResourcePageSearchField,
  ResourceCardGrid,
} from "@/shared/ui/resource-page";
import "./style.css";

type View =
  | "library"
  | "create"
  | "raw"
  | "upload"
  | "organize"
  | "running"
  | "wiki"
  | "source"
  | "relations"
  | "history"
  | "project"
  | "chat"
  | "failure"
  | "devwiki";
const params = new URLSearchParams(location.search);
const screens: { id: View; label: string; caption: string }[] = [
  {
    id: "library",
    label: "01 知识库",
    caption: "系统级入口 · 通用与项目分开管理",
  },
  {
    id: "create",
    label: "02 新建知识库",
    caption: "选择类型 · 配置用途与项目关联",
  },
  {
    id: "raw",
    label: "03 原始资料",
    caption: "上传只保存 raw · 新资料保持待整理",
  },
  {
    id: "upload",
    label: "04 上传资料",
    caption: "批量保存文件 · 不自动启动模型",
  },
  {
    id: "organize",
    label: "05 发起整理",
    caption: "在右侧选择智能体和模型 · 处理新增资料",
  },
  {
    id: "running",
    label: "06 整理过程",
    caption: "同一对话跟随执行 · 左侧保留现有知识",
  },
  {
    id: "wiki",
    label: "07 Wiki 与续改",
    caption: "右侧持续修订 · 左侧目录与文章",
  },
  { id: "source", label: "08 来源阅读", caption: "从 Wiki 的引用定位原始资料" },
  {
    id: "relations",
    label: "09 知识关联",
    caption: "概念、主题、来源的关系视图",
  },
  {
    id: "history",
    label: "10 修改历史",
    caption: "逐轮保存记录 · 查看具体内容差异",
  },
  { id: "project", label: "11 项目关联", caption: "同一代码仓共用项目知识库" },
  { id: "chat", label: "12 普通聊天入库", caption: "沿用当前聊天智能体与模型" },
  {
    id: "failure",
    label: "13 失败与重试",
    caption: "保留 raw 与已完成内容 · 明确失败范围",
  },
  {
    id: "devwiki",
    label: "14 软件工程 Wiki",
    caption: "项目内使用 · 功能、实现说明与代码依据",
  },
];
const libraries = [
  {
    title: "知识管理研究",
    desc: "整理知识管理、智能体记忆与 LLM Wiki 的文章和阅读资料。",
    type: "通用",
    sources: 27,
    pages: 63,
    pending: 3,
    icon: BookOpen,
    updated: "刚刚更新",
  },
  {
    title: "产品与行业资料",
    desc: "产品手册、行业白皮书与客户研究，按主题持续积累。",
    type: "通用",
    sources: 18,
    pages: 42,
    pending: 0,
    icon: Files,
    updated: "今天 09:42",
  },
  {
    title: "Atlas 项目 Wiki",
    desc: "项目功能、架构与实现说明，与代码及设计资料保持关联。",
    type: "项目",
    sources: 36,
    pages: 85,
    pending: 2,
    icon: FolderGit2,
    updated: "今天 08:30",
  },
  {
    title: "团队经验手册",
    desc: "工作方法、复盘和实践记录，让零散经验成为可复用知识。",
    type: "通用",
    sources: 12,
    pages: 28,
    pending: 0,
    icon: Users,
    updated: "昨天 16:20",
  },
  {
    title: "阅读与学习",
    desc: "保存文章、书籍笔记与学习材料，连接观点与概念。",
    type: "通用",
    sources: 9,
    pages: 19,
    pending: 1,
    icon: BookOpen,
    updated: "昨天 11:06",
  },
  {
    title: "Studio 项目 Wiki",
    desc: "桌面应用的功能边界、核心流程及常见问题。",
    type: "项目",
    sources: 21,
    pages: 54,
    pending: 0,
    icon: Code2,
    updated: "10 月 15 日",
  },
];
const rawFiles = [
  {
    name: "知识管理白皮书.pdf",
    kind: "PDF",
    size: "1.8 MB",
    time: "今天 10:24",
    state: "待整理",
  },
  {
    name: "团队经验整理.docx",
    kind: "Word",
    size: "246 KB",
    time: "今天 10:22",
    state: "待整理",
  },
  {
    name: "阅读笔记.md",
    kind: "Markdown",
    size: "8 KB",
    time: "今天 10:20",
    state: "待整理",
  },
  {
    name: "智能体记忆综述.pdf",
    kind: "PDF",
    size: "2.3 MB",
    time: "昨天 16:45",
    state: "已整理",
  },
  {
    name: "知识组织方法.md",
    kind: "Markdown",
    size: "12 KB",
    time: "昨天 15:30",
    state: "已整理",
  },
  {
    name: "主题研究记录.docx",
    kind: "Word",
    size: "184 KB",
    time: "10 月 15 日",
    state: "已整理",
  },
];

function IconButton({
  icon: Icon,
  label,
  onClick,
}: {
  icon: any;
  label: string;
  onClick?: () => void;
}) {
  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label={label}
      title={label}
      onClick={onClick}
    >
      <Icon strokeWidth={1.5} />
    </Button>
  );
}
function Chip({
  children,
  tone = "muted",
}: {
  children: React.ReactNode;
  tone?: string;
}) {
  return <span className={`chip ${tone}`}>{children}</span>;
}

function App() {
  const [view, setView] = useState<View>(
    (params.get("view") as View) || "library",
  );
  const [type, setType] = useState(
    params.get("type") === "project" ? "项目" : "通用",
  );
  const [filter, setFilter] = useState("全部");
  const [search, setSearch] = useState("");
  const [runtime, setRuntime] = useState("Codex");
  const [model, setModel] = useState("GPT-5.4");
  const [menu, setMenu] = useState(params.get("menu") || "");
  const [draft, setDraft] = useState("");
  const [notice, setNotice] = useState("");
  const [dark, setDark] = useState(params.get("theme") === "dark");
  const capture = params.get("capture") === "1";
  const [isDev, setIsDev] = useState(
    params.get("view") === "devwiki" || params.get("scope") === "project",
  );
  const [maintenanceState, setMaintenanceState] = useState<
    "empty" | "running" | "completed" | "failure"
  >(
    params.get("view") === "running"
      ? "running"
      : params.get("view") === "failure"
        ? "failure"
        : ["wiki", "source", "relations", "history", "devwiki"].includes(
              params.get("view") || "",
            )
          ? "completed"
          : "empty",
  );
  const [chatFollowups, setChatFollowups] = useState<string[]>([]);
  const [sourcesDone, setSourcesDone] = useState(
    ["wiki", "source", "relations", "history", "devwiki"].includes(
      params.get("view") || "",
    ),
  );
  const [retryOnly, setRetryOnly] = useState(false);
  const batchFiles = retryOnly ? rawFiles.slice(0, 1) : rawFiles.slice(0, 3);
  // 内容标签只切换左侧阅读位置，知识库身份和维护对话独立保存。
  const go = (next: View) => {
    setView(next);
    setMenu("");
    setNotice("");
    if (next === "library") {
      setIsDev(false);
      setMaintenanceState("empty");
      setRetryOnly(false);
      setSourcesDone(false);
    }
    if (next === "chat") setIsDev(false);
    if (next === "devwiki") {
      setIsDev(true);
      setMaintenanceState("completed");
    }
    if (
      maintenanceState === "failure" &&
      (next === "organize" || next === "running")
    )
      setRetryOnly(true);
    if (next === "running") setMaintenanceState("running");
    if (next === "failure") setMaintenanceState("failure");

    history.replaceState(
      null,
      "",
      `?view=${next}${capture ? "&capture=1" : ""}${dark ? "&theme=dark" : ""}${next === "devwiki" || isDev ? "&scope=project" : ""}`,
    );
  };
  const finishDemo = () => {
    setMaintenanceState("completed");
    setSourcesDone(true);
    go("wiki");
  };
  const detail = [
    "raw",
    "upload",
    "organize",
    "running",
    "wiki",
    "source",
    "relations",
    "history",
    "failure",
    "devwiki",
  ].includes(view);
  const activeTab = ["raw", "upload", "organize", "failure"].includes(view)
    ? "原始资料"
    : view === "relations"
      ? "关联"
      : view === "history"
        ? "历史"
        : "Wiki";
  const nav = [
    ["工作台", Columns2],
    ["知识库", BookOpen],
    ["工作流", Workflow],
    ["自动任务", Clock3],
  ] as const;
  const rows = libraries.filter(
    (k) => (filter === "全部" || k.type === filter) && k.title.includes(search),
  );
  const showNotice = (text: string) => {
    setNotice(text);
    window.setTimeout(() => setNotice(""), 2400);
  };

  return (
    <div className="prototype-root" data-theme={dark ? "dark" : "light"}>
      {!capture && (
        <div className="review-strip">
          <span>
            <b>NeuxAlly</b> 知识库设计原型
          </span>
          <select
            aria-label="选择原型页面"
            value={view}
            onChange={(e) => go(e.target.value as View)}
          >
            {screens.map((s) => (
              <option key={s.id} value={s.id}>
                {s.label}
              </option>
            ))}
          </select>
          <span className="review-caption">
            {screens.find((s) => s.id === view)?.caption}
          </span>
          <button onClick={() => setDark(!dark)}>
            {dark ? "浅色" : "暗色"}
          </button>
          <span className="review-note">演示数据 · 不调用真实业务</span>
        </div>
      )}
      <div className="app-window">
        <aside className="global-sidebar">
          <div className="mac-top">
            <i />
            <i />
            <i />
            <span />
            <PanelLeft size={15} />
          </div>
          <div className="brand">
            <img src="/ally-logo.svg" />
            <span>NeuxAlly</span>
            <IconButton
              icon={MoreHorizontal}
              label="更多"
              onClick={() => showNotice("原型仅演示知识库流程")}
            />
          </div>
          <button className="new-chat" onClick={() => go("chat")}>
            <MessageSquare size={15} />
            发起对话
            <Plus size={14} />
          </button>
          <nav>
            {nav.map(([label, Icon]) => (
              <button
                key={label}
                className={
                  label === "知识库" && view !== "project" && view !== "chat"
                    ? "active"
                    : ""
                }
                onClick={() =>
                  label === "知识库"
                    ? go("library")
                    : label === "工作台"
                      ? go("chat")
                      : showNotice("保持现有模块入口")
                }
              >
                <Icon size={17} strokeWidth={1.5} />
                <span>{label}</span>
                {label === "知识库" && <span className="nav-count">6</span>}
              </button>
            ))}
          </nav>
          <div className="side-section">
            <span>伙伴</span>
            <Plus size={13} />
          </div>
          <button
            className={`partner ${view === "chat" ? "selected" : ""}`}
            onClick={() => go("chat")}
          >
            <span className="partner-avatar">研</span>
            <span>研究助手</span>
            <span className="online-dot" />
          </button>
          <button
            className={`partner ${view === "project" ? "selected" : ""}`}
            onClick={() => go("project")}
          >
            <span className="partner-avatar developer">开</span>
            <span>开发伙伴</span>
          </button>
          <div className="side-section">
            <span>最近对话</span>
            <MoreHorizontal size={14} />
          </div>
          {["整理知识管理资料", "Atlas 项目开发", "讨论产品手册"].map((t) => (
            <button
              key={t}
              className="recent"
              onClick={() => go(t.includes("Atlas") ? "project" : "chat")}
            >
              <MessageSquare size={13} />
              {t}
            </button>
          ))}
          <div className="sidebar-bottom">
            <button onClick={() => showNotice("复用现有设置入口")}>
              <Settings2 size={16} />
              设置
            </button>
            <IconButton
              icon={Bell}
              label="通知"
              onClick={() => showNotice("暂无新的通知")}
            />
          </div>
        </aside>
        <main className="main-shell">
          <div className="scene-chrome">
            <IconButton
              icon={ArrowLeft}
              label="返回"
              onClick={() => go("library")}
            />
            <IconButton
              icon={ArrowRight}
              label="前进"
              onClick={() => go("wiki")}
            />
            <div className="scene-tab">
              <BookOpen size={14} />
              {view === "project"
                ? "Atlas 项目"
                : view === "chat"
                  ? "研究助手"
                  : detail
                    ? isDev
                      ? "Atlas 项目 Wiki"
                      : "知识管理研究"
                    : "知识库"}
              <X size={12} />
            </div>
            <button className="chrome-add" onClick={() => go("create")}>
              <Plus size={15} />
            </button>
            <span className="flex-space" />
            <IconButton
              icon={PanelRight}
              label="工作台"
              onClick={() => go("source")}
            />
          </div>
          <div className="scene-content">
            {(view === "library" || view === "create") && (
              <ResourcePageSurface>
                <ResourcePageHeader
                  title="知识库"
                  description="把原始资料整理为可持续维护的 Wiki，让智能体按需查阅。"
                  actions={
                    <PageAction
                      intent="primary"
                      label="新建知识库"
                      onClick={() => go("create")}
                    >
                      <Plus />
                    </PageAction>
                  }
                />
                <ResourcePageToolbar
                  filters={
                    <>
                      <select
                        className="filter-control"
                        aria-label="知识库类型"
                        value={filter}
                        onChange={(e) => setFilter(e.target.value)}
                      >
                        <option>全部</option>
                        <option>通用</option>
                        <option>项目</option>
                      </select>
                      <ResourcePageSearchField
                        placeholder="搜索知识库"
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                      />
                    </>
                  }
                >
                  <h2 className="type-body font-medium">全部知识库</h2>
                  <span className="type-caption text-muted-foreground">
                    {rows.length} 个
                  </span>
                </ResourcePageToolbar>
                <ResourceCardGrid className="mt-6">
                  {rows.map((k) => (
                    <Card variant="resource" key={k.title}>
                      <button
                        className="kb-card-main"
                        onClick={() =>
                          go(k.type === "项目" ? "devwiki" : "raw")
                        }
                      >
                        <k.icon size={21} strokeWidth={1.5} />
                        <div>
                          <div className="card-title-line">
                            <h2>{k.title}</h2>
                            <Chip>{k.type}</Chip>
                          </div>
                          <p>{k.desc}</p>
                          <div className="card-stats">
                            <span>{k.sources} 份资料</span>
                            <span>{k.pages} 篇 Wiki</span>
                            {k.pending > 0 && (
                              <span className="pending-dot">
                                {k.pending} 份待整理
                              </span>
                            )}
                          </div>
                        </div>
                      </button>
                      <CardFooter>
                        <span className="type-caption text-muted-foreground">
                          {k.updated}
                        </span>
                        <span className="flex-space" />
                        {k.type === "项目" ? (
                          <span className="scope-label">
                            <FolderGit2 size={12} />
                            项目内使用
                          </span>
                        ) : (
                          <span className="scope-label">
                            <Users size={12} />
                            通用知识
                          </span>
                        )}
                        <IconButton
                          icon={MoreHorizontal}
                          label="知识库操作"
                          onClick={() => go("history")}
                        />
                      </CardFooter>
                    </Card>
                  ))}
                </ResourceCardGrid>
                <div className="library-footnote">
                  <Info size={14} />
                  <span>
                    通用知识库供伙伴按需查询；项目知识库随当前项目提供。
                  </span>
                </div>
              </ResourcePageSurface>
            )}

            {detail && (
              <div className="knowledge-detail">
                <header className="kb-detail-header">
                  <button className="breadcrumb" onClick={() => go("library")}>
                    知识库
                    <ChevronRight size={13} />
                  </button>
                  <BookOpen size={19} strokeWidth={1.5} />
                  <h1>{isDev ? "Atlas 项目 Wiki" : "知识管理研究"}</h1>
                  <Chip>{isDev ? "项目" : "通用"}</Chip>
                  <span className="detail-purpose">
                    {isDev
                      ? "当前项目：Atlas · 两个项目共用"
                      : "文章、白皮书与研究笔记"}
                  </span>
                  <span className="flex-space" />
                  <Button variant="secondary" onClick={() => go("upload")}>
                    <Upload />
                    上传资料
                  </Button>
                  <IconButton
                    icon={Settings2}
                    label="知识库设置"
                    onClick={() => go("create")}
                  />
                </header>
                <div className="knowledge-workspace">
                  <section className="knowledge-content">
                    <div className="knowledge-tabs">
                      {["原始资料", "Wiki", "关联", "历史"].map((t) => (
                        <button
                          className={activeTab === t ? "selected" : ""}
                          key={t}
                          onClick={() =>
                            go(
                              t === "原始资料"
                                ? "raw"
                                : t === "Wiki"
                                  ? "wiki"
                                  : t === "关联"
                                    ? "relations"
                                    : "history",
                            )
                          }
                        >
                          {t === "原始资料" ? (
                            <Files size={14} />
                          ) : t === "Wiki" ? (
                            <BookOpen size={14} />
                          ) : t === "关联" ? (
                            <Network size={14} />
                          ) : (
                            <History size={14} />
                          )}
                          <span>{t}</span>
                          {t === "原始资料" && (
                            <small>{isDev ? "36" : "27"}</small>
                          )}
                          {t === "Wiki" && <small>{isDev ? "85" : "63"}</small>}
                        </button>
                      ))}
                      <span className="flex-space" />
                      <IconButton
                        icon={Search}
                        label="搜索内容"
                        onClick={() => go("wiki")}
                      />
                      <IconButton
                        icon={PanelRight}
                        label="来源面板"
                        onClick={() => go("source")}
                      />
                    </div>
                    {["raw", "upload", "organize", "failure"].includes(view) ? (
                      <div className="raw-content">
                        <div className="content-heading">
                          <div>
                            <h2>原始资料</h2>
                            <p>保存上传原件，整理后可从 Wiki 回溯来源。</p>
                          </div>
                          <Button
                            disabled={sourcesDone}
                            onClick={() => go("organize")}
                          >
                            <Sparkles />
                            {maintenanceState === "failure"
                              ? "重试资料"
                              : "整理新增资料"}
                            <small className="button-count">
                              {sourcesDone
                                ? 0
                                : maintenanceState === "failure" || retryOnly
                                  ? 1
                                  : 3}
                            </small>
                          </Button>
                        </div>
                        <div className="pending-banner">
                          <Inbox size={17} />
                          <div>
                            <b>
                              {sourcesDone
                                ? "本次资料已整理"
                                : maintenanceState === "failure"
                                  ? "1 份资料需要重试"
                                  : retryOnly
                                    ? "1 份失败资料待重试"
                                    : "3 份新增资料待整理"}
                            </b>
                            <span>
                              {sourcesDone
                                ? "原件与来源引用已保存，可继续上传新资料。"
                                : maintenanceState === "failure"
                                  ? "原件已保留，已完成的 Wiki 内容不受影响。"
                                  : "上传仅保存原件；点击整理后，才会更新 Wiki。"}
                            </span>
                          </div>
                          <span className="flex-space" />
                          <button
                            onClick={() =>
                              go(sourcesDone ? "wiki" : "organize")
                            }
                          >
                            {sourcesDone ? "查看 Wiki" : "查看本次范围"}
                            <ChevronRight size={12} />
                          </button>
                        </div>
                        <div className="raw-toolbar">
                          <button className="folder-button">
                            <Folder size={14} />
                            全部资料
                            <ChevronDown size={12} />
                          </button>
                          <span className="flex-space" />
                          <div className="mini-search">
                            <Search size={13} />
                            <span>搜索资料名称</span>
                          </div>
                          <IconButton
                            icon={MoreHorizontal}
                            label="资料操作"
                            onClick={() =>
                              showNotice("可在资料详情中查看来源和版本")
                            }
                          />
                        </div>
                        <table className="raw-table">
                          <thead>
                            <tr>
                              <th>
                                <input
                                  type="checkbox"
                                  aria-label="选择全部资料"
                                />
                              </th>
                              <th>名称</th>
                              <th>整理状态</th>
                              <th>大小</th>
                              <th>添加时间</th>
                              <th />
                            </tr>
                          </thead>
                          <tbody>
                            {rawFiles.map((f, i) => (
                              <tr key={f.name}>
                                <td>
                                  <input
                                    type="checkbox"
                                    aria-label={`选择${f.name}`}
                                  />
                                </td>
                                <td>
                                  <button
                                    className="file-name"
                                    onClick={() => go("source")}
                                  >
                                    <span
                                      className={`file-icon ${f.kind === "PDF" ? "pdf" : f.kind === "Word" ? "word" : ""}`}
                                    >
                                      <FileText size={16} />
                                    </span>
                                    <span>
                                      {f.name}
                                      <small>{f.kind}</small>
                                    </span>
                                  </button>
                                </td>
                                <td>
                                  <Chip
                                    tone={
                                      sourcesDone
                                        ? "success"
                                        : maintenanceState === "failure"
                                          ? i === 0
                                            ? "danger"
                                            : "success"
                                          : f.state === "待整理"
                                            ? "warning"
                                            : "success"
                                    }
                                  >
                                    {sourcesDone
                                      ? "已整理"
                                      : maintenanceState === "failure"
                                        ? i === 0
                                          ? "提取失败"
                                          : "已整理"
                                        : f.state}
                                  </Chip>
                                </td>
                                <td>{f.size}</td>
                                <td>{f.time}</td>
                                <td>
                                  <IconButton
                                    icon={MoreHorizontal}
                                    label={`${f.name}操作`}
                                    onClick={() => go("source")}
                                  />
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                        <div className="table-footer">
                          显示 6 / {isDev ? 36 : 27} 份资料
                          <span className="flex-space" />
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => showNotice("当前为原型示例资料")}
                          >
                            加载更多
                          </Button>
                        </div>
                      </div>
                    ) : view === "relations" ? (
                      <Relations go={go} />
                    ) : view === "history" ? (
                      <HistoryView go={go} />
                    ) : view === "source" ? (
                      <SourceView go={go} />
                    ) : isDev ? (
                      <DevWikiView go={go} />
                    ) : (
                      <WikiView
                        running={maintenanceState === "running"}
                        go={go}
                      />
                    )}
                  </section>
                  <section className="maintenance-chat">
                    <div className="pane-title">
                      <MessageSquare size={15} />
                      <b>维护对话</b>
                      <span className="flex-space" />
                      <IconButton
                        icon={History}
                        label="历史对话"
                        onClick={() => go("history")}
                      />
                      <IconButton
                        icon={Plus}
                        label="新对话"
                        onClick={() => go("organize")}
                      />
                    </div>
                    <div className="chat-context">
                      <BookOpen size={12} />
                      {isDev ? "Atlas 项目 Wiki" : "知识管理研究"}
                      <span />
                      <span>{isDev ? "项目内使用" : "通用知识库"}</span>
                    </div>
                    <div
                      className="chat-messages"
                      tabIndex={0}
                      aria-label="维护对话消息"
                    >
                      {isDev ? (
                        <DevChat />
                      ) : maintenanceState === "empty" ? (
                        <div className="chat-intro">
                          <span className="intro-icon">
                            <Sparkles size={23} strokeWidth={1.5} />
                          </span>
                          <h2>一起维护这份知识</h2>
                          <p>
                            整理新增资料，连接概念和主题。
                            <br />
                            对结果不满意，随时继续告诉我。
                          </p>
                          {view === "organize" && (
                            <div className="batch-preview">
                              <b>本次待整理</b>
                              <span>
                                {batchFiles.length} 份
                                {retryOnly ? "失败资料" : "新增资料"}
                              </span>
                              {batchFiles.map((f) => (
                                <div key={f.name}>
                                  <FileText size={13} />
                                  {f.name}
                                </div>
                              ))}
                            </div>
                          )}
                          <div className="starter-prompts">
                            {[
                              "整理新增资料",
                              "检查来源引用",
                              "合并重复概念",
                            ].map((t) => (
                              <button
                                key={t}
                                onClick={() =>
                                  t === "整理新增资料"
                                    ? go("organize")
                                    : setDraft(t)
                                }
                              >
                                {t}
                                <ArrowUp size={12} />
                              </button>
                            ))}
                          </div>
                        </div>
                      ) : (
                        <>
                          <div className="user-message">
                            {maintenanceState === "failure"
                              ? "整理刚上传的这三份资料。"
                              : retryOnly
                                ? "只重试知识管理白皮书.pdf，保留其他已整理成果。"
                                : "整理新增的 3 份资料，重点补充智能体记忆和知识组织之间的关系。"}
                          </div>
                          <div className="assistant-id">
                            <span className="assistant-glyph">
                              <Sparkles size={13} />
                            </span>
                            <b>知识整理</b>
                            <span>{runtime}</span>
                          </div>
                          <p className="assistant-text">
                            我会先阅读新增资料，再查找已有主题和概念，合并相关内容并保留来源引用。
                          </p>
                          <button
                            className={`process-card ${maintenanceState === "running" ? "is-running" : ""}`}
                            onClick={() =>
                              maintenanceState === "running"
                                ? finishDemo()
                                : go("history")
                            }
                          >
                            <span className="process-heading">
                              {maintenanceState === "running" ? (
                                <LoaderCircle size={14} />
                              ) : maintenanceState === "failure" ? (
                                <CircleAlert size={14} />
                              ) : (
                                <CheckCheck size={14} />
                              )}
                              <b>
                                {maintenanceState === "running"
                                  ? "正在整理新增资料"
                                  : maintenanceState === "failure"
                                    ? "部分资料需要重试"
                                    : "已完成本次整理"}
                              </b>
                              <ChevronDown size={13} />
                            </span>
                            <span className="process-step">
                              <Check size={12} />
                              读取 {batchFiles.length} 份原始资料
                            </span>
                            <span className="process-step">
                              <Check size={12} />
                              定位 8 篇相关 Wiki
                            </span>
                            <span className="process-step">
                              {maintenanceState === "running" ? (
                                <LoaderCircle size={12} />
                              ) : (
                                <Check size={12} />
                              )}
                              {maintenanceState === "running"
                                ? "正在合并概念与主题"
                                : "更新页面及来源引用"}
                            </span>
                          </button>
                          {maintenanceState === "running" ? (
                            <p className="assistant-text subtle">
                              正在完善“智能体长期记忆”的来源与相关概念。
                            </p>
                          ) : maintenanceState === "failure" ? (
                            <>
                              <p className="assistant-text">
                                已完成 2
                                份资料的整理。“知识管理白皮书.pdf”未能提取内容，原件已保留，其他成果已保存。
                              </p>
                              <Button
                                size="sm"
                                variant="secondary"
                                onClick={() => go("running")}
                              >
                                重试这份资料
                              </Button>
                            </>
                          ) : (
                            <>
                              <p className="assistant-text">
                                已新增 <b>5 篇</b> Wiki，更新 <b>8 篇</b>
                                已有页面，补充了来源和关联。
                              </p>
                              <div className="changed-pages">
                                <button onClick={() => go("wiki")}>
                                  <FileText size={13} />
                                  智能体长期记忆
                                  <Chip tone="success">已更新</Chip>
                                </button>
                                <button onClick={() => go("relations")}>
                                  <Network size={13} />
                                  知识组织与主题关联
                                  <ChevronRight size={12} />
                                </button>
                              </div>
                              <p className="assistant-text">
                                你可以在左侧阅读，也可以继续告诉我哪些内容需要调整。
                              </p>
                              <div className="user-message followup">
                                把“智能体记忆”和“长期记忆”合并，保留不同来源对概念边界的解释。
                              </div>
                              <div className="assistant-id">
                                <span className="assistant-glyph">
                                  <Sparkles size={13} />
                                </span>
                                <b>知识整理</b>
                              </div>
                              <p className="assistant-text">
                                已合并，并补充“适用边界”与来源差异说明。相关链接也已同步更新。
                              </p>
                              <button
                                className="saved-receipt"
                                onClick={() => go("history")}
                              >
                                <Check size={13} />
                                已保存 · 版本 12
                                <ArrowRight size={12} />
                              </button>
                            </>
                          )}
                        </>
                      )}
                      {maintenanceState !== "empty" && view === "organize" && (
                        <div className="batch-preview">
                          <b>本次待整理</b>
                          <span>
                            {batchFiles.length} 份
                            {retryOnly ? "失败资料" : "新增资料"}
                          </span>
                          {batchFiles.map((f) => (
                            <div key={f.name}>
                              <FileText size={13} />
                              {f.name}
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                    <div className="chat-composer">
                      <div className="composer-field">
                        <textarea
                          aria-label="知识库维护要求"
                          placeholder="描述整理要求，或继续修改这份 Wiki…"
                          value={draft}
                          onChange={(e) => setDraft(e.target.value)}
                        />
                        <div className="composer-tools">
                          <IconButton
                            icon={Plus}
                            label="添加资料"
                            onClick={() => go("upload")}
                          />
                          <IconButton
                            icon={Paperclip}
                            label="附件"
                            onClick={() => go("upload")}
                          />
                          <span className="flex-space" />
                          <button
                            className="send-button"
                            title={
                              maintenanceState === "running"
                                ? "停止整理"
                                : "发送要求"
                            }
                            aria-label={
                              maintenanceState === "running"
                                ? "停止整理"
                                : "发送要求"
                            }
                            onClick={() => {
                              if (maintenanceState === "running") go("failure");
                              else if (
                                view === "organize" ||
                                maintenanceState === "empty"
                              )
                                go("running");
                              else {
                                go("wiki");
                                showNotice("已在当前维护对话中更新 Wiki");
                              }
                            }}
                          >
                            {maintenanceState === "running" ? (
                              <Square size={13} />
                            ) : (
                              <ArrowUp size={17} />
                            )}
                          </button>
                        </div>
                      </div>
                      <div className="composer-config">
                        <button
                          onClick={() =>
                            setMenu(menu === "runtime" ? "" : "runtime")
                          }
                        >
                          <span className="runtime-mark">C</span>
                          {runtime}
                          <ChevronDown size={11} />
                        </button>
                        <button
                          onClick={() =>
                            setMenu(menu === "model" ? "" : "model")
                          }
                        >
                          {model}
                          <ChevronDown size={11} />
                        </button>
                        <span className="flex-space" />
                        <button
                          onClick={() => showNotice("使用当前执行的默认权限")}
                        >
                          <ShieldCheck size={11} />
                          默认权限
                        </button>
                      </div>
                      {menu && (
                        <div className="config-menu">
                          <span>
                            {menu === "runtime" ? "选择智能体" : "选择模型"}
                          </span>
                          {(menu === "runtime"
                            ? ["Codex", "OpenCode", "Claude Code"]
                            : ["GPT-5.4", "当前默认模型"]
                          ).map((t) => (
                            <button
                              key={t}
                              onClick={() => {
                                menu === "runtime"
                                  ? setRuntime(t)
                                  : setModel(t);
                                setMenu("");
                              }}
                            >
                              {t}
                              {t === (menu === "runtime" ? runtime : model) && (
                                <Check size={13} />
                              )}
                            </button>
                          ))}
                        </div>
                      )}
                    </div>
                  </section>
                </div>
              </div>
            )}

            {view === "project" && (
              <ProjectView go={go} showNotice={showNotice} />
            )}
            {view === "chat" && (
              <OrdinaryChat
                go={go}
                followups={chatFollowups}
                onSend={(message) =>
                  setChatFollowups((messages) => [...messages, message])
                }
              />
            )}
          </div>
        </main>
      </div>
      {view === "create" && (
        <div className="modal-backdrop">
          <div className="dialog-shell">
            <div className="dialog-heading">
              <div>
                <h2>新建知识库</h2>
                <p>选择知识类型，之后上传资料并整理为 Wiki。</p>
              </div>
              <IconButton icon={X} label="关闭" onClick={() => go("library")} />
            </div>
            <div className="form-group">
              <label>知识库名称</label>
              <Input
                key={type}
                size="md"
                defaultValue={
                  type === "项目" ? "Atlas 项目 Wiki" : "知识管理研究"
                }
                placeholder="例如：知识管理研究"
              />
              <p>用于识别知识范围，也会展示给智能体。</p>
            </div>
            <div className="form-group">
              <label>类型</label>
              <div className="type-options">
                {["通用", "项目"].map((t) => (
                  <button
                    className={type === t ? "chosen" : ""}
                    key={t}
                    onClick={() => setType(t)}
                  >
                    {t === "通用" ? (
                      <BookOpen size={20} />
                    ) : (
                      <FolderGit2 size={20} />
                    )}
                    <b>{t}知识库</b>
                    <p>
                      {t === "通用"
                        ? "文章、概念、主题与资料关联"
                        : "项目功能、实现说明与代码关联"}
                    </p>
                    {type === t && <Check size={14} />}
                  </button>
                ))}
              </div>
            </div>
            {type === "项目" && (
              <div className="form-group">
                <label>关联项目</label>
                <button className="project-select">
                  <FolderGit2 size={15} />
                  Atlas 项目
                  <span className="flex-space" />
                  <ChevronDown size={13} />
                </button>
                <p>
                  仅在关联项目的工作范围中提供给智能体；同一代码仓可以共用知识库。
                </p>
              </div>
            )}
            <div className="form-group">
              <label>用途说明</label>
              <Textarea
                key={type}
                defaultValue={
                  type === "项目"
                    ? "维护 Atlas 项目的功能、架构、实现说明与代码依据。"
                    : "整理知识管理、智能体记忆与 LLM Wiki 的文章和阅读资料。"
                }
                placeholder="描述这份知识库适合回答哪些问题"
              />
              <p>帮助智能体判断何时查询，不需要填写技术配置。</p>
            </div>
            <div className="dialog-note">
              <Info size={14} />
              {type === "通用"
                ? "所有伙伴均可按任务需要查询这份通用知识库。"
                : "不同伙伴的相关项目可以关联同一份项目知识库。"}
            </div>
            <div className="dialog-footer">
              <Button variant="secondary" onClick={() => go("library")}>
                取消
              </Button>
              <Button
                onClick={() => {
                  setIsDev(type === "项目");
                  go(type === "项目" ? "devwiki" : "raw");
                }}
              >
                创建知识库
              </Button>
            </div>
          </div>
        </div>
      )}
      {view === "upload" && (
        <div className="modal-backdrop">
          <div className="dialog-shell upload-dialog">
            <div className="dialog-heading">
              <div>
                <h2>上传原始资料</h2>
                <p>{isDev ? "Atlas 项目 Wiki" : "知识管理研究"}</p>
              </div>
              <IconButton icon={X} label="关闭上传" onClick={() => go("raw")} />
            </div>
            <div className="upload-zone">
              <Upload size={28} strokeWidth={1.5} />
              <b>将文件拖到这里，或选择文件</b>
              <p>PDF、Word、Markdown</p>
              <Button
                variant="secondary"
                onClick={() => showNotice("原型使用下方的 3 份示例文件")}
              >
                选择文件
              </Button>
            </div>
            <div className="upload-file-list">
              {rawFiles.slice(0, 3).map((f) => (
                <div key={f.name}>
                  <FileText size={18} />
                  <span>
                    <b>{f.name}</b>
                    <small>{f.size}</small>
                  </span>
                  <Chip tone="success">待保存</Chip>
                  <IconButton
                    icon={X}
                    label={`移除${f.name}`}
                    onClick={() => showNotice("当前为上传队列示例")}
                  />
                </div>
              ))}
            </div>
            <div className="dialog-note">
              <Inbox size={15} />
              上传只保存为原始资料，之后由你发起整理。
            </div>
            <div className="dialog-footer">
              <Button variant="secondary" onClick={() => go("raw")}>
                取消
              </Button>
              <Button
                onClick={() => {
                  go("raw");
                  showNotice("已保存 3 份原始资料，尚未开始整理");
                }}
              >
                保存 3 份资料
              </Button>
            </div>
          </div>
        </div>
      )}
      {notice && (
        <div className="prototype-toast">
          <Check size={14} />
          {notice}
        </div>
      )}
    </div>
  );
}

function WikiView({
  running,
  go,
}: {
  running: boolean;
  go: (v: View) => void;
}) {
  return (
    <div className="wiki-layout">
      <WikiTree go={go} />
      <article className="wiki-article">
        <div className="article-eyebrow">
          <span>主题 / 智能体记忆</span>
          <span className="flex-space" />
          <button onClick={() => go("history")}>
            <History size={12} />
            版本 12
          </button>
          <button onClick={() => go("wiki")}>
            <Pencil size={12} />
          </button>
        </div>
        <h1>智能体长期记忆</h1>
        <p className="article-lead">
          连接原始经验、持续积累的知识与智能体的当前任务。
        </p>
        <div className="article-meta">
          <Chip>概念</Chip>
          <span>5 份来源</span>
          <span>8 个关联</span>
          <span>今天 10:38 更新</span>
        </div>
        {!running && (
          <div className="saved-inline">
            <Check size={13} />
            已根据维护对话合并概念，并更新来源说明。
          </div>
        )}
        <h2>概念与目标</h2>
        <p>
          长期记忆让智能体能够在不同任务之间复用已经积累的知识。它不仅保存过去发生的内容，也需要组织这些内容与当前问题的关系。
          <button className="citation" onClick={() => go("source")}>
            1
          </button>
        </p>
        <p>
          在知识库中，原始资料保留证据，Wiki
          将多个来源组织为可阅读、可引用的概念和主题。智能体查询时可以先理解已有知识，再回到来源核对细节。
        </p>
        <h2>与知识组织的关系</h2>
        <div className="article-callout">
          <BookOpen size={18} />
          <div>
            <b>先组织知识，再按需查阅</b>
            <p>
              每份新资料可以补充已有主题；相关概念通过链接连接，来源关系帮助判断结论的依据。
            </p>
          </div>
        </div>
        <ul>
          <li>
            <b>概念与别名：</b>统一同一概念的不同表达，保留必要的语义边界。
          </li>
          <li>
            <b>主题与关联：</b>跨资料归纳知识，连接对象、方法和具体实践。
          </li>
          <li>
            <b>来源与变化：</b>保留引用，并在新资料出现时检查旧结论。
          </li>
        </ul>
        <h2>适用边界与来源差异</h2>
        <p>
          不同来源对“记忆”的使用范围有所不同。知识文章关注可复用的理解，个人偏好和执行过程则应保留各自的上下文，避免混淆。
        </p>
        <div className="related-links">
          <span>相关页面</span>
          <button onClick={() => go("relations")}>
            LLM Wiki
            <ArrowRight size={12} />
          </button>
          <button onClick={() => go("relations")}>
            知识组织
            <ArrowRight size={12} />
          </button>
          <button onClick={() => go("relations")}>
            来源追溯
            <ArrowRight size={12} />
          </button>
        </div>
        <div className="article-sources">
          <h3>来源</h3>
          <button onClick={() => go("source")}>
            <span>1</span>
            <FileText size={13} />
            知识管理白皮书.pdf<small>第 12–14 页</small>
            <ChevronRight size={12} />
          </button>
          <button onClick={() => go("source")}>
            <span>2</span>
            <FileText size={13} />
            智能体记忆综述.pdf<small>第二章</small>
            <ChevronRight size={12} />
          </button>
        </div>
      </article>
    </div>
  );
}

function WikiTree({ go }: { go: (v: View) => void }) {
  return (
    <aside className="wiki-tree">
      <div className="tree-search">
        <Search size={13} />
        <span>搜索 Wiki</span>
      </div>
      <div className="tree-section">
        <ChevronDown size={12} />
        主题<span>12</span>
      </div>
      {["知识管理", "智能体记忆", "文档组织", "团队经验"].map((t, i) => (
        <button
          key={t}
          className={i === 1 ? "tree-active" : ""}
          onClick={() => go("wiki")}
        >
          <FileText size={13} />
          {t}
        </button>
      ))}
      <div className="tree-section">
        <ChevronDown size={12} />
        概念<span>28</span>
      </div>
      {["智能体长期记忆", "LLM Wiki", "知识组织", "来源追溯", "增量维护"].map(
        (t, i) => (
          <button
            key={t}
            className={i === 0 ? "tree-current" : ""}
            onClick={() => go("wiki")}
          >
            <FileText size={13} />
            {t}
          </button>
        ),
      )}
      <div className="tree-section">
        <ChevronRight size={12} />
        对象<span>9</span>
      </div>
      <div className="tree-section">
        <ChevronRight size={12} />
        综合与比较<span>8</span>
      </div>
      <div className="tree-section">
        <ChevronRight size={12} />
        来源页<span>6</span>
      </div>
      <div className="tree-bottom">
        <button onClick={() => go("relations")}>
          <Network size={13} />
          查看知识关联
        </button>
      </div>
    </aside>
  );
}

function SourceView({ go }: { go: (v: View) => void }) {
  return (
    <div className="source-layout">
      <div className="source-reader">
        <div className="source-reader-toolbar">
          <button onClick={() => go("wiki")}>
            <ArrowLeft size={14} />
            返回引用页面
          </button>
          <span className="flex-space" />
          <span>12 / 36</span>
          <button>
            100%
            <ChevronDown size={11} />
          </button>
          <IconButton icon={MoreHorizontal} label="原件操作" />
        </div>
        <div className="paper-wrap">
          <div className="pdf-page">
            <div className="pdf-kicker">知识管理研究 · 白皮书</div>
            <div className="pdf-rule" />
            <h2>02　知识的持续积累</h2>
            <h3>2.3　从资料到可复用的知识</h3>
            <p>
              组织知识的目的，在于使经验能够在新的问题中被理解和使用。单独保存材料并不足够，还需要持续建立主题、概念和证据之间的联系。
            </p>
            <p className="source-highlight">
              原始资料保留事实与上下文，知识文章则将多个来源中的相关内容组织起来。通过来源引用，读者可以回到原文核对结论的依据。
            </p>
            <p>
              在持续维护的过程中，新资料既可能补充已有观点，也可能带来不同的解释。记录这些关系有助于保持知识的准确性与可追溯性。
            </p>
            <h3>关联与查阅</h3>
            <p>
              主题连接分散的资料，概念帮助读者理解不同表达之间的关系。检索和阅读应当保留足够的上下文。
            </p>
            <div className="pdf-footer">
              知识管理白皮书<span>12</span>
            </div>
          </div>
        </div>
      </div>
      <aside className="source-inspector">
        <div className="inspector-title">
          <Link2 size={15} />
          <b>引用来源</b>
          <IconButton icon={X} label="关闭来源" onClick={() => go("wiki")} />
        </div>
        <span className="source-type">原始资料</span>
        <h3>知识管理白皮书.pdf</h3>
        <dl>
          <dt>文件版本</dt>
          <dd>版本 1</dd>
          <dt>添加时间</dt>
          <dd>今天 10:24</dd>
          <dt>引用位置</dt>
          <dd>第 12 页 · 2.3 节</dd>
        </dl>
        <div className="quoted-text">
          “原始资料保留事实与上下文，知识文章则将多个来源中的相关内容组织起来。”
        </div>
        <h4>被这些页面引用</h4>
        {["智能体长期记忆", "知识组织", "来源追溯"].map((t) => (
          <button className="inspector-link" key={t} onClick={() => go("wiki")}>
            <FileText size={13} />
            {t}
            <ChevronRight size={12} />
          </button>
        ))}
        <div className="source-inspector-bottom">
          <ShieldCheck size={13} />
          保留原件，引用可回溯
        </div>
      </aside>
    </div>
  );
}

function Relations({ go }: { go: (v: View) => void }) {
  const nodes = [
    { x: 430, y: 260, w: 172, t: "智能体长期记忆", k: "概念", main: true },
    { x: 140, y: 125, w: 130, t: "LLM Wiki", k: "概念" },
    { x: 700, y: 115, w: 130, t: "知识组织", k: "主题" },
    { x: 122, y: 400, w: 152, t: "知识管理白皮书", k: "来源" },
    { x: 710, y: 400, w: 150, t: "来源追溯", k: "概念" },
    { x: 425, y: 470, w: 148, t: "团队经验整理", k: "来源" },
    { x: 450, y: 70, w: 148, t: "记忆与知识管理", k: "综合" },
  ];
  return (
    <div className="relations-content">
      <div className="content-heading">
        <div>
          <h2>知识关联</h2>
          <p>从概念、主题与来源之间的关系探索知识。</p>
        </div>
        <Button variant="secondary" onClick={() => go("wiki")}>
          <ListTree />
          目录阅读
        </Button>
      </div>
      <div className="relation-filters">
        <Chip tone="success">全部关系</Chip>
        <Chip>概念</Chip>
        <Chip>主题</Chip>
        <Chip>来源</Chip>
        <span className="flex-space" />
        <span>选中：智能体长期记忆</span>
      </div>
      <div className="graph-canvas">
        <svg
          viewBox="0 0 980 610"
          role="img"
          aria-label="智能体长期记忆与知识组织、来源的关系图"
        >
          <defs>
            <marker
              id="arrow"
              markerWidth="7"
              markerHeight="7"
              refX="6"
              refY="3.5"
              orient="auto"
            >
              <path d="M0 0 L7 3.5 L0 7" fill="var(--border-strong)" />
            </marker>
          </defs>
          {nodes
            .filter((n) => !n.main)
            .map((n, i) => (
              <g key={n.t}>
                <path
                  d={`M ${n.x + n.w / 2} ${n.y + 25} Q ${i % 2 ? 570 : 330} ${240 + i * 13} 516 285`}
                  fill="none"
                  stroke="var(--border-default)"
                  strokeWidth="1.2"
                  markerEnd="url(#arrow)"
                />
                <text
                  x={(n.x + n.w / 2 + 516) / 2 + 5}
                  y={(n.y + 25 + 285) / 2 - 8}
                  className="graph-edge-label"
                >
                  {i === 2 || i === 4 ? "来源支持" : "相关知识"}
                </text>
              </g>
            ))}
          {nodes.map((n) => (
            <g
              className={n.main ? "graph-node main" : "graph-node"}
              key={n.t}
              onClick={() => go("wiki")}
            >
              <rect x={n.x} y={n.y} width={n.w} height="57" rx="12" />
              <text x={n.x + 14} y={n.y + 22}>
                {n.t}
              </text>
              <text x={n.x + 14} y={n.y + 42} className="graph-kind">
                {n.k}
              </text>
            </g>
          ))}
        </svg>
        <div className="graph-controls">
          <button>−</button>
          <span>100%</span>
          <button>+</button>
          <button onClick={() => go("relations")}>
            <Columns2 size={13} />
          </button>
        </div>
      </div>
      <div className="graph-footnote">
        <Link2 size={13} />
        关系来自 Wiki 页面链接与来源引用，可点击节点继续阅读。
      </div>
    </div>
  );
}

function HistoryView({ go }: { go: (v: View) => void }) {
  return (
    <div className="history-layout">
      <aside className="history-list">
        <div className="history-heading">
          <History size={15} />
          <b>修改记录</b>
        </div>
        {[
          {
            v: 12,
            title: "合并记忆概念并补充边界",
            time: "今天 10:38",
            s: "3 篇页面更新",
          },
          {
            v: 11,
            title: "整理 3 份新增资料",
            time: "今天 10:32",
            s: "新增 5 篇 · 更新 8 篇",
          },
          {
            v: 10,
            title: "完善知识组织主题",
            time: "昨天 16:45",
            s: "4 篇页面更新",
          },
          { v: 9, title: "整理阅读资料", time: "昨天 15:30", s: "新增 6 篇" },
        ].map((r, i) => (
          <button className={i === 0 ? "selected" : ""} key={r.v}>
            <div>
              <b>版本 {r.v}</b>
              {i === 0 && <Chip tone="success">当前</Chip>}
            </div>
            <strong>{r.title}</strong>
            <p>{r.s}</p>
            <small>{r.time}</small>
          </button>
        ))}
      </aside>
      <section className="history-diff">
        <div className="content-heading">
          <div>
            <h2>合并记忆概念并补充边界</h2>
            <p>版本 11 → 版本 12 · 来自维护对话</p>
          </div>
          <Button variant="secondary" onClick={() => go("wiki")}>
            查看当前 Wiki
          </Button>
        </div>
        <div className="change-summary">
          <Chip tone="success">3 篇更新</Chip>
          <span>
            <Plus size={12} />
            24 行
          </span>
          <span className="removed">− 8 行</span>
          <span className="flex-space" />
          <button onClick={() => go("wiki")}>
            <MessageSquare size={13} />
            回到维护对话
          </button>
        </div>
        <div className="diff-file-title">
          <FileText size={14} />
          概念 / 智能体长期记忆
        </div>
        <div className="diff-lines">
          <div>
            <span>18</span>
            <code>## 适用边界与来源差异</code>
          </div>
          <div className="minus">
            <span>19</span>
            <code>− 智能体记忆用于保存长期信息。</code>
          </div>
          <div className="plus">
            <span>19</span>
            <code>+ 不同来源对“记忆”的使用范围有所不同。</code>
          </div>
          <div className="plus">
            <span>20</span>
            <code>+ 知识文章关注可复用的理解，个人偏好和执行过程</code>
          </div>
          <div className="plus">
            <span>21</span>
            <code>+ 则保留各自的上下文，避免混淆。</code>
          </div>
          <div>
            <span>22</span>
            <code></code>
          </div>
          <div className="plus">
            <span>23</span>
            <code>+ 相关概念：知识组织、LLM Wiki、来源追溯。</code>
          </div>
          <div className="plus">
            <span>24</span>
            <code>+ 来源：知识管理白皮书，第 12–14 页。</code>
          </div>
        </div>
        <div className="version-note">
          <Info size={14} />
          <p>每轮修改保留版本和来源。后续对话基于当前 Wiki 继续完善。</p>
        </div>
        <Button variant="ghost" size="sm" onClick={() => go("wiki")}>
          <ArrowLeft />
          返回当前内容
        </Button>
      </section>
    </div>
  );
}

function ProjectView({
  go,
  showNotice,
}: {
  go: (v: View) => void;
  showNotice: (t: string) => void;
}) {
  return (
    <ResourcePageSurface>
      <ResourcePageHeader
        title="Atlas 项目"
        description="开发伙伴 / 项目 · 关联本地代码仓与项目知识"
        actions={
          <PageAction
            intent="secondary"
            label="编辑项目"
            onClick={() => showNotice("原型仅演示知识库关联")}
          >
            <Pencil />
          </PageAction>
        }
      />
      <div className="project-tabs">
        <button>概览</button>
        <button>目录</button>
        <button className="selected">知识库</button>
        <button>记忆</button>
      </div>
      <div className="project-section-heading">
        <div>
          <h2>项目知识库</h2>
          <p>在这个项目的工作范围中，智能体会获得以下知识入口。</p>
        </div>
        <Button variant="secondary" onClick={() => go("create")}>
          <Link2 />
          关联知识库
        </Button>
      </div>
      <Card variant="section">
        <div className="project-kb">
          <span className="project-kb-icon">
            <FolderGit2 size={22} />
          </span>
          <div>
            <h3>Atlas 项目 Wiki</h3>
            <p>功能规则、架构、实现说明与项目代码关联。</p>
            <div className="project-kb-stats">
              36 份原始资料
              <span />
              85 篇 Wiki
              <span />
              今天 08:30 更新
            </div>
          </div>
          <span className="flex-space" />
          <Chip>项目知识库</Chip>
          <Button variant="secondary" onClick={() => go("devwiki")}>
            打开知识库
            <ArrowRight />
          </Button>
        </div>
        <div className="shared-projects">
          <Users size={15} />
          <div>
            <b>这份知识库由相关项目共同使用</b>
            <p>
              开发伙伴 / Atlas 项目<span>研究助手 / Atlas 项目</span>
            </p>
            <small>同一代码仓复用同一份内容和版本，修改对关联项目生效。</small>
          </div>
        </div>
      </Card>
      <div className="project-scope-note">
        <ShieldCheck size={17} />
        <div>
          <b>只在相关项目内使用</b>
          <p>
            离开这个项目后，不向智能体提供这份项目知识库。通用知识库仍可按任务相关性查询。
          </p>
        </div>
      </div>
      <div className="project-section-heading compact">
        <div>
          <h2>通用知识</h2>
          <p>通用知识库由系统统一管理，伙伴按需查询。</p>
        </div>
        <Button variant="ghost" onClick={() => go("library")}>
          查看通用知识库
          <ArrowRight />
        </Button>
      </div>
    </ResourcePageSurface>
  );
}

function OrdinaryChat({
  go,
  followups,
  onSend,
}: {
  go: (v: View) => void;
  followups: string[];
  onSend: (text: string) => void;
}) {
  const [message, setMessage] = useState("");
  return (
    <div className="ordinary-chat">
      <header>
        <span className="partner-avatar">研</span>
        <div>
          <h1>研究助手</h1>
          <p>整理知识管理资料</p>
        </div>
        <span className="flex-space" />
        <IconButton icon={History} label="聊天历史" />
        <IconButton icon={MoreHorizontal} label="对话操作" />
      </header>
      <div className="ordinary-messages">
        <div className="ordinary-user">
          <div className="attachment-pill">
            <FileText size={22} />
            <div>
              <b>知识管理白皮书.pdf</b>
              <span>PDF · 1.8 MB</span>
            </div>
          </div>
          <p>
            把这份白皮书整理到“知识管理研究”的
            Wiki，补充已有主题，不要重复建页。
          </p>
        </div>
        <div className="ordinary-assistant">
          <div className="assistant-id">
            <span className="assistant-glyph">
              <Sparkles size={15} />
            </span>
            <b>研究助手</b>
          </div>
          <p>
            我会沿用当前聊天的智能体和模型，将附件保存为该知识库的原始资料，再检查已有主题并更新
            Wiki。
          </p>
          <div className="chat-operation">
            <div>
              <Check size={14} />
              <b>已保存原始资料</b>
              <Chip tone="success">已完成</Chip>
            </div>
            <p>知识管理白皮书.pdf → 知识管理研究</p>
          </div>
          <div className="chat-operation">
            <div>
              <CheckCheck size={14} />
              <b>Wiki 已更新</b>
              <Chip tone="success">版本 13</Chip>
            </div>
            <p>更新 3 篇页面 · 补充 6 处来源引用</p>
            <button onClick={() => go("wiki")}>
              <BookOpen size={14} />
              打开知识库查看
              <ArrowRight size={13} />
            </button>
          </div>
          <p>
            已补充“智能体长期记忆”“知识组织”和“来源追溯”。原件也已由知识库保存，你可以继续在这里要求修改。
          </p>
        </div>
        {followups.map((followup, index) => (
          <React.Fragment key={index}>
            <div className="user-message">{followup}</div>
            <div className="ordinary-assistant">
              <p>已按本轮要求更新同一知识库，保留来源引用。</p>
              <div className="chat-operation">
                <div>
                  <CheckCheck size={14} />
                  <b>Wiki 已更新</b>
                  <Chip tone="success">版本 {14 + index}</Chip>
                </div>
                <p>本轮修改继续保留在当前聊天中。</p>
              </div>
            </div>
          </React.Fragment>
        ))}
      </div>
      <div className="ordinary-composer">
        <div className="composer-context">
          <BookOpen size={13} />
          本次整理：知识管理研究<Chip>通用</Chip>
        </div>
        <textarea
          placeholder="继续追问或修改整理结果…"
          aria-label="继续对话"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
        />
        <div className="ordinary-tools">
          <IconButton icon={Plus} label="添加附件" />
          <button>
            <span className="runtime-mark">C</span>Codex
            <ChevronDown size={11} />
          </button>
          <button>
            GPT-5.4
            <ChevronDown size={11} />
          </button>
          <span className="flex-space" />
          <button
            className="send-button"
            onClick={() => {
              if (message.trim()) {
                onSend(message);
                setMessage("");
              }
            }}
            aria-label="发送"
          >
            <ArrowUp size={17} />
          </button>
        </div>
      </div>
    </div>
  );
}

function DevChat() {
  return (
    <>
      <div className="user-message">
        对照当前代码，更新登录流程与错误处理的项目 Wiki，保留关键代码入口。
      </div>
      <div className="assistant-id">
        <span className="assistant-glyph">
          <Sparkles size={13} />
        </span>
        <b>项目知识维护</b>
        <span>Codex</span>
      </div>
      <p className="assistant-text">
        我会先读取现有主题与实现说明，再核对当前项目中的相关代码。
      </p>
      <div className="process-card">
        <span className="process-heading">
          <CheckCheck size={14} />
          <b>已完成代码与 Wiki 核对</b>
        </span>
        <span className="process-step">
          <Check size={12} />
          读取登录相关的 4 个代码入口
        </span>
        <span className="process-step">
          <Check size={12} />
          更新功能边界与实现说明
        </span>
        <span className="process-step">
          <Check size={12} />
          保留代码版本及来源引用
        </span>
      </div>
      <p className="assistant-text">
        已更新“身份认证”和“登录链路”，补充了会话失效后的处理。
      </p>
      <div className="user-message followup">
        再补充哪些错误可以重试，哪些需要重新登录。
      </div>
      <div className="assistant-id">
        <span className="assistant-glyph">
          <Sparkles size={13} />
        </span>
        <b>项目知识维护</b>
      </div>
      <p className="assistant-text">
        已补充，并链接到“登录异常排查”。这份 Wiki
        会由关联到同一代码仓的两个项目共用。
      </p>
      <span className="saved-receipt">
        <Check size={13} />
        已保存 · 版本 28
      </span>
    </>
  );
}

function DevWikiView({ go }: { go: (v: View) => void }) {
  return (
    <div className="wiki-layout">
      <aside className="wiki-tree">
        <div className="tree-search">
          <Search size={13} />
          <span>搜索项目 Wiki</span>
        </div>
        <div className="tree-section">
          <ChevronDown size={12} />
          功能主题<span>24</span>
        </div>
        {["项目概览", "身份认证", "工作台", "资料管理"].map((t, i) => (
          <button className={i === 1 ? "tree-active" : ""} key={t}>
            <FileText size={13} />
            {t}
          </button>
        ))}
        <div className="tree-section">
          <ChevronDown size={12} />
          实现说明<span>38</span>
        </div>
        {["登录链路", "会话生命周期", "错误与重试"].map((t, i) => (
          <button className={i === 0 ? "tree-current" : ""} key={t}>
            <FileCode2 size={13} />
            {t}
          </button>
        ))}
        <div className="tree-section">
          <ChevronRight size={12} />
          故障排查<span>16</span>
        </div>
        <div className="tree-section">
          <ChevronRight size={12} />
          术语表<span>7</span>
        </div>
        <div className="tree-bottom">
          <button onClick={() => go("project")}>
            <FolderGit2 size={13} />
            查看关联项目
          </button>
        </div>
      </aside>
      <article className="wiki-article">
        <div className="article-eyebrow">
          <span>实现说明 / 身份认证</span>
          <span className="flex-space" />
          <span>版本 28</span>
        </div>
        <h1>登录链路</h1>
        <p className="article-lead">
          从用户提交登录信息，到会话建立与失效处理的实现路径。
        </p>
        <div className="article-meta">
          <Chip>实现说明</Chip>
          <span>4 个代码入口</span>
          <span>核验于当前项目</span>
        </div>
        <div className="saved-inline">
          <Check size={13} />
          已根据代码补充错误处理与重试边界。
        </div>
        <h2>功能边界</h2>
        <p>
          登录建立当前用户的有效会话。账号校验、会话持久化和客户端状态分别由对应模块处理。
        </p>
        <div className="article-callout">
          <FolderGit2 size={18} />
          <div>
            <b>当前项目：Atlas</b>
            <p>
              知识库在这个项目范围内使用。开发伙伴和研究助手的相关项目共用同一份
              Wiki。
            </p>
          </div>
        </div>
        <h2>调用路径与代码入口</h2>
        <div className="code-anchor-list">
          <div>
            <FileCode2 size={14} />
            <code>web/auth/login.ts</code>
            <span>提交登录请求</span>
          </div>
          <div>
            <FileCode2 size={14} />
            <code>server/auth/session.go</code>
            <span>校验与会话建立</span>
          </div>
          <div>
            <FileCode2 size={14} />
            <code>web/api/client.ts</code>
            <span>处理会话失效</span>
          </div>
        </div>
        <h2>错误与重试</h2>
        <ul>
          <li>
            <b>请求中断：</b>保留输入，用户可以重新发起登录。
          </li>
          <li>
            <b>凭据错误：</b>提示修改账号信息，不自动重复请求。
          </li>
          <li>
            <b>会话失效：</b>清理当前会话状态并引导重新登录。
          </li>
        </ul>
        <h2>相关知识</h2>
        <div className="related-links">
          <button>
            身份认证
            <ArrowRight size={12} />
          </button>
          <button>
            会话生命周期
            <ArrowRight size={12} />
          </button>
          <button>
            登录异常排查
            <ArrowRight size={12} />
          </button>
        </div>
        <div className="article-sources">
          <h3>代码与资料依据</h3>
          <button>
            <FileCode2 size={13} />
            当前代码版本 <small>7f2a9c1 · 示例</small>
          </button>
          <button>
            <FileText size={13} />
            身份认证说明.md <small>原始资料 · 版本 2</small>
          </button>
        </div>
      </article>
    </div>
  );
}

// 热更新沿用同一挂载根，避免设计调整时重复创建 React 根。
const previewRoot =
  import.meta.hot?.data.previewRoot ??
  createRoot(document.getElementById("root")!);
if (import.meta.hot) import.meta.hot.data.previewRoot = previewRoot;
previewRoot.render(<App />);
