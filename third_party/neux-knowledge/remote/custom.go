package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/JieWaZi/neux-knowledge/core"
)

const customPath = "/api/knowledge/v1"

// ResourceRef 固定完整远端资源目标，不能由当前界面选中库隐式覆盖。
type ResourceRef struct {
	Backend         string `json:"backend"`         // 自有服务固定 custom，官方服务为 weknora。
	ConnectionID    string `json:"connectionId"`    // 宿主持久连接身份。
	KnowledgeBaseID string `json:"knowledgeBaseId"` // 权威库身份；仅连接发现允许为空。
}

// Custom 仅保存 HTTP 连接和资源引用，没有本地模型、阅读器或索引。
type Custom struct {
	transport *WeKnora    // 复用有界且禁止重定向的 HTTP 传输配置。
	ref       ResourceRef // 已冻结的完整资源目标。
}

// OutcomeUnknownError 表示写请求可能已提交，调用方须查询原回执，不能更换请求身份重试。
type OutcomeUnknownError struct {
	RequestID string // 原始请求身份，用于查询真实回执。
}

func (e *OutcomeUnknownError) Error() string { return "knowledge remote: outcome_unknown" }
func (e *OutcomeUnknownError) Unwrap() error { return &Error{Code: "outcome_unknown"} }
func NewCustom(o Options, ref ResourceRef) (*Custom, error) {
	if ref.Backend != "custom" || !safeID(ref.ConnectionID) || (ref.KnowledgeBaseID != "" && !safeID(ref.KnowledgeBaseID)) || (o.AuthMode != "" && o.AuthMode != "bearer") {
		return nil, &Error{Code: "invalid_connection"}
	}
	transport, err := NewWeKnora(o)
	if err != nil {
		return nil, err
	}
	return &Custom{transport: transport, ref: ref}, nil
}
func (c *Custom) Resource() ResourceRef { return c.ref }

// CreateLibrary 不自动扩展凭据库范围；宿主需按真实回执显式绑定新资源。
func (c *Custom) CreateLibrary(ctx context.Context, in core.CreateLibraryInput) (*core.Receipt, error) {
	if in.RequestID == "" {
		return nil, &Error{Code: "invalid_request"}
	}
	b, _, err := c.request(ctx, "POST", customPath+"/libraries", nil, in, in.RequestID)
	if err != nil {
		return nil, err
	}
	r, err := customDecode[core.Receipt](b, "requestId", "libraryId", "resourceId", "operation", "state", "version")
	if err != nil || r.RequestID != in.RequestID || r.LibraryID == "" || r.LibraryID != r.ResourceID || r.State != "saved" {
		return nil, &OutcomeUnknownError{RequestID: in.RequestID}
	}
	return r, nil
}

// GetCreationReceipt 只查询服务器已核对当前创建权限与原始主体的连接级创建回执。
func (c *Custom) GetCreationReceipt(ctx context.Context, requestID string) (*core.Receipt, error) {
	if requestID == "" {
		return nil, &Error{Code: "invalid_resource"}
	}
	b, _, err := c.request(ctx, "GET", customPath+"/libraries/creation-receipts", url.Values{"requestId": {requestID}}, nil, "")
	if err != nil {
		return nil, err
	}
	v, err := customDecode[core.Receipt](b, "requestId", "libraryId", "resourceId", "operation", "state", "version")
	if err != nil || v.RequestID != requestID || v.Operation != "library.create" || !safeID(v.LibraryID) || v.ResourceID != v.LibraryID || v.State != "saved" || v.Version < 1 {
		return nil, protocolError()
	}
	return v, nil
}

