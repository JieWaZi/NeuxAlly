package catalog

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogLoadsRealBundledFeishu(t *testing.T) {
	source := filepath.Join("..", "..", "..", "..", "..", "desktop", "resources", "plugins")
	if _, err := os.Stat(filepath.Join(source, "catalog.json")); os.IsNotExist(err) {
		t.Skip("运行 pnpm plugins:prepare 后验证真实目录")
	}
	entries, err := List(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Key != "feishu-office" || len(entries[0].Package.Skills) < 5 {
		t.Fatalf("real catalog incomplete: %+v", entries)
	}
	if _, err := Resolve(source, "../outside"); err == nil {
		t.Fatal("accepted traversal key")
	}
	if _, err := Resolve(source, "feishu-office"); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogRejectsChangedZIP(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "catalog")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	archive := []byte("fixed archive bytes")
	sum := sha256.Sum256(archive)
	index := fmt.Sprintf(`{"schemaVersion":1,"entries":[{"key":"example","directory":"catalog/example.zip","sourceCommit":"1111111111111111111111111111111111111111","archiveSHA256":"%x","package":{"manifest":{"name":"example"}}}]}`, sum)
	if err := os.WriteFile(filepath.Join(root, "catalog.json"), []byte(index), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "example.zip"), archive, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(root, "example"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "example.zip"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(root, "example"); err == nil {
		t.Fatal("accepted modified ZIP")
	}
}
