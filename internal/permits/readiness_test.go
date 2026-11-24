package permits

import "testing"

func TestReadinessOf(t *testing.T) {
	cases := []struct {
		name            string
		done            []bool
		docRef          string
		wantDone        int
		wantTotal       int
		wantPercent     int
		wantComplete    bool
		wantOutstanding int
	}{
		{
			name: "no checklist at all shows a dash, not nought per cent",
			done: nil, wantDone: 0, wantTotal: 0, wantPercent: 0, wantComplete: false, wantOutstanding: 0,
		},
		{
			name: "nothing ticked",
			done: []bool{false, false, false}, wantDone: 0, wantTotal: 3, wantPercent: 0, wantOutstanding: 3,
		},
		{
			name: "one of three rounds to 33",
			done: []bool{true, false, false}, wantDone: 1, wantTotal: 3, wantPercent: 33, wantOutstanding: 2,
		},
		{
			name: "two of three rounds to 67",
			done: []bool{true, true, false}, wantDone: 2, wantTotal: 3, wantPercent: 67, wantOutstanding: 1,
		},
		{
			name: "one of two is exactly half",
			done: []bool{true, false}, wantDone: 1, wantTotal: 2, wantPercent: 50, wantOutstanding: 1,
		},
		{
			name:     "one of six rounds to 17",
			done:     []bool{true, false, false, false, false, false},
			wantDone: 1, wantTotal: 6, wantPercent: 17, wantOutstanding: 5,
		},
		{
			name: "everything ticked",
			done: []bool{true, true}, wantDone: 2, wantTotal: 2, wantPercent: 100,
			wantComplete: true, wantOutstanding: 0,
		},
		{
			name: "one line, ticked",
			done: []bool{true}, wantDone: 1, wantTotal: 1, wantPercent: 100,
			wantComplete: true, wantOutstanding: 0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			permit := Permit{Kind: "test", Label: "Test", DocRef: c.docRef}
			for i, done := range c.done {
				permit.Checklist = append(permit.Checklist, ChecklistItem{
					Text: string(rune('a' + i)), Done: done,
				})
			}
			got := ReadinessOf(permit)
			if got.Done != c.wantDone {
				t.Errorf("done = %d, want %d", got.Done, c.wantDone)
			}
			if got.Total != c.wantTotal {
				t.Errorf("total = %d, want %d", got.Total, c.wantTotal)
			}
			if got.Percent != c.wantPercent {
				t.Errorf("percent = %d, want %d", got.Percent, c.wantPercent)
			}
			if got.Complete != c.wantComplete {
				t.Errorf("complete = %v, want %v", got.Complete, c.wantComplete)
			}
			if len(got.Outstanding) != c.wantOutstanding {
				t.Errorf("outstanding = %v, want %d lines", got.Outstanding, c.wantOutstanding)
			}
		})
	}
}

func TestReadinessOutstandingKeepsChecklistOrder(t *testing.T) {
	permit := Permit{Kind: "test", Label: "Test", Checklist: []ChecklistItem{
		{Text: "first", Done: true},
		{Text: "second"},
		{Text: "third", Done: true},
		{Text: "fourth"},
	}}
	got := ReadinessOf(permit)
	want := []string{"second", "fourth"}
	if len(got.Outstanding) != len(want) {
		t.Fatalf("outstanding = %v, want %v", got.Outstanding, want)
	}
	for i := range want {
		if got.Outstanding[i] != want[i] {
			t.Fatalf("outstanding = %v, want %v in checklist order", got.Outstanding, want)
		}
	}
}

func TestReadinessReportsWhetherTheOwnerSaidWhereTheCertificateIs(t *testing.T) {
	if ReadinessOf(Permit{Kind: "test", Label: "Test"}).HasDocRef {
		t.Error("with no document reference HasDocRef must be false")
	}
	withRef := ReadinessOf(Permit{Kind: "test", Label: "Test", DocRef: "permits/2027/sanitary.pdf"})
	if !withRef.HasDocRef {
		t.Error("with a document reference HasDocRef must be true")
	}
}

func TestReadinessOutstandingIsNeverNil(t *testing.T) {
	// The interface iterates it, and a nil slice marshals to null rather than
	// to an empty array, which breaks a for-of in the browser.
	if ReadinessOf(Permit{Kind: "test", Label: "Test"}).Outstanding == nil {
		t.Fatal("Outstanding must be an empty slice, not nil")
	}
}
