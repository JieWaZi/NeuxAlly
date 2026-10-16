// Package skills 提供技能导入、扫描和应用数据目录归档能力。
package skills

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

var plainCloneContext = git.PlainCloneContext

const (
	maxZipSourceBytes       int64 = 100 * 1024 * 1024
	maxImportedBytes        int64 = 500 * 1024 * 1024
	maxImportedFileBytes    int64 = 50 * 1024 * 1024
	maxImportFileCount            = 5000
	maxImportPathBytes            = 512
	stagedBatchManifestName       = "batch.json"
	stagedBatchContentName        = "content"
)

// Source 描述一次 Git 或 ZIP 技能导入。
type Source struct {
	// Kind 是导入类型，取值为 git 或 zip。
	Kind constants.SkillSource
	// Location 是 Git 地址或本地 ZIP 文件路径。
	Location string
	// Ref 是 Git 分支、标签或提交引用。
	Ref string
	// Subpath 是 Git 仓库内的扫描子目录。
	Subpath string
}

// Package 是扫描到的一个技能包目录。
type Package struct {
	// Name 是 SKILL.md Frontmatter 声明的 Runtime 技能名。
	Name string `json:"name"`
	// Description 是 SKILL.md Frontmatter 声明的技能说明。
	Description string `json:"description"`
	// Directory 是 staging 中技能包的绝对路径。
	Directory string `json:"-"`
	// RelativePath 是批次内容根目录内的候选相对路径。
	RelativePath string `json:"relativePath"`
}

// StagedBatch 是完成来源获取和结构校验、等待用户选择的导入批次。
type StagedBatch struct {
	// Token 是后续安装时引用该服务端批次的不可猜测令牌。
	Token string
	// Root 是批次受管临时目录，仅供服务端清理。
	Root string
	// Source 是完成规范化且可以记录到技能包的来源。
	Source Source
	// Packages 是本批次扫描到的候选技能包。
	Packages []Package
}

// stagedBatchManifest 保存跨请求重新打开导入批次所需的服务端事实。
type stagedBatchManifest struct {
	// Source 是完成规范化的 Git 或 ZIP 来源。
	Source Source `json:"source"`
	// Packages 是只保留相对路径和元数据的候选列表。
	Packages []Package `json:"packages"`
	// ContentDigest 是扫描完成时整棵 staging 内容树的不可逆摘要。
	ContentDigest string `json:"contentDigest"`
}

// Import 将来源拉取或解压到受管临时目录，并扫描根、一级和二级目录中的 SKILL.md。
func Import(ctx context.Context, temporaryParent string, source Source) ([]Package, string, error) {
	batch, err := StageImport(ctx, temporaryParent, source)
	if err != nil {
		return nil, "", err
	}
	return batch.Packages, batch.Root, nil
}

