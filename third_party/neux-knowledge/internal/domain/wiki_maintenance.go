package types

// ReplacePageTextInput 精确替换当前正文中的全部匹配文本，保留其他人工保护。
type ReplacePageTextInput struct {
	RequestID             string        // 稳定幂等请求身份。
	LibraryID             string        // 已授权目标库。
	PageID                string        // 稳定页面身份。
	ExpectedVersion       int           // 当前正文版本。
	StructureVersion      int64         // 当前目录代次。
	ExpectedPolicyVersion int64         // 冻结权限代次，零表示不额外冻结。
	OldText               string        // 必须实际命中的精确原文本。
	NewText               string        // 替换内容，空值表示明确删除。
	Evidence              *PageEvidence // 可选完整证据替换，省略时保留原证据。
}

// RenamePageInput 更新页面可读标识，保持页面身份并原子级联链接。
type RenamePageInput struct {
	RequestID             string // 稳定幂等请求身份。
	LibraryID             string // 已授权目标库。
	PageID                string // 稳定页面身份。
	ExpectedVersion       int    // 当前正文版本。
	StructureVersion      int64  // 当前目录代次。
	ExpectedPolicyVersion int64  // 冻结权限代次，零表示不额外冻结。
	NewSlug               string // 新的库内唯一可读标识。
}

// DeletePageInput 归档页面并原子移除当前入链，保留历史与证据。
type DeletePageInput struct {
	RequestID             string // 稳定幂等请求身份。
	LibraryID             string // 已授权目标库。
	PageID                string // 稳定页面身份。
	ExpectedVersion       int    // 当前正文版本。
	StructureVersion      int64  // 当前目录代次。
	ExpectedPolicyVersion int64  // 冻结权限代次，零表示不额外冻结。
}
