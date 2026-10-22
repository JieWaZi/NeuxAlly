package remote

import (
	"context"
	"net/url"
	"strconv"

	"github.com/JieWaZi/neux-knowledge/core"
)

// customActivityList 保留远端活动流的实际成功状态与末条游标。
type customActivityList struct {
	Success    bool             `json:"success"`     // 服务器明确确认查询成功。
	Data       []*core.AuditLog `json:"data"`        // 当前授权库的持久活动。
	NextCursor uint64           `json:"next_cursor"` // 本页末条活动身份；空页为零。
}

// ListLibraryActivity 沿服务端配置编辑授权读取，不把普通 read 推断为可查看活动。
func (c *Custom) ListLibraryActivity(ctx context.Context, query core.ActivityQuery) ([]*core.AuditLog, uint64, error) {
	values := url.Values{"after_id": {strconv.FormatUint(query.AfterID, 10)}, "limit": {strconv.Itoa(query.Limit)}, "action": {string(query.Action)}, "outcome": {string(query.Outcome)}, "actor": {query.ActorUserID}}
	v, err := customGet[customActivityList](ctx, c, "/activity", values, "success", "data", "next_cursor")
	if err != nil {
		return nil, 0, err
	}
	if !v.Success || v.Data == nil || (len(v.Data) == 0 && v.NextCursor != 0) {
		return nil, 0, protocolError()
	}
	for _, entry := range v.Data {
		if entry == nil || entry.ID == 0 || entry.ScopeID != c.ref.KnowledgeBaseID {
			return nil, 0, protocolError()
		}
	}
	if len(v.Data) > 0 && v.NextCursor != v.Data[len(v.Data)-1].ID {
		return nil, 0, protocolError()
	}
	return v.Data, v.NextCursor, nil
}
