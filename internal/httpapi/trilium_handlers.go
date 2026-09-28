package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"showmethestory/internal/story"
	"showmethestory/internal/trilium"
)

func (h *Handlers) triliumConfig() (trilium.Config, error) {
	return trilium.LoadConfig(trilium.ConfigPath(h.progDir))
}

func (h *Handlers) GetTriliumConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.triliumConfig()
	if err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "trilium_config_failed", err)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"base_url": cfg.BaseURL, "configured": cfg.BaseURL != "" && cfg.Token != ""})
}

func (h *Handlers) PutTriliumConfig(w http.ResponseWriter, r *http.Request) {
	if h.rejectIfTaskRunning(w, r) {
		return
	}
	var body trilium.Config
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "invalid_json", err)
		return
	}
	old, err := h.triliumConfig()
	if err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "trilium_config_failed", err)
		return
	}
	if body.Token == "" {
		body.Token = old.Token
	}
	if body.Token == "" {
		h.writeErrorReq(w, r, http.StatusBadRequest, "trilium_unconfigured")
		return
	}
	if strings.TrimSpace(body.BaseURL) == "" {
		h.writeErrorReq(w, r, http.StatusBadRequest, "trilium_config_invalid")
		return
	}
	if _, err := trilium.NormalizeBaseURL(body.BaseURL); err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "trilium_config_invalid")
		return
	}
	if err := trilium.SaveConfig(trilium.ConfigPath(h.progDir), body); err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "trilium_config_failed", err)
		return
	}
	h.GetTriliumConfig(w, r)
}

func (h *Handlers) PostTriliumTest(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.triliumConfig()
	if err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "trilium_config_failed", err)
		return
	}
	client, err := trilium.NewClient(cfg)
	if err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "trilium_unconfigured")
		return
	}
	if err := client.Test(); err != nil {
		h.writeErrorReq(w, r, http.StatusBadGateway, "trilium_request_failed", err)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handlers) PostTriliumOutlineRead(w http.ResponseWriter, r *http.Request) {
	if !h.ensureProject(w, r) {
		return
	}
	var body struct {
		NoteID string `json:"note_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "invalid_json", err)
		return
	}
	cfg, err := h.triliumConfig()
	if err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "trilium_config_failed", err)
		return
	}
	client, err := trilium.NewClient(cfg)
	if err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "trilium_unconfigured")
		return
	}
	note, synopsis, err := client.GetNote(strings.TrimSpace(body.NoteID))
	if err != nil {
		h.writeErrorReq(w, r, http.StatusBadGateway, "trilium_request_failed", err)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"note_id": note.NoteID, "title": note.Title, "synopsis": synopsis, "base_url": client.BaseURL()})
}

func (h *Handlers) PostTriliumPublish(w http.ResponseWriter, r *http.Request) {
	if !h.ensureProject(w, r) {
		return
	}
	if !h.tryStartTask() {
		h.writeErrorReq(w, r, http.StatusConflict, "task_running_wait")
		return
	}
	defer h.endTask()
	source := h.state.TriliumSource
	if source == nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "trilium_source_missing")
		return
	}
	cfg, err := h.triliumConfig()
	if err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "trilium_config_failed", err)
		return
	}
	client, err := trilium.NewClient(cfg)
	if err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "trilium_unconfigured")
		return
	}
	if client.BaseURL() != source.BaseURL {
		h.writeErrorReq(w, r, http.StatusBadRequest, "trilium_server_mismatch")
		return
	}
	chapters := make([]trilium.ArticleChapter, 0, len(h.state.Chapters))
	for _, ch := range h.state.Chapters {
		if ch.Inherited {
			continue
		}
		if ch.Status != story.StatusAccepted || strings.TrimSpace(ch.Content) == "" {
			h.writeErrorReq(w, r, http.StatusBadRequest, "trilium_chapters_incomplete")
			return
		}
		chapters = append(chapters, trilium.ArticleChapter{Num: ch.Num, Title: ch.Title, Content: ch.Content})
	}
	if len(chapters) == 0 {
		h.writeErrorReq(w, r, http.StatusBadRequest, "trilium_chapters_incomplete")
		return
	}
	sort.Slice(chapters, func(i, j int) bool { return chapters[i].Num < chapters[j].Num })
	title := fmt.Sprintf("%s (%s) %s", h.state.Title, h.projectName, time.Now().Format("2006-01-02 15:04:05"))
	id, err := client.CreateTextNote(source.NoteID, title, trilium.ArticleHTML(chapters, h.cfg.Language == "en"))
	if err != nil {
		h.writeErrorReq(w, r, http.StatusBadGateway, "trilium_publish_failed", err)
		return
	}
	h.writeJSON(w, http.StatusCreated, map[string]string{"note_id": id, "title": title})
}

// Bind the source only after a successful first outline batch has been saved.
func (h *Handlers) bindTriliumOutline(req *story.OutlineBatchRequest) error {
	if req.TriliumNoteID == "" {
		return nil
	}
	if len(h.state.Chapters) != 0 || req.Mode == "replace_last" || h.state.TriliumSource != nil {
		return fmt.Errorf("Trilium source can only be set for the first outline batch")
	}
	cfg, err := h.triliumConfig()
	if err != nil {
		return err
	}
	client, err := trilium.NewClient(cfg)
	if err != nil {
		return err
	}
	note, _, err := client.GetNote(req.TriliumNoteID)
	if err != nil {
		return err
	}
	req.Source = &story.TriliumSource{BaseURL: client.BaseURL(), NoteID: note.NoteID, Title: note.Title}
	return nil
}
