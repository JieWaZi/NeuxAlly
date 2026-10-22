package core

import (
	"context"
	"github.com/JieWaZi/neux-knowledge/internal/store"
)

// Options 指定独立数据库与受管原件根目录。
type Options struct {
	DBPath string // 独立 SQLite 文件路径。
	RawDir string // 原件对象受管根目录。
}

// Engine 拥有独立知识数据库生命周期；不启动后台模型。
type Engine struct {
	store *store.Store // 内部持久化与领域事务边界。
}

// Open 打开独立知识核心，仅建立空库当前结构或校验当前结构。
func Open(ctx context.Context, options Options) (*Engine, error) {
	s, err := store.Open(ctx, options.DBPath, options.RawDir)
	if err != nil {
		return nil, err
	}
	return &Engine{store: s}, nil
}

// Close 释放核心资源，重复关闭保持安全。
func (e *Engine) Close() error { return e.store.Close() }
