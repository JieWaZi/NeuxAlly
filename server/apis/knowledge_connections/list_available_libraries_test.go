package knowledge_connections

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/apis/knowledge"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/JieWaZi/neux-knowledge/core"
	"github.com/gin-gonic/gin"
)

// catalogConnections 只替换连接仓库，聚合窗口使用真实知识引擎和远端HTTP。
type catalogConnections struct {
	// KnowledgeConnectionController 保留本测试不使用的写接口。
	controllers.KnowledgeConnectionController
	// rows 是权威连接列表。
	rows []models.KnowledgeConnection
	// failure 模拟连接列表读取失败。
	failure error
}

func (f catalogConnections) List(context.Context) ([]models.KnowledgeConnection, error) {
	return f.rows, f.failure
}
func (f catalogConnections) Snapshot(_ context.Context, id string) (models.KnowledgeConnection, string, error) {
	for _, v := range f.rows {
		if v.ID == id {
			return v, "secret", nil
		}
	}
	return models.KnowledgeConnection{}, "", status_error.KnowledgeNotFound
}
func catalogContext(t *testing.T) *gin.Context {
	t.Helper()
	oldEngine, oldConnections, oldSummaries := global.KnowledgeEngine, global.KnowledgeConnectionControllerManager, global.KnowledgeControllerManager
	engine, err := core.Open(context.Background(), core.Options{DBPath: filepath.Join(t.TempDir(), "knowledge.db"), RawDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	global.KnowledgeEngine = engine
	global.KnowledgeControllerManager = nil
	global.KnowledgeConnectionControllerManager = catalogConnections{}
	t.Cleanup(func() {
		engine.Close()
		global.KnowledgeEngine = oldEngine
		global.KnowledgeConnectionControllerManager = oldConnections
		global.KnowledgeControllerManager = oldSummaries
	})
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	return ctx
}
func catalogServer(t *testing.T, total int, prefix string, calls *[][2]int) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		*calls = append(*calls, [2]int{offset, limit})
		if r.URL.Path != "/api/knowledge/v1/libraries" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("unexpected request %s", r.URL)
		}
		items := []*core.Library{}
		for i := offset; i < min(offset+limit, total); i++ {
			items = append(items, &core.Library{ID: fmt.Sprint(i), Name: prefix, Kind: "general"})
		}
		json.NewEncoder(w).Encode(map[string]any{"items": items, "total": total, "offset": offset, "limit": limit})
	}))
	t.Cleanup(server.Close)
	return server.URL
}
func TestAvailableLibrariesUsesBoundedGlobalWindowAndFullReferences(t *testing.T) {
	ctx := catalogContext(t)
	for i := 0; i < 3; i++ {
		if _, err := global.KnowledgeEngine.CreateLibrary(ctx, core.CreateLibraryInput{RequestID: fmt.Sprint("local", i), Name: "本地", Kind: "general", WikiLanguage: "zh-CN"}); err != nil {
			t.Fatal(err)
		}
	}
	a, b := [][2]int{}, [][2]int{}
	first := models.KnowledgeConnection{ID: "a", Backend: "custom", Enabled: true, AuthMode: "bearer", BaseURL: catalogServer(t, 70, "重名", &a)}
	second := models.KnowledgeConnection{ID: "b", Backend: "custom", Enabled: true, AuthMode: "bearer", BaseURL: catalogServer(t, 2, "重名", &b)}
	global.KnowledgeConnectionControllerManager = catalogConnections{rows: []models.KnowledgeConnection{second, first, {ID: "disabled", Enabled: false}}}
	raw, err := (&ListAvailableLibraries{Location: "all", Offset: 2, Limit: 4}).Output(ctx)
	if err != nil {
		t.Fatal(err)
	}
	out := raw.(knowledge.LibraryList)
	if out.Total != 75 || len(out.Items) != 4 || out.Items[0].Ref.Backend != "local" || out.Items[1].Ref.ConnectionID != "a" {
		t.Fatalf("wrong aggregate: %+v", out)
	}
	if fmt.Sprint(a) != "[[0 1] [0 3]]" || fmt.Sprint(b) != "[[0 1]]" {
		t.Fatalf("unbounded or incorrect reads: %v %v", a, b)
	}
	raw, err = (&ListAvailableLibraries{Location: "remote", Offset: 69, Limit: 3}).Output(ctx)
	if err != nil {
		t.Fatal(err)
	}
	out = raw.(knowledge.LibraryList)
	if out.Total != 72 || len(out.Items) != 3 || out.Items[0].Ref.ConnectionID != "a" || out.Items[1].Ref.ConnectionID != "b" {
		t.Fatalf("lost source identity: %+v", out)
	}
	raw, err = (&ListAvailableLibraries{Location: "remote", Offset: 0, Limit: 100}).Output(ctx)
	if err != nil {
		t.Fatal(err)
	}
	out = raw.(knowledge.LibraryList)
	if len(out.Items) != 72 || out.Items[0].ID != out.Items[70].ID || out.Items[0].Ref.ConnectionID == out.Items[70].Ref.ConnectionID {
		t.Fatalf("same ID across connections was merged: %+v", out)
	}

}
func TestAvailableLibrariesDoesNotHideRemoteFailuresOrReadExcludedSources(t *testing.T) {
	ctx := catalogContext(t)
	global.KnowledgeConnectionControllerManager = catalogConnections{failure: status_error.DatabaseError}
	raw, err := (&ListAvailableLibraries{Location: "local", Limit: 10}).Output(ctx)
	if err != nil || raw.(knowledge.LibraryList).Total != 0 {
		t.Fatal(raw, err)
	}
	if _, err := (&ListAvailableLibraries{Location: "all", Limit: 10}).Output(ctx); !errors.Is(err, status_error.DatabaseError) {
		t.Fatalf("connection failure became empty: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer server.Close()
	global.KnowledgeConnectionControllerManager = catalogConnections{rows: []models.KnowledgeConnection{{ID: "broken", Backend: "custom", Enabled: true, AuthMode: "bearer", BaseURL: server.URL}}}
	global.KnowledgeEngine = nil
	if _, err := (&ListAvailableLibraries{Location: "remote", Limit: 10}).Output(ctx); !errors.Is(err, status_error.KnowledgeRemoteUnreachable) {
		t.Fatalf("remote failure became empty or touched local: %v", err)
	}
}

func TestAvailableLibrariesStaticRouteCoexistsWithConnectionRoutes(t *testing.T) {
	router := gin.New()
	group := router.Group(Router.BasePath())
	group.GET((&ListAvailableLibraries{}).Path(), func(ctx *gin.Context) { ctx.String(http.StatusOK, "catalog") })
	group.GET((&ListRemoteLibraries{}).Path(), func(ctx *gin.Context) { ctx.String(http.StatusOK, ctx.Param("connectionId")) })
	group.GET((&GetRemoteLibrary{}).Path(), func(ctx *gin.Context) { ctx.String(http.StatusOK, ctx.Param("libraryId")) })
	for path, want := range map[string]string{
		"/knowledge/connections/libraries":                    "catalog",
		"/knowledge/connections/connection/libraries":         "connection",
		"/knowledge/connections/connection/libraries/library": "library",
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != http.StatusOK || response.Body.String() != want {
			t.Fatalf("route %s: %d %s", path, response.Code, response.Body.String())
		}
	}
}
