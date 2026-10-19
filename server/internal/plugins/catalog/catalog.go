// Package catalog 读取 HTTP 或本地目录元数据，按摘要下载不可变安装包。
package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins"
)

var keyPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// Entry 描述一个本地目录项及其不可变来源。
type Entry struct {
	// SourceKey 是来源限定的逻辑身份，不凭同名合并。
	SourceKey string `json:"sourceKey"`
	// Key 是目录项稳定身份。
	Key string `json:"key"`
	// Directory 是分发目录内的包路径，不回传绝对位置。
	Directory string `json:"-"`
	// SourceCommit 是独立插件源固定提交。
	SourceCommit string `json:"sourceCommit"`
	// ArchiveSHA256 是完整压缩包摘要。
	ArchiveSHA256 string `json:"archiveSHA256"`
	// ArchiveURL 是不可变 HTTP 下载地址。
	ArchiveURL string `json:"archiveURL,omitempty"`
	// PackagePath 是完整来源中的逻辑入口。
	PackagePath string `json:"packagePath,omitempty"`
	// Package 是本地包解析得到的能力与诊断。
	Package plugins.Package `json:"package"`
}

// document 是本地构建格式，不作为前端业务模型。
type document struct {
	// SchemaVersion 是目录格式版本。
	SchemaVersion int `json:"schemaVersion"`
	// Entries 是全部有限目录项。
	Entries []struct {
		// ArchiveURL 是不可变 HTTP 下载地址。
		ArchiveURL string `json:"archiveURL"`
		// PackagePath 是逻辑入口目录。
		PackagePath string `json:"packagePath"`
		// Name 是目录显示名称。
		Name string `json:"name"`
		// Version 是入口版本。
		Version string `json:"version"`
		// Description 是目录短描述。
		Description string `json:"description"`
		// Format 是包格式。
		Format string `json:"format"`
		// Key 是稳定目录键。
		Key string `json:"key"`
		// Directory 是受控相对位置。
		Directory string `json:"directory"`
		// SourceCommit 是固定源提交。
		SourceCommit string `json:"sourceCommit"`
		// ArchiveSHA256 是构建产物摘要。
		ArchiveSHA256 string `json:"archiveSHA256"`
		// Package 是由正式包解析器生成的构建元数据。
		Package plugins.Package `json:"package"`
	} `json:"entries"`
}

// List 从本地分发清单读取全部条目；未分发目录显示准确空状态。
func List(root string) ([]Entry, error) {
	data, err := readIndex(root)
	if err != nil {
		return nil, err
	}
	entries, err := parseIndex(data)
	if err != nil {
		return nil, err
	}
	for index := range entries {
		entries[index].SourceKey = "catalog:" + entries[index].Key
		if root != constants.PluginCatalogHTTPSource {
			sum := sha256.Sum256([]byte(root))
			entries[index].SourceKey = "catalog:" + hex.EncodeToString(sum[:8]) + ":" + entries[index].Key
		}
	}
	return entries, nil
}

func parseIndex(data []byte) ([]Entry, error) {
	result := []Entry{}
	if len(data) > 4*1024*1024 {
		return nil, status_error.PluginContentLimitExceeded
	}
	var index document
	if json.Unmarshal(data, &index) != nil || (index.SchemaVersion != 1 && index.SchemaVersion != 2) {
		return nil, status_error.PluginContentInvalid
	}
	seen := map[string]bool{}
	for _, item := range index.Entries {
		if !keyPattern.MatchString(item.Key) || (index.SchemaVersion == 1 && item.Directory != "catalog/"+item.Key+".zip") || seen[item.Key] || len(item.SourceCommit) != 40 || len(item.ArchiveSHA256) != 64 {
			return nil, status_error.PluginContentInvalid
		}
		if _, err := hex.DecodeString(item.SourceCommit); err != nil {
			return nil, status_error.PluginContentInvalid
		}
		if _, err := hex.DecodeString(item.ArchiveSHA256); err != nil {
			return nil, status_error.PluginContentInvalid
		}
		seen[item.Key] = true
		if index.SchemaVersion == 2 {
			if !httpAddress(item.ArchiveURL) || item.PackagePath == "" || filepath.IsAbs(item.PackagePath) || strings.Contains(item.PackagePath, "..") {
				return nil, status_error.PluginContentInvalid
			}
			item.Package = plugins.Package{Manifest: plugins.Manifest{Name: item.Name, Version: item.Version, Description: item.Description}, Format: item.Format}
		}
		if item.Package.Manifest.Name == "" {
			return nil, status_error.PluginContentInvalid
		}
		result = append(result, Entry{Key: item.Key, Directory: item.Directory, SourceCommit: item.SourceCommit, ArchiveSHA256: item.ArchiveSHA256, Package: item.Package, ArchiveURL: item.ArchiveURL, PackagePath: item.PackagePath})
	}
	return result, nil
}

// Resolve 在创建受管快照之前校验标准 ZIP 的完整来源摘要。
func Resolve(root, key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", status_error.PluginContentInvalid
	}
	entries, err := List(root)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.Key != key {
			continue
		}
		return ResolveEntry(root, entry)
	}
	return "", status_error.PluginNotFound
}

