package permits

import (
	"regexp"
	"strings"
	"testing"
)

func TestEveryPresetIsComplete(t *testing.T) {
	for _, preset := range AllPresets() {
		t.Run(string(preset.Kind), func(t *testing.T) {
			if _, err := ParseKind(string(preset.Kind)); err != nil {
				t.Fatalf("the kind is not a usable slug: %v", err)
			}
			if _, err := ParseJurisdiction(string(preset.Jurisdiction)); err != nil {
				t.Fatalf("the market is not one this tool knows: %v", err)
			}
			if preset.Label == "" {
				t.Error("no label")
			}
			// Naming the office is the whole point of a register across
			// authorities: "renew the sanitary permit" is useless without
			// "at the city health office".
			if preset.Authority == "" {
				t.Error("no authority named — the owner has to know which door to go to")
			}
			if preset.AppliesTo == "" {
				t.Error("no statement of who needs it, so an owner cannot tell whether to delete the row")
			}
			if preset.Note == "" {
				t.Error("no note, so nothing says how much the period varies locally")
			}
			if len(preset.Checklist) < 3 {
				t.Errorf("a renewal checklist of %d lines is not a checklist", len(preset.Checklist))
			}
			for i, item := range preset.Checklist {
				if strings.TrimSpace(item) == "" {
					t.Errorf("checklist line %d is empty", i+1)
				}
			}
			if preset.ValidityMonths < 0 || preset.ValidityMonths > MaxValidityMonths {
				t.Errorf("validity of %d months is out of range", preset.ValidityMonths)
			}
			if preset.LeadDays < 0 || preset.LeadDays > MaxLeadDays {
				t.Errorf("lead time of %d days is out of range", preset.LeadDays)
			}
			if preset.NoExpiry && preset.ValidityMonths != 0 {
				t.Error("a permit that does not expire cannot carry a validity period")
			}
			if preset.NoExpiry && preset.SeasonAnchor {
				t.Error("a permit that does not expire cannot fall due in a renewal season")
			}
			// Every entry has to survive being saved.
			register := New()
			if _, err := register.Put(preset.ToPermit()); err != nil {
				t.Fatalf("the preset will not save: %v", err)
			}
		})
	}
}

func TestEveryPresetWithNoValidityPeriodSaysWhy(t *testing.T) {
	// Shipping no period is a deliberate act of honesty, and the note has to
	// carry the reason or it just looks like a gap somebody forgot to fill.
	for _, preset := range AllPresets() {
		if preset.ValidityMonths != 0 || preset.NoExpiry {
			continue
		}
		lower := strings.ToLower(preset.Note)
		explained := strings.Contains(lower, "vary") || strings.Contains(lower, "varies") ||
			strings.Contains(lower, "no default period") || strings.Contains(lower, "no single period") ||
			strings.Contains(lower, "no default validity") || strings.Contains(lower, "conflict")
		if !explained {
			t.Errorf("%s ships no validity period but the note does not say why: %q", preset.Kind, preset.Note)
		}
	}
}

func TestEveryPresetThatDoesNotExpireSaysSo(t *testing.T) {
	for _, preset := range AllPresets() {
		if !preset.NoExpiry {
			continue
		}
		if !strings.Contains(strings.ToLower(preset.Note), "not expire") {
			t.Errorf("%s is marked as never expiring but the note does not say so: %q", preset.Kind, preset.Note)
		}
	}
}

func TestShippedPrerequisitesPointAtRealPermits(t *testing.T) {
	known := map[Kind]Preset{}
	for _, preset := range AllPresets() {
		known[preset.Kind] = preset
	}
	for _, preset := range AllPresets() {
		for _, dep := range preset.Prereqs {
			target, ok := known[dep.Kind]
			if !ok {
				t.Errorf("%s waits on %q, which is not a permit this tool ships", preset.Kind, dep.Kind)
				continue
			}
			// A prerequisite across markets would be nonsense: an Indian trade
			// licence cannot wait on a barangay clearance.
			if target.Jurisdiction != preset.Jurisdiction {
				t.Errorf("%s (%s) waits on %s (%s) — a prerequisite cannot cross markets",
					preset.Kind, preset.Jurisdiction, dep.Kind, target.Jurisdiction)
			}
			if dep.Kind == preset.Kind {
				t.Errorf("%s waits on itself", preset.Kind)
			}
			if _, err := ParseBasis(string(dep.Basis)); err != nil {
				t.Errorf("%s waits on %s with basis %q: %v", preset.Kind, dep.Kind, dep.Basis, err)
			}
			if dep.Basis != FromStatute && dep.Basis != FromConvention {
				t.Errorf("a shipped prerequisite must be either a statute or an admitted convention; %s -> %s is %q",
					preset.Kind, dep.Kind, dep.Basis)
			}
			if strings.TrimSpace(dep.Why) == "" {
				t.Errorf("%s waits on %s with no reason given", preset.Kind, dep.Kind)
			}
		}
	}
}

