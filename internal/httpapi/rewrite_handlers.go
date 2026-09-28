package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"showmethestory/internal/config"
	"showmethestory/internal/story"
)

// rewriteProgress keeps the script while giving this project its own prose history.
func rewriteProgress(source *story.Progress) *story.Progress {
	if source == nil {
		return &story.Progress{Phase: "outline", BookStatus: story.BookStatusActive}
	}
	next := &story.Progress{
		Phase:               "outline",
		BookStatus:          story.BookStatusActive,
		Title:               source.Title,
		CorePrompt:          source.CorePrompt,
		LongTermDirection:   source.LongTermDirection,
		StoryConfigSnapshot: source.StoryConfigSnapshot,
		NextMemoryID:        source.NextMemoryID,
		OutlineBatches:      append([]story.OutlineBatch(nil), source.OutlineBatches...),
	}
	for _, ch := range source.Chapters {
		copy := story.ChapterState{
			Inherited:  ch.Inherited,
			Num:        ch.Num,
			Title:      ch.Title,
			Outline:    ch.Outline,
			Characters: append([]story.OutlineChapterCharacter(nil), ch.Characters...),
			Status:     story.StatusPending,
		}
		if ch.Inherited {
			copy.Status = story.StatusAccepted
			copy.Summary = ch.Summary
		}
		next.Chapters = append(next.Chapters, copy)
	}
	for _, fs := range source.Foreshadows {
		fs.Events = append([]story.ForeshadowEvent(nil), fs.Events...)
		if !fs.Inherited {
			fs.Status = story.ForeshadowPlanted
			fs.Events = nil
			fs.Resolution = ""
		}
		next.Foreshadows = append(next.Foreshadows, fs)
	}
	for _, fact := range source.MemoryEntries {
		if fact.Inherited {
			fact.References = nil
			fact.Snippet = ""
			next.MemoryEntries = append(next.MemoryEntries, fact)
		}
	}
	for i, ch := range next.Chapters {
		if !ch.Inherited {
			next.CurrentChapterIndex = i
			break
		}
		next.CurrentChapterIndex = i + 1
	}
	if len(next.Chapters) > 0 {
		next.Phase = "writing"
	}
	return next
}

func (h *Handlers) PostProjectRewrite(w http.ResponseWriter, r *http.Request) {
	if h.rejectIfTaskRunning(w, r) {
		return
	}
	sourceName := r.PathValue("name")
	if !validProjectName(sourceName) {
		h.writeErrorReq(w, r, http.StatusBadRequest, "project_name_invalid_chars")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || !validProjectName(req.Name) {
		h.writeErrorReq(w, r, http.StatusBadRequest, "project_name_invalid_chars")
		return
	}
	sourceDir := filepath.Join(h.storysDir(), sourceName)
	sourceInfo, err := os.Lstat(sourceDir)
	if err != nil || !sourceInfo.IsDir() {
		h.writeErrorReq(w, r, http.StatusNotFound, "project_not_found")
		return
	}
	dest := filepath.Join(h.storysDir(), req.Name)
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		h.writeErrorReq(w, r, http.StatusConflict, "project_exists")
		return
	}
	if err := ensureProjectCompatible(sourceDir); err != nil {
		h.writeErrorReq(w, r, http.StatusConflict, "project_incompatible")
		return
	}
	if err := validateBackupProject(sourceDir); err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "rewrite_failed", err)
		return
	}
	cfg, err := config.LoadConfig(filepath.Join(sourceDir, "config.json"))
	if err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "rewrite_failed", err)
		return
	}
	state, err := story.LoadProgress(filepath.Join(sourceDir, "progress.json"))
	if err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "rewrite_failed", err)
		return
	}
	settings, err := story.LoadProjectSettings(filepath.Join(sourceDir, "settings.json"))
	if err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "rewrite_failed", err)
		return
	}
	settings.StoryChanges = nil
	settings.StorySynced = nil
	cfg.CreatedWithVersion = h.version
	cfg.ProjectFormatVersion = config.ProjectFormatVersion
	stage, err := os.MkdirTemp(h.storysDir(), ".restore-")
	if err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "rewrite_failed", err)
		return
	}
	defer os.RemoveAll(stage)
	if err := config.SaveConfig(filepath.Join(stage, "config.json"), cfg); err == nil {
		err = story.SaveProjectSettings(filepath.Join(stage, "settings.json"), settings)
	}
	if err == nil {
		err = story.SaveProgress(filepath.Join(stage, "progress.json"), rewriteProgress(state))
	}
	if err == nil {
		err = validateBackupProject(stage)
	}
	if err == nil {
		err = r.Context().Err()
	}
	if err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "rewrite_failed", err)
		return
	}
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		h.writeErrorReq(w, r, http.StatusConflict, "project_exists")
		return
	}
	if err := os.Rename(stage, dest); err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "rewrite_failed", err)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"name": req.Name, "language": cfg.Language})
}
