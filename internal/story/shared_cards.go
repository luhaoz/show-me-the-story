package story

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"showmethestory/internal/config"
	"showmethestory/internal/fsutil"
)

const SharedCardSchemaVersion = 1

const (
	SharedCardAPI          = "api"
	SharedCardStory        = "story"
	SharedCardStyle        = "style"
	SharedCardCharacter    = "character"
	SharedCardWorldview    = "worldview"
	SharedCardOrganization = "organization"
	SharedCardRelation     = "relation"
)

type SharedStoryCard struct {
	Type                  string `json:"type"`
	Title                 string `json:"title"`
	TargetWordsPerChapter int    `json:"target_words_per_chapter"`
}

type SharedStyleCard struct {
	WritingStyle string `json:"writing_style"`
	WritingPOV   string `json:"writing_pov"`
}

type SharedCardDependencies struct {
	Characters    []Character      `json:"characters,omitempty"`
	Worldview     []WorldviewEntry `json:"worldview,omitempty"`
	Organizations []Organization   `json:"organizations,omitempty"`
}

type SharedCard struct {
	SchemaVersion  int                    `json:"schema_version"`
	ID             string                 `json:"id"`
	Kind           string                 `json:"kind"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description,omitempty"`
	SourceProject  string                 `json:"source_project,omitempty"`
	SourceLanguage string                 `json:"source_language,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	API            *config.APIConfig      `json:"api,omitempty"`
	Story          *SharedStoryCard       `json:"story,omitempty"`
	Style          *SharedStyleCard       `json:"style,omitempty"`
	Character      *Character             `json:"character,omitempty"`
	Worldview      *WorldviewEntry        `json:"worldview,omitempty"`
	Organization   *Organization          `json:"organization,omitempty"`
	Relation       *Relation              `json:"relation,omitempty"`
	Dependencies   SharedCardDependencies `json:"dependencies,omitempty"`
}

type SharedCardCaptureRequest struct {
	Kind        string `json:"kind"`
	EntityID    string `json:"entity_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type SharedCardImportResult struct {
	RootID                 string         `json:"root_id,omitempty"`
	Created                map[string]int `json:"created"`
	Reused                 map[string]int `json:"reused"`
	RequiresReconciliation bool           `json:"requires_reconciliation,omitempty"`
}

func SharedCardsDir(progDir string) string { return filepath.Join(progDir, "shared-cards") }

func newSharedCardID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func validSharedCardID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func sharedCardPath(progDir, id string) (string, error) {
	if !validSharedCardID(id) {
		return "", errors.New("invalid shared card id")
	}
	return filepath.Join(SharedCardsDir(progDir), id+".json"), nil
}

func cloneCardValue[T any](value T) T {
	data, _ := json.Marshal(value)
	var out T
	_ = json.Unmarshal(data, &out)
	return out
}