func (c *Custom) path(suffix string) (string, error) {
	if !safeID(c.ref.KnowledgeBaseID) {
		return "", &Error{Code: "invalid_resource"}
	}
	return customPath + "/libraries/" + c.ref.KnowledgeBaseID + suffix, nil
}
func (c *Custom) target(library string) error {
	if !safeID(c.ref.KnowledgeBaseID) || library != c.ref.KnowledgeBaseID {
		return &Error{Code: "scope_violation"}
	}
	return nil
}
func (c *Custom) request(ctx context.Context, method, path string, q url.Values, in any, requestID string) ([]byte, http.Header, error) {
	var reader io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, nil, &Error{Code: "invalid_request"}
		}
		reader = bytes.NewReader(b)
	}
	u := *c.transport.base
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawPath = ""
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return nil, nil, &Error{Code: "invalid_request"}
	}
	req.Header.Set("Authorization", "Bearer "+c.transport.token)
	req.Header.Set("Accept", "application/json, application/octet-stream")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// 禁止自动重试：服务端可能已提交但响应丢失；只允许调用方查询原 requestId。
	resp, err := c.transport.http.Do(req)
	unknown := func() ([]byte, http.Header, error) { return nil, nil, &OutcomeUnknownError{RequestID: requestID} }
	if err != nil {
		if method != "GET" {
			return unknown()
		}
		return nil, nil, &Error{Code: "unreachable"}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, (48<<20)+1))
	if err != nil || len(data) > 48<<20 {
		if method != "GET" {
			return unknown()
		}
		return nil, nil, &Error{Code: "response_incomplete", Status: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if method != "GET" && resp.StatusCode >= 500 && resp.StatusCode != 501 {
			return unknown()
		}
		code := "server_error"
		switch resp.StatusCode {
		case 400:
			code = "invalid"
		case 401:
			code = "unauthenticated"
		case 403:
			code = "forbidden"
		case 404:
			code = "not_found"
		case 409:
			code = "version_conflict"
		case 501:
			code = "unsupported"
		}
		var v struct {
			Code string `json:"code"` // 服务端稳定机器错误码。
		}
		if json.Unmarshal(data, &v) == nil {
			switch v.Code {
			case "invalid", "unauthenticated", "forbidden", "not_found", "scope_violation", "version_conflict", "idempotency_conflict", "cancelled", "stale_attempt", "policy_changed", "write_ip_forbidden", "unsupported", "unsupported_profile", "manual_protected", "candidate_invalid", "execution_not_successful":
				code = v.Code
			}
		}
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			code = "redirect_rejected"
		}
		return nil, nil, &Error{Code: code, Status: resp.StatusCode}
	}
	return data, resp.Header, nil
}
func customDecode[T any](b []byte, required ...string) (*T, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil || fields == nil {
		return nil, protocolError()
	}
	for _, key := range required {
		if v, ok := fields[key]; !ok || string(v) == "null" {
			return nil, protocolError()
		}
	}
	var v T
	if json.Unmarshal(b, &v) != nil {
		return nil, protocolError()
	}
	return &v, nil
}
func customGet[T any](ctx context.Context, c *Custom, suffix string, q url.Values, required ...string) (*T, error) {
	path, err := c.path(suffix)
	if err != nil {
		return nil, err
	}
	b, _, err := c.request(ctx, "GET", path, q, nil, "")
	if err != nil {
		return nil, err
	}
	return customDecode[T](b, required...)
}

// CustomList 保留自有服务器实际分页和总量。
type CustomList[T any] struct {
	Items  []T   `json:"items"`  // 本页真实记录。
	Total  int64 `json:"total"`  // 服务端授权范围内总量。
	Offset int   `json:"offset"` // 起始偏移。
	Limit  int   `json:"limit"`  // 有界上限。
}

