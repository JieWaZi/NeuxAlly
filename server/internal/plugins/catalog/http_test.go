package catalog

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPQueriesReturnCachedIndexWhileNetworkWaits(t *testing.T) {
	gate := make(chan struct{})
	defer close(gate)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-gate
		fmt.Fprint(w, `{"schemaVersion":2,"entries":[]}`)
	}))
	// 先释放请求再等待 HTTP 服务退出，失败路径同样有界回收。
	t.Cleanup(source.Close)
	location := source.URL + "/catalog.json"
	cache := indexCache(location)
	if err := os.MkdirAll(filepath.Dir(cache), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache, []byte(`{"schemaVersion":2,"entries":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(cache) })
	old := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(cache, old, old); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := List(location); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cached catalog query waited for network instead of returning the previous snapshot")
	}
}

func TestHTTPQueriesDoNotRefreshAfterFiveMinutes(t *testing.T) {
	var requests atomic.Int32
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{"schemaVersion":2,"entries":[]}`)
	}))
	defer source.Close()
	location := source.URL + "/catalog.json"
	if _, err := List(location); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(indexCache(location), old, old); err != nil {
		t.Fatal(err)
	}
	for range 10 {
		if _, err := List(location); err != nil {
			t.Fatal(err)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("page/search queries refetched remote index %d times", requests.Load())
	}
}

func TestHTTPMetadataAndDigestDownload(t *testing.T) {
	archive := []byte("fixed ZIP placeholder")
	sum := sha256.Sum256(archive)
	downloads := 0
	var source *httptest.Server
	source = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/catalog.json" {
			fmt.Fprintf(w, `{"schemaVersion":2,"entries":[{"key":"github","sourceCommit":"1111111111111111111111111111111111111111","archiveURL":%q,"archiveSHA256":"%x","packagePath":"neuxally-plugin/github","name":"github","version":"2","description":"GitHub tools","format":"codex"}]}`, source.URL+"/github.zip", sum)
			return
		}
		downloads++
		w.Write(archive)
	}))
	defer source.Close()
	entries, err := List(source.URL + "/catalog.json")
	if err != nil || len(entries) != 1 || entries[0].Package.Manifest.Version != "2" || downloads != 0 {
		t.Fatalf("metadata fetch downloaded package: %+v %v", entries, err)
	}
	location, err := Resolve(source.URL+"/catalog.json", "github")
	if err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(location)
	if string(content) != string(archive) {
		t.Fatal("wrong download")
	}
	archive = []byte("tampered")
	os.Remove(location)
	if _, err := Resolve(source.URL+"/catalog.json", "github"); err == nil {
		t.Fatal("accepted digest mismatch")
	}
}

func TestHTTPDirectoryFailurePreservesStaleMetadata(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"schemaVersion":2,"entries":[]}`) }))
	location := source.URL + "/catalog.json"
	if entries, err := List(location); err != nil || len(entries) != 0 {
		t.Fatalf("initial catalog: %v", err)
	}
	source.Close()
	updated := make(chan struct{}, 1)
	stop := Start(context.Background(), location, filepath.Dir(indexCache(location)), func() { updated <- struct{}{} })
	<-updated
	stop()
	if entries, err := List(location); err != nil || len(entries) != 0 || State(location).State != "stale" {
		t.Fatalf("stale cache disappeared: %v %+v", err, State(location))
	}
	fresh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer fresh.Close()
	if _, err := List(fresh.URL + "/catalog.json"); err == nil {
		t.Fatal("first sync failure disguised as empty catalog")
	}
}

func TestDifferentCatalogSourcesDoNotMergeSameName(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"schemaVersion":2,"entries":[{"key":"same-name","sourceCommit":"1111111111111111111111111111111111111111","archiveURL":"https://example.invalid/fixed.zip","archiveSHA256":"1111111111111111111111111111111111111111111111111111111111111111","packagePath":"neuxally-plugin/same-name","name":"same-name","version":"1","format":"codex"}]}`)
	})
	first := httptest.NewServer(handler)
	defer first.Close()
	second := httptest.NewServer(handler)
	defer second.Close()
	one, err := List(first.URL + "/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	two, err := List(second.URL + "/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	if one[0].SourceKey == two[0].SourceKey {
		t.Fatal("merged unrelated sources by display name")
	}
}

func startupCatalog(version string) string {
	return fmt.Sprintf(`{"schemaVersion":2,"entries":[{"key":"same-name","sourceCommit":"1111111111111111111111111111111111111111","archiveURL":"https://example.invalid/fixed.zip","archiveSHA256":"1111111111111111111111111111111111111111111111111111111111111111","packagePath":"neuxally-plugin/same-name","name":"same-name","version":%q,"format":"codex"}]}`, version)
}

func awaitCatalogSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("catalog startup operation did not finish")
	}
}