func CaptureSharedCard(req SharedCardCaptureRequest, project, language string, api *config.APIConfig, cfg *config.Config, settings *ProjectSettings) (*SharedCard, error) {
	id, err := newSharedCardID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	card := &SharedCard{SchemaVersion: SharedCardSchemaVersion, ID: id, Kind: req.Kind, Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), SourceProject: project, SourceLanguage: language, CreatedAt: now, UpdatedAt: now}
	switch req.Kind {
	case SharedCardAPI:
		card.API = ptr(cloneCardValue(*api))
		if card.Name == "" {
			card.Name = api.Model
			if card.Name == "" {
				card.Name = "API"
			}
		}
	case SharedCardStory:
		card.Story = &SharedStoryCard{Type: cfg.Story.Type, Title: cfg.Story.Title, TargetWordsPerChapter: cfg.Story.TargetWordsPerChapter}
		if card.Name == "" {
			card.Name = cfg.Story.Title
			if card.Name == "" {
				card.Name = cfg.Story.Type
				if card.Name == "" {
					card.Name = "Story"
				}
			}
		}
	case SharedCardStyle:
		card.Style = &SharedStyleCard{WritingStyle: cfg.Story.WritingStyle, WritingPOV: cfg.Story.WritingPOV}
		if card.Name == "" {
			card.Name = "Writing style"
		}
	case SharedCardCharacter:
		entity := findCharacter(settings.Characters, req.EntityID)
		if entity == nil {
			return nil, errors.New("character not found")
		}
		card.Character = ptr(cloneCardValue(*entity))
		if card.Name == "" {
			card.Name = entity.Name
		}
	case SharedCardWorldview:
		entity := findWorldview(settings.Worldview, req.EntityID)
		if entity == nil {
			return nil, errors.New("worldview not found")
		}
		card.Worldview = ptr(cloneCardValue(*entity))
		if card.Name == "" {
			card.Name = entity.Name
		}
	case SharedCardOrganization:
		entity := findOrganization(settings.Organizations, req.EntityID)
		if entity == nil {
			return nil, errors.New("organization not found")
		}
		card.Organization = ptr(cloneCardValue(*entity))
		if card.Name == "" {
			card.Name = entity.Name
		}
		for _, member := range entity.Members {
			character := findCharacter(settings.Characters, member)
			if character == nil {
				return nil, fmt.Errorf("organization member %s not found", member)
			}
			card.Dependencies.Characters = append(card.Dependencies.Characters, cloneCardValue(*character))
		}
	case SharedCardRelation:
		entity := findRelation(settings.Relations, req.EntityID)
		if entity == nil {
			return nil, errors.New("relation not found")
		}
		card.Relation = ptr(cloneCardValue(*entity))
		if card.Name == "" {
			card.Name = entity.Label
		}
		if err := captureRelationEndpoint(card, settings, entity.SourceType, entity.SourceID); err != nil {
			return nil, err
		}
		if err := captureRelationEndpoint(card, settings, entity.TargetType, entity.TargetID); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported shared card kind")
	}
	card.Name = truncateSharedCardName(strings.TrimSpace(card.Name), 100)
	dedupeDependencies(&card.Dependencies)
	if err := ValidateSharedCard(card); err != nil {
		return nil, err
	}
	return card, nil
}

