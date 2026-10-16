package workspace

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/util"
)

// AttachmentInput 是用户选择的一份本地文件。
type AttachmentInput struct {
	// SourcePath 是桌面文件选择器返回的本地绝对或相对路径。
	SourcePath string
}

// AttachmentReference 是可以安全写入 Chat JSONL 的文件引用。
type AttachmentReference struct {
	// Size 是固化交付文件的真实字节数。
	Size int64 `json:"size,omitempty"`
	// Name 是向用户和 Agent 展示的原始文件名。
	Name string `json:"name"`
	// Path 是相对 Chat 工作目录的稳定路径。
	Path string `json:"path"`
	// MediaType 是根据扩展名推断的 MIME 类型。
	MediaType string `json:"mediaType,omitempty"`
	// Managed 表示文件是否复制到 NeuxAlly 归属目录。
	Managed bool `json:"managed"`
}

// PreparedAttachments 是附件校验、复制与回滚所需的结果。
type PreparedAttachments struct {
	// References 是保持用户选择顺序的 JSONL 安全引用。
	References []AttachmentReference
	// RuntimePaths 是 Runtime 可以读取的已解析绝对路径。
	RuntimePaths []string
	// Rollback 删除本次新建且带合法归属标记的附件目录。
	Rollback func() error
}

// resolvedAttachment 保存路径穿越校验后的单个附件事实。
type resolvedAttachment struct {
	// sourcePath 是 Runtime 可以读取的已解析绝对路径。
	sourcePath string
	// relativePath 是工作目录内附件已有的相对路径。
	relativePath string
	// name 是向用户展示的原始文件名。
	name string
	// mediaType 是根据扩展名推断的 MIME 类型。
	mediaType string
	// managed 表示附件需要复制到 NeuxAlly 归属目录。
	managed bool
}

// PrepareChatAttachments 把工作区外附件复制到 Chat 根 attachments/。
func PrepareChatAttachments(
	workingDirectory string,
	chatID string,
	messageID string,
	inputs []AttachmentInput,
) (PreparedAttachments, error) {
	if len(inputs) == 0 {
		return PreparedAttachments{Rollback: func() error { return nil }}, nil
	}
	workspace, err := filepath.EvalSymlinks(workingDirectory)
	if err != nil {
		return PreparedAttachments{}, status_error.ManagedWorkspaceError
	}
	resolvedAttachments := make([]resolvedAttachment, 0, len(inputs))
	externalCount := 0
	for _, input := range inputs {
		absolute, err := filepath.Abs(strings.TrimSpace(input.SourcePath))
		if err != nil {
			return PreparedAttachments{}, status_error.MessageAttachmentInvalid
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return PreparedAttachments{}, status_error.MessageAttachmentInvalid
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			return PreparedAttachments{}, status_error.MessageAttachmentInvalid
		}
		file, err := os.Open(resolved)
		if err != nil {
			return PreparedAttachments{}, status_error.MessageAttachmentInvalid
		}
		if err := file.Close(); err != nil {
			return PreparedAttachments{}, status_error.MessageAttachmentInvalid
		}
		relative, err := filepath.Rel(workspace, resolved)
		inside := err == nil && relative != ".." &&
			!strings.HasPrefix(relative, ".."+string(filepath.Separator)) &&
			!filepath.IsAbs(relative)
		if inside {
			resolvedAttachments = append(resolvedAttachments, resolvedAttachment{
				sourcePath:   resolved,
				relativePath: relative,
				name:         filepath.Base(resolved),
				mediaType:    mime.TypeByExtension(filepath.Ext(resolved)),
			})
			continue
		}
		externalCount++
		resolvedAttachments = append(resolvedAttachments, resolvedAttachment{
			sourcePath: resolved,
			name:       filepath.Base(resolved),
			mediaType:  mime.TypeByExtension(filepath.Ext(resolved)),
			managed:    true,
		})
	}

	target := filepath.Join(workspace, constants.ExecutionWorkspaceAttachmentsDirectory, messageID)
	if externalCount > 0 {
		attachmentsRoot, rootErr := ensureManagedRoot(
			workspace,
			constants.ExecutionWorkspaceAttachmentsDirectory,
		)
		if rootErr != nil {
			return PreparedAttachments{}, rootErr
		}
		target = filepath.Join(attachmentsRoot, messageID)
	}
	references, runtimePaths, err := materializeAttachmentReferences(
		resolvedAttachments,
		target,
		chatID,
		messageID,
	)
	if err != nil {
		return PreparedAttachments{}, err
	}
	return PreparedAttachments{
		References:   references,
		RuntimePaths: runtimePaths,
		Rollback: func() error {
			if externalCount == 0 {
				return nil
			}
			if err := validateOwner(
				target,
				constants.ChatMessageAttachmentsOwnerKind,
				chatID,
				messageID,
			); err != nil {
				return err
			}
			return removeValidatedDirectory(target)
		},
	}, nil
}

