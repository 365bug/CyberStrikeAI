package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"cyberstrike-ai/internal/c2"
	"cyberstrike-ai/internal/database"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestC2ListenerEditRejectsInvalidFieldsWithoutSaving(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "listener.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	original := &database.C2Listener{ID: "listener", Name: "original", Type: "http_beacon", BindHost: "127.0.0.1", BindPort: 8080, Status: "stopped", CreatedAt: time.Now()}
	if err := db.CreateC2Listener(original); err != nil {
		t.Fatal(err)
	}
	h := NewC2Handler(c2.NewManager(db, zap.NewNop(), t.TempDir()), zap.NewNop())
	r := gin.New()
	r.PUT("/listeners/:id", h.UpdateListener)
	for _, tc := range []struct {
		name   string
		port   int
		status int
	}{{"", 8080, 400}, {"  ", 8080, 400}, {"changed", 0, 400}, {"changed", 65536, 400}, {"changed", 8081, 200}} {
		body := fmt.Sprintf(`{"name":%q,"bind_host":"127.0.0.1","bind_port":%d,"remark":"new remark"}`, tc.name, tc.port)
		req := httptest.NewRequest(http.MethodPut, "/listeners/listener", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		saved, err := db.GetC2Listener("listener")
		if err != nil {
			t.Fatal(err)
		}
		if tc.status == 400 && (saved.Name != "original" || saved.BindPort != 8080 || saved.Remark != "") {
			t.Fatalf("invalid request saved fields: %#v", saved)
		}
		if tc.status == 200 && (saved.Name != "changed" || saved.BindPort != 8081 || saved.Remark != "new remark") {
			t.Fatalf("valid edit not saved: %#v", saved)
		}
	}
}