// StageImport 获取并校验来源，持久化一个等待用户选择的服务端导入批次。
func StageImport(ctx context.Context, temporaryParent string, source Source) (StagedBatch, error) {
	temporaryParent = strings.TrimSpace(temporaryParent)
	if temporaryParent == "" {
		return StagedBatch{}, status_error.SkillStorageError
	}
	if err := os.MkdirAll(temporaryParent, 0o700); err != nil {
		return StagedBatch{}, status_error.SkillStorageError
	}
	temporaryRoot, err := os.MkdirTemp(temporaryParent, "import-")
	if err != nil {
		return StagedBatch{}, status_error.SkillStorageError
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(temporaryRoot)
		}
	}()
	contentRoot := filepath.Join(temporaryRoot, stagedBatchContentName)
	root := contentRoot
	switch source.Kind {
	case constants.SkillSourceGit:
		normalized, normalizeErr := normalizeGitSource(source)
		if normalizeErr != nil {
			return StagedBatch{}, normalizeErr
		}
		if err := cloneGitSource(ctx, normalized, contentRoot); err != nil {
			return StagedBatch{}, err
		}
		source = normalized
	case constants.SkillSourceZip:
		info, statErr := os.Stat(source.Location)
		if statErr != nil {
			return StagedBatch{}, status_error.SkillImportFailed
		}
		if info.Size() > maxZipSourceBytes {
			return StagedBatch{}, status_error.SkillContentLimitExceeded
		}
		if err := os.MkdirAll(contentRoot, 0o700); err != nil {
			return StagedBatch{}, status_error.SkillStorageError
		}
		if err := extractZip(source.Location, contentRoot); err != nil {
			return StagedBatch{}, err
		}
	default:
		return StagedBatch{}, status_error.SkillImportSourceInvalid
	}
	contentDigest, err := validateStagedTree(contentRoot)
	if err != nil {
		return StagedBatch{}, err
	}
	if strings.TrimSpace(source.Subpath) != "" {
		subpath := filepath.Clean(source.Subpath)
		if filepath.IsAbs(subpath) || subpath == ".." || strings.HasPrefix(subpath, ".."+string(filepath.Separator)) {
			return StagedBatch{}, status_error.SkillImportSourceInvalid
		}
		root = filepath.Join(contentRoot, subpath)
	}
	packages, err := scan(root)
	if err != nil {
		return StagedBatch{}, err
	}
	for index := range packages {
		relative, relativeErr := filepath.Rel(contentRoot, packages[index].Directory)
		if relativeErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return StagedBatch{}, status_error.SkillContentInvalid
		}
		packages[index].RelativePath = filepath.ToSlash(relative)
	}
	manifestPackages := append([]Package(nil), packages...)
	manifest := stagedBatchManifest{
		Source: source, Packages: manifestPackages, ContentDigest: contentDigest,
	}
	for index := range manifest.Packages {
		manifest.Packages[index].Directory = ""
	}
	content, err := json.Marshal(manifest)
	if err != nil {
		return StagedBatch{}, status_error.SkillStorageError
	}
	if err := os.WriteFile(filepath.Join(temporaryRoot, stagedBatchManifestName), append(content, '\n'), 0o600); err != nil {
		return StagedBatch{}, status_error.SkillStorageError
	}
	cleanup = false
	return StagedBatch{
		Token: filepath.Base(temporaryRoot), Root: temporaryRoot,
		Source: source, Packages: packages,
	}, nil
}

// OpenStagedImport 重新打开服务端保存的导入批次并恢复候选绝对路径。
func OpenStagedImport(temporaryParent string, token string) (StagedBatch, error) {
	temporaryParent = strings.TrimSpace(temporaryParent)
	token = strings.TrimSpace(token)
	if temporaryParent == "" || token == "" || filepath.Base(token) != token || !strings.HasPrefix(token, "import-") {
		return StagedBatch{}, status_error.SkillImportIntegrityInvalid
	}
	root := filepath.Join(temporaryParent, token)
	manifestPath := filepath.Join(root, stagedBatchManifestName)
	info, err := os.Lstat(manifestPath)
	if err != nil || !info.Mode().IsRegular() {
		return StagedBatch{}, status_error.SkillImportIntegrityInvalid
	}
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return StagedBatch{}, status_error.SkillStorageError
	}
	var manifest stagedBatchManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return StagedBatch{}, status_error.SkillImportIntegrityInvalid
	}
	contentRoot := filepath.Join(root, stagedBatchContentName)
	contentDigest, err := validateStagedTree(contentRoot)
	if err != nil || manifest.ContentDigest == "" || contentDigest != manifest.ContentDigest {
		return StagedBatch{}, status_error.SkillImportIntegrityInvalid
	}
	for index := range manifest.Packages {
		relative := filepath.Clean(filepath.FromSlash(manifest.Packages[index].RelativePath))
		if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return StagedBatch{}, status_error.SkillImportIntegrityInvalid
		}
		directory := filepath.Join(contentRoot, relative)
		metadata, metadataErr := ReadMetadata(directory)
		if metadataErr != nil || metadata.Name != manifest.Packages[index].Name || metadata.Description != manifest.Packages[index].Description {
			return StagedBatch{}, status_error.SkillImportIntegrityInvalid
		}
		manifest.Packages[index].Directory = directory
	}
	return StagedBatch{
		Token: token, Root: root, Source: manifest.Source,
		Packages: manifest.Packages,
	}, nil
}