func TestEveryStatutoryPrerequisiteCitesAnInstrument(t *testing.T) {
	// A link marked "fixed by law" has to name the law, or the owner cannot
	// check it and should not trust it.
	actNumber := regexp.MustCompile(`(?i)(republic act|presidential decree|regulations?,? \d{4}|act,? \d{4})`)
	for _, preset := range AllPresets() {
		for _, dep := range preset.Prereqs {
			if dep.Basis != FromStatute {
				continue
			}
			if !actNumber.MatchString(dep.Why) {
				t.Errorf("%s -> %s is marked as fixed by law but cites nothing: %q", preset.Kind, dep.Kind, dep.Why)
			}
		}
	}
}

func TestEveryConventionalPrerequisiteInvitesTheOwnerToChangeIt(t *testing.T) {
	for _, preset := range AllPresets() {
		for _, dep := range preset.Prereqs {
			if dep.Basis != FromConvention {
				continue
			}
			lower := strings.ToLower(dep.Why)
			invites := strings.Contains(lower, "delete this link") || strings.Contains(lower, "reorder") ||
				strings.Contains(lower, "not asked for everywhere") || strings.Contains(lower, "if yours does not") ||
				strings.Contains(lower, "does not expire")
			if !invites {
				t.Errorf("%s -> %s is a convention but the reason does not tell the owner they may change it: %q",
					preset.Kind, dep.Kind, dep.Why)
			}
		}
	}
}

func TestTheShippedPresetsContainNoPrerequisiteLoop(t *testing.T) {
	// A loop in the shipped graph would make the season plan unorderable for
	// every user, so it is worth a test of its own rather than relying on the
	// season's loop reporting to catch it.
	edges := map[Kind][]Kind{}
	for _, preset := range AllPresets() {
		for _, dep := range preset.Prereqs {
			edges[preset.Kind] = append(edges[preset.Kind], dep.Kind)
		}
	}
	const (
		white = 0
		grey  = 1
		black = 2
	)
	colour := map[Kind]int{}
	var walk func(Kind, []Kind) []Kind
	walk = func(kind Kind, path []Kind) []Kind {
		switch colour[kind] {
		case grey:
			return append(path, kind)
		case black:
			return nil
		}
		colour[kind] = grey
		for _, dep := range edges[kind] {
			if loop := walk(dep, append(path, kind)); loop != nil {
				return loop
			}
		}
		colour[kind] = black
		return nil
	}
	for _, preset := range AllPresets() {
		if loop := walk(preset.Kind, nil); loop != nil {
			t.Fatalf("the shipped presets contain a prerequisite loop: %v", loop)
		}
	}
}

func TestThePresetsShipNoFeeFigure(t *testing.T) {
	// The brief is strict about this: no authoritative fee table anywhere. A
	// currency symbol next to a number in a preset would be exactly that.
	//
	// The one allowed mention is the Philippine annual registration fee that
	// RA 11976 abolished, because saying "the ₱500 fee was removed" is the
	// opposite of asserting a fee.
	money := regexp.MustCompile(`[₹₱]\s?[\d,]`)
	for _, preset := range AllPresets() {
		fields := append([]string{preset.Note, preset.Label, preset.Authority, preset.AppliesTo}, preset.Checklist...)
		for _, dep := range preset.Prereqs {
			fields = append(fields, dep.Why)
		}
		for _, text := range fields {
			for _, hit := range money.FindAllString(text, -1) {
				if strings.Contains(text, "were removed by the Ease of Paying Taxes Act") {
					continue
				}
				t.Errorf("%s quotes a fee (%q) in: %s", preset.Kind, hit, text)
			}
		}
	}
}