func customList[T any](ctx context.Context, c *Custom, suffix string, q url.Values, offset, limit int) (*CustomList[T], error) {
	if offset < 0 || limit < 0 || limit > 100 {
		return nil, &Error{Code: "invalid_pagination"}
	}
	if limit == 0 {
		limit = 50
	}
	if q == nil {
		q = url.Values{}
	}
	q.Set("offset", strconv.Itoa(offset))
	q.Set("limit", strconv.Itoa(limit))
	v, err := customGet[CustomList[T]](ctx, c, suffix, q, "items", "total", "offset", "limit")
	if err == nil && (v.Total < 0 || v.Offset != offset || v.Limit != limit || len(v.Items) > limit) {
		return nil, protocolError()
	}
	return v, err
}
func (c *Custom) write(ctx context.Context, suffix, requestID string, in any) (*core.Receipt, error) {
	if requestID == "" {
		return nil, &Error{Code: "invalid_request"}
	}
	path, err := c.path(suffix)
	if err != nil {
		return nil, err
	}
	b, _, err := c.request(ctx, "POST", path, nil, in, requestID)
	if err != nil {
		return nil, err
	}
	v, err := customDecode[core.Receipt](b, "requestId", "libraryId", "resourceId", "operation", "state", "version")
	if err != nil || v.RequestID != requestID || v.LibraryID != c.ref.KnowledgeBaseID || !validReceiptResource(v) || (v.ResourceID == "" && suffix != "/organize") || v.Operation == "" || v.State == "" {
		return nil, &OutcomeUnknownError{RequestID: requestID}
	}
	return v, nil
}
func (c *Custom) ListLibraries(ctx context.Context, offset, limit int) (*CustomList[*core.Library], error) {
	if offset < 0 || limit < 0 || limit > 100 {
		return nil, &Error{Code: "invalid_pagination"}
	}
	if limit == 0 {
		limit = 50
	}
	b, _, err := c.request(ctx, "GET", customPath+"/libraries", url.Values{"offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)}}, nil, "")
	if err != nil {
		return nil, err
	}
	v, err := customDecode[CustomList[*core.Library]](b, "items", "total", "offset", "limit")
	if err == nil && (v.Total < 0 || v.Offset != offset || v.Limit != limit || len(v.Items) > limit) {
		return nil, protocolError()
	}
	if err == nil {
		for _, x := range v.Items {
			if x == nil || x.ID == "" {
				return nil, protocolError()
			}
		}
	}
	return v, err
}
func (c *Custom) GetLibrary(ctx context.Context) (*core.Library, error) {
	v, err := customGet[core.Library](ctx, c, "", nil, "id", "kind", "version", "policyVersion", "structureVersion")
	if err == nil && v.ID != c.ref.KnowledgeBaseID {
		return nil, protocolError()
	}
	return v, err
}
func (c *Custom) UpdateLibrary(ctx context.Context, in core.UpdateLibraryInput) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	return c.write(ctx, "", in.RequestID, in)
}
func (c *Custom) ListPages(ctx context.Context, q core.PageQuery) (*CustomList[*core.Page], error) {
	if err := c.target(q.LibraryID); err != nil {
		return nil, err
	}
	values := url.Values{"query": {q.Query}, "pageType": {q.PageType}, "status": {q.Status}}
	if q.ThemeID != nil {
		values.Set("themeId", *q.ThemeID)
	}
	v, err := customList[*core.Page](ctx, c, "/pages", values, q.Offset, q.Limit)
	if err == nil {
		for _, p := range v.Items {
			if p == nil || p.KnowledgeBaseID != c.ref.KnowledgeBaseID || p.ID == "" || p.Version < 1 {
				return nil, protocolError()
			}
		}
	}
	return v, err
}
func (c *Custom) GetPage(ctx context.Context, id string) (*core.Page, error) {
	if !safeID(id) {
		return nil, &Error{Code: "invalid_resource"}
	}
	v, err := customGet[core.Page](ctx, c, "/pages/"+id, nil, "id", "knowledge_base_id", "version", "content")
	if err == nil && (v.ID != id || v.KnowledgeBaseID != c.ref.KnowledgeBaseID || v.Version < 1) {
		return nil, protocolError()
	}
	return v, err
}
func (c *Custom) GetPageBySlug(ctx context.Context, slug string) (*core.Page, error) {
	if !safeSlug(slug) {
		return nil, &Error{Code: "invalid_resource"}
	}
	v, err := customGet[core.Page](ctx, c, "/page-by-slug", url.Values{"slug": {slug}}, "id", "knowledge_base_id", "version", "content")
	if err == nil && (v.Slug != slug || v.KnowledgeBaseID != c.ref.KnowledgeBaseID || v.ID == "" || v.Version < 1) {
		return nil, protocolError()
	}
	return v, err
}
func (c *Custom) WritePage(ctx context.Context, in core.WritePageInput) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	return c.write(ctx, "/pages", in.RequestID, in)
}
func (c *Custom) RestorePage(ctx context.Context, in core.RestorePageInput) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	if !safeID(in.PageID) {
		return nil, &Error{Code: "invalid_resource"}
	}
	return c.write(ctx, "/pages/"+in.PageID+"/restore", in.RequestID, in)
}
func (c *Custom) ListRevisions(ctx context.Context, pageID string, offset, limit int) (*CustomList[*core.Revision], error) {
	if !safeID(pageID) {
		return nil, &Error{Code: "invalid_resource"}
	}
	return customList[*core.Revision](ctx, c, "/pages/"+pageID+"/revisions", nil, offset, limit)
}
func (c *Custom) GetRevision(ctx context.Context, pageID string, version int) (*core.Revision, error) {
	if !safeID(pageID) || version < 1 {
		return nil, &Error{Code: "invalid_resource"}
	}
	v, err := customGet[core.Revision](ctx, c, "/pages/"+pageID+"/revisions/"+strconv.Itoa(version), nil, "id", "page_id", "version", "content")
	if err == nil && (v.PageID != pageID || v.Version != version) {
		return nil, protocolError()
	}
	return v, err
}
func (c *Custom) ListSources(ctx context.Context, offset, limit int, options ...core.SourceQuery) (*CustomList[*core.SourceVersion], error) {
	if len(options) > 1 {
		return nil, &Error{Code: "invalid_query"}
	}
	q := url.Values{}
	if len(options) == 1 {
		q.Set("query", options[0].Query)
		q.Set("state", options[0].State)
		q.Set("sort", options[0].Sort)
	}
	v, err := customList[*core.SourceVersion](ctx, c, "/sources", q, offset, limit)
	if err == nil {
		for _, s := range v.Items {
			if s == nil || s.LibraryID != c.ref.KnowledgeBaseID || s.ID == "" {
				return nil, protocolError()
			}
		}
	}
	return v, err
}
func (c *Custom) GetSource(ctx context.Context, id string) (*core.SourceVersion, error) {
	if !safeID(id) {
		return nil, &Error{Code: "invalid_resource"}
	}
	v, err := customGet[core.SourceVersion](ctx, c, "/sources/"+id, nil, "id", "libraryId", "sha256", "rowVersion")
	if err == nil && (v.ID != id || v.LibraryID != c.ref.KnowledgeBaseID) {
		return nil, protocolError()
	}
	return v, err
}
func (c *Custom) ImportSource(ctx context.Context, in core.ImportSourceInput) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	return c.write(ctx, "/sources", in.RequestID, in)
}
func (c *Custom) ArchiveSource(ctx context.Context, in core.ArchiveSourceInput) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	if !safeID(in.SourceVersionID) {
		return nil, &Error{Code: "invalid_resource"}
	}
	return c.write(ctx, "/sources/"+in.SourceVersionID+"/archive", in.RequestID, in)
}
func (c *Custom) DownloadSource(ctx context.Context, id string) (*Download, error) {
	if !safeID(id) {
		return nil, &Error{Code: "invalid_resource"}
	}
	path, err := c.path("/sources/" + id + "/raw")
	if err != nil {
		return nil, err
	}
	b, h, err := c.request(ctx, "GET", path, nil, nil, "")
	if err != nil {
		return nil, err
	}
	_, params, err := mime.ParseMediaType(h.Get("Content-Disposition"))
	if err != nil || params["filename"] == "" {
		return nil, protocolError()
	}
	return &Download{Bytes: b, Filename: params["filename"], MIME: h.Get("Content-Type")}, nil
}

