package story

import (
	"showmethestory/internal/config"
	"strings"
	"testing"
)

func TestAdultFictionBuiltinSkills(t *testing.T) {
	byID := make(map[string]Skill)
	for _, skill := range LoadBuiltinSkills() {
		byID[skill.ID] = skill
	}

	wantScopes := []string{
		SkillScopeOutlineGenerate,
		SkillScopeOutlineRevise,
		SkillScopeChapterGenerate,
		SkillScopeChapterRevise,
		SkillScopeChapterPolish,
	}
	for _, tt := range []struct {
		id   string
		lang string
	}{
		{id: "adult-fiction-zh", lang: "zh"},
		{id: "adult-fiction-en", lang: "en"},
	} {
		skill, ok := byID[tt.id]
		if !ok {
			t.Fatalf("missing builtin skill %s", tt.id)
		}
		if skill.Lang != tt.lang {
			t.Fatalf("%s language = %q, want %q", tt.id, skill.Lang, tt.lang)
		}
		if strings.Join(skill.AppliesTo, ",") != strings.Join(wantScopes, ",") {
			t.Fatalf("%s scopes = %v, want %v", tt.id, skill.AppliesTo, wantScopes)
		}
	}

	zh := byID["adult-fiction-zh"].Content
	for _, invariant := range []string{
		"明确为成年人", "明确合意", "身体与生理反应", "触碰、动作、力度与节奏",
		"感官表达", "欲望、犹豫、主动与回应", "对话、呼吸、停顿与非语言反馈",
		"衣物、空间与环境互动", "高潮前后与关系余波", "场景过渡与事后照顾",
		"成文规则", "避雷清单",
	} {
		if !strings.Contains(zh, invariant) {
			t.Errorf("Chinese adult skill is missing required guidance %q", invariant)
		}
	}
}

func TestAdultFictionSkillsResolveOnlyForLanguageAndDeclaredScopes(t *testing.T) {
	skills := LoadBuiltinSkills()
	sc := &config.SkillConfig{EnabledSkills: map[string]bool{
		"adult-fiction-zh": true,
		"adult-fiction-en": true,
	}}

	for _, scope := range []string{
		SkillScopeOutlineGenerate,
		SkillScopeOutlineRevise,
		SkillScopeChapterGenerate,
		SkillScopeChapterRevise,
		SkillScopeChapterPolish,
	} {
		for _, tt := range []struct {
			lang string
			id   string
		}{
			{lang: "zh", id: "adult-fiction-zh"},
			{lang: "en", id: "adult-fiction-en"},
		} {
			got := ResolveSkills(skills, sc, scope, tt.lang)
			if !containsSkillID(got, tt.id) {
				t.Errorf("scope %s language %s did not resolve %s", scope, tt.lang, tt.id)
			}
		}
	}

	for _, scope := range []string{
		SkillScopeAssistantChat,
		SkillScopeChapterFactCheck,
		SkillScopeForeshadowPlan,
		SkillScopeBookDiagnose,
		SkillScopeBookRoadmap,
		SkillScopeBookExecute,
		SkillScopeImportAnalyze,
	} {
		got := ResolveSkills(skills, sc, scope, "zh")
		if containsSkillID(got, "adult-fiction-zh") {
			t.Errorf("adult skill unexpectedly resolved for %s", scope)
		}
	}
}

func containsSkillID(skills []Skill, id string) bool {
	for _, skill := range skills {
		if skill.ID == id {
			return true
		}
	}
	return false
}
