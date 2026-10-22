package types

import (
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// SourcePurgeReview 是页面清理标记的安全投影，不包含自由元数据或原文。
type SourcePurgeReview struct {
	NeedsReview           bool     `json:"needsReview"`           // 保留的当前知识仍需人工核查。
	UnavailableSourceIDs  []string `json:"unavailableSourceIds"`  // 已不可用的稳定来源身份。
	UnavailableVersionIDs []string `json:"unavailableVersionIds"` // 已不可用的来源版本身份。
	PurgeIDs              []string `json:"purgeIds"`              // 对应持久清理清单身份。
}

func purgeReviewIDs(ids []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		// 这些身份由核心生成；不允许把自由文本伪装成清理元数据导出。
		decoded, err := hex.DecodeString(id)
		if err != nil || len(decoded) != 16 || strings.ToLower(id) != id || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

// ParseSourcePurgeReview 只投影持久 sourcePurge 的已知字段，不生成第二份清理事实。
func ParseSourcePurgeReview(metadata JSON) *SourcePurgeReview {
	var envelope struct {
		SourcePurge *SourcePurgeReview `json:"sourcePurge"` // 唯一认可的持久清理标记。
	}
	if json.Unmarshal(metadata, &envelope) != nil || envelope.SourcePurge == nil || !envelope.SourcePurge.NeedsReview {
		return nil
	}
	marker := envelope.SourcePurge
	marker.UnavailableSourceIDs = purgeReviewIDs(marker.UnavailableSourceIDs)
	marker.UnavailableVersionIDs = purgeReviewIDs(marker.UnavailableVersionIDs)
	marker.PurgeIDs = purgeReviewIDs(marker.PurgeIDs)
	return marker
}
