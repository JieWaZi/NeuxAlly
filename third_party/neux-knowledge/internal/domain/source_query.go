package types

// SourceQuery 指定未归档来源的展示状态筛选与全局分页。
type SourceQuery struct {
	Offset int    // 匹配结果的起始偏移，负值按零处理。
	Limit  int    // 单页数量，零使用默认值，最大一百。
	Query  string // 文件名不区分大小写的字面量包含条件。
	State  string // 展示状态：all、pending、queued、processing、processed、unchanged、failed 或 cancelled。
	Sort   string // 创建时间顺序：newest 或 oldest，同时间以身份稳定排序。
}
