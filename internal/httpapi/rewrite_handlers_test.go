package httpapi

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"showmethestory/internal/config"
	"showmethestory/internal/sse"
	"showmethestory/internal/story"
)

func TestProjectRewriteStartsNewProseWithoutChangingSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "storys", "original")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.Story.Title = "The Script"
	if err := config.SaveConfig(filepath.Join(source, "config.json"), cfg); err != nil {
		t.Fatal(err)
	}
	settings := &story.ProjectSettings{Characters: []story.Character{{ID: "c1", Name: "Alice"}}, StorySynced: map[int]string{1: "old"}}
	if err := story.SaveProjectSettings(filepath.Join(source, "settings.json"), settings); err != nil {
		t.Fatal(err)
	}
	before := &story.Progress{
		Phase: "writing", BookStatus: story.BookStatusCompleted, Title: "The Script", CurrentChapterIndex: 1,
		OutlineBatches: []story.OutlineBatch{{ID: 1, StartCh: 1, EndCh: 1, Synopsis: "script", PlannedFinal: true}},
		Chapters:       []story.ChapterState{{Num: 1, Title: "Opening", Outline: "outline", Content: "old prose", Summary: "old summary", Status: story.StatusAccepted, KnowledgeTracked: true}},
		MemoryEntries:  []story.MemoryEntry{{ID: 1, Content: "old prose fact"}},
		Foreshadows:    []story.Foreshadow{{ID: 1, Name: "clue", Description: "plan", Status: story.ForeshadowResolved, Events: []story.ForeshadowEvent{{Chapter: 1, Note: "old"}}, Resolution: "old ending"}},
	}
	if err := story.SaveProgress(filepath.Join(source, "progress.json"), before); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(&config.APIConfig{}, "", sse.NewLogBroadcaster(), root, "test")
	r := httptest.NewRequest("POST", "/api/projects/original/rewrite", strings.NewReader(`{"name":"alternate"}`))
	r.SetPathValue("name", "original")
	w := httptest.NewRecorder()
	h.PostProjectRewrite(w, r)
	if w.Code != 200 {
		t.Fatalf("rewrite returned %d: %s", w.Code, w.Body.String())
	}
	resultDir := filepath.Join(root, "storys", "alternate")
	got, err := story.LoadProgress(filepath.Join(resultDir, "progress.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != "writing" || got.BookStatus != story.BookStatusActive || got.CurrentChapterIndex != 0 || len(got.Chapters) != 1 || got.Chapters[0].Status != story.StatusPending || got.Chapters[0].Content != "" || got.Chapters[0].Summary != "" || got.Chapters[0].Outline != "outline" || got.Chapters[0].KnowledgeTracked || len(got.MemoryEntries) != 0 {
		t.Fatalf("unexpected new prose state: %+v", got)
	}
	if !got.OutlineBatches[0].PlannedFinal || got.Foreshadows[0].Status != story.ForeshadowPlanted || len(got.Foreshadows[0].Events) != 0 || got.Foreshadows[0].Resolution != "" {
		t.Fatalf("planning was not reset correctly: %+v", got)
	}
	newSettings, err := story.LoadProjectSettings(filepath.Join(resultDir, "settings.json"))
	if err != nil || len(newSettings.Characters) != 1 || len(newSettings.StorySynced) != 0 {
		t.Fatalf("settings copy: %+v, %v", newSettings, err)
	}
	original, err := story.LoadProgress(filepath.Join(source, "progress.json"))
	if err != nil || original.Chapters[0].Content != "old prose" {
		t.Fatalf("source changed: %+v, %v", original, err)
	}
	if data, err := os.ReadFile(filepath.Join(resultDir, "chapters", "000001.json")); err != nil || strings.Contains(string(data), "old prose") {
		t.Fatalf("old prose copied: %v", err)
	}

	duplicate := httptest.NewRequest("POST", "/api/projects/original/rewrite", strings.NewReader(`{"name":"alternate"}`))
	duplicate.SetPathValue("name", "original")
	duplicateResult := httptest.NewRecorder()
	h.PostProjectRewrite(duplicateResult, duplicate)
	if duplicateResult.Code != 409 {
		t.Fatalf("duplicate returned %d", duplicateResult.Code)
	}
}

func TestRewriteProgressKeepsInheritedPrequelContext(t *testing.T) {
	got := rewriteProgress(&story.Progress{
		Chapters: []story.ChapterState{
			{Num: 1, Inherited: true, Summary: "prequel", Status: story.StatusAccepted},
			{Num: 2, Content: "sequel prose", Status: story.StatusAccepted},
		},
		MemoryEntries: []story.MemoryEntry{{ID: 1, Content: "prequel fact", Inherited: true}, {ID: 2, Content: "sequel fact"}},
		Foreshadows:   []story.Foreshadow{{ID: 1, Inherited: true, Status: story.ForeshadowResolved}, {ID: 2, Status: story.ForeshadowResolved}},
	})
	if got.CurrentChapterIndex != 1 || got.Chapters[0].Summary != "prequel" || got.Chapters[1].Status != story.StatusPending || len(got.MemoryEntries) != 1 || got.Foreshadows[0].Status != story.ForeshadowResolved || got.Foreshadows[1].Status != story.ForeshadowPlanted {
		t.Fatalf("unexpected inherited context: %+v", got)
	}
}
