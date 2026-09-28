package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"showmethestory/internal/fsutil"
	"showmethestory/internal/story"
)

// PostOutlineRestart clears this project's generated outline and prose so its
// existing configuration and authored settings can seed a new first batch.
func (h *Handlers) PostOutlineRestart(w http.ResponseWriter, r *http.Request) {
	if !h.ensureProject(w, r) {
		return
	}
	if !h.tryStartTask() {
		h.writeErrorReq(w, r, http.StatusConflict, "task_running_wait")
		return
	}
	h.skipKnowledgeSync = true
	defer h.endTask()

	oldChapters := append([]story.ChapterState(nil), h.state.Chapters...)
	next := &story.Progress{
		Phase: "outline", BookStatus: story.BookStatusActive,
		Title: h.state.Title, LongTermDirection: h.state.LongTermDirection,
		TriliumSource: h.state.TriliumSource,
	}
	if err := story.SaveProgress(h.progressPath, next); err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "outline_restart_failed", err)
		return
	}
	h.state = next
	if err := h.finishOutlineRestart(oldChapters); err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "outline_restart_failed", err)
		return
	}
	h.broadcastProgress()
	h.writeJSON(w, http.StatusOK, story.ProgressView(h.state))
}

func (h *Handlers) finishOutlineRestart(oldChapters []story.ChapterState) error {
	settings := *h.settings
	settings.StoryChanges = nil
	settings.StorySynced = nil
	if err := story.SaveProjectSettings(h.settingsPath, &settings); err != nil {
		return fmt.Errorf("reset settings sync: %w", err)
	}
	h.settings = &settings
	postprocess := story.NewProofreadState()
	if err := story.SavePostProcess(h.postprocessPath, postprocess); err != nil {
		return fmt.Errorf("reset proofreading: %w", err)
	}
	h.postprocess = postprocess
	if err := story.SavePendingConfigChanges(story.PendingConfigChangesPath(h.progressPath), nil); err != nil {
		return fmt.Errorf("reset pending configuration: %w", err)
	}
	for _, path := range []string{story.ImportStatePath(h.projectDir()), story.ForeshadowRoadmapPath(h.projectDir())} {
		if err := removeIfExists(path); err != nil {
			return err
		}
	}
	for _, ch := range oldChapters {
		if err := removeIfExists(story.ChapterMarkdownPath(h.projectDir(), ch.Num)); err != nil {
			return err
		}
		if err := removeIfExists(filepath.Join(h.projectDir(), "chapters", fmt.Sprintf("%06d.json", ch.Num))); err != nil {
			return err
		}
	}
	// SaveProgress commits empty metadata before orphan chapter files are removed.
	return nil
}

func removeIfExists(path string) error {
	if err := fsutil.Delete(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}
