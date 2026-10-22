package store

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	types "github.com/JieWaZi/neux-knowledge/internal/domain"
)

// LockProcessor 非阻塞占有精确阶段尝试；锁由实际调用栈持有，进程退出时由系统释放。
// 文件始终保留，避免删除后两个进程分别锁住不同 inode；锁名不使用可遍历的阶段原文。
func (s *Store) LockProcessor(ctx context.Context, stageID string, attempt int) (release func(), acquired bool, err error) {
	if err = ctx.Err(); err != nil {
		return nil, false, err
	}
	if stageID == "" || attempt < 1 {
		return nil, false, fail(types.ErrInvalid, "processor.lock", "stage and attempt are required")
	}
	s.processorMu.Lock()
	if s.processorClosing {
		s.processorMu.Unlock()
		return nil, false, fail(types.ErrStorage, "processor.lock", "store is closing")
	}
	s.processors.Add(1)
	s.processorMu.Unlock()
	defer func() {
		if !acquired {
			s.processors.Done()
		}
	}()
	if err = os.MkdirAll(s.processorDir, 0700); err != nil {
		return nil, false, wrap(types.ErrStorage, "processor.lock", err)
	}
	name := hash([]byte(stageID+":"+strconv.Itoa(attempt))) + ".lock"
	f, err := os.OpenFile(filepath.Join(s.processorDir, name), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, false, wrap(types.ErrStorage, "processor.lock", err)
	}
	acquired, err = tryProcessorLock(f)
	if err != nil || !acquired {
		_ = f.Close()
		if err != nil {
			return nil, false, wrap(types.ErrStorage, "processor.lock", err)
		}
		return nil, false, nil
	}
	var once sync.Once
	return func() { once.Do(func() { _ = f.Close(); s.processors.Done() }) }, true, nil
}
