package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"showmethestory/internal/config"
	"showmethestory/internal/fsutil"
	"showmethestory/internal/story"
)

func sharedCardView(card *story.SharedCard, detail bool) map[string]any {
	view := map[string]any{
		"schema_version": card.SchemaVersion, "id": card.ID, "kind": card.Kind, "name": card.Name,
		"description": card.Description, "source_project": card.SourceProject, "source_language": card.SourceLanguage,
		"created_at": card.CreatedAt, "updated_at": card.UpdatedAt,
		"dependency_counts": map[string]int{"characters": len(card.Dependencies.Characters), "worldview": len(card.Dependencies.Worldview), "organizations": len(card.Dependencies.Organizations)},
	}
	if card.API != nil {
		view["has_api_key"] = card.API.APIKey != ""
	}
	if card.Planning != nil {
		view["planning_direction"] = story.SharedPlanningSummary(card.Planning.LongTermDirection)
		view["planning_ending_intent"] = card.Planning.EndingIntent
		view["planning_ending_style"] = card.Planning.EndingStyle
	}
	if !detail {
		return view
	}
	copy := *card
	if copy.API != nil {
		api := *copy.API
		api.APIKey = ""
		copy.API = &api
	}
	view["card"] = copy
	return view
}

func (h *Handlers) GetSharedCards(w http.ResponseWriter, r *http.Request) {
	cards, err := story.ListSharedCards(h.progDir)
	if err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "shared_card_list_failed", err)
		return
	}
	views := make([]map[string]any, 0, len(cards))
	for i := range cards {
		views = append(views, sharedCardView(&cards[i], false))
	}
	h.writeJSON(w, http.StatusOK, views)
}

func (h *Handlers) GetSharedCard(w http.ResponseWriter, r *http.Request) {
	card, err := story.LoadSharedCard(h.progDir, r.PathValue("id"))
	if os.IsNotExist(err) {
		h.writeErrorReq(w, r, http.StatusNotFound, "shared_card_not_found")
		return
	}
	if err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "shared_card_load_failed", err)
		return
	}
	h.writeJSON(w, http.StatusOK, sharedCardView(card, true))
}

func (h *Handlers) PostSharedCard(w http.ResponseWriter, r *http.Request) {
	if !h.ensureProject(w, r) || h.rejectIfTaskRunning(w, r) {
		return
	}
	var req story.SharedCardCaptureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	card, err := story.CaptureSharedCard(req, h.projectName, h.cfg.Language, h.apiCfg, h.cfg, h.settings)
	if err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "shared_card_invalid", err)
		return
	}
	if err := story.SaveSharedCard(h.progDir, card); err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "shared_card_save_failed", err)
		return
	}
	h.writeJSON(w, http.StatusCreated, sharedCardView(card, false))
}

func (h *Handlers) PutSharedCard(w http.ResponseWriter, r *http.Request) {
	if h.rejectIfTaskRunning(w, r) {
		return
	}
	card, err := story.LoadSharedCard(h.progDir, r.PathValue("id"))
	if err != nil {
		h.writeErrorReq(w, r, http.StatusNotFound, "shared_card_not_found")
		return
	}
	var req struct {
		Name         string                    `json:"name"`
		Description  string                    `json:"description"`
		API          *config.APIConfig         `json:"api"`
		Story        *story.SharedStoryCard    `json:"story"`
		Style        *story.SharedStyleCard    `json:"style"`
		Planning     *story.SharedPlanningCard `json:"planning"`
		Character    *story.Character          `json:"character"`
		Worldview    *story.WorldviewEntry     `json:"worldview"`
		Organization *story.Organization       `json:"organization"`
		Relation     *story.Relation           `json:"relation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	card.Name, card.Description, card.UpdatedAt = strings.TrimSpace(req.Name), strings.TrimSpace(req.Description), time.Now().UTC()
	switch card.Kind {
	case story.SharedCardAPI:
		if req.API != nil {
			if req.API.APIKey == "" && card.API != nil {
				req.API.APIKey = card.API.APIKey
			}
			card.API = req.API
		}
	case story.SharedCardStory:
		if req.Story != nil {
			card.Story = req.Story
		}
	case story.SharedCardStyle:
		if req.Style != nil {
			card.Style = req.Style
		}
	case story.SharedCardPlanning:
		if req.Planning != nil {
			card.Planning = req.Planning
		}
	case story.SharedCardCharacter:
		if req.Character != nil {
			req.Character.ID = card.Character.ID
			card.Character = req.Character
		}
	case story.SharedCardWorldview:
		if req.Worldview != nil {
			req.Worldview.ID = card.Worldview.ID
			card.Worldview = req.Worldview
		}
	case story.SharedCardOrganization:
		if req.Organization != nil {
			req.Organization.ID = card.Organization.ID
			card.Organization = req.Organization
		}
	case story.SharedCardRelation:
		if req.Relation != nil {
			req.Relation.ID = card.Relation.ID
			card.Relation = req.Relation
		}
	}
	if err := story.SaveSharedCard(h.progDir, card); err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "shared_card_save_failed", err)
		return
	}
	h.writeJSON(w, http.StatusOK, sharedCardView(card, false))
}

func (h *Handlers) DeleteSharedCard(w http.ResponseWriter, r *http.Request) {
	if h.rejectIfTaskRunning(w, r) {
		return
	}
	err := story.DeleteSharedCard(h.progDir, r.PathValue("id"))
	if os.IsNotExist(err) {
		h.writeErrorReq(w, r, http.StatusNotFound, "shared_card_not_found")
		return
	}
	if err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "shared_card_delete_failed", err)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handlers) PostSharedCardImport(w http.ResponseWriter, r *http.Request) {
	if !h.ensureProject(w, r) || h.rejectIfTaskRunning(w, r) {
		return
	}
	card, err := story.LoadSharedCard(h.progDir, r.PathValue("id"))
	if err != nil {
		h.writeErrorReq(w, r, http.StatusNotFound, "shared_card_not_found")
		return
	}
	var req struct {
		ConfirmWritten bool `json:"confirm_written"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorReq(w, r, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	nextAPI, nextConfig, nextSettings, result, err := story.ImportSharedCard(card, h.apiCfg, h.cfg, h.settings, h.state, req.ConfirmWritten)
	if err != nil {
		if err.Error() == "written project confirmation required" {
			h.writeJSON(w, http.StatusConflict, map[string]any{"error": "written project confirmation required", "code": "shared_card_written_confirm"})
			return
		}
		h.writeErrorReq(w, r, http.StatusBadRequest, "shared_card_import_failed", err)
		return
	}
	switch card.Kind {
	case story.SharedCardPlanning:
		// Planning cards only fill the client's outline form; no project state is written.
		h.writeJSON(w, http.StatusOK, result)
		return
	case story.SharedCardAPI:
		data, marshalErr := json.MarshalIndent(nextAPI, "", "  ")
		if marshalErr != nil {
			err = marshalErr
		} else {
			err = fsutil.WriteFileAtomic(h.apiCfgPath, data)
		}
	case story.SharedCardStory, story.SharedCardStyle:
		err = config.SaveConfig(h.cfgPath, nextConfig)
	default:
		err = story.SaveProjectSettings(h.settingsPath, nextSettings)
	}
	if err != nil {
		h.writeErrorReq(w, r, http.StatusInternalServerError, "shared_card_import_failed", err)
		return
	}
	h.apiCfg, h.cfg, h.settings = nextAPI, nextConfig, nextSettings
	h.logger.SettingsUpdated()
	h.writeJSON(w, http.StatusOK, result)
}