func httpAddress(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

// SourceState 描述目录缓存与本次同步状态，不把同步失败伪装为空库。
type SourceState struct {
	// State 是available/cached/stale/local。
	State string `json:"state"`
	// FetchedAt 是最后成功同步的毫秒时间。
	FetchedAt int64 `json:"fetchedAt"`
	// ErrorCode 是同步失败的稳定错误。
	ErrorCode string `json:"errorCode,omitempty"`
}

var sourceStates sync.Map

// State 返回本次目录读取的来源状态。
func State(root string) SourceState {
	if value, ok := sourceStates.Load(root); ok {
		return value.(SourceState)
	}
	return SourceState{State: "local"}
}

func indexCache(root string) string {
	sum := sha256.Sum256([]byte(root))
	return filepath.Join(os.TempDir(), "neuxally-plugin-catalog", hex.EncodeToString(sum[:])+".json")
}

func readIndex(root string) ([]byte, error) {
	if !httpAddress(root) {
		return os.ReadFile(filepath.Join(root, "catalog.json"))
	}
	cache := indexCache(root)
	previous, readErr := os.ReadFile(cache)
	info, statErr := os.Stat(cache)
	cached := readErr == nil && statErr == nil
	if cached {
		if _, err := parseIndex(previous); err != nil {
			cached = false
		}
	}
	state := SourceState{}
	if cached {
		state.FetchedAt = info.ModTime().UnixMilli()
		if time.Since(info.ModTime()) < 5*time.Minute {
			state.State = "cached"
			sourceStates.Store(root, state)
			return previous, nil
		}
	}
	failed := func() ([]byte, error) {
		state.State = "unavailable"
		state.ErrorCode = status_error.PluginNotReady.Key()
		if cached {
			state.State = "stale"
			sourceStates.Store(root, state)
			return previous, nil
		}
		sourceStates.Store(root, state)
		return nil, status_error.PluginNotReady
	}
	client := http.Client{Timeout: 15 * time.Second}
	response, err := client.Get(root)
	if err != nil {
		return failed()
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return failed()
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024+1))
	if err != nil {
		return failed()
	}
	if _, err := parseIndex(data); err != nil {
		if cached {
			return failed()
		}
		return nil, err
	}
	if os.MkdirAll(filepath.Dir(cache), 0700) != nil {
		return nil, status_error.PluginStorageError
	}
	temporary, err := os.CreateTemp(filepath.Dir(cache), "index-")
	if err != nil {
		return nil, status_error.PluginStorageError
	}
	defer os.Remove(temporary.Name())
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return nil, status_error.PluginStorageError
	}
	if temporary.Close() != nil || os.Rename(temporary.Name(), cache) != nil {
		return nil, status_error.PluginStorageError
	}
	state.State = "available"
	state.FetchedAt = time.Now().UnixMilli()
	sourceStates.Store(root, state)
	return data, nil
}

func download(entry Entry) (string, error) {
	parent := filepath.Join(os.TempDir(), "neuxally-plugin-downloads")
	if os.MkdirAll(parent, 0700) != nil {
		return "", status_error.PluginStorageError
	}
	location := filepath.Join(parent, entry.ArchiveSHA256+".zip")
	if data, err := os.ReadFile(location); err == nil {
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) == entry.ArchiveSHA256 {
			return location, nil
		}
	}
	client := http.Client{Timeout: 2 * time.Minute}
	response, err := client.Get(entry.ArchiveURL)
	if err != nil {
		return "", status_error.PluginNotReady
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", status_error.PluginNotReady
	}
	temporary, err := os.CreateTemp(parent, "download-*.zip")
	if err != nil {
		return "", status_error.PluginStorageError
	}
	defer os.Remove(temporary.Name())
	defer temporary.Close()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(response.Body, 100*1024*1024+1))
	if err != nil {
		return "", status_error.PluginNotReady
	}
	if size > 100*1024*1024 {
		return "", status_error.PluginContentLimitExceeded
	}
	if hex.EncodeToString(hash.Sum(nil)) != entry.ArchiveSHA256 {
		return "", status_error.PluginContentInvalid
	}
	if temporary.Close() != nil || os.Rename(temporary.Name(), location) != nil {
		return "", status_error.PluginStorageError
	}
	return location, nil
}

// Find 返回已验证目录中的精确入口，安装不得自行猜测包根。
func Find(root, key string) (Entry, error) {
	entries, err := List(root)
	if err != nil {
		return Entry{}, err
	}
	for _, entry := range entries {
		if entry.Key == key {
			return entry, nil
		}
	}
	return Entry{}, status_error.PluginNotFound
}

// ResolveEntry 下载已经选定的目录版本，不在下载前重新查询latest。
func ResolveEntry(root string, entry Entry) (string, error) {
	var err error
	location := filepath.Join(root, entry.Directory)
	if entry.ArchiveURL != "" {
		location, err = download(entry)
		if err != nil {
			return "", err
		}
	}
	bytes, err := os.ReadFile(location)
	if err != nil {
		return "", status_error.PluginStorageError
	}
	if len(bytes) > 100*1024*1024 {
		return "", status_error.PluginContentLimitExceeded
	}
	sum := sha256.Sum256(bytes)
	if hex.EncodeToString(sum[:]) != entry.ArchiveSHA256 {
		return "", status_error.PluginContentInvalid
	}
	return location, nil
}
