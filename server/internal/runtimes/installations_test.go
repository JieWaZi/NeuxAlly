package runtimes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestDiscoverInstallationsKeepsMultipleVersionsAndMissingHistory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses POSIX executable scripts")
	}
	directories := []string{t.TempDir(), t.TempDir()}
	for index, directory := range directories {
		version := "1.0"
		if index == 1 {
			version = "2.0"
		}
		if err := os.WriteFile(filepath.Join(directory, "codex"), []byte("#!/bin/sh\nprintf '"+version+"\\n'\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	sdk, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	request := DetectRequest{
		RuntimeID:       RuntimeIDCodex,
		HomeDirectory:   t.TempDir(),
		BaseEnvironment: []string{"PATH=" + directories[0] + string(filepath.ListSeparator) + directories[1]},
	}
	missing := filepath.Join(t.TempDir(), "codex")
	installations := sdk.DiscoverInstallations(context.Background(), request, []CLIInstallation{{
		Path: missing, Version: "0.9", Available: true,
	}})
	versions := make(map[string]CLIInstallation, len(installations))
	for _, installation := range installations {
		versions[installation.Path] = installation
	}
	for index, directory := range directories {
		path := filepath.Join(directory, "codex")
		want := "1.0"
		if index == 1 {
			want = "2.0"
		}
		if !versions[path].Available || versions[path].Version != want {
			t.Fatalf("installation %s = %+v, want %s", path, versions[path], want)
		}
	}
	if versions[missing].Available || versions[missing].Version != "0.9" {
		t.Fatalf("missing installation = %+v", versions[missing])
	}
}

// TestDiscoverInstallationsProbesPathsConcurrently 验证每个版本命令先启动后才等待其他安装路径。
func TestDiscoverInstallationsProbesPathsConcurrently(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses POSIX executable scripts")
	}
	root := t.TempDir()
	paths := []string{filepath.Join(root, "first"), filepath.Join(root, "second")}
	for index, path := range paths {
		// 启动屏障使用 shell 内建重定向，避免额外 touch 进程消耗版本命令自身的两秒预算。
		script := fmt.Sprintf("#!/bin/sh\n: > '%s/started-%d'\nwhile [ ! -f '%s/started-%d' ]; do /bin/sleep 0.01; done\nprintf '1.%d.0\\n'\n", root, index, root, 1-index, index)
		if err := os.WriteFile(path, []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	sdk, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	definition := sdk.definitions[RuntimeIDCodex]
	definition.binaryCandidates = paths
	sdk.definitions[RuntimeIDCodex] = definition
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	got := sdk.DiscoverInstallations(ctx, DetectRequest{RuntimeID: RuntimeIDCodex, HomeDirectory: root, BaseEnvironment: []string{"PATH="}}, []CLIInstallation{{Path: paths[0]}, {Path: paths[1]}})
	if len(got) != 2 {
		t.Fatalf("installations=%+v", got)
	}
	for index, installation := range got {
		if installation.Version != fmt.Sprintf("1.%d.0", index) {
			t.Fatalf("version probes were serialized: %+v", got)
		}
	}
}

// TestDiscoverInstallationsKeepsVersionAfterUnknownOutput 验证未知输出不覆盖已知安装版本。
func TestDiscoverInstallationsKeepsVersionAfterUnknownOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses POSIX executable scripts")
	}
	root := t.TempDir()
	path := filepath.Join(root, "codex")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'unknown\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	sdk, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	definition := sdk.definitions[RuntimeIDCodex]
	definition.binaryCandidates = []string{path}
	sdk.definitions[RuntimeIDCodex] = definition
	got := sdk.DiscoverInstallations(context.Background(), DetectRequest{RuntimeID: RuntimeIDCodex, HomeDirectory: root, BaseEnvironment: []string{"PATH="}}, []CLIInstallation{{Path: path, Version: "1.2.3"}})
	if len(got) != 1 || got[0].Version != "1.2.3" {
		t.Fatalf("known installation version overwritten: %+v", got)
	}
}
