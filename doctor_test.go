package slate

import "testing"

func TestDoctor_CleanDB(t *testing.T) {
	store := tempDB(t)
	store.Create(ctx, CreateParams{Title: "Task 1"})
	store.Create(ctx, CreateParams{Title: "Task 2"})

	report, err := store.Doctor(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if report.HasIssues() {
		for _, d := range report.Diagnostics {
			if d.Level != DiagOK {
				t.Errorf("unexpected issue: %s: %s", d.Name, d.Message)
			}
		}
	}
}

func TestDoctor_Integrity(t *testing.T) {
	store := tempDB(t)

	report, err := store.Doctor(ctx)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, d := range report.Diagnostics {
		if d.Name == "integrity" && d.Level == DiagOK {
			found = true
		}
	}
	if !found {
		t.Error("integrity check not found or not OK")
	}
}