// ListSourceChunks 只读取来源同一版本的实际解析产物，不在客户端重新解析或运行模型。
func (c *Custom) ListSourceChunks(ctx context.Context, sourceID string) ([]core.IndexedDocumentChunk, error) {
	if !safeID(sourceID) {
		return nil, &Error{Code: "invalid_resource"}
	}
	path, err := c.path("/sources/" + sourceID + "/chunks")
	if err != nil {
		return nil, err
	}
	b, _, err := c.request(ctx, "GET", path, nil, nil, "")
	if err != nil {
		return nil, err
	}
	var rows []core.IndexedDocumentChunk
	if json.Unmarshal(b, &rows) != nil || rows == nil {
		return nil, protocolError()
	}
	for _, row := range rows {
		if row.ID == "" || row.LibraryID != c.ref.KnowledgeBaseID || row.SourceVersionID != sourceID {
			return nil, protocolError()
		}
	}
	return rows, nil
}

func (c *Custom) ListThemes(ctx context.Context) ([]*core.Theme, error) {
	path, err := c.path("/themes")
	if err != nil {
		return nil, err
	}
	b, _, err := c.request(ctx, "GET", path, nil, nil, "")
	if err != nil {
		return nil, err
	}
	var v []*core.Theme
	if json.Unmarshal(b, &v) != nil || v == nil {
		return nil, protocolError()
	}
	for _, t := range v {
		if t == nil || t.KnowledgeBaseID != c.ref.KnowledgeBaseID || t.ID == "" {
			return nil, protocolError()
		}
	}
	return v, nil
}
func (c *Custom) WriteTheme(ctx context.Context, in core.WriteThemeInput) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	return c.write(ctx, "/themes", in.RequestID, in)
}
func (c *Custom) ListJobs(ctx context.Context, offset, limit int) (*CustomList[*core.Job], error) {
	v, err := customList[*core.Job](ctx, c, "/jobs", nil, offset, limit)
	if err == nil {
		for _, j := range v.Items {
			if j == nil || j.ID == "" || j.LibraryID != c.ref.KnowledgeBaseID {
				return nil, protocolError()
			}
		}
	}
	return v, err
}
func (c *Custom) GetJob(ctx context.Context, id string) (*core.Job, error) {
	if !safeID(id) {
		return nil, &Error{Code: "invalid_resource"}
	}
	v, err := customGet[core.Job](ctx, c, "/jobs/"+id, nil, "id", "libraryId", "state", "version", "attempt")
	if err == nil && (v.ID != id || v.LibraryID != c.ref.KnowledgeBaseID || v.State == "") {
		return nil, protocolError()
	}
	return v, err
}
func (c *Custom) GetReceipt(ctx context.Context, requestID string) (*core.Receipt, error) {
	if requestID == "" {
		return nil, &Error{Code: "invalid_resource"}
	}
	v, err := customGet[core.Receipt](ctx, c, "/receipts", url.Values{"requestId": {requestID}}, "requestId", "libraryId", "resourceId", "operation", "state")
	if err == nil && (v.RequestID != requestID || v.LibraryID != c.ref.KnowledgeBaseID || !validReceiptResource(v)) {
		return nil, protocolError()
	}
	return v, err
}