func normalizeGitSource(source Source) (Source, error) {
	location := strings.TrimSpace(source.Location)
	parsed, err := url.Parse(location)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return Source{}, status_error.SkillImportSourceInvalid
	}
	if parsed.User != nil {
		return Source{}, status_error.SkillImportSourceInvalid
	}
	if parsed.RawQuery != "" {
		return Source{}, status_error.SkillImportSourceInvalid
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return Source{}, status_error.SkillImportSourceInvalid
	}

	normalized := source
	normalized.Location = location
	normalized.Ref = strings.TrimSpace(source.Ref)
	normalized.Subpath = strings.TrimSpace(source.Subpath)
	if !strings.EqualFold(parsed.Hostname(), "github.com") {
		return normalized, nil
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 2 {
		return Source{}, status_error.SkillImportSourceInvalid
	}
	if len(segments) >= 4 && segments[2] == "tree" {
		parsed.Path = "/" + segments[0] + "/" + strings.TrimSuffix(segments[1], ".git") + ".git"
		parsed.RawPath = ""
		parsed.RawQuery = ""
		parsed.Fragment = ""
		normalized.Location = parsed.String()
		if normalized.Ref == "" {
			normalized.Ref = segments[3]
		}
		urlSubpath := strings.Join(segments[4:], "/")
		if urlSubpath != "" {
			normalized.Subpath = filepath.ToSlash(filepath.Join(urlSubpath, normalized.Subpath))
		}
	}
	return normalized, nil
}

func cloneGitSource(ctx context.Context, source Source, destination string) error {
	ref := strings.TrimSpace(source.Ref)
	options := &git.CloneOptions{
		URL:          source.Location,
		Depth:        1,
		SingleBranch: true,
		Tags:         git.NoTags,
	}
	if parsed, err := url.Parse(source.Location); err == nil && parsed.Scheme == "" {
		options.Depth = 0
	}
	if ref != "" && !plumbing.IsHash(ref) {
		if strings.HasPrefix(ref, "refs/") {
			options.ReferenceName = plumbing.ReferenceName(ref)
		} else {
			options.ReferenceName = plumbing.NewBranchReferenceName(ref)
		}
	}
	repository, err := plainCloneContext(ctx, destination, false, options)
	if err != nil && ref != "" && !plumbing.IsHash(ref) && !strings.HasPrefix(ref, "refs/") {
		if cleanupErr := os.RemoveAll(destination); cleanupErr != nil {
			return status_error.SkillImportFailed
		}
		options.ReferenceName = plumbing.NewTagReferenceName(ref)
		repository, err = plainCloneContext(ctx, destination, false, options)
	}
	if err != nil {
		return status_error.SkillImportFailed
	}
	if !plumbing.IsHash(ref) {
		return nil
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return status_error.SkillImportFailed
	}
	if err := worktree.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(ref)}); err != nil {
		return status_error.SkillImportFailed
	}
	return nil
}