func TestTheIndiaPresetDoesNotRebuildTierDetermination(t *testing.T) {
	// Deciding which FSSAI tier a business needs is a different tool's job.
	// This one must point at it rather than reimplement it, and nothing in the
	// presets should read as a tier threshold.
	fssai, ok := PresetFor(KindFSSAI)
	if !ok {
		t.Fatal("the FSSAI preset is missing")
	}
	if !strings.Contains(fssai.Note, "does not try to answer it") {
		t.Errorf("the FSSAI note must disclaim tier determination; got %q", fssai.Note)
	}
	for _, phrase := range []string{"crore", "lakh", "metric tonne", "litres a day"} {
		for _, preset := range AllPresets() {
			if strings.Contains(strings.ToLower(preset.Note), phrase) {
				t.Errorf("%s quotes a tier threshold (%q); that belongs to the licence navigator", preset.Kind, phrase)
			}
		}
	}
}

func TestPresetsForEachMarket(t *testing.T) {
	cases := []struct {
		j    Jurisdiction
		want []Kind
	}{
		{India, []Kind{KindFSSAI, KindGST, KindTradeLicence, KindFireNOC}},
		{Philippines, []Kind{
			KindMayorsPermit, KindSanitaryPermit, KindBarangayClearance,
			KindBIRRegistration, KindFSIC, KindFDALTO,
		}},
	}
	for _, c := range cases {
		t.Run(c.j.Label(), func(t *testing.T) {
			got := map[Kind]bool{}
			for _, preset := range PresetsFor(c.j) {
				if preset.Jurisdiction != c.j {
					t.Fatalf("%s is in the %s list but belongs to %s", preset.Kind, c.j, preset.Jurisdiction)
				}
				got[preset.Kind] = true
			}
			// Every permit the brief names has to be in its market's list.
			for _, kind := range c.want {
				if !got[kind] {
					t.Errorf("%s is missing from the %s preset", kind, c.j.Label())
				}
			}
			if len(got) < 6 || len(got) > 8 {
				t.Errorf("the %s preset holds %d permits; six to eight was asked for", c.j.Label(), len(got))
			}
		})
	}
	if len(PresetsFor("ZZ")) != 0 {
		t.Error("there should be no preset for a market this tool does not know")
	}
}

func TestPresetFor(t *testing.T) {
	if _, ok := PresetFor(KindMayorsPermit); !ok {
		t.Error("the mayor's permit preset is missing")
	}
	if _, ok := PresetFor("invented-permit"); ok {
		t.Error("PresetFor found a preset for a kind that does not exist")
	}
}

func TestThePhilippineJanuarySetIsAnchoredToTheSeason(t *testing.T) {
	anchored := map[Kind]bool{}
	for _, preset := range PresetsFor(Philippines) {
		if preset.SeasonAnchor {
			anchored[preset.Kind] = true
		}
	}
	for _, kind := range []Kind{
		KindBarangayClearance, KindSanitaryPermit, KindHealthCertificate, KindFSIC, KindMayorsPermit,
	} {
		if !anchored[kind] {
			t.Errorf("%s renews with the January rush and should be anchored to the season", kind)
		}
	}
	// A DTI business name runs on its own five-year clock, not with January.
	for _, kind := range []Kind{KindBusinessName, KindFDALTO, KindBIRRegistration} {
		if anchored[kind] {
			t.Errorf("%s runs on its own cycle and must not be anchored to January", kind)
		}
	}
	// Nothing in the India preset is anchored, because India has no season.
	for _, preset := range PresetsFor(India) {
		if preset.SeasonAnchor {
			t.Errorf("%s is anchored to a season, but India has no single renewal date", preset.Kind)
		}
	}
}

func TestWhatBlocksTradingIsDeliberate(t *testing.T) {
	// A permit marked as blocking says that trading without it is unlawful.
	// That is a strong claim, so the set is pinned here: a change to it should
	// be a change somebody meant to make.
	wantBlocking := map[Kind]bool{
		KindFSSAI:             true,
		KindGST:               true,
		KindTradeLicence:      true,
		KindShopsEstab:        false,
		KindFireNOC:           false,
		KindEatingHouse:       false,
		KindLegalMetrology:    false,
		KindPollutionNOC:      false,
		KindBarangayClearance: true,
		KindHealthCertificate: false,
		KindSanitaryPermit:    true,
		KindFSIC:              true,
		KindMayorsPermit:      true,
		KindBusinessName:      true,
		KindBIRRegistration:   true,
		KindFDALTO:            true,
	}
	for _, preset := range AllPresets() {
		want, listed := wantBlocking[preset.Kind]
		if !listed {
			t.Errorf("%s is not in the blocking list; decide deliberately whether it stops trading", preset.Kind)
			continue
		}
		if preset.Blocks != want {
			t.Errorf("%s blocks = %v, want %v", preset.Kind, preset.Blocks, want)
		}
	}
	if len(wantBlocking) != len(AllPresets()) {
		t.Errorf("the blocking list has %d entries for %d presets", len(wantBlocking), len(AllPresets()))
	}
}

