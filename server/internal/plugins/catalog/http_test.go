package catalog

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

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
	old := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(indexCache(location), old, old); err != nil {
		t.Fatal(err)
	}
	source.Close()
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
