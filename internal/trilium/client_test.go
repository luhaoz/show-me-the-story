package trilium

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadAndCreateNote(t *testing.T) {
	var created struct{ ParentNoteID, Title, Type, Content string }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "secret" {
			t.Errorf("missing ETAPI authorization")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /etapi/notes/outline123":
			w.Write([]byte(`{"noteId":"outline123","title":"Outline","type":"text"}`))
		case "GET /etapi/notes/outline123/content":
			w.Write([]byte(`<h1>Plot</h1><p>One &amp; two</p><script>hidden</script>`))
		case "POST /etapi/create-note":
			if err := json.NewDecoder(r.Body).Decode(&created); err != nil {
				t.Error(err)
			}
			w.Write([]byte(`{"note":{"noteId":"created123"}}`))
		default:
			t.Errorf("unexpected ETAPI request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL + "/etapi", Token: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	note, synopsis, err := client.GetNote("outline123")
	if err != nil {
		t.Fatal(err)
	}
	if note.Title != "Outline" || !strings.Contains(synopsis, "Plot\nOne & two") || strings.Contains(synopsis, "hidden") {
		t.Fatalf("unexpected read: %#v %q", note, synopsis)
	}
	html := ArticleHTML([]ArticleChapter{{Num: 1, Title: "A < B", Content: "Hello & world\n\nNext"}}, true)
	id, err := client.CreateTextNote(note.NoteID, "Article", html)
	if err != nil {
		t.Fatal(err)
	}
	if id != "created123" || created.ParentNoteID != "outline123" || created.Type != "text" || !strings.Contains(created.Content, "A &lt; B") || !strings.Contains(created.Content, "Hello &amp; world") {
		t.Fatalf("unexpected create: id=%s request=%+v", id, created)
	}
}

func TestRejectInvalidNoteIDAndRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.com", http.StatusFound)
	}))
	defer server.Close()
	client, _ := NewClient(Config{BaseURL: server.URL, Token: "secret"})
	if _, _, err := client.GetNote("../bad"); err == nil {
		t.Fatal("invalid ID accepted")
	}
	if err := client.Test(); err == nil {
		t.Fatal("redirect accepted")
	}
}
