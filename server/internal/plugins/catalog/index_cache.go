package catalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/logx"
)

// SourceState 描述目录缓存与本次同步状态，不把同步失败伪装为空库。
type SourceState struct {
	// State 是available/cached/stale/local。
	State string `json:"state"`
	// FetchedAt 是最后成功同步的毫秒时间。
	FetchedAt int64 `json:"fetchedAt"`
	// ErrorCode 是同步失败的稳定错误。
	ErrorCode string `json:"errorCode,omitempty"`
}

// storedIndex 将完整目录与 HTTP 校验信息一起原子保存，避免元数据和正文串版本。
type storedIndex struct {
	// 上次核验成功的完整目录
	Content json.RawMessage `json:"content"`
	// 上游内容实体标识
	ETag string `json:"etag,omitempty"`
	// 上游内容修改时间
	LastModified string `json:"lastModified,omitempty"`
	// 上次核验成功时间
	FetchedAt int64 `json:"fetchedAt"`
}

// remoteIndex 在一次 Core 生命周期内共享一个来源的更新和读取快照。
type remoteIndex struct {
	// 快照与状态读写保护
	mu sync.RWMutex
	// 当前已核验的持久快照
	snapshot storedIndex
	// 对外同步状态
	state SourceState
	// 无缓存时的同步错误
	err error
	// 当前来源的固定目录地址
	source string
	// 持久缓存位置
	filename string
	// 启动同步的取消边界
	ctx context.Context
	// 每次启动只同步一次
	once sync.Once
	// 同步退出及通知完成信号
	done chan struct{}
	// 同步完成后唤醒既有资源查询
	changed func()
}

var remoteIndexes sync.Map

// Start 注册持久缓存并在后台同步一次；返回的关闭函数取消并等待本次同步退出。
func Start(ctx context.Context, source, directory string, changed func()) func() {
	if !httpAddress(source) {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	index := newRemoteIndex(ctx, source, cacheFilename(directory, source), changed)
	remoteIndexes.Store(source, index)
	go index.refresh()
	return func() { cancel(); <-index.done }
}

func cacheFilename(directory, source string) string {
	sum := sha256.Sum256([]byte(source))
	return filepath.Join(directory, hex.EncodeToString(sum[:])+".json")
}

func indexCache(source string) string {
	if value, ok := remoteIndexes.Load(source); ok {
		return value.(*remoteIndex).filename
	}
	return cacheFilename(filepath.Join(os.TempDir(), "neuxally-plugin-catalog"), source)
}

func newRemoteIndex(ctx context.Context, source, filename string, changed func()) *remoteIndex {
	index := &remoteIndex{ctx: ctx, source: source, filename: filename, done: make(chan struct{}), changed: changed, state: SourceState{State: "unavailable"}}
	if data, err := os.ReadFile(filename); err == nil && len(data) <= 5*1024*1024 {
		// 保留可读的旧目录缓存；新写入统一带校验信息的原子快照。
		if json.Unmarshal(data, &index.snapshot) != nil || len(index.snapshot.Content) == 0 {
			index.snapshot = storedIndex{Content: data}
			if info, err := os.Stat(filename); err == nil {
				index.snapshot.FetchedAt = info.ModTime().UnixMilli()
			}
		}
		if _, err := parseIndex(index.snapshot.Content); err == nil {
			index.state = SourceState{State: "cached", FetchedAt: index.snapshot.FetchedAt}
		} else {
			index.snapshot = storedIndex{}
		}
	}
	return index
}

func remoteSource(source string) *remoteIndex {
	if value, ok := remoteIndexes.Load(source); ok {
		return value.(*remoteIndex)
	}
	index := newRemoteIndex(context.Background(), source, indexCache(source), nil)
	actual, _ := remoteIndexes.LoadOrStore(source, index)
	return actual.(*remoteIndex)
}

// State 返回当前来源的同步状态，读取本身不会访问远端。
func State(source string) SourceState {
	if value, ok := remoteIndexes.Load(source); ok {
		index := value.(*remoteIndex)
		index.mu.RLock()
		defer index.mu.RUnlock()
		return index.state
	}
	return SourceState{State: "local"}
}

func readIndex(source string) ([]byte, error) {
	if !httpAddress(source) {
		return os.ReadFile(filepath.Join(source, "catalog.json"))
	}
	index := remoteSource(source)
	index.mu.RLock()
	previous := index.snapshot.Content
	index.mu.RUnlock()
	if len(previous) != 0 {
		return previous, nil
	}
	// 首次无缓存的并发查询复用启动同步，既不伪造空目录，也不重复请求。
	index.refresh()
	index.mu.RLock()
	defer index.mu.RUnlock()
	return index.snapshot.Content, index.err
}

func (index *remoteIndex) refresh() {
	index.once.Do(func() {
		defer close(index.done)
		index.fetch()
		if index.changed != nil && index.ctx.Err() == nil {
			index.changed()
		}
	})
}

func (index *remoteIndex) fail(err error) {
	index.mu.Lock()
	defer index.mu.Unlock()
	index.err = err
	index.state.ErrorCode = status_error.PluginNotReady.Key()
	index.state.State = "unavailable"
	if len(index.snapshot.Content) != 0 {
		index.state.State = "stale"
		index.err = nil
	}
}

func (index *remoteIndex) fetch() {
	index.mu.RLock()
	next := index.snapshot
	index.mu.RUnlock()
	request, err := http.NewRequestWithContext(index.ctx, http.MethodGet, index.source, nil)
	if err != nil {
		index.fail(status_error.PluginContentInvalid)
		return
	}
	if next.ETag != "" {
		request.Header.Set("If-None-Match", next.ETag)
	}
	if next.LastModified != "" {
		request.Header.Set("If-Modified-Since", next.LastModified)
	}
	client := http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		index.fail(status_error.PluginNotReady)
		return
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusNotModified:
		if len(next.Content) == 0 {
			index.fail(status_error.PluginContentInvalid)
			return
		}
	case http.StatusOK:
		data, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024+1))
		if err != nil {
			index.fail(status_error.PluginNotReady)
			return
		}
		if _, err := parseIndex(data); err != nil {
			index.fail(err)
			return
		}
		next.Content = data
		next.ETag = response.Header.Get("ETag")
		next.LastModified = response.Header.Get("Last-Modified")
	default:
		index.fail(status_error.PluginNotReady)
		return
	}
	if index.ctx.Err() != nil {
		index.fail(status_error.PluginNotReady)
		return
	}
	next.FetchedAt = time.Now().UnixMilli()
	if err := index.save(next); err != nil {
		logx.Warnf("persist plugin catalog failed: error=%v", err)
		index.fail(status_error.PluginStorageError)
		return
	}
	index.mu.Lock()
	defer index.mu.Unlock()
	index.snapshot = next
	index.err = nil
	index.state = SourceState{State: "available", FetchedAt: next.FetchedAt}
}

func (index *remoteIndex) save(next storedIndex) error {
	if err := os.MkdirAll(filepath.Dir(index.filename), 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(index.filename), "index-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	// 编码保持目录 JSON 的语义内容；HTTP 实体标识与同一份内容一并提交。
	var data bytes.Buffer
	if err := json.NewEncoder(&data).Encode(next); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data.Bytes()); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), index.filename)
}
