package projectstage

import (
	"testing"

	"vibebiz/internal/model"
)

func TestInitialStages(t *testing.T) {
	stages := InitialStages("p1")
	if len(stages) != 7 {
		t.Fatalf("stages = %d", len(stages))
	}
	if stages[0].StageKey != "requirements" || stages[0].Status != model.ProjectStageActive {
		t.Fatalf("first stage = %+v", stages[0])
	}
	if stages[1].Status != model.ProjectStageLocked || stages[4].Status != model.ProjectStageLocked {
		t.Fatalf("open stages = %+v", stages[:5])
	}
	if stages[5].Status != model.ProjectStagePlaceholder || stages[6].Status != model.ProjectStagePlaceholder {
		t.Fatalf("placeholder stages = %+v", stages[5:])
	}
	if stages[0].ArtifactPath != "docs/requirements.md" {
		t.Fatalf("artifact = %q", stages[0].ArtifactPath)
	}
}

func TestDefinitionFor(t *testing.T) {
	definition, ok := DefinitionFor("test_plan")
	if !ok || definition.RoleCode != "qa" || definition.ArtifactPath != "docs/test-plan.md" {
		t.Fatalf("definition = %+v ok=%v", definition, ok)
	}
	if _, ok := DefinitionFor("unknown"); ok {
		t.Fatal("unknown stage should not exist")
	}
}
