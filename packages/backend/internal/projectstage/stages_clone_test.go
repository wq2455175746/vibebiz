package projectstage

import (
	"testing"

	"vibebiz/internal/model"
)

func TestCloneDefinitions(t *testing.T) {
	definitions := DefinitionsForMode(model.ProjectModeClone)
	if len(definitions) != 5 {
		t.Fatalf("definitions = %d, want 5", len(definitions))
	}
	keys := []string{"base_design", "architecture", "test", "development", "artifacts"}
	for index, key := range keys {
		if definitions[index].Key != key {
			t.Fatalf("definition[%d] = %q, want %q", index, definitions[index].Key, key)
		}
	}
	if definitions[2].Workspace != WorkspaceTest {
		t.Fatalf("test workspace = %q", definitions[2].Workspace)
	}
	if definitions[4].Workspace != WorkspaceArtifact {
		t.Fatalf("artifact workspace = %q", definitions[4].Workspace)
	}
	if _, ok := DefinitionForMode(model.ProjectModeClone, "delivery"); ok {
		t.Fatal("clone mode must not contain delivery")
	}
}

func TestCloneInitialStages(t *testing.T) {
	stages := InitialStagesForMode("p1", model.ProjectModeClone)
	if len(stages) != 5 {
		t.Fatalf("stages = %d, want 5", len(stages))
	}
	if stages[0].StageKey != "base_design" || stages[0].Status != model.ProjectStageActive {
		t.Fatalf("first stage = %+v", stages[0])
	}
	for _, stage := range stages[1:] {
		if stage.Status != model.ProjectStageLocked {
			t.Fatalf("stage %s = %q, want locked", stage.StageKey, stage.Status)
		}
	}
	if stages[0].ArtifactPath != "docs/clone/base-design.md" {
		t.Fatalf("artifact = %q", stages[0].ArtifactPath)
	}
}

func TestEnterpriseDefinitionsStayUnchanged(t *testing.T) {
	definitions := DefinitionsForMode(model.ProjectModeEnterprise)
	if len(definitions) != 7 || definitions[0].Key != "requirements" || definitions[6].Key != "delivery" {
		t.Fatalf("definitions = %+v", definitions)
	}
}
