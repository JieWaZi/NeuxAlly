package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewReadsActualContentAndDigest(t *testing.T) {
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "report.html")
	if err := os.WriteFile(target, []byte(`<script>window.bad=true</script><h1>真实报告</h1>`), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := ReadPreview(context.Background(), directory, "report.html")
	if err != nil || first.Reason != "" || !first.Text || first.MediaType != "text/plain; charset=utf-8" || first.Digest == "" {
		t.Fatalf("HTML must remain text: %+v %v", first, err)
	}
	if err := os.WriteFile(target, []byte("changed report"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := ReadPreview(context.Background(), directory, target)
	if err != nil || changed.Reason != "" || changed.Digest == first.Digest || string(changed.Content) != "changed report" {
		t.Fatalf("actual content not checked: %+v %v", changed, err)
	}
	missing, err := ReadPreview(context.Background(), directory, "missing.md")
	if err != nil || len(missing.Content) > 0 || missing.Reason != "missing" {
		t.Fatalf("missing artifact: %+v %v", missing, err)
	}
}

func TestPreviewRejectsEscapingLinksAndReboundScope(t *testing.T) {
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "private.txt"), []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "private.txt"), filepath.Join(directory, "link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, path := range []string{"link.txt", filepath.Join(outside, "private.txt"), "../" + filepath.Base(outside) + "/private.txt"} {
		view, err := ReadPreview(context.Background(), directory, path)
		if err != nil || len(view.Content) > 0 || view.Reason != "outside_scope" {
			t.Fatalf("scope escaped: %s %+v %v", path, view, err)
		}
	}
	if err := os.Rename(directory, directory+"-old"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory + "-old") })
	if err := os.Symlink(outside, directory); err != nil {
		t.Fatal(err)
	}
	view, err := ReadPreview(context.Background(), directory, "private.txt")
	if err != nil || len(view.Content) > 0 || view.Reason != "scope_changed" {
		t.Fatalf("frozen scope rebound: %+v %v", view, err)
	}
}

func TestPreviewRejectsUnboundedOrWrongTypeContent(t *testing.T) {
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{"bad.png": []byte("not a PNG"), "binary.txt": {0xff, 0xfe}, "large.md": make([]byte, (1<<20)+1)} {
		if err := os.WriteFile(filepath.Join(directory, name), content, 0600); err != nil {
			t.Fatal(err)
		}
		view, err := ReadPreview(context.Background(), directory, name)
		if err != nil || view.Reason == "" || len(view.Content) > 0 {
			t.Fatalf("invalid artifact became readable: %s %+v %v", name, view, err)
		}
	}
}
