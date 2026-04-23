package pipeline

import "testing"

func TestRevertStatusForCoversEveryStage(t *testing.T) {
	for ready, stage := range Stages {
		got, ok := RevertStatusFor(stage.InProgressStatus)
		if !ok {
			t.Fatalf("expected RevertStatusFor(%q) to return ok for stage %q", stage.InProgressStatus, ready)
		}
		if got != ready {
			t.Fatalf("RevertStatusFor(%q) = %q, want %q", stage.InProgressStatus, got, ready)
		}
	}
}

func TestRevertStatusForClassifier(t *testing.T) {
	got, ok := RevertStatusFor(ClassifierStage.InProgressStatus)
	if !ok {
		t.Fatalf("expected RevertStatusFor(%q) to return ok", ClassifierStage.InProgressStatus)
	}
	if got != "backlog" {
		t.Fatalf("RevertStatusFor(%q) = %q, want %q", ClassifierStage.InProgressStatus, got, "backlog")
	}
}

func TestRevertStatusForUnknown(t *testing.T) {
	cases := []string{"backlog", "done", "ready_dev", "blocked", ""}
	for _, status := range cases {
		if got, ok := RevertStatusFor(status); ok {
			t.Fatalf("RevertStatusFor(%q) returned (%q, true); expected (\"\", false)", status, got)
		}
	}
}
