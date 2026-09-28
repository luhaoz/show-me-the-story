package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigEnablesMatchingAdultFictionSkill(t *testing.T) {
	tests := []struct {
		lang     string
		enabled  string
		disabled string
	}{
		{lang: "zh", enabled: "adult-fiction-zh", disabled: "adult-fiction-en"},
		{lang: "en", enabled: "adult-fiction-en", disabled: "adult-fiction-zh"},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			cfg := DefaultConfigForLang(tt.lang)
			if !cfg.SkillConfig.EnabledSkills[tt.enabled] {
				t.Fatalf("expected %s to be enabled for %s", tt.enabled, tt.lang)
			}
			if cfg.SkillConfig.EnabledSkills[tt.disabled] {
				t.Fatalf("did not expect %s to be enabled for %s", tt.disabled, tt.lang)
			}
		})
	}
}

func TestLoadConfigDoesNotEnableAdultSkillForExistingProject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"project_format_version":4,"language":"zh","story":{"target_words_per_chapter":5000},"prompts":{}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SkillConfig == nil {
		t.Fatal("expected missing skill_config to be initialized")
	}
	if cfg.SkillConfig.EnabledSkills["adult-fiction-zh"] || cfg.SkillConfig.EnabledSkills["adult-fiction-en"] {
		t.Fatal("existing project must not receive newly enabled skills while loading")
	}
}
