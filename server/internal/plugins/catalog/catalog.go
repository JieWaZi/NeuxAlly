// Package catalog 读取随应用交付的固定本地目录，不访问网络或执行插件。
package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins"
)

var keyPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// Entry 描述一个本地目录项及其不可变来源。
type Entry struct {
	// Key 是目录项稳定身份。
	Key string `json:"key"`
	// Directory 是分发目录内的包路径，不回传绝对位置。
	Directory string `json:"-"`
	// SourceCommit 是独立插件源固定提交。
	SourceCommit string `json:"sourceCommit"`
	// ArchiveSHA256 是完整压缩包摘要。
	ArchiveSHA256 string `json:"-"`
	// Package 是本地包解析得到的能力与诊断。
	Package plugins.Package `json:"package"`
}

// document 是本地构建格式，不作为前端业务模型。
type document struct {
	// SchemaVersion 是目录格式版本。
	SchemaVersion int `json:"schemaVersion"`
	// Entries 是全部有限目录项。
	Entries []struct {
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
	result := []Entry{}
	data, err := os.ReadFile(filepath.Join(root, "catalog.json"))
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, status_error.PluginStorageError
	}
	if len(data) > 4*1024*1024 {
		return nil, status_error.PluginContentLimitExceeded
	}
	var index document
	if json.Unmarshal(data, &index) != nil || index.SchemaVersion != 1 {
		return nil, status_error.PluginContentInvalid
	}
	seen := map[string]bool{}
	for _, item := range index.Entries {
		if !keyPattern.MatchString(item.Key) || item.Directory != "catalog/"+item.Key+".zip" || seen[item.Key] || len(item.SourceCommit) != 40 || len(item.ArchiveSHA256) != 64 {
			return nil, status_error.PluginContentInvalid
		}
		seen[item.Key] = true
		if item.Package.Manifest.Name == "" {
			return nil, status_error.PluginContentInvalid
		}
		result = append(result, Entry{Key: item.Key, Directory: item.Directory, SourceCommit: item.SourceCommit, ArchiveSHA256: item.ArchiveSHA256, Package: item.Package})
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
		location := filepath.Join(root, entry.Directory)
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
	return "", status_error.PluginNotFound
}
