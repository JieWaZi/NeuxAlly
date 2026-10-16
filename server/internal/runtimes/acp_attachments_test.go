package runtimes

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// TestACPPromptRespectsNativeImageCapability 验证文本适配器保留通用附件路径，而声明图片能力的适配器仍收到原生图片。
func TestACPPromptRespectsNativeImageCapability(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	image := []byte("fixture-image-bytes")
	if err := os.WriteFile(path, image, 0600); err != nil {
		t.Fatal(err)
	}
	prompt := "Read attachment: " + path
	attachments := []Attachment{{Name: "image.png", RuntimePath: path, MediaType: "image/png"}}
	blocks, err := acpPromptBlocks(prompt, attachments, false)
	if err != nil || len(blocks) != 1 || blocks[0].Text == nil || blocks[0].Text.Text != prompt {
		t.Fatalf("text attachment lost: %+v %v", blocks, err)
	}
	blocks, err = acpPromptBlocks(prompt, attachments, true)
	if err != nil || len(blocks) != 2 || blocks[1].Image == nil || blocks[1].Image.Data != base64.StdEncoding.EncodeToString(image) {
		t.Fatalf("native image lost: %+v %v", blocks, err)
	}
}