func materializeAttachmentReferences(
	attachments []resolvedAttachment,
	target string,
	chatID string,
	messageID string,
) ([]AttachmentReference, []string, error) {
	references := make([]AttachmentReference, 0, len(attachments))
	runtimePaths := make([]string, 0, len(attachments))
	externalCount := 0
	for _, attachment := range attachments {
		if attachment.managed {
			externalCount++
		}
	}
	if externalCount == 0 {
		for _, attachment := range attachments {
			references = append(references, AttachmentReference{
				Name:      attachment.name,
				Path:      filepath.ToSlash(attachment.relativePath),
				MediaType: attachment.mediaType,
			})
			runtimePaths = append(runtimePaths, attachment.sourcePath)
		}
		return references, runtimePaths, nil
	}
	root := filepath.Dir(target)
	if exists, err := directoryExists(target); err != nil {
		return nil, nil, err
	} else if exists {
		return nil, nil, status_error.MessageAttachmentStorageError
	}
	staging, err := os.MkdirTemp(root, "attachment-staging-"+messageID+"-")
	if err != nil {
		return nil, nil, status_error.MessageAttachmentStorageError
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(staging)
		}
	}()
	if err := writeJSON(
		filepath.Join(staging, constants.ChatContextOwnerFileName),
		ownerMarker{
			Kind:      constants.ChatMessageAttachmentsOwnerKind,
			ChatID:    chatID,
			MessageID: messageID,
		},
		status_error.MessageAttachmentStorageError,
	); err != nil {
		return nil, nil, err
	}
	usedNames := make(map[string]struct{}, externalCount)
	for index, attachment := range attachments {
		if !attachment.managed {
			references = append(references, AttachmentReference{
				Name:      attachment.name,
				Path:      filepath.ToSlash(attachment.relativePath),
				MediaType: attachment.mediaType,
			})
			runtimePaths = append(runtimePaths, attachment.sourcePath)
			continue
		}
		name := uniqueAttachmentName(attachment.name, index, usedNames)
		destination := filepath.Join(staging, name)
		if err := copyFile(attachment.sourcePath, destination); err != nil {
			return nil, nil, err
		}
		relative := filepath.Join(
			constants.ExecutionWorkspaceAttachmentsDirectory,
			messageID,
			name,
		)
		references = append(references, AttachmentReference{
			Name:      attachment.name,
			Path:      filepath.ToSlash(relative),
			MediaType: attachment.mediaType,
			Managed:   true,
		})
		runtimePaths = append(runtimePaths, filepath.Join(target, name))
	}
	if err := os.Rename(staging, target); err != nil {
		return nil, nil, status_error.MessageAttachmentStorageError
	}
	cleanup = false
	return references, runtimePaths, nil
}

func uniqueAttachmentName(
	baseName string,
	index int,
	used map[string]struct{},
) string {
	name := baseName
	if _, exists := used[name]; exists {
		name = fmt.Sprintf("%d-%s", index+1, baseName)
	}
	used[name] = struct{}{}
	return name
}

func copyFile(sourcePath string, destinationPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return status_error.MessageAttachmentInvalid
	}
	defer source.Close()
	destination, err := os.OpenFile(
		destinationPath,
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		return status_error.MessageAttachmentStorageError
	}
	if _, err := io.Copy(destination, source); err != nil {
		_ = destination.Close()
		return status_error.MessageAttachmentStorageError
	}
	if err := destination.Close(); err != nil {
		return status_error.MessageAttachmentStorageError
	}
	return nil
}