func extractZip(path string, destination string) error {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return status_error.SkillContentInvalid
	}
	defer archive.Close()
	fileCount := 0
	var importedBytes int64
	foldedPaths := make(map[string]string, len(archive.File))
	for _, file := range archive.File {
		name, err := validateImportRelativePath(file.Name)
		if err != nil {
			return err
		}
		folded := strings.ToLower(filepath.ToSlash(name))
		if _, exists := foldedPaths[folded]; exists {
			return status_error.SkillContentInvalid
		}
		foldedPaths[folded] = name
		target := filepath.Join(destination, name)
		if err := validateZipParentPath(destination, filepath.Dir(target), name); err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return status_error.SkillImportFailed
			}
			continue
		}
		isSymlink := file.Mode()&os.ModeSymlink != 0
		if !file.Mode().IsRegular() && !isSymlink {
			return status_error.SkillContentInvalid
		}
		fileCount++
		if fileCount > maxImportFileCount {
			return status_error.SkillContentLimitExceeded
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return status_error.SkillImportFailed
		}
		input, err := file.Open()
		if err != nil {
			return status_error.SkillImportFailed
		}
		if isSymlink {
			linkTarget, readErr := io.ReadAll(io.LimitReader(input, maxImportedFileBytes+1))
			closeErr := input.Close()
			if readErr != nil {
				return status_error.SkillImportFailed
			}
			if closeErr != nil {
				return status_error.SkillImportFailed
			}
			if int64(len(linkTarget)) > maxImportedFileBytes {
				return status_error.SkillContentLimitExceeded
			}
			if err := os.Symlink(string(linkTarget), target); err != nil {
				return status_error.SkillImportFailed
			}
			importedBytes += int64(len(linkTarget))
			if importedBytes > maxImportedBytes {
				return status_error.SkillContentLimitExceeded
			}
			continue
		}
		mode := os.FileMode(0o600)
		if file.Mode().Perm()&0o111 != 0 && pathContainsDirectory(name, "scripts") {
			mode = 0o700
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		var written int64
		if err == nil {
			written, err = io.Copy(output, io.LimitReader(input, maxImportedFileBytes+1))
			_ = output.Close()
		}
		_ = input.Close()
		if err != nil {
			return status_error.SkillImportFailed
		}
		if written > maxImportedFileBytes {
			return status_error.SkillContentLimitExceeded
		}
		importedBytes += written
		if importedBytes > maxImportedBytes {
			return status_error.SkillContentLimitExceeded
		}
	}
	return nil
}

func pathContainsDirectory(relative string, directory string) bool {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	for index := 0; index+1 < len(parts); index++ {
		if parts[index] == directory {
			return true
		}
	}
	return false
}

func validateImportRelativePath(value string) (string, error) {
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return "", status_error.SkillContentInvalid
	}
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if cleaned == "." || filepath.IsAbs(filepath.FromSlash(cleaned)) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", status_error.SkillContentInvalid
	}
	if len([]byte(cleaned)) > maxImportPathBytes {
		return "", status_error.SkillContentLimitExceeded
	}
	for _, component := range strings.Split(cleaned, "/") {
		if component == "" || strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
			return "", status_error.SkillContentInvalid
		}
		for _, character := range component {
			if character < 32 || strings.ContainsRune(`<>:"\|?*`, character) {
				return "", status_error.SkillContentInvalid
			}
		}
		base := strings.ToUpper(strings.SplitN(component, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" ||
			(len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
			return "", status_error.SkillContentInvalid
		}
	}
	return filepath.Clean(filepath.FromSlash(cleaned)), nil
}

