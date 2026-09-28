package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"showmethestory/internal/config"
	"showmethestory/internal/sse"
	"showmethestory/internal/story"
)

func TestRestartOutlineClearsChaptersAndProseButKeepsSettings(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "storys", "novel")
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(&config.APIConfig{}, "", sse.NewLogBroadcaster(), root, "test")
	h.projectName = "novel"
	h.progressPath = filepath.Join(project, "progress.json")
	h.settingsPath = filepath.Join(project, "settings.json")
	h.postprocessPath = filepath.Join(project, "postprocess.json")
	h.state = &story.Progress{
		Phase: "writing", BookStatus: story.BookStatusCompleted, Title: "Book", CorePrompt: "old outline rule",
		LongTermDirection: "author direction", TriliumSource: &story.TriliumSource{BaseURL: "http://trilium.local", NoteID: "outline123"},
		OutlineBatches: []story.OutlineBatch{{ID: 1, StartCh: 1, EndCh: 1, Synopsis: "plot"}},
		Chapters:       []story.ChapterState{{Num: 1, Title: "Opening", Outline: "old outline", Content: "old prose", Status: story.StatusAccepted}},
		MemoryEntries:  []story.MemoryEntry{{ID: 1, Content: "old fact"}},
		Foreshadows:    []story.Foreshadow{{ID: 1, Name: "old clue"}},
	}
	h.settings = &story.ProjectSettings{Characters: []story.Character{{ID: "c1", Name: "Alice"}}, StorySynced: map[int]string{1: "old"}, StoryChanges: []story.SettingChange{{ID: 1}}}
	h.postprocess = &story.PostProcessState{SchemaVersion: story.ProofreadSchemaVersion, ProofreadAnalyzedAt: "old"}
	if err := story.SaveProgress(h.progressPath, h.state); err != nil {
		t.Fatal(err)
	}
	personalFile := filepath.Join(project, "chapters", "notes.txt")
	if err := os.WriteFile(personalFile, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := story.SaveProjectSettings(h.settingsPath, h.settings); err != nil {
		t.Fatal(err)
	}
	if err := story.SavePostProcess(h.postprocessPath, h.postprocess); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{story.ImportStatePath(project), story.ForeshadowRoadmapPath(project), story.ChapterMarkdownPath(project, 1)} {
		if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	w := httptest.NewRecorder()
	h.PostOutlineRestart(w, httptest.NewRequest("POST", "/api/outline/restart", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("restart: %d %s", w.Code, w.Body.String())
	}
	loaded, err := story.LoadProgress(h.progressPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Phase != "outline" || loaded.BookStatus != story.BookStatusActive || loaded.Title != "Book" || loaded.LongTermDirection != "author direction" || loaded.TriliumSource == nil || len(loaded.Chapters) != 0 || len(loaded.OutlineBatches) != 0 || len(loaded.MemoryEntries) != 0 || len(loaded.Foreshadows) != 0 || loaded.CorePrompt != "" {
		t.Fatalf("wrong restart state: %+v", loaded)
	}
	if h.isTaskRunning() {
		t.Fatal("task lock leaked")
	}
	settings, err := story.LoadProjectSettings(h.settingsPath)
	if err != nil || len(settings.Characters) != 1 || len(settings.StorySynced) != 0 || len(settings.StoryChanges) != 0 {
		t.Fatalf("wrong settings: %+v %v", settings, err)
	}
	pp, err := story.LoadPostProcess(h.postprocessPath)
	if err != nil || pp.ProofreadAnalyzedAt != "" {
		t.Fatalf("old proofreading remains: %+v %v", pp, err)
	}
	for _, path := range []string{story.ImportStatePath(project), story.ForeshadowRoadmapPath(project), story.ChapterMarkdownPath(project, 1), filepath.Join(project, "chapters", "000001.json")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("old artifact remains: %s (%v)", path, err)
		}
	}
	if data, err := os.ReadFile(personalFile); err != nil || string(data) != "keep" {
		t.Fatalf("unrelated file was removed: %v", err)
	}
}

func TestRestartOutlineRefusesRunningTask(t *testing.T) {
	h := NewHandlers(&config.APIConfig{}, "", sse.NewLogBroadcaster(), t.TempDir(), "test")
	h.projectName = "novel"
	h.state.Chapters = []story.ChapterState{{Num: 1, Status: story.StatusAccepted, Content: "keep"}}
	if !h.tryStartTask() {
		t.Fatal("could not reserve task")
	}
	w := httptest.NewRecorder()
	h.PostOutlineRestart(w, httptest.NewRequest("POST", "/api/outline/restart", nil))
	if w.Code != http.StatusConflict || h.state.Chapters[0].Content != "keep" {
		t.Fatalf("running task changed: %d %+v", w.Code, h.state)
	}
	h.skipKnowledgeSync = true
	h.endTask()
}

func TestRestartOutlineSaveFailureKeepsCurrentChapters(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(&config.APIConfig{}, "", sse.NewLogBroadcaster(), root, "test")
	h.projectName = "novel"
	h.progressPath = filepath.Join(blocked, "progress.json")
	h.state.Chapters = []story.ChapterState{{Num: 1, Status: story.StatusAccepted, Content: "keep"}}
	w := httptest.NewRecorder()
	h.PostOutlineRestart(w, httptest.NewRequest("POST", "/api/outline/restart", nil))
	if w.Code != http.StatusInternalServerError || h.state.Chapters[0].Content != "keep" || h.isTaskRunning() {
		t.Fatalf("failed reset changed state: %d %+v", w.Code, h.state)
	}
}