func TestAllJurisdictionsAndLabels(t *testing.T) {
	if got := AllJurisdictions(); len(got) != 3 {
		t.Fatalf("AllJurisdictions = %v", got)
	}
	cases := map[Jurisdiction]string{India: "India", Philippines: "Philippines", Other: "Other", "ZZ": "ZZ"}
	for j, want := range cases {
		if got := j.Label(); got != want {
			t.Errorf("%q.Label() = %q, want %q", j, got, want)
		}
	}
}

func TestParseJurisdiction(t *testing.T) {
	cases := []struct {
		in      string
		want    Jurisdiction
		wantErr bool
	}{
		{"IN", India, false},
		{"in", India, false},
		{"India", India, false},
		{"PH", Philippines, false},
		{"philippines", Philippines, false},
		{"PHL", Philippines, false},
		{"", Other, false},
		{"OTHER", Other, false},
		{"US", "", true},
	}
	for _, c := range cases {
		got, err := ParseJurisdiction(c.in)
		if c.wantErr != (err != nil) {
			t.Errorf("ParseJurisdiction(%q) err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("ParseJurisdiction(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseKind(t *testing.T) {
	cases := []struct {
		in      string
		want    Kind
		wantErr bool
	}{
		{"mayors-permit", KindMayorsPermit, false},
		{"MAYORS-PERMIT", KindMayorsPermit, false},
		{" mayors-permit ", KindMayorsPermit, false},
		{"my-own-permit-2", "my-own-permit-2", false},
		{"", "", true},
		{"has spaces", "", true},
		{"has_underscore", "", true},
		{"has.dot", "", true},
		{"has/slash", "", true},
		{strings.Repeat("a", 61), "", true},
	}
	for _, c := range cases {
		got, err := ParseKind(c.in)
		if c.wantErr != (err != nil) {
			t.Errorf("ParseKind(%q) err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("ParseKind(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseBasisAndLabels(t *testing.T) {
	cases := []struct {
		in      string
		want    Basis
		wantErr bool
	}{
		{"statute", FromStatute, false},
		{"STATUTE", FromStatute, false},
		{"convention", FromConvention, false},
		{"owner", FromOwner, false},
		{"", FromOwner, false},
		{"nonsense", "", true},
	}
	for _, c := range cases {
		got, err := ParseBasis(c.in)
		if c.wantErr != (err != nil) {
			t.Errorf("ParseBasis(%q) err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("ParseBasis(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	for basis, want := range map[Basis]string{
		FromStatute:    "fixed by law",
		FromConvention: "convention — check locally",
		FromOwner:      "you set this",
		"odd":          "odd",
	} {
		if got := basis.Label(); got != want {
			t.Errorf("%q.Label() = %q, want %q", basis, got, want)
		}
	}
}

func TestPresetNotesPointAtTheIssuingAuthority(t *testing.T) {
	// Every figure in this tool is meant to be confirmed with the office that
	// issues the permit, so each market's list has to carry that instruction
	// somewhere the owner will read it.
	for _, j := range []Jurisdiction{India, Philippines} {
		pointed := 0
		for _, preset := range PresetsFor(j) {
			lower := strings.ToLower(preset.Note)
			if strings.Contains(lower, "confirm") || strings.Contains(lower, "check with") ||
				strings.Contains(lower, "ask ") || strings.Contains(lower, "enter the date") ||
				strings.Contains(lower, "enter what") || strings.Contains(lower, "enter the one") {
				pointed++
			}
		}
		if pointed < len(PresetsFor(j))/2 {
			t.Errorf("only %d of the %d %s presets tell the owner to confirm the figure with the authority",
				pointed, len(PresetsFor(j)), j.Label())
		}
	}
}
