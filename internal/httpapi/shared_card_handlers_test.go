package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"showmethestory/internal/config"
	"showmethestory/internal/fsutil"
	"showmethestory/internal/sse"
	"showmethestory/internal/story"
)

func sharedCardTestHandler(t *testing.T) *Handlers {
	t.Helper()
	dir := t.TempDir()
	apiPath := filepath.Join(dir, "api.json")
	api := &config.APIConfig{BaseURL: "https://example.test", APIKey: "secret", Model: "model"}
	data, _ := json.Marshal(api)
	if err := fsutil.WriteFileAtomic(apiPath, data); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(api, apiPath, sse.NewLogBroadcaster(), dir, "test")
	projectDir := filepath.Join(h.storysDir(), "book")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveConfig(filepath.Join(projectDir, "config.json"), config.DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	settings := &story.ProjectSettings{
		Characters:    []story.Character{{ID: "c_1", Name: "Alice"}},
		Worldview:     []story.WorldviewEntry{{ID: "w_1", Category: "rule", Name: "Magic", Description: "Old rule"}},
		Organizations: []story.Organization{{ID: "o_1", Name: "Guild", Type: "order", Description: "Old group", Members: []string{"c_1"}}},
		Relations:     []story.Relation{{ID: "r_1", SourceID: "c_1", SourceType: "character", TargetID: "w_1", TargetType: "worldview", Label: "knows"}},
	}
	if err := story.SaveProjectSettings(filepath.Join(projectDir, "settings.json"), settings); err != nil {
		t.Fatal(err)
	}
	if err := h.switchProject("book"); err != nil {
		t.Fatal(err)
	}
	return h
}

func createSharedCard(t *testing.T, h *Handlers, body string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.PostSharedCard(rec, httptest.NewRequest("POST", "/api/shared-cards", strings.NewReader(body)))
	if rec.Code != 201 {
		t.Fatalf("create card: %d %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	if json.Unmarshal(rec.Body.Bytes(), &result) != nil {
		t.Fatal("invalid response")
	}
	return result["id"].(string)
}

func TestSharedAPICardMasksSecret(t *testing.T) {
	h := sharedCardTestHandler(t)
	id := createSharedCard(t, h, `{"kind":"api"}`)
	req := httptest.NewRequest("GET", "/api/shared-cards/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	h.GetSharedCard(rec, req)
	if rec.Code != 200 || bytes.Contains(rec.Body.Bytes(), []byte("secret")) || !bytes.Contains(rec.Body.Bytes(), []byte(`"has_api_key":true`)) {
		t.Fatalf("secret masking failed: %s", rec.Body.String())
	}
}

func TestSharedCharacterCardImportsAsNewEntity(t *testing.T) {
	h := sharedCardTestHandler(t)
	id := createSharedCard(t, h, `{"kind":"character","entity_id":"c_1"}`)
	req := httptest.NewRequest("POST", "/api/shared-cards/"+id+"/import", strings.NewReader(`{"confirm_written":false}`))
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	h.PostSharedCardImport(rec, req)
	if rec.Code != 200 || len(h.settings.Characters) != 2 || h.settings.Characters[1].ID == "c_1" {
		t.Fatalf("card import failed: %d %s %#v", rec.Code, rec.Body.String(), h.settings.Characters)
	}
}

func TestSharedCardMutationBlockedDuringTask(t *testing.T) {
	h := sharedCardTestHandler(t)
	h.taskRunning = true
	rec := httptest.NewRecorder()
	h.PostSharedCard(rec, httptest.NewRequest("POST", "/api/shared-cards", strings.NewReader(`{"kind":"api"}`)))
	if rec.Code != 409 {
		t.Fatalf("card capture allowed during task: %d", rec.Code)
	}
}

func TestSharedCharacterCardUpdatesFullPayload(t *testing.T) {
	h := sharedCardTestHandler(t)
	id := createSharedCard(t, h, `{"kind":"character","entity_id":"c_1"}`)
	req := httptest.NewRequest("PUT", "/api/shared-cards/"+id, strings.NewReader(`{"name":"Hero card","description":"Reusable","character":{"name":"Alice Prime","age":"30","appearance":"silver hair","personality":"calm","background":"pilot","motivation":"home","abilities":"flight","notes":"lead"}}`))
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	h.PutSharedCard(rec, req)
	if rec.Code != 200 {
		t.Fatalf("update card: %d %s", rec.Code, rec.Body.String())
	}
	card, err := story.LoadSharedCard(h.progDir, id)
	if err != nil {
		t.Fatal(err)
	}
	if card.Character.Name != "Alice Prime" || card.Character.Appearance != "silver hair" || card.Character.ID != "c_1" {
		t.Fatalf("character payload not updated: %#v", card.Character)
	}
}

func TestSharedAPICardEditPreservesMaskedSecret(t *testing.T) {
	h := sharedCardTestHandler(t)
	id := createSharedCard(t, h, `{"kind":"api"}`)
	req := httptest.NewRequest("PUT", "/api/shared-cards/"+id, strings.NewReader(`{"name":"API card","description":"","api":{"base_url":"https://new.test","model":"new-model","api_key":"","http_timeout_seconds":120,"max_tokens":4096,"context_budget_tokens":100000}}`))
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	h.PutSharedCard(rec, req)
	if rec.Code != 200 {
		t.Fatalf("update api card: %d %s", rec.Code, rec.Body.String())
	}
	card, err := story.LoadSharedCard(h.progDir, id)
	if err != nil {
		t.Fatal(err)
	}
	if card.API.APIKey != "secret" || card.API.Model != "new-model" {
		t.Fatalf("api secret or fields changed incorrectly: %#v", card.API)
	}
}

func TestSharedCardEveryPayloadTypeIsEditable(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		entityID string
		body     string
		contains string
	}{
		{name: "story", kind: "story", body: `{"name":"Story","story":{"type":"mystery","title":"New title","target_words_per_chapter":3200}}`, contains: `"New title"`},
		{name: "style", kind: "style", body: `{"name":"Style","style":{"writing_style":"lyrical marker","writing_pov":"first person"}}`, contains: `"lyrical marker"`},
		{name: "worldview", kind: "worldview", entityID: "w_1", body: `{"name":"World","worldview":{"name":"Magic revised","category":"rule","description":"New rule marker","tags":"magic"}}`, contains: `"New rule marker"`},
		{name: "organization", kind: "organization", entityID: "o_1", body: `{"name":"Org","organization":{"name":"Guild revised","type":"company","description":"New group marker","members":["c_1"]}}`, contains: `"New group marker"`},
		{name: "relation", kind: "relation", entityID: "r_1", body: `{"name":"Relation","relation":{"source_id":"c_1","source_type":"character","target_id":"w_1","target_type":"worldview","label":"studies marker"}}`, contains: `"studies marker"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := sharedCardTestHandler(t)
			createBody := `{"kind":"` + test.kind + `"`
			if test.entityID != "" {
				createBody += `,"entity_id":"` + test.entityID + `"`
			}
			createBody += `}`
			id := createSharedCard(t, h, createBody)
			req := httptest.NewRequest("PUT", "/api/shared-cards/"+id, strings.NewReader(test.body))
			req.SetPathValue("id", id)
			rec := httptest.NewRecorder()
			h.PutSharedCard(rec, req)
			if rec.Code != 200 {
				t.Fatalf("update %s card: %d %s", test.kind, rec.Code, rec.Body.String())
			}
			card, err := story.LoadSharedCard(h.progDir, id)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(card)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(data, []byte(test.contains)) {
				t.Fatalf("updated %s payload missing marker: %s", test.kind, data)
			}
		})
	}
}
