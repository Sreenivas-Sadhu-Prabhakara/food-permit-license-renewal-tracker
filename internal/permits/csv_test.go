package permits

import (
	"encoding/csv"
	"strings"
	"testing"
)

func TestWriteRegisterCSVRoundTrips(t *testing.T) {
	register := phRegister(t)
	if err := register.SetChecklistItem(KindMayorsPermit, "", 0, true); err != nil {
		t.Fatalf("SetChecklistItem: %v", err)
	}
	mayors, _ := register.Find(KindMayorsPermit, "")
	mayors.DocRef = "permits/2026/mayors-permit.pdf"
	mayors.Notes = "Paid in full, no instalments"
	if _, err := register.Put(mayors); err != nil {
		t.Fatalf("Put: %v", err)
	}

	var out strings.Builder
	if err := WriteRegisterCSV(&out, register); err != nil {
		t.Fatalf("WriteRegisterCSV: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatalf("the export is not valid CSV: %v", err)
	}
	if len(records) != len(register.Permits)+1 {
		t.Fatalf("%d lines for %d permits plus a header", len(records), len(register.Permits))
	}
	for i, row := range records {
		if len(row) != len(registerHeader) {
			t.Fatalf("line %d has %d columns, want %d", i+1, len(row), len(registerHeader))
		}
	}

	back, warnings, err := ReadRegisterCSV(strings.NewReader(out.String()))
	if err != nil {
		t.Fatalf("ReadRegisterCSV: %v (warnings %v)", err, warnings)
	}
	if len(warnings) != 0 {
		t.Fatalf("a clean round trip produced warnings: %v", warnings)
	}
	if len(back) != len(register.Permits) {
		t.Fatalf("read back %d permits, want %d", len(back), len(register.Permits))
	}

	byKind := map[Kind]Permit{}
	for _, p := range back {
		byKind[p.Kind] = p
	}
	original, _ := register.Find(KindMayorsPermit, "")
	got := byKind[KindMayorsPermit]
	for _, c := range []struct {
		field     string
		got, want string
	}{
		{"label", got.Label, original.Label},
		{"authority", got.Authority, original.Authority},
		{"number", got.Number, original.Number},
		{"issued", got.IssuedOn.String(), original.IssuedOn.String()},
		{"expires", got.ExpiresOn.String(), original.ExpiresOn.String()},
		{"document reference", got.DocRef, original.DocRef},
		{"notes", got.Notes, original.Notes},
		{"market", string(got.Jurisdiction), string(original.Jurisdiction)},
	} {
		if c.got != c.want {
			t.Errorf("%s came back as %q, want %q", c.field, c.got, c.want)
		}
	}
	if got.Blocks != original.Blocks || got.NoExpiry != original.NoExpiry || got.SeasonAnchor != original.SeasonAnchor {
		t.Errorf("the flags did not survive the round trip: %+v", got)
	}
	if got.LeadDays != original.LeadDays || got.ValidityMonths != original.ValidityMonths {
		t.Errorf("the figures did not survive: lead %d/%d, validity %d/%d",
			got.LeadDays, original.LeadDays, got.ValidityMonths, original.ValidityMonths)
	}
	// Ticks do not survive, by design, but the shipped checklist comes back so
	// the owner can work through it again.
	if len(got.Checklist) != len(original.Checklist) {
		t.Errorf("the checklist came back with %d lines, want %d", len(got.Checklist), len(original.Checklist))
	}
	for _, item := range got.Checklist {
		if item.Done {
			t.Error("a reimported checklist must come back unticked")
		}
	}
	if len(got.Prereqs) != len(original.Prereqs) {
		t.Errorf("prerequisites came back as %+v, want %d", got.Prereqs, len(original.Prereqs))
	}

	// The no-expiry registration must come back as no-expiry, not as a permit
	// that lapsed in the year nought.
	bir := byKind[KindBIRRegistration]
	if !bir.NoExpiry || !bir.ExpiresOn.Zero() {
		t.Errorf("the BIR registration came back as %+v, want no expiry", bir)
	}
}

func TestReadRegisterCSVWarnsRatherThanRefusingTheWholeFile(t *testing.T) {
	file := strings.Join([]string{
		strings.Join(registerHeader, ","),
		`,PH,mayors-permit,Mayor's permit,BPLO,MP-1,2026-01-20,2026-12-31,no,yes,12,10,yes,,,,0,9`,
		`,PH,sanitary-permit,Sanitary permit,CHO,SP-1,2026-01-20,2026-02-30,no,yes,12,10,yes,,,,0,5`,
		`,PH,,A row with no kind,Nobody,,,,no,no,0,0,no,,,,0,0`,
		`,PH,fsic,Fire certificate,BFP,F-1,2027-12-31,2026-12-31,no,yes,12,10,yes,,,,0,6`,
		``,
		`,ZZ,barangay-clearance,Barangay clearance,Barangay,B-1,2026-01-05,2026-12-31,no,yes,12,5,yes,,,,0,4`,
	}, "\n")

	permits, warnings, err := ReadRegisterCSV(strings.NewReader(file))
	if err != nil {
		t.Fatalf("ReadRegisterCSV refused the whole file: %v", err)
	}
	if len(permits) != 3 {
		t.Fatalf("read %d permits, want 3 (the mayor's permit, the sanitary permit with its bad date blanked, and the barangay clearance)", len(permits))
	}
	if len(warnings) < 3 {
		t.Fatalf("warnings = %v, want one each for the impossible date, the missing kind, the bad market and the expiry before issue", warnings)
	}
	joined := strings.Join(warnings, " | ")
	for _, want := range []string{"2026-02-30", "line 4", "ZZ", "Fire certificate"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings %q do not mention %q", joined, want)
		}
	}

	byKind := map[Kind]Permit{}
	for _, p := range permits {
		byKind[p.Kind] = p
	}
	// The impossible date is blanked, not invented, and the row is kept.
	if sanitary, ok := byKind[KindSanitaryPermit]; !ok || !sanitary.ExpiresOn.Zero() {
		t.Errorf("the sanitary permit came back as %+v, want it kept with no expiry date", sanitary)
	}
	// An unknown market is filed under Other rather than dropped.
	if barangay, ok := byKind[KindBarangayClearance]; !ok || barangay.Jurisdiction != Other {
		t.Errorf("the barangay clearance came back as %+v, want it filed under Other", barangay)
	}
	// A row that expires before it was issued is refused outright, because
	// there is no safe repair for it.
	if _, ok := byKind[KindFSIC]; ok {
		t.Error("a permit that expires before it was issued must be refused, not guessed at")
	}
}

