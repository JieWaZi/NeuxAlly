// Package remote 只通过 HTTP 访问权威端，不持有本地数据库或模型端口。
package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/JieWaZi/neux-knowledge/core"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// WeKnoraProfile 是已按固定源码核对的合同，不表示已探测运行服务器版本。
const WeKnoraProfile = "weknora-8776a2d92d11ad3e76865d51cf3192e37e2a9976"

// Options 配置独立 HTTP 客户端；认证仅用于这一连接。
type Options struct {
	// BaseURL 是部署地址，可包含反向代理前缀。
	BaseURL string
	// Token 是内存中的认证凭据，不在错误或响应中返回。
	Token string
	// AuthMode 为 bearer 或 api_key，默认 bearer。
	AuthMode string
	// HTTPClient 可注入传输层；重定向始终关闭以避免凭据跨站传递。
	HTTPClient *http.Client
}

// Error 区分网络、认证、权限、资源与协议错误；不包含远端敏感响应正文。
type Error struct {
	// Code 为稳定机器错误码。
	Code string `json:"code"`
	// Status 为 HTTP 状态，网络失败时为零。
	Status int `json:"status"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("knowledge remote: %s (HTTP %d)", e.Code, e.Status)
}

// Capability 只说明固定合同的服务器能力；权限仍由每次请求的服务器判定。
type Capability struct {
	// Supported 表示已知协议支持，不等同于凭据授权。
	Supported bool `json:"supported"`
	// Permission 默认 unknown，不能从连接成功推断可写。
	Permission string `json:"permission"`
	// Reason 解释未开放动作的合同差异。
	Reason string `json:"reason,omitempty"`
}

// WeKnora 是官方固定协议适配器，没有本地 Reader 或 Generator 引用。
type WeKnora struct {
	// base 保存已校验的部署地址。
	base *url.URL
	// token 只注入当前服务器请求头。
	token string
	// authMode 选择服务支持的认证头。
	authMode string
	// http 承担有界且禁止重定向的请求。
	http *http.Client
}

func NewWeKnora(o Options) (*WeKnora, error) {
	u, err := url.Parse(o.BaseURL)
	if err != nil || u == nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.TrimSpace(o.Token) == "" {
		return nil, &Error{Code: "invalid_connection"}
	}
	if o.AuthMode == "" {
		o.AuthMode = "bearer"
	}
	if o.AuthMode != "bearer" && o.AuthMode != "api_key" {
		return nil, &Error{Code: "invalid_connection"}
	}
	c := http.Client{Timeout: 30 * time.Second}
	if o.HTTPClient != nil {
		c = *o.HTTPClient
		if c.Timeout == 0 {
			c.Timeout = 30 * time.Second
		}
	}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &WeKnora{base: u, token: o.Token, authMode: o.AuthMode, http: &c}, nil
}
func (c *WeKnora) Capabilities() map[string]Capability {
	caps := map[string]Capability{}
	for _, op := range []string{"libraries", "pages", "folders", "graph", "revisions", "search", "sources", "source_download"} {
		caps[op] = Capability{Supported: true, Permission: "unknown"}
	}
	for _, op := range []string{"libraryDelete", "library_delete", "raw_upload", "organize", "archive", "atomic_restore", "versioned_edit", "software_rules", "manual_placement_protection", "no_source_summaries"} {
		caps[op] = Capability{Permission: "unknown", Reason: "official_profile_has_no_verified_custom_contract"}
	}
	return caps
}

func (c *WeKnora) request(ctx context.Context, path string, q url.Values, max int64) ([]byte, http.Header, error) {
	u := *c.base
	u.Path = strings.TrimRight(c.base.Path, "/") + path
	u.RawPath = ""
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, &Error{Code: "invalid_request"}
	}
	if c.authMode == "api_key" {
		req.Header.Set("X-API-Key", c.token)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/json, application/octet-stream")
	resp, err := c.http.Do(req)
	if err != nil {
		code := "unreachable"
		if ctx.Err() != nil {
			code = "cancelled"
		}
		return nil, nil, &Error{Code: code}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := "server_error"
		switch resp.StatusCode {
		case 401:
			code = "unauthenticated"
		case 403:
			code = "forbidden"
		case 404:
			code = "not_found"
		case 409:
			code = "version_conflict"
		case 429:
			code = "rate_limited"
		}
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			code = "redirect_rejected"
		}
		return nil, nil, &Error{Code: code, Status: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, nil, &Error{Code: "response_incomplete"}
	}
	if int64(len(data)) > max {
		return nil, nil, &Error{Code: "response_too_large"}
	}
	return data, resp.Header, nil
}
func protocolError() error { return &Error{Code: "protocol_incompatible"} }
func (c *WeKnora) object(ctx context.Context, path string, q url.Values, required []string, out any) error {
	b, _, err := c.request(ctx, path, q, 16<<20)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil || fields == nil {
		return protocolError()
	}
	for _, key := range required {
		v, ok := fields[key]
		if !ok || string(v) == "null" {
			return protocolError()
		}
	}
	if json.Unmarshal(b, out) != nil {
		return protocolError()
	}
	return nil
}
func safeID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, "/\\\x00\r\n")
}
func safeSlug(slug string) bool {
	for _, p := range strings.Split(slug, "/") {
		if !safeID(p) {
			return false
		}
	}
	return true
}
func wikiPath(kb, suffix string) (string, error) {
	if !safeID(kb) {
		return "", &Error{Code: "invalid_resource"}
	}
	return "/api/v1/knowledgebase/" + kb + "/wiki/" + suffix, nil
}
func positivePagination(page, size int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 50
	}
	if page < 1 || size < 1 || size > 100 {
		return 0, 0, &Error{Code: "invalid_pagination"}
	}
	return page, size, nil
}

// Library 保留官方知识库类型与权限提示，不冒充定制 general/software。
type Library struct {
	// ID 为远端原始身份。
	ID string `json:"id"`
	// Name 为远端名称。
	Name string `json:"name"`
	// Type 为官方类型，例如 wiki。
	Type string `json:"type"`
	// Permission 为服务器返回的可选共享权限提示。
	Permission string `json:"my_permission"`
}

func (c *WeKnora) ListLibraries(ctx context.Context) ([]Library, error) {
	var v struct {
		// Success 为官方业务成功标志。
		Success bool `json:"success"`
		// Data 为服务器可见库。
		Data []Library `json:"data"`
	}
	err := c.object(ctx, "/api/v1/knowledge-bases", nil, []string{"success", "data"}, &v)
	if err != nil {
		return nil, err
	}
	if !v.Success {
		return nil, protocolError()
	}
	for _, x := range v.Data {
		if x.ID == "" || x.Name == "" {
			return nil, protocolError()
		}
	}
	return v.Data, nil
}
func (c *WeKnora) GetLibrary(ctx context.Context, id string) (*Library, error) {
	if !safeID(id) {
		return nil, &Error{Code: "invalid_resource"}
	}
	var v struct {
		// Success 为官方业务成功标志。
		Success bool `json:"success"`
		// Data 为库详情。
		Data Library `json:"data"`
	}
	err := c.object(ctx, "/api/v1/knowledge-bases/"+id, nil, []string{"success", "data"}, &v)
	if err != nil {
		return nil, err
	}
	if !v.Success || v.Data.ID != id {
		return nil, protocolError()
	}
	return &v.Data, nil
}

// Page 是固定官方 Wiki DTO 的可展示字段。
type Page struct {
	// ID 为稳定页面身份。
	ID string `json:"id"`
	// KnowledgeBaseID 为权威库身份。
	KnowledgeBaseID string `json:"knowledge_base_id"`
	// Slug 保留分层原值。
	Slug string `json:"slug"`
	// Title 为标题。
	Title string `json:"title"`
	// Summary 为页面当前说明，与历史快照使用相同字段比较。
	Summary string `json:"summary"`

	// PageType 为官方页面类型。
	PageType string `json:"page_type"`
	// Content 为读取时返回的 Markdown。
	Content string `json:"content"`
	// Version 为远端内容版本。
	Version int `json:"version"`
	// FolderID 为远端主题身份。
	FolderID string `json:"folder_id"`
	// Aliases 为查询别名。
	Aliases []string `json:"aliases"`
	// SourceRefs 为官方原件引用。
	SourceRefs []string `json:"source_refs"`
	// InLinks 为入链句柄。
	InLinks []string `json:"in_links"`
	// OutLinks 为出链句柄。
	OutLinks []string `json:"out_links"`
}

// PageOptions 使用官方一基页码，不能将 offset 直接误传成 page。
type PageOptions struct {
	// Page 为页码。
	Page int
	// PageSize 为有界分页数量。
	PageSize int
	// Query 为服务端关键词。
	Query string
	// PageType 为页面筛选。
	PageType string
	// FolderID 为可选精确主题；空指针表示不筛选。
	FolderID *string
}

// PageList 保留官方分页事实。
type PageList struct {
	// Pages 为本页列表。
	Pages []Page `json:"pages"`
	// Total 为总量。
	Total int64 `json:"total"`
	// Page 为服务器页码。
	Page int `json:"page"`
	// PageSize 为分页上限。
	PageSize int `json:"page_size"`
	// TotalPages 为服务端总页数。
	TotalPages int `json:"total_pages"`
}

func (c *WeKnora) ListPages(ctx context.Context, kb string, o PageOptions) (*PageList, error) {
	path, err := wikiPath(kb, "pages")
	if err != nil {
		return nil, err
	}
	page, size, err := positivePagination(o.Page, o.PageSize)
	if err != nil {
		return nil, err
	}
	q := url.Values{"page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(size)}, "query": {o.Query}, "page_type": {o.PageType}}
	if o.FolderID != nil {
		q.Set("folder_id", *o.FolderID)
	}
	var v PageList
	if err = c.object(ctx, path, q, []string{"pages", "total", "page", "page_size", "total_pages"}, &v); err != nil {
		return nil, err
	}
	if v.Page < 1 || v.PageSize < 1 || v.Total < 0 || v.TotalPages < 0 {
		return nil, protocolError()
	}
	for _, p := range v.Pages {
		if p.ID == "" || p.Slug == "" || p.KnowledgeBaseID != kb || p.Version < 1 {
			return nil, protocolError()
		}
	}
	return &v, nil
}
func (c *WeKnora) GetPage(ctx context.Context, kb, slug string) (*Page, error) {
	if !safeSlug(slug) {
		return nil, &Error{Code: "invalid_resource"}
	}
	path, err := wikiPath(kb, "pages/"+slug)
	if err != nil {
		return nil, err
	}
	var v Page
	if err = c.object(ctx, path, nil, []string{"id", "knowledge_base_id", "slug", "content", "version"}, &v); err != nil {
		return nil, err
	}
	if v.ID == "" || v.KnowledgeBaseID != kb || v.Slug != slug || v.Version < 1 {
		return nil, protocolError()
	}
	return &v, nil
}

// Revision 保留官方历史身份与内容，当前页不在历史列表内。
type Revision struct {
	// ID 为修订身份。
	ID string `json:"id"`
	// PageID 为所属页面。
	PageID string `json:"page_id"`
	// Version 为历史内容版本。
	Version int `json:"version"`
	// Title 为当时标题。
	Title string `json:"title"`
	// Summary 为当时页面说明，参与历史差异比较。
	Summary string `json:"summary"`

	// Content 在 content_version 查询时才返回完整内容。
	Content string `json:"content"`
	// ContentPresent 区分合法空正文和未返回正文。
	ContentPresent bool `json:"-"`
	// EditSource 为远端编辑来源。
	EditSource string `json:"edit_source"`
	// EditedAt 为远端时间。
	EditedAt string `json:"edited_at"`
}

// RevisionOptions 选择一段历史或明确读取单版正文。
type RevisionOptions struct {
	// Offset 为起始位置。
	Offset int
	// Limit 为上限。
	Limit int
	// ContentVersion 指定列表中携带正文的版本。
	ContentVersion int
}

// RevisionList 保留当前版本与历史总数。
type RevisionList struct {
	// Revisions 为历史列表。
	Revisions []Revision `json:"revisions"`
	// Total 为历史总数。
	Total int64 `json:"total"`
	// CurrentVersion 为当前正文版本。
	CurrentVersion int `json:"current_version"`
}

func (c *WeKnora) ListRevisions(ctx context.Context, kb, slug string, o RevisionOptions) (*RevisionList, error) {
	if !safeSlug(slug) || o.Offset < 0 || o.ContentVersion < 0 {
		return nil, &Error{Code: "invalid_resource"}
	}
	if o.Limit == 0 {
		o.Limit = 50
	}
	if o.Limit < 1 || o.Limit > 100 {
		return nil, &Error{Code: "invalid_pagination"}
	}
	path, err := wikiPath(kb, "revisions/"+slug)
	if err != nil {
		return nil, err
	}
	q := url.Values{"offset": {strconv.Itoa(o.Offset)}, "limit": {strconv.Itoa(o.Limit)}}
	if o.ContentVersion > 0 {
		q.Set("content_version", strconv.Itoa(o.ContentVersion))
	}
	var v RevisionList
	if err = c.object(ctx, path, q, []string{"revisions", "total", "current_version"}, &v); err != nil {
		return nil, err
	}
	if v.CurrentVersion < 1 || v.Total < 0 {
		return nil, protocolError()
	}
	for _, r := range v.Revisions {
		if r.ID == "" || r.PageID == "" || r.Version < 1 {
			return nil, protocolError()
		}
	}
	if o.ContentVersion > 0 {
		if len(v.Revisions) != 1 || v.Revisions[0].Version != o.ContentVersion || !v.Revisions[0].ContentPresent {
			return nil, protocolError()
		}
	}
	return &v, nil
}

// Folder 为官方树节点，不将整棵树当成单次响应。
type Folder struct {
	// ID 为目录身份。
	ID string `json:"id"`
	// KnowledgeBaseID 为所属库。
	KnowledgeBaseID string `json:"knowledge_base_id"`
	// ParentID 为父目录。
	ParentID string `json:"parent_id"`
	// Name 为目录名称。
	Name string `json:"name"`
	// Path 为显示路径。
	Path string `json:"path"`
	// SortOrder 为排序。
	SortOrder int `json:"sort_order"`
	// PageCount 为直接页面数。
	PageCount int64 `json:"page_count"`
	// HasChildren 指示是否继续读取。
	HasChildren bool `json:"has_children"`
}

// FolderList 返回一个父目录下的节点。
type FolderList struct {
	// ParentID 为本次父目录。
	ParentID string `json:"parent_id"`
	// Folders 为直接子目录。
	Folders []Folder `json:"folders"`
}

func (c *WeKnora) ListFolders(ctx context.Context, kb, parent string) (*FolderList, error) {
	path, err := wikiPath(kb, "folders")
	if err != nil {
		return nil, err
	}
	var v FolderList
	if err = c.object(ctx, path, url.Values{"parent_id": {parent}}, []string{"parent_id", "folders"}, &v); err != nil {
		return nil, err
	}
	if v.ParentID != parent {
		return nil, protocolError()
	}
	for _, f := range v.Folders {
		if f.ID == "" || f.KnowledgeBaseID != kb {
			return nil, protocolError()
		}
	}
	return &v, nil
}

// Download 保留原件二进制；下载权限独立于 Wiki 读取权限。
type Download struct {
	// Bytes 为未经转换的原始字节。
	Bytes []byte
	// Filename 来自合法 Content-Disposition，不用作本机文件路径。
	Filename string
	// MIME 为远端内容类型。
	MIME string
}

func (c *WeKnora) DownloadSource(ctx context.Context, id string) (*Download, error) {
	if !safeID(id) {
		return nil, &Error{Code: "invalid_resource"}
	}
	b, h, err := c.request(ctx, "/api/v1/knowledge/"+id+"/download", nil, 32<<20)
	if err != nil {
		return nil, err
	}
	_, params, _ := mime.ParseMediaType(h.Get("Content-Disposition"))
	return &Download{Bytes: b, Filename: params["filename"], MIME: h.Get("Content-Type")}, nil
}

func (c *WeKnora) Search(ctx context.Context, kb, query string, limit int) ([]Page, error) {
	if strings.TrimSpace(query) == "" || limit < 1 || limit > 100 {
		return nil, &Error{Code: "invalid_query"}
	}
	path, err := wikiPath(kb, "search")
	if err != nil {
		return nil, err
	}
	var v struct {
		// Pages 为实际命中的官方页面。
		Pages []Page `json:"pages"`
	}
	if err = c.object(ctx, path, url.Values{"q": {query}, "limit": {strconv.Itoa(limit)}}, []string{"pages"}, &v); err != nil {
		return nil, err
	}
	for _, p := range v.Pages {
		if p.ID == "" || p.KnowledgeBaseID != kb || p.Slug == "" {
			return nil, protocolError()
		}
	}
	return v.Pages, nil
}

// GraphOptions 指定官方有界关系图，不伪造全图覆盖。
type GraphOptions struct {
	// Mode 为 overview 或 ego。
	Mode string
	// Center 为邻域中心页面句柄。
	Center string
	// Depth 为邻域层数。
	Depth int
	// Limit 为节点数量上限。
	Limit int
}

// GraphNode 为官方关系节点。
type GraphNode struct {
	// Slug 为原始页面句柄。
	Slug string `json:"slug"`
	// Title 为标题。
	Title string `json:"title"`
	// PageType 为类型。
	PageType string `json:"page_type"`
	// LinkCount 为总关联数量。
	LinkCount int `json:"link_count"`
}

// GraphEdge 为定向知识关系。
type GraphEdge struct {
	// Source 为起点句柄。
	Source string `json:"source"`
	// Target 为终点句柄。
	Target string `json:"target"`
}

// GraphMeta 保留部分返回事实。
type GraphMeta struct {
	// Mode 为服务端实际模式。
	Mode string `json:"mode"`
	// Total 为总节点数。
	Total int `json:"total"`
	// Returned 为本次节点数。
	Returned int `json:"returned"`
	// Truncated 表示未覆盖全图。
	Truncated bool `json:"truncated"`
}

// Graph 为服务器返回的关系图。
type Graph struct {
	// Nodes 为实际节点。
	Nodes []GraphNode `json:"nodes"`
	// Edges 为实际关系。
	Edges []GraphEdge `json:"edges"`
	// Meta 为覆盖说明。
	Meta GraphMeta `json:"meta"`
}

func (c *WeKnora) Graph(ctx context.Context, kb string, o GraphOptions) (*Graph, error) {
	path, err := wikiPath(kb, "graph")
	if err != nil {
		return nil, err
	}
	if o.Mode == "" {
		o.Mode = "overview"
	}
	if o.Limit == 0 {
		o.Limit = 100
	}
	if o.Depth == 0 {
		o.Depth = 1
	}
	if (o.Mode != "overview" && o.Mode != "ego") || o.Limit < 1 || o.Limit > 200 || o.Depth < 1 || o.Depth > 3 || (o.Mode == "ego" && !safeSlug(o.Center)) {
		return nil, &Error{Code: "invalid_query"}
	}
	q := url.Values{"mode": {o.Mode}, "limit": {strconv.Itoa(o.Limit)}, "depth": {strconv.Itoa(o.Depth)}, "center": {o.Center}}
	var v Graph
	if err = c.object(ctx, path, q, []string{"nodes", "edges", "meta"}, &v); err != nil {
		return nil, err
	}
	if v.Meta.Mode == "" || v.Meta.Total < 0 || v.Meta.Returned != len(v.Nodes) {
		return nil, protocolError()
	}
	for _, n := range v.Nodes {
		if n.Slug == "" {
			return nil, protocolError()
		}
	}
	return &v, nil
}

// Source 保留官方原件身份和原始处理状态，不映射成定制Wiki已发布。
type Source struct {
	// ID 为官方知识原件身份。
	ID string `json:"id"`
	// KnowledgeBaseID 为所属库。
	KnowledgeBaseID string `json:"knowledge_base_id"`
	// Filename 为原始文件名。
	Filename string `json:"file_name"`
	// FileType 为官方文件类型。
	FileType string `json:"file_type"`
	// ParseStatus 为官方原始解析状态。
	ParseStatus string `json:"parse_status"`
	// Title 为显示标题。
	Title string `json:"title"`
}

// SourceList 对应官方success/data和分页同层响应。
type SourceList struct {
	// Success 为业务成功标志。
	Success bool `json:"success"`
	// Data 为当前分页原件。
	Data []Source `json:"data"`
	// Total 为总数量。
	Total int64 `json:"total"`
	// Page 为当前页码。
	Page int `json:"page"`
	// PageSize 为分页大小。
	PageSize int `json:"page_size"`
}

func (c *WeKnora) ListSources(ctx context.Context, kb string, page, size int, options ...core.SourceQuery) (*SourceList, error) {
	if !safeID(kb) {
		return nil, &Error{Code: "invalid_resource"}
	}
	page, size, err := positivePagination(page, size)
	if err != nil {
		return nil, err
	}
	if len(options) > 1 {
		return nil, &Error{Code: "invalid_query"}
	}
	q := url.Values{"page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(size)}}
	if len(options) == 1 {
		in := options[0]
		q.Set("keyword", in.Query)
		switch in.State {
		case "", "all":
		case "pending", "processing", "failed", "cancelled":
			q.Set("parse_status", in.State)
		case "queued", "processed", "unchanged":
			return nil, &Error{Code: "unsupported_source_state"}
		default:
			return nil, &Error{Code: "invalid_query"}
		}
		q.Set("sort_by", "created_at")
		switch in.Sort {
		case "", "newest":
			q.Set("sort_order", "desc")
		case "oldest":
			q.Set("sort_order", "asc")
		default:
			return nil, &Error{Code: "invalid_query"}
		}
	}
	var v SourceList
	if err = c.object(ctx, "/api/v1/knowledge-bases/"+kb+"/knowledge", q, []string{"success", "data", "total", "page", "page_size"}, &v); err != nil {
		return nil, err
	}
	if !v.Success || v.Total < 0 || v.Page < 1 || v.PageSize < 1 {
		return nil, protocolError()
	}
	for _, s := range v.Data {
		if s.ID == "" || s.KnowledgeBaseID != kb {
			return nil, protocolError()
		}
	}
	return &v, nil
}

// GetSource 核验官方原件的真实所属库，拒绝把任意原件身份挂到当前库。
func (c *WeKnora) GetSource(ctx context.Context, kb, id string) (*Source, error) {
	if !safeID(kb) || !safeID(id) {
		return nil, &Error{Code: "invalid_resource"}
	}
	var response struct {
		Success bool   `json:"success"` // 官方业务成功标志。
		Data    Source `json:"data"`    // 原件身份与真实归属。
	}
	if err := c.object(ctx, "/api/v1/knowledge/"+id, nil, []string{"success", "data"}, &response); err != nil {
		return nil, err
	}
	if !response.Success || response.Data.ID != id {
		return nil, protocolError()
	}
	if response.Data.KnowledgeBaseID != kb {
		return nil, &Error{Code: "scope_violation", Status: 403}
	}
	return &response.Data, nil
}

// DownloadSourceInLibrary 在下载前后检查真实归属；官方端无原子范围下载合同，不能宣称版本快照。
func (c *WeKnora) DownloadSourceInLibrary(ctx context.Context, kb, id string) (*Download, error) {
	if _, err := c.GetSource(ctx, kb, id); err != nil {
		return nil, err
	}
	value, err := c.DownloadSource(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := c.GetSource(ctx, kb, id); err != nil {
		return nil, err
	}
	return value, nil
}

// UnmarshalJSON 保留官方字段是否实际出现，避免把缺失历史正文解释为清空内容。
func (r *Revision) UnmarshalJSON(b []byte) error {
	type plain Revision
	var value plain
	if err := json.Unmarshal(b, &value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	raw, ok := fields["content"]
	value.ContentPresent = ok && string(raw) != "null"
	*r = Revision(value)
	return nil
}
