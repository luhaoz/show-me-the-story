package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"showmethestory/internal/config"
	"showmethestory/internal/sse"
	"showmethestory/internal/story"
	"showmethestory/internal/trilium"
)

func TestTriliumPublishCreatesNewChildAndRequiresAcceptedChapters(t *testing.T) {
	created := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "secret" || r.URL.Path != "/etapi/create-note" {
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var body struct{ ParentNoteID, Title, Type, Content string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.ParentNoteID != "outline123" || body.Type != "text" || !strings.Contains(body.Content, "<h2>第 1 章") || strings.Contains(body.Content, "Inherited") {
			t.Errorf("bad child: %+v", body)
		}
		created++
		w.Write([]byte(`{"note":{"noteId":"created123"}}`))
	}))
	defer server.Close()
	root := t.TempDir()
	if err := trilium.SaveConfig(trilium.ConfigPath(root), trilium.Config{BaseURL: server.URL, Token: "secret"}); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(&config.APIConfig{}, "", sse.NewLogBroadcaster(), root, "test")
	h.projectName = "story"
	h.state.TriliumSource = &story.TriliumSource{BaseURL: server.URL, NoteID: "outline123"}
	h.state.Title = "Book"
	h.state.Chapters = []story.ChapterState{{Num: 0, Title: "Inherited", Inherited: true, Status: story.StatusAccepted}, {Num: 1, Title: "Opening", Content: "Body", Status: story.StatusReview}}
	call := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.PostTriliumPublish(w, httptest.NewRequest("POST", "/api/trilium/publish", nil))
		return w
	}
	if w := call(); w.Code != http.StatusBadRequest || created != 0 {
		t.Fatalf("incomplete publication: %d %s", w.Code, w.Body.String())
	}
	h.state.Chapters[1].Status = story.StatusAccepted
	for i := 0; i < 2; i++ {
		if w := call(); w.Code != http.StatusCreated {
			t.Fatalf("publication %d: %d %s", i, w.Code, w.Body.String())
		}
	}
	if created != 2 {
		t.Fatalf("expected two new notes, got %d", created)
	}
}

func TestTriliumConfigHidesTokenAndSourceBinding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/etapi/notes/outline123":
			w.Write([]byte(`{"noteId":"outline123","title":"Outline","type":"text"}`))
		case "/etapi/notes/outline123/content":
			w.Write([]byte(`<p>Plot</p>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	root := t.TempDir()
	if err := trilium.SaveConfig(trilium.ConfigPath(root), trilium.Config{BaseURL: server.URL, Token: "secret"}); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(&config.APIConfig{}, "", sse.NewLogBroadcaster(), root, "test")
	w := httptest.NewRecorder()
	h.GetTriliumConfig(w, httptest.NewRequest("GET", "/api/config/trilium", nil))
	if strings.Contains(w.Body.String(), "secret") || !strings.Contains(w.Body.String(), `"configured":true`) {
		t.Fatalf("configuration leaked or missing: %s", w.Body.String())
	}
	h.projectName = "story"
	w = httptest.NewRecorder()
	h.PostTriliumOutlineRead(w, httptest.NewRequest("POST", "/api/trilium/outline/read", strings.NewReader(`{"note_id":"outline123"}`)))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Plot") {
		t.Fatalf("read failed: %d %s", w.Code, w.Body.String())
	}
	req := story.OutlineBatchRequest{TriliumNoteID: "outline123"}
	if err := h.bindTriliumOutline(&req); err != nil || req.Source == nil || req.Source.Title != "Outline" {
		t.Fatalf("source binding: %+v %v", req.Source, err)
	}
}