func TestReadRegisterCSVRefusesSomethingThatIsNotARegister(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"a header with no kind column", "name,address\nLola's,Quezon City\n"},
		{"a header only", strings.Join(registerHeader, ",") + "\n"},
		{"nothing readable", strings.Join(registerHeader, ",") + "\n,PH,,,,,,,,,,,,,,,,\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, err := ReadRegisterCSV(strings.NewReader(c.in)); err == nil {
				t.Fatalf("ReadRegisterCSV accepted %s", c.name)
			}
		})
	}
}

func TestReadRegisterCSVToleratesAReorderedSpreadsheet(t *testing.T) {
	// Somebody will open the export, move the columns about and save it.
	file := "kind,expires_on,permit,blocks\n" +
		"mayors-permit,2027-12-31,Mayor's permit,yes\n"
	permits, _, err := ReadRegisterCSV(strings.NewReader(file))
	if err != nil {
		t.Fatalf("ReadRegisterCSV: %v", err)
	}
	if len(permits) != 1 {
		t.Fatalf("read %d permits, want 1", len(permits))
	}
	if permits[0].ExpiresOn.String() != "2027-12-31" || !permits[0].Blocks {
		t.Fatalf("read back %+v", permits[0])
	}
	// Missing columns fall back to the preset rather than to nothing.
	if permits[0].Authority == "" {
		t.Error("a missing authority column should fall back to the preset's")
	}
}

func TestWriteSeasonCSVCarriesItsOwnProvenance(t *testing.T) {
	register := phRegister(t)
	season := register.PHSeason("", 2027, MustParseDate("2026-12-02"))

	var out strings.Builder
	if err := WriteSeasonCSV(&out, season); err != nil {
		t.Fatalf("WriteSeasonCSV: %v", err)
	}
	text := out.String()

	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("the season export is not valid CSV: %v", err)
	}

	// The sheet has to state the deadline, where it comes from, and that the
	// lead times are this tool's own convention — a printed sheet gets carried
	// about and separated from the screen that explained it.
	for _, want := range []string{"2027-01-20", "7160", "statute", "convention", "2026-12-02"} {
		if !strings.Contains(text, want) {
			t.Errorf("the season sheet does not mention %q", want)
		}
	}

	// The data rows must be in plan order.
	headerAt := -1
	for i, row := range records {
		if len(row) > 0 && row[0] == "order" {
			headerAt = i
			break
		}
	}
	if headerAt < 0 {
		t.Fatal("the season sheet has no column header")
	}
	data := records[headerAt+1:]
	if len(data) != len(season.Rows) {
		t.Fatalf("%d data rows for %d plan rows", len(data), len(season.Rows))
	}
	for i, row := range data {
		if row[0] != itoa(i+1) {
			t.Fatalf("data row %d carries order %q", i, row[0])
		}
		if len(row) != len(seasonHeader) {
			t.Fatalf("data row %d has %d columns, want %d", i, len(row), len(seasonHeader))
		}
	}
	// The last row is the mayor's permit, since everything waits on it.
	if !strings.Contains(data[len(data)-1][2], "Mayor") {
		t.Fatalf("the last row of the plan is %q, want the mayor's permit", data[len(data)-1][2])
	}
}

func TestWriteSeasonCSVOnAnEmptyPlan(t *testing.T) {
	register := New()
	season := register.PHSeason("", 2027, MustParseDate("2026-12-02"))
	var out strings.Builder
	if err := WriteSeasonCSV(&out, season); err != nil {
		t.Fatalf("WriteSeasonCSV on an empty plan: %v", err)
	}
	if !strings.Contains(out.String(), "the main premises") {
		t.Errorf("an unnamed site should read as the main premises; got %q", out.String())
	}
}

func TestYesNoAndTruthy(t *testing.T) {
	if yesNo(true) != "yes" || yesNo(false) != "no" {
		t.Fatal("yesNo is wrong")
	}
	for _, s := range []string{"yes", "YES", "y", "true", "1", " yes "} {
		if !truthy(s) {
			t.Errorf("truthy(%q) = false", s)
		}
	}
	for _, s := range []string{"no", "", "maybe", "0", "false"} {
		if truthy(s) {
			t.Errorf("truthy(%q) = true", s)
		}
	}
}

func TestNumberIsForgiving(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"12", 12},
		{" 12 ", 12},
		{"", 0},
		{"-4", 0},
		{"twelve", 0},
		{"12.5", 0},
	}
	for _, c := range cases {
		if got := number(c.in); got != c.want {
			t.Errorf("number(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