func validateStagedTree(root string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", status_error.SkillContentInvalid
	}
	fileCount := 0
	var importedBytes int64
	foldedPaths := make(map[string]string)
	seenFiles := make([]os.FileInfo, 0)
	hash := sha256.New()
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return status_error.SkillImportFailed
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return status_error.SkillImportFailed
		}
		if relative == "." {
			return nil
		}
		if relative == ".git" && entry.IsDir() {
			return filepath.SkipDir
		}
		normalized, err := validateImportRelativePath(filepath.ToSlash(relative))
		if err != nil {
			return err
		}
		normalized = filepath.ToSlash(normalized)
		folded := strings.ToLower(normalized)
		if _, exists := foldedPaths[folded]; exists {
			return status_error.SkillContentInvalid
		}
		foldedPaths[folded] = normalized
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return status_error.SkillImportFailed
			}
			fileCount++
			if fileCount > maxImportFileCount {
				return status_error.SkillContentLimitExceeded
			}
			if int64(len(linkTarget)) > maxImportedFileBytes {
				return status_error.SkillContentLimitExceeded
			}
			if _, err := io.WriteString(hash, normalized+"\x00"); err != nil {
				return status_error.SkillImportFailed
			}
			if _, err := hash.Write([]byte{2}); err != nil {
				return status_error.SkillImportFailed
			}
			if _, err := io.WriteString(hash, linkTarget); err != nil {
				return status_error.SkillImportFailed
			}
			importedBytes += int64(len(linkTarget))
			if importedBytes > maxImportedBytes {
				return status_error.SkillContentLimitExceeded
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return status_error.SkillImportFailed
		}
		if !info.Mode().IsRegular() {
			return status_error.SkillContentInvalid
		}
		for _, existing := range seenFiles {
			if os.SameFile(existing, info) {
				return status_error.SkillContentInvalid
			}
		}
		seenFiles = append(seenFiles, info)
		fileCount++
		if fileCount > maxImportFileCount {
			return status_error.SkillContentLimitExceeded
		}
		file, err := os.Open(path)
		if err != nil {
			return status_error.SkillImportFailed
		}
		if _, err := io.WriteString(hash, normalized+"\x00"); err != nil {
			_ = file.Close()
			return status_error.SkillImportFailed
		}
		modeMarker := byte(0)
		if info.Mode().Perm()&0o111 != 0 {
			modeMarker = 1
		}
		if _, err := hash.Write([]byte{modeMarker}); err != nil {
			_ = file.Close()
			return status_error.SkillImportFailed
		}
		written, copyErr := io.Copy(hash, io.LimitReader(file, maxImportedFileBytes+1))
		closeErr := file.Close()
		if copyErr != nil {
			return status_error.SkillImportFailed
		}
		if closeErr != nil {
			return status_error.SkillImportFailed
		}
		if written > maxImportedFileBytes {
			return status_error.SkillContentLimitExceeded
		}
		importedBytes += written
		if importedBytes > maxImportedBytes {
			return status_error.SkillContentLimitExceeded
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func scan(root string) ([]Package, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, status_error.SkillContentInvalid
	}
	var result []Package
	var visit func(string, int) error
	visit = func(directory string, depth int) error {
		if depth > 2 {
			return nil
		}
		if info, err := os.Stat(filepath.Join(directory, "SKILL.md")); err == nil && !info.IsDir() {
			metadata, metadataErr := ReadMetadata(directory)
			if metadataErr != nil {
				return metadataErr
			}
			result = append(result, Package{
				Name:        metadata.Name,
				Description: metadata.Description,
				Directory:   directory,
			})
		}
		if depth == 2 {
			return nil
		}
		entries, err := os.ReadDir(directory)
		if err != nil {
			return status_error.SkillImportFailed
		}
		for _, entry := range entries {
			if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
				if err := visit(filepath.Join(directory, entry.Name()), depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(root, 0); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, status_error.SkillContentInvalid
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// Digest 校验完整技能目录，并返回包含脚本执行位的 SHA-256 摘要。
func Digest(directory string) (string, error) {
	return validateStagedTree(directory)
}

// CopyVersion 通过同文件系统 staging 原子保存一个不可变技能版本目录。
func CopyVersion(dataDirectory string, packageID string, version string, sourceDirectory string) (string, string, error) {
	dataDirectory = strings.TrimSpace(dataDirectory)
	packageID = strings.TrimSpace(packageID)
	version = strings.TrimSpace(version)
	if dataDirectory == "" || packageID == "" || filepath.Base(packageID) != packageID || version == "" || filepath.Base(version) != version {
		return "", "", status_error.SkillContentInvalid
	}
	relative := filepath.Join("skills", "versions", packageID, version)
	target := filepath.Join(dataDirectory, relative)
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", "", status_error.SkillStorageError
	}
	if _, err := os.Stat(target); err == nil {
		return "", "", status_error.Conflict
	} else if !os.IsNotExist(err) {
		return "", "", status_error.SkillStorageError
	}
	staging, err := os.MkdirTemp(parent, "version-staging-"+version+"-")
	if err != nil {
		return "", "", status_error.SkillStorageError
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(staging)
		}
	}()
	if err := CopyDirectory(sourceDirectory, staging); err != nil {
		return "", "", err
	}
	digest, err := Digest(staging)
	if err != nil {
		return "", "", err
	}
	if err := os.Rename(staging, target); err != nil {
		return "", "", status_error.SkillStorageError
	}
	cleanup = false
	return relative, digest, nil
}

// CopyDirectory 安全复制完整技能包目录并保留软链接、拒绝其他特殊文件。
func CopyDirectory(source, target string) error {
	seenFiles := make([]os.FileInfo, 0)
	foldedPaths := make(map[string]string)
	fileCount := 0
	var copiedBytes int64
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return status_error.SkillImportFailed
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return status_error.SkillImportFailed
		}
		if relative == "." {
			return nil
		}
		if relative == ".git" && entry.IsDir() {
			return filepath.SkipDir
		}
		normalized, pathErr := validateImportRelativePath(filepath.ToSlash(relative))
		if pathErr != nil {
			return pathErr
		}
		normalized = filepath.ToSlash(normalized)
		folded := strings.ToLower(normalized)
		if _, exists := foldedPaths[folded]; exists {
			return status_error.SkillContentInvalid
		}
		foldedPaths[folded] = normalized
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			if err := os.MkdirAll(destination, 0o700); err != nil {
				return status_error.SkillImportFailed
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return status_error.SkillImportFailed
			}
			fileCount++
			if fileCount > maxImportFileCount {
				return status_error.SkillContentLimitExceeded
			}
			if int64(len(linkTarget)) > maxImportedFileBytes {
				return status_error.SkillContentLimitExceeded
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
				return status_error.SkillImportFailed
			}
			if err := os.Symlink(linkTarget, destination); err != nil {
				return status_error.SkillImportFailed
			}
			copiedBytes += int64(len(linkTarget))
			if copiedBytes > maxImportedBytes {
				return status_error.SkillContentLimitExceeded
			}
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return status_error.SkillImportFailed
		}
		if !info.Mode().IsRegular() {
			return status_error.SkillContentInvalid
		}
		for _, existing := range seenFiles {
			if os.SameFile(existing, info) {
				return status_error.SkillContentInvalid
			}
		}
		seenFiles = append(seenFiles, info)
		fileCount++
		if fileCount > maxImportFileCount {
			return status_error.SkillContentLimitExceeded
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return status_error.SkillImportFailed
		}
		input, err := os.Open(path)
		if err != nil {
			return status_error.SkillImportFailed
		}
		mode := os.FileMode(0o600)
		if info.Mode().Perm()&0o111 != 0 && pathContainsDirectory(relative, "scripts") {
			mode = 0o700
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			_ = input.Close()
			return status_error.SkillImportFailed
		}
		written, copyErr := io.Copy(output, io.LimitReader(input, maxImportedFileBytes+1))
		inputCloseErr := input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return status_error.SkillImportFailed
		}
		if inputCloseErr != nil {
			return status_error.SkillImportFailed
		}
		if closeErr != nil {
			return status_error.SkillImportFailed
		}
		if written > maxImportedFileBytes {
			return status_error.SkillContentLimitExceeded
		}
		copiedBytes += written
		if copiedBytes > maxImportedBytes {
			return status_error.SkillContentLimitExceeded
		}
		return nil
	})
}

func validateZipParentPath(root string, parent string, entryName string) error {
	relative, err := filepath.Rel(root, parent)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return status_error.SkillContentInvalid
	}
	if relative == "." {
		return nil
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			return nil
		}
		if statErr != nil {
			return status_error.SkillImportFailed
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return status_error.SkillContentInvalid
		}
	}
	return nil
}