func TestStartupCachePersistsAndRevalidatesOnce(t *testing.T) {
	var requests atomic.Int32
	gate := make(chan struct{})
	entered := make(chan struct{}, 1)
	validators := make(chan string, 3)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requests.Add(1)
		validators <- r.Header.Get("If-None-Match") + "|" + r.Header.Get("If-Modified-Since")
		if count == 2 {
			entered <- struct{}{}
			<-gate
		}
		w.Header().Set("ETag", `"v2"`)
		w.Header().Set("Last-Modified", "Mon, 19 Oct 2026 01:00:00 GMT")
		if count == 3 {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if count == 1 {
			w.Header().Set("ETag", `"v1"`)
			fmt.Fprint(w, startupCatalog("0.1.0"))
			return
		}
		fmt.Fprint(w, startupCatalog("0.1.1"))
	}))
	defer source.Close()
	directory := t.TempDir()
	location := source.URL + "/catalog.json"
	changed := make(chan struct{}, 3)
	stop := Start(context.Background(), location, directory, func() { changed <- struct{}{} })
	awaitCatalogSignal(t, changed)
	stop()
	if filepath.Dir(indexCache(location)) != directory {
		t.Fatal("catalog was not stored in application data directory")
	}
	if _, err := os.Stat(indexCache(location)); err != nil {
		t.Fatal(err)
	}

	// 重启后已知内容立即可读；更新等待期间的查询和搜索不会再次拉取远端。
	stop = Start(context.Background(), location, directory, func() { changed <- struct{}{} })
	awaitCatalogSignal(t, entered)
	done := make(chan error, 1)
	go func() {
		entries, err := List(location)
		if err == nil && entries[0].Package.Manifest.Version != "0.1.0" {
			err = fmt.Errorf("previous version lost")
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("persisted catalog waited for startup refresh")
	}
	for range 10 {
		if _, err := List(location); err != nil {
			t.Error(err)
		}
	}
	close(gate)
	awaitCatalogSignal(t, changed)
	stop()
	entries, err := List(location)
	if err != nil || entries[0].Package.Manifest.Version != "0.1.1" {
		t.Fatalf("new snapshot not published: %+v %v", entries, err)
	}

	stop = Start(context.Background(), location, directory, func() { changed <- struct{}{} })
	awaitCatalogSignal(t, changed)
	stop()
	entries, err = List(location)
	if err != nil || entries[0].Package.Manifest.Version != "0.1.1" || requests.Load() != 3 {
		t.Fatalf("304 lost content or duplicate startup fetch: %v requests=%d", err, requests.Load())
	}
	if <-validators != "|" || <-validators != `"v1"|Mon, 19 Oct 2026 01:00:00 GMT` || <-validators != `"v2"|Mon, 19 Oct 2026 01:00:00 GMT` {
		t.Fatal("conditional request validators were not restored")
	}
	if State(location).State != "available" {
		t.Fatal("304 failed to mark snapshot checked")
	}
}

func TestColdStartupConcurrentQueriesShareOneRequest(t *testing.T) {
	var requests atomic.Int32
	entered := make(chan struct{}, 1)
	gate := make(chan struct{})
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		entered <- struct{}{}
		<-gate
		fmt.Fprint(w, startupCatalog("0.1.0"))
	}))
	defer source.Close()
	location := source.URL + "/catalog.json"
	stop := Start(context.Background(), location, t.TempDir(), nil)
	awaitCatalogSignal(t, entered)
	var clients sync.WaitGroup
	failures := make(chan error, 20)
	for range 20 {
		clients.Add(1)
		go func() {
			defer clients.Done()
			entries, err := List(location)
			if err == nil && len(entries) != 1 {
				err = fmt.Errorf("cold catalog returned a fake empty result")
			}
			failures <- err
		}()
	}
	close(gate)
	clients.Wait()
	stop()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Error(err)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("concurrent cold queries made %d requests", requests.Load())
	}
}

func TestStartupRefreshFailureNeverReplacesGoodSnapshot(t *testing.T) {
	var mode atomic.Int32
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch mode.Load() {
		case 1:
			w.WriteHeader(http.StatusServiceUnavailable)
		case 2:
			fmt.Fprint(w, `{"schemaVersion":2,"entries":[`)
		default:
			fmt.Fprint(w, startupCatalog("0.1.0"))
		}
	}))
	defer source.Close()
	location := source.URL + "/catalog.json"
	directory := t.TempDir()
	changed := make(chan struct{}, 1)
	stop := Start(context.Background(), location, directory, func() { changed <- struct{}{} })
	awaitCatalogSignal(t, changed)
	stop()
	before, err := os.ReadFile(indexCache(location))
	if err != nil {
		t.Fatal(err)
	}
	for _, failureMode := range []int32{1, 2} {
		mode.Store(failureMode)
		stop = Start(context.Background(), location, directory, func() { changed <- struct{}{} })
		awaitCatalogSignal(t, changed)
		stop()
		entries, err := List(location)
		if err != nil || len(entries) != 1 || entries[0].Package.Manifest.Version != "0.1.0" || State(location).State != "stale" {
			t.Fatalf("failure lost known data: %v %+v", err, State(location))
		}
		after, err := os.ReadFile(indexCache(location))
		if err != nil || string(before) != string(after) {
			t.Fatal("failed refresh overwrote persistent cache")
		}
	}
}

func TestStartupCloseCancelsRefreshAndDoesNotRetryOnQuery(t *testing.T) {
	var requests atomic.Int32
	entered := make(chan struct{}, 1)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		entered <- struct{}{}
		<-r.Context().Done()
	}))
	defer source.Close()
	location := source.URL + "/catalog.json"
	stop := Start(context.Background(), location, t.TempDir(), nil)
	awaitCatalogSignal(t, entered)
	closed := make(chan struct{})
	go func() { stop(); close(closed) }()
	awaitCatalogSignal(t, closed)
	if _, err := List(location); err == nil {
		t.Fatal("cancelled first sync presented a successful empty catalog")
	}
	if requests.Load() != 1 {
		t.Fatal("query retried a stopped startup operation")
	}
}