// OrganizeRequest 使用服务端预配身份，不含客户端模型配置。
type OrganizeRequest struct {
	RequestID        string   `json:"requestId"`             // 幂等请求身份。
	LibraryID        string   `json:"libraryId"`             // 明确库目标。
	ExpectedVersion  int64    `json:"expectedVersion"`       // 库版本。
	StructureVersion int64    `json:"structureVersion"`      // 目录代次。
	SourceVersionIDs []string `json:"sourceVersionIds"`      // 冻结原件集合。
	ProfileID        string   `json:"profileId"`             // 服务端配置身份。
	Rebuild          bool     `json:"rebuild,omitempty"`     // 显式重建所选原件，不能以普通整理代替。
	Instruction      string   `json:"instruction,omitempty"` // 用户本次整理要求，不包含本机运行配置。
}

// MaintenanceRequest 冻结确切目标页及版本，防止自然语言扩展写入范围。
type MaintenanceRequest struct {
	RequestID           string `json:"requestId"`           // 幂等请求身份。
	LibraryID           string `json:"libraryId"`           // 明确库目标。
	ExpectedVersion     int64  `json:"expectedVersion"`     // 库版本。
	StructureVersion    int64  `json:"structureVersion"`    // 目录代次。
	ProfileID           string `json:"profileId"`           // 服务端配置身份。
	Instruction         string `json:"instruction"`         // 用户维护要求。
	PageID              string `json:"pageId"`              // 精确目标页。
	ExpectedPageVersion int    `json:"expectedPageVersion"` // 已读取的目标页版本。
}

func (c *Custom) Organize(ctx context.Context, in OrganizeRequest) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	return c.write(ctx, "/organize", in.RequestID, in)
}
func (c *Custom) Maintain(ctx context.Context, in MaintenanceRequest) (*core.Receipt, error) {
	if err := c.target(in.LibraryID); err != nil {
		return nil, err
	}
	return c.write(ctx, "/maintenance", in.RequestID, in)
}

// ServerProfile 是服务器公布的非秘密配置摘要。
type ServerProfile struct {
	ID   string `json:"id"`   // 服务器固定配置身份。
	Name string `json:"name"` // 用户可见名称。
}

// CustomCapabilities 保留服务器装配与当前授权事实；每次请求仍重新授权。
type CustomCapabilities struct {
	Profile    string                            `json:"profile"`    // 固定协议身份。
	Operations map[string]bool                   `json:"operations"` // 当前资源实际可用动作。
	Profiles   []ServerProfile                   `json:"profiles"`   // 可选择的服务器配置，不包含秘密。
	Details    map[string]CustomCapabilityStatus `json:"details"`    // 分别保留支持、权限和资源条件。
}

// CustomCapabilityStatus 不把连接成功或协议支持推断为可写。
type CustomCapabilityStatus struct {
	Supported  bool   `json:"supported"`  // 服务器实际装配能力。
	Permission string `json:"permission"` // 当前主体 allowed 或 denied。
	Condition  string `json:"condition"`  // 当前策略和执行配置状态。
}

func (c *Custom) Capabilities(ctx context.Context) (*CustomCapabilities, error) {
	v, err := customGet[CustomCapabilities](ctx, c, "/capabilities", nil, "profile", "operations", "details")
	if err == nil && v.Profile != "neux-knowledge-v1" {
		return nil, protocolError()
	}
	return v, err
}

// validReceiptResource 只允许核心明确无待处理来源的接纳回执不创建任务身份。
func validReceiptResource(v *core.Receipt) bool {
	if v.ResourceID != "" {
		return true
	}
	return v.Operation == "job.accept" && v.State == "no-op" && v.JobID == "" && len(v.PageIDs) == 0
}