// ReadDeliveredAttachment 恢复同一业务身份已固化的附件，缺失时返回 found=false。
func ReadDeliveredAttachment(workingDirectory, chatID, identity, name string) (AttachmentReference, bool, error) {
	if err := ValidateExecutionWorkspaceDeletion(workingDirectory, chatID); err != nil {
		return AttachmentReference{}, false, err
	}
	if identity == "" || filepath.Base(identity) != identity || name == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return AttachmentReference{}, false, status_error.MessageAttachmentInvalid
	}
	root, err := os.OpenRoot(workingDirectory)
	if err != nil {
		return AttachmentReference{}, false, status_error.MessageAttachmentStorageError
	}
	defer root.Close()
	directory := filepath.Join(constants.ExecutionWorkspaceAttachmentsDirectory, identity)
	info, err := root.Lstat(directory)
	if os.IsNotExist(err) {
		return AttachmentReference{}, false, nil
	}
	if err != nil || !info.IsDir() {
		return AttachmentReference{}, false, status_error.MessageAttachmentStorageError
	}
	markerFile, err := root.Open(filepath.Join(directory, constants.ChatContextOwnerFileName))
	if err != nil {
		return AttachmentReference{}, false, status_error.MessageAttachmentStorageError
	}
	var marker ownerMarker
	err = json.NewDecoder(markerFile).Decode(&marker)
	markerFile.Close()
	if err != nil || marker.Kind != constants.ChatMessageAttachmentsOwnerKind || marker.ChatID != chatID || marker.MessageID != identity {
		return AttachmentReference{}, false, status_error.ChatDirectoryOwnershipInvalid
	}
	path := filepath.Join(directory, name)
	info, err = root.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return AttachmentReference{}, false, status_error.MessageAttachmentStorageError
	}
	file, err := root.Open(path)
	if err != nil {
		return AttachmentReference{}, false, status_error.MessageAttachmentStorageError
	}
	file.Close()
	return AttachmentReference{Size: info.Size(), Name: name, Path: filepath.ToSlash(path), MediaType: mime.TypeByExtension(filepath.Ext(name)), Managed: true}, true, nil
}

// SaveDeliveredAttachment 接收已授权打开的真实字节，原子固化到 Chat 受管目录；稳定身份禁止覆盖。
func SaveDeliveredAttachment(workingDirectory, chatID, identity, name string, input io.Reader) (AttachmentReference, error) {
	if ref, found, err := ReadDeliveredAttachment(workingDirectory, chatID, identity, name); err != nil || found {
		return ref, err
	}
	if _, err := ensureManagedRoot(workingDirectory, constants.ExecutionWorkspaceAttachmentsDirectory); err != nil {
		return AttachmentReference{}, err
	}
	root, err := os.OpenRoot(workingDirectory)
	if err != nil {
		return AttachmentReference{}, status_error.MessageAttachmentStorageError
	}
	defer root.Close()
	staging := filepath.Join(constants.ExecutionWorkspaceAttachmentsDirectory, "delivery-staging-"+util.New())
	if err := root.Mkdir(staging, 0700); err != nil {
		return AttachmentReference{}, status_error.MessageAttachmentStorageError
	}
	defer root.RemoveAll(staging)
	marker, err := root.OpenFile(filepath.Join(staging, constants.ChatContextOwnerFileName), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return AttachmentReference{}, status_error.MessageAttachmentStorageError
	}
	encodeErr := json.NewEncoder(marker).Encode(ownerMarker{Kind: constants.ChatMessageAttachmentsOwnerKind, ChatID: chatID, MessageID: identity})
	syncErr, closeErr := marker.Sync(), marker.Close()
	if encodeErr != nil || syncErr != nil || closeErr != nil {
		return AttachmentReference{}, status_error.MessageAttachmentStorageError
	}
	file, err := root.OpenFile(filepath.Join(staging, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return AttachmentReference{}, status_error.MessageAttachmentStorageError
	}
	count, copyErr := io.Copy(file, io.LimitReader(input, constants.WorkflowFileMaximumBytes+1))
	syncErr, closeErr = file.Sync(), file.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		return AttachmentReference{}, status_error.MessageAttachmentStorageError
	}
	if count > constants.WorkflowFileMaximumBytes {
		return AttachmentReference{}, status_error.MessageAttachmentInvalid
	}
	target := filepath.Join(constants.ExecutionWorkspaceAttachmentsDirectory, identity)
	if err := root.Rename(staging, target); err != nil {
		if ref, found, readErr := ReadDeliveredAttachment(workingDirectory, chatID, identity, name); found && readErr == nil {
			return ref, nil
		}
		return AttachmentReference{}, status_error.MessageAttachmentStorageError
	}
	ref, _, err := ReadDeliveredAttachment(workingDirectory, chatID, identity, name)
	return ref, err
}