func ptr[T any](value T) *T { return &value }
func truncateSharedCardName(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func captureRelationEndpoint(card *SharedCard, settings *ProjectSettings, kind, id string) error {
	switch kind {
	case SharedCardCharacter:
		entity := findCharacter(settings.Characters, id)
		if entity == nil {
			return fmt.Errorf("relation character %s not found", id)
		}
		card.Dependencies.Characters = append(card.Dependencies.Characters, cloneCardValue(*entity))
	case SharedCardWorldview:
		entity := findWorldview(settings.Worldview, id)
		if entity == nil {
			return fmt.Errorf("relation worldview %s not found", id)
		}
		card.Dependencies.Worldview = append(card.Dependencies.Worldview, cloneCardValue(*entity))
	case SharedCardOrganization:
		entity := findOrganization(settings.Organizations, id)
		if entity == nil {
			return fmt.Errorf("relation organization %s not found", id)
		}
		card.Dependencies.Organizations = append(card.Dependencies.Organizations, cloneCardValue(*entity))
		for _, member := range entity.Members {
			character := findCharacter(settings.Characters, member)
			if character == nil {
				return fmt.Errorf("organization member %s not found", member)
			}
			card.Dependencies.Characters = append(card.Dependencies.Characters, cloneCardValue(*character))
		}
	default:
		return fmt.Errorf("unsupported relation endpoint type %s", kind)
	}
	return nil
}

func dedupeDependencies(deps *SharedCardDependencies) {
	chars, worlds, orgs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	filterCharacters := deps.Characters[:0]
	for _, e := range deps.Characters {
		if !chars[e.ID] {
			chars[e.ID] = true
			filterCharacters = append(filterCharacters, e)
		}
	}
	deps.Characters = filterCharacters
	filterWorlds := deps.Worldview[:0]
	for _, e := range deps.Worldview {
		if !worlds[e.ID] {
			worlds[e.ID] = true
			filterWorlds = append(filterWorlds, e)
		}
	}
	deps.Worldview = filterWorlds
	filterOrgs := deps.Organizations[:0]
	for _, e := range deps.Organizations {
		if !orgs[e.ID] {
			orgs[e.ID] = true
			filterOrgs = append(filterOrgs, e)
		}
	}
	deps.Organizations = filterOrgs
}

func ValidateSharedCard(card *SharedCard) error {
	if card == nil || card.SchemaVersion != SharedCardSchemaVersion || !validSharedCardID(card.ID) {
		return errors.New("invalid shared card schema or id")
	}
	card.Name, card.Description = strings.TrimSpace(card.Name), strings.TrimSpace(card.Description)
	if card.Name == "" || len([]rune(card.Name)) > 100 || len([]rune(card.Description)) > 1000 {
		return errors.New("invalid shared card metadata")
	}
	payloads := 0
	for _, present := range []bool{card.API != nil, card.Story != nil, card.Style != nil, card.Character != nil, card.Worldview != nil, card.Organization != nil, card.Relation != nil} {
		if present {
			payloads++
		}
	}
	if payloads != 1 {
		return errors.New("shared card must contain exactly one root payload")
	}
	validKind := (card.Kind == SharedCardAPI && card.API != nil) || (card.Kind == SharedCardStory && card.Story != nil) || (card.Kind == SharedCardStyle && card.Style != nil) || (card.Kind == SharedCardCharacter && card.Character != nil) || (card.Kind == SharedCardWorldview && card.Worldview != nil) || (card.Kind == SharedCardOrganization && card.Organization != nil) || (card.Kind == SharedCardRelation && card.Relation != nil)
	if !validKind {
		return errors.New("shared card kind does not match payload")
	}
	if card.Character != nil && (card.Character.ID == "" || strings.TrimSpace(card.Character.Name) == "") {
		return errors.New("invalid character card")
	}
	if card.Worldview != nil && (card.Worldview.ID == "" || strings.TrimSpace(card.Worldview.Name) == "" || strings.TrimSpace(card.Worldview.Description) == "") {
		return errors.New("invalid worldview card")
	}
	if card.Organization != nil && (card.Organization.ID == "" || strings.TrimSpace(card.Organization.Name) == "") {
		return errors.New("invalid organization card")
	}
	if card.Relation != nil && card.Relation.ID == "" {
		return errors.New("invalid relation card")
	}
	if card.Story != nil && card.Story.TargetWordsPerChapter <= 0 {
		return errors.New("invalid story card")
	}
	ids, typed := map[string]bool{}, map[string]map[string]bool{SharedCardCharacter: {}, SharedCardWorldview: {}, SharedCardOrganization: {}}
	add := func(kind, id string) error {
		if id == "" || ids[id] {
			return errors.New("empty or duplicate dependency id")
		}
		ids[id], typed[kind][id] = true, true
		return nil
	}
	for _, e := range card.Dependencies.Characters {
		if strings.TrimSpace(e.Name) == "" || add(SharedCardCharacter, e.ID) != nil {
			return errors.New("invalid character dependency")
		}
	}
	for _, e := range card.Dependencies.Worldview {
		if strings.TrimSpace(e.Name) == "" || strings.TrimSpace(e.Description) == "" || add(SharedCardWorldview, e.ID) != nil {
			return errors.New("invalid worldview dependency")
		}
	}
	for _, e := range card.Dependencies.Organizations {
		if strings.TrimSpace(e.Name) == "" || add(SharedCardOrganization, e.ID) != nil {
			return errors.New("invalid organization dependency")
		}
		for _, member := range e.Members {
			if !typed[SharedCardCharacter][member] {
				return errors.New("organization dependency has missing member")
			}
		}
	}
	if card.Organization != nil {
		for _, member := range card.Organization.Members {
			if !typed[SharedCardCharacter][member] {
				return errors.New("organization card has missing member")
			}
		}
	}
	if card.Relation != nil {
		if strings.TrimSpace(card.Relation.Label) == "" || !typed[card.Relation.SourceType][card.Relation.SourceID] || !typed[card.Relation.TargetType][card.Relation.TargetID] {
			return errors.New("relation card has missing endpoint")
		}
		if card.Relation.SourceType == card.Relation.TargetType && card.Relation.SourceID == card.Relation.TargetID {
			return errors.New("relation card endpoints must be different")
		}
	}
	return nil
}

func SaveSharedCard(progDir string, card *SharedCard) error {
	if err := ValidateSharedCard(card); err != nil {
		return err
	}
	dir := SharedCardsDir(progDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		return err
	}
	path, _ := sharedCardPath(progDir, card.ID)
	if err := fsutil.WriteFileAtomic(path, data); err != nil {
		return err
	}
	_ = os.Chmod(path, 0600)
	return nil
}

func LoadSharedCard(progDir, id string) (*SharedCard, error) {
	path, err := sharedCardPath(progDir, id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var card SharedCard
	if err := json.Unmarshal(data, &card); err != nil {
		return nil, err
	}
	if card.ID != id {
		return nil, errors.New("shared card id does not match filename")
	}
	if err := ValidateSharedCard(&card); err != nil {
		return nil, err
	}
	return &card, nil
}

func ListSharedCards(progDir string) ([]SharedCard, error) {
	entries, err := os.ReadDir(SharedCardsDir(progDir))
	if os.IsNotExist(err) {
		return []SharedCard{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []SharedCard{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		card, err := LoadSharedCard(progDir, strings.TrimSuffix(entry.Name(), ".json"))
		if err == nil {
			out = append(out, *card)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func DeleteSharedCard(progDir, id string) error {
	path, err := sharedCardPath(progDir, id)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func ImportSharedCard(card *SharedCard, api *config.APIConfig, cfg *config.Config, settings *ProjectSettings, state *Progress, confirmWritten bool) (*config.APIConfig, *config.Config, *ProjectSettings, SharedCardImportResult, error) {
	if err := ValidateSharedCard(card); err != nil {
		return nil, nil, nil, SharedCardImportResult{}, err
	}
	nextAPI, nextConfig, nextSettings := cloneCardValue(*api), cloneCardValue(*cfg), cloneCardValue(*settings)
	result := SharedCardImportResult{Created: map[string]int{}, Reused: map[string]int{}}
	hasAccepted := false
	if state != nil {
		for _, ch := range state.Chapters {
			if ch.Status == StatusAccepted {
				hasAccepted = true
				break
			}
		}
	}
	if hasAccepted && (card.Kind == SharedCardStory || card.Kind == SharedCardStyle) && !confirmWritten {
		return nil, nil, nil, result, errors.New("written project confirmation required")
	}
	switch card.Kind {
	case SharedCardAPI:
		nextAPI = cloneCardValue(*card.API)
	case SharedCardStory:
		result.RequiresReconciliation = hasAccepted && nextConfig.Story.Type != card.Story.Type
		nextConfig.Story.Type, nextConfig.Story.Title, nextConfig.Story.TargetWordsPerChapter = card.Story.Type, card.Story.Title, card.Story.TargetWordsPerChapter
	case SharedCardStyle:
		result.RequiresReconciliation = hasAccepted && (nextConfig.Story.WritingStyle != card.Style.WritingStyle || nextConfig.Story.WritingPOV != card.Style.WritingPOV)
		nextConfig.Story.WritingStyle, nextConfig.Story.WritingPOV = card.Style.WritingStyle, card.Style.WritingPOV
	case SharedCardCharacter:
		entity := cloneCardValue(*card.Character)
		entity.ID = nextSettings.NextCharacterID()
		result.RootID = entity.ID
		nextSettings.Characters = append(nextSettings.Characters, entity)
		result.Created[SharedCardCharacter]++
	case SharedCardWorldview:
		entity := cloneCardValue(*card.Worldview)
		entity.ID = nextSettings.NextWorldviewID()
		result.RootID = entity.ID
		nextSettings.Worldview = append(nextSettings.Worldview, entity)
		result.Created[SharedCardWorldview]++
	case SharedCardOrganization, SharedCardRelation:
		mapping := importCardDependencies(&nextSettings, card.Dependencies, &result)
		if card.Kind == SharedCardOrganization {
			entity := cloneCardValue(*card.Organization)
			entity.ID = nextSettings.NextOrganizationID()
			for i, member := range entity.Members {
				entity.Members[i] = mapping[member]
			}
			result.RootID = entity.ID
			nextSettings.Organizations = append(nextSettings.Organizations, entity)
			result.Created[SharedCardOrganization]++
		} else {
			entity := cloneCardValue(*card.Relation)
			entity.SourceID, entity.TargetID = mapping[entity.SourceID], mapping[entity.TargetID]
			for _, existing := range nextSettings.Relations {
				if existing.SourceID == entity.SourceID && existing.SourceType == entity.SourceType && existing.TargetID == entity.TargetID && existing.TargetType == entity.TargetType && strings.EqualFold(strings.TrimSpace(existing.Label), strings.TrimSpace(entity.Label)) {
					result.RootID = existing.ID
					result.Reused[SharedCardRelation]++
					return &nextAPI, &nextConfig, &nextSettings, result, nil
				}
			}
			entity.ID = nextSettings.NextRelationID()
			result.RootID = entity.ID
			nextSettings.Relations = append(nextSettings.Relations, entity)
			result.Created[SharedCardRelation]++
		}
	}
	return &nextAPI, &nextConfig, &nextSettings, result, nil
}

func importCardDependencies(settings *ProjectSettings, deps SharedCardDependencies, result *SharedCardImportResult) map[string]string {
	mapping := map[string]string{}
	for _, source := range deps.Characters {
		if existing := findCharacterByName(settings.Characters, source.Name); existing != nil {
			mapping[source.ID] = existing.ID
			result.Reused[SharedCardCharacter]++
			continue
		}
		entity := cloneCardValue(source)
		entity.ID = settings.NextCharacterID()
		mapping[source.ID] = entity.ID
		settings.Characters = append(settings.Characters, entity)
		result.Created[SharedCardCharacter]++
	}
	for _, source := range deps.Worldview {
		if existing := findWorldviewByName(settings.Worldview, source.Name, source.Category); existing != nil {
			mapping[source.ID] = existing.ID
			result.Reused[SharedCardWorldview]++
			continue
		}
		entity := cloneCardValue(source)
		entity.ID = settings.NextWorldviewID()
		mapping[source.ID] = entity.ID
		settings.Worldview = append(settings.Worldview, entity)
		result.Created[SharedCardWorldview]++
	}
	for _, source := range deps.Organizations {
		if existing := findOrganizationByName(settings.Organizations, source.Name); existing != nil {
			mapping[source.ID] = existing.ID
			result.Reused[SharedCardOrganization]++
			members := map[string]bool{}
			for _, id := range existing.Members {
				members[id] = true
			}
			for _, sourceMember := range source.Members {
				if id := mapping[sourceMember]; id != "" && !members[id] {
					existing.Members = append(existing.Members, id)
					members[id] = true
				}
			}
			continue
		}
		entity := cloneCardValue(source)
		entity.ID = settings.NextOrganizationID()
		mapping[source.ID] = entity.ID
		for i, member := range entity.Members {
			entity.Members[i] = mapping[member]
		}
		settings.Organizations = append(settings.Organizations, entity)
		result.Created[SharedCardOrganization]++
	}
	return mapping
}

func findCharacter(items []Character, id string) *Character {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}
func findWorldview(items []WorldviewEntry, id string) *WorldviewEntry {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}
func findOrganization(items []Organization, id string) *Organization {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}
func findRelation(items []Relation, id string) *Relation {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}
func findCharacterByName(items []Character, name string) *Character {
	for i := range items {
		if sameCardName(items[i].Name, name) {
			return &items[i]
		}
	}
	return nil
}
func findWorldviewByName(items []WorldviewEntry, name, category string) *WorldviewEntry {
	for i := range items {
		if sameCardName(items[i].Name, name) && strings.EqualFold(strings.TrimSpace(items[i].Category), strings.TrimSpace(category)) {
			return &items[i]
		}
	}
	return nil
}
func findOrganizationByName(items []Organization, name string) *Organization {
	for i := range items {
		if sameCardName(items[i].Name, name) {
			return &items[i]
		}
	}
	return nil
}
func sameCardName(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
