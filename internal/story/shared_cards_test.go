package story

import (
	"os"
	"path/filepath"
	"testing"

	"showmethestory/internal/config"
)

func sharedCardFixture(t *testing.T, kind, id string) (*SharedCard, *config.APIConfig, *config.Config, *ProjectSettings) {
	t.Helper()
	api := &config.APIConfig{BaseURL: "https://example.test", APIKey: "secret", Model: "model"}
	cfg := config.DefaultConfig()
	cfg.Story = config.StoryConfig{Type: "fantasy", Title: "Book", TargetWordsPerChapter: 2000, WritingStyle: "concise", WritingPOV: "third person"}
	settings := &ProjectSettings{
		Characters:    []Character{{ID: "c_1", Name: "Alice"}, {ID: "c_2", Name: "Bob"}},
		Worldview:     []WorldviewEntry{{ID: "w_1", Category: "knowledge", Name: "Magic", Description: "Rules"}},
		Organizations: []Organization{{ID: "o_1", Name: "Guild", Members: []string{"c_1", "c_2"}}},
		Relations:     []Relation{{ID: "r_1", SourceID: "c_1", SourceType: "character", TargetID: "o_1", TargetType: "organization", Label: "member"}},
	}
	card, err := CaptureSharedCard(SharedCardCaptureRequest{Kind: kind, EntityID: id}, "source", "zh", api, cfg, settings)
	if err != nil {
		t.Fatal(err)
	}
	return card, api, cfg, settings
}

func TestCaptureRelationCardIncludesEndpointDependencies(t *testing.T) {
	card, _, _, _ := sharedCardFixture(t, SharedCardRelation, "r_1")
	if card.Relation == nil || len(card.Dependencies.Organizations) != 1 || len(card.Dependencies.Characters) != 2 {
		t.Fatalf("relation dependencies missing: %#v", card)
	}
	if err := ValidateSharedCard(card); err != nil {
		t.Fatal(err)
	}
}

func TestImportRelationCardReusesExistingAndCreatesMissingDependencies(t *testing.T) {
	card, api, cfg, _ := sharedCardFixture(t, SharedCardRelation, "r_1")
	target := &ProjectSettings{Characters: []Character{{ID: "c_9", Name: "Alice"}}}
	_, _, next, result, err := ImportSharedCard(card, api, cfg, target, &Progress{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Reused[SharedCardCharacter] != 1 || result.Created[SharedCardCharacter] != 1 || result.Created[SharedCardOrganization] != 1 || result.Created[SharedCardRelation] != 1 {
		t.Fatalf("unexpected import result: %#v", result)
	}
	if len(next.Relations) != 1 || next.Relations[0].SourceID != "c_9" || next.Relations[0].TargetID != next.Organizations[0].ID {
		t.Fatalf("relation was not remapped: %#v", next)
	}
	if len(next.Organizations[0].Members) != 2 {
		t.Fatalf("members not imported: %#v", next.Organizations[0])
	}
}

func TestImportRelationCardAddsMissingMembersToReusedOrganization(t *testing.T) {
	card, api, cfg, _ := sharedCardFixture(t, SharedCardRelation, "r_1")
	target := &ProjectSettings{
		Characters:    []Character{{ID: "c_9", Name: "Alice"}},
		Organizations: []Organization{{ID: "o_9", Name: "Guild", Members: []string{"c_9"}}},
	}
	_, _, next, result, err := ImportSharedCard(card, api, cfg, target, &Progress{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Reused[SharedCardOrganization] != 1 || result.Created[SharedCardCharacter] != 1 {
		t.Fatalf("unexpected dependency result: %#v", result)
	}
	if len(next.Organizations) != 1 || len(next.Organizations[0].Members) != 2 {
		t.Fatalf("missing imported member on reused organization: %#v", next.Organizations)
	}
}

func TestImportStyleCardRequiresConfirmationForAcceptedProse(t *testing.T) {
	card, api, cfg, settings := sharedCardFixture(t, SharedCardStyle, "")
	cfg.Story.WritingStyle = "old"
	state := &Progress{Chapters: []ChapterState{{Num: 1, Status: StatusAccepted}}}
	if _, _, _, _, err := ImportSharedCard(card, api, cfg, settings, state, false); err == nil {
		t.Fatal("style import accepted without confirmation")
	}
	_, next, _, result, err := ImportSharedCard(card, api, cfg, settings, state, true)
	if err != nil {
		t.Fatal(err)
	}
	if next.Story.WritingStyle != "concise" || !result.RequiresReconciliation {
		t.Fatalf("style import incomplete: %#v %#v", next.Story, result)
	}
}

func TestImportStoryCardMarksTypeChangeForReconciliation(t *testing.T) {
	card, api, cfg, settings := sharedCardFixture(t, SharedCardStory, "")
	cfg.Story.Type = "mystery"
	state := &Progress{Chapters: []ChapterState{{Num: 1, Status: StatusAccepted}}}
	_, next, _, result, err := ImportSharedCard(card, api, cfg, settings, state, true)
	if err != nil {
		t.Fatal(err)
	}
	if next.Story.Type != "fantasy" || !result.RequiresReconciliation {
		t.Fatalf("story import did not request reconciliation: %#v %#v", next.Story, result)
	}
}

func TestSharedCardStorageSkipsInvalidFiles(t *testing.T) {
	card, _, _, _ := sharedCardFixture(t, SharedCardCharacter, "c_1")
	dir := t.TempDir()
	if err := SaveSharedCard(dir, card); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(SharedCardsDir(dir), "bad.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	items, err := ListSharedCards(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != card.ID {
		t.Fatalf("unexpected cards: %#v", items)
	}
}
