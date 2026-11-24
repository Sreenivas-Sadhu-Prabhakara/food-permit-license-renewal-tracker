package permits

import (
	"encoding/json"
	"strings"
	"testing"
)

func samplePermit() Permit {
	return Permit{
		Kind:         KindSanitaryPermit,
		Jurisdiction: Philippines,
		Label:        "Sanitary permit",
		Authority:    "City health office",
		Number:       "SP-2026-0199",
		IssuedOn:     MustParseDate("2026-01-15"),
		ExpiresOn:    MustParseDate("2026-12-31"),
		Blocks:       true,
	}
}

func TestPutValidates(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Permit)
		wantErr string
	}{
		{"a good permit", func(*Permit) {}, ""},
		{"no kind", func(p *Permit) { p.Kind = "" }, "short code"},
		{"a kind with spaces", func(p *Permit) { p.Kind = "sanitary permit" }, "a-z"},
		{"a kind with punctuation", func(p *Permit) { p.Kind = "sanitary_permit" }, "a-z"},
		{"an unknown market", func(p *Permit) { p.Jurisdiction = "ZZ" }, "not a market"},
		{
			"both never expires and has an expiry date",
			func(p *Permit) { p.NoExpiry = true; p.ExpiresOn = MustParseDate("2026-12-31") },
			"clear one of the two",
		},
		{
			"expires before it was issued",
			func(p *Permit) { p.IssuedOn = MustParseDate("2026-12-31"); p.ExpiresOn = MustParseDate("2026-01-15") },
			"before it was issued",
		},
		{"a year that is a typing slip", func(p *Permit) { p.ExpiresOn = NewDate(226, 12, 31) }, "typing slip"},
		{"a validity beyond the cap", func(p *Permit) { p.ValidityMonths = MaxValidityMonths + 1 }, "months"},
		{"a lead time beyond the cap", func(p *Permit) { p.LeadDays = MaxLeadDays + 1 }, "lead time"},
		{
			"a prerequisite on itself",
			func(p *Permit) { p.Prereqs = []Prereq{{Kind: KindSanitaryPermit, Basis: FromOwner}} },
			"",
		},
		{
			"a prerequisite with a bad basis",
			func(p *Permit) {
				p.Prereqs = []Prereq{{Kind: KindFSIC, Basis: "made-it-up"}}
			},
			"not a basis",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			register := New()
			permit := samplePermit()
			c.mutate(&permit)

			saved, err := register.Put(permit)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("Put returned %v, wanted it accepted", err)
				}
				if len(register.Permits) != 1 {
					t.Fatalf("the register holds %d permits, want 1", len(register.Permits))
				}
				_ = saved
				return
			}
			if err == nil {
				t.Fatalf("Put accepted %s; it should have been refused", c.name)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("Put error was %q, wanted it to mention %q", err, c.wantErr)
			}
			if len(register.Permits) != 0 {
				t.Fatal("a refused edit must leave the register untouched")
			}
		})
	}
}

func TestPutDropsASelfPrerequisiteRatherThanRefusing(t *testing.T) {
	// tidy strips an edge from a permit to itself before Validate sees it, so
	// the permit saves and the nonsense edge is gone. Confirm both halves.
	register := New()
	permit := samplePermit()
	permit.Prereqs = []Prereq{
		{Kind: KindSanitaryPermit, Basis: FromOwner},
		{Kind: KindFSIC, Basis: FromOwner},
	}
	saved, err := register.Put(permit)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if len(saved.Prereqs) != 1 || saved.Prereqs[0].Kind != KindFSIC {
		t.Fatalf("prerequisites after saving = %+v, want just the fire certificate", saved.Prereqs)
	}
}

func TestPutReplacesTheSameKindAtTheSameSite(t *testing.T) {
	register := New()
	if _, err := register.Put(samplePermit()); err != nil {
		t.Fatalf("Put: %v", err)
	}

	renewed := samplePermit()
	renewed.Number = "SP-2027-0412"
	renewed.IssuedOn = MustParseDate("2027-01-18")
	renewed.ExpiresOn = MustParseDate("2027-12-31")
	if _, err := register.Put(renewed); err != nil {
		t.Fatalf("Put the renewal: %v", err)
	}
	if len(register.Permits) != 1 {
		t.Fatalf("recording a renewal left %d rows, want 1", len(register.Permits))
	}
	found, ok := register.Find(KindSanitaryPermit, "")
	if !ok {
		t.Fatal("the renewed permit is not on the register")
	}
	if found.Number != "SP-2027-0412" || found.ExpiresOn.String() != "2027-12-31" {
		t.Fatalf("the renewal did not replace the old record: %+v", found)
	}
}

func TestTheSameKindAtTwoSitesAreTwoPermits(t *testing.T) {
	// A Philippine business permit is issued per location, so two branches
	// means two permits on their own dates.
	register := New()
	first := samplePermit()
	first.Site = "Kamuning branch"
	second := samplePermit()
	second.Site = "Cubao branch"
	second.ExpiresOn = MustParseDate("2027-06-30")

	for _, p := range []Permit{first, second} {
		if _, err := register.Put(p); err != nil {
			t.Fatalf("Put(%s): %v", p.Site, err)
		}
	}
	if len(register.Permits) != 2 {
		t.Fatalf("two sites gave %d permits, want 2", len(register.Permits))
	}
	cubao, ok := register.Find(KindSanitaryPermit, "Cubao branch")
	if !ok || cubao.ExpiresOn.String() != "2027-06-30" {
		t.Fatalf("the Cubao permit is wrong: %+v (found %v)", cubao, ok)
	}
	// Site matching ignores case, since the owner types it twice.
	if _, ok := register.Find(KindSanitaryPermit, "cubao branch"); !ok {
		t.Fatal("a site should match whatever case the owner typed")
	}
}

func TestRemove(t *testing.T) {
	register := New()
	if _, err := register.Put(samplePermit()); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !register.Remove(KindSanitaryPermit, "") {
		t.Fatal("Remove reported nothing to remove")
	}
	if len(register.Permits) != 0 {
		t.Fatalf("after Remove the register holds %d permits", len(register.Permits))
	}
	if register.Remove(KindSanitaryPermit, "") {
		t.Fatal("Remove reported a second removal of the same permit")
	}
}

func TestAddJurisdictionPresetAndGaps(t *testing.T) {
	for _, j := range []Jurisdiction{India, Philippines} {
		t.Run(j.Label(), func(t *testing.T) {
			register := New()
			added, err := register.AddJurisdictionPreset(j, "")
			if err != nil {
				t.Fatalf("AddJurisdictionPreset: %v", err)
			}
			wantCount := len(PresetsFor(j))
			if wantCount < 6 || wantCount > 8 {
				t.Fatalf("the %s preset ships %d permits; the brief asks for six to eight", j.Label(), wantCount)
			}
			if len(added) != wantCount {
				t.Fatalf("added %d permits, want %d", len(added), wantCount)
			}
			if gaps := register.Gaps(j, ""); len(gaps) != 0 {
				t.Fatalf("after adding the whole preset there are still %d gaps", len(gaps))
			}

			// Adding it twice must not duplicate, and must not wipe a date.
			first, ok := register.Find(PresetsFor(j)[0].Kind, "")
			if !ok {
				t.Fatal("the first preset permit is missing")
			}
			first.ExpiresOn = MustParseDate("2027-12-31")
			first.NoExpiry = false
			if _, err := register.Put(first); err != nil {
				t.Fatalf("recording a date: %v", err)
			}
			againAdded, err := register.AddJurisdictionPreset(j, "")
			if err != nil {
				t.Fatalf("AddJurisdictionPreset a second time: %v", err)
			}
			if len(againAdded) != 0 {
				t.Fatalf("a second pass added %d permits, want none", len(againAdded))
			}
			if len(register.Permits) != wantCount {
				t.Fatalf("the register now holds %d permits, want %d", len(register.Permits), wantCount)
			}
			kept, _ := register.Find(first.Kind, "")
			if kept.ExpiresOn.String() != "2027-12-31" {
				t.Fatalf("a second preset pass overwrote a date the owner typed: %v", kept.ExpiresOn)
			}
		})
	}
}

func TestGapsAreNotFindings(t *testing.T) {
	// A gap must never make the business look unlawful: whether a food
	// business needs an FDA licence to operate depends on what it does.
	register := New()
	if _, err := register.AddPreset(KindMayorsPermit, ""); err != nil {
		t.Fatalf("AddPreset: %v", err)
	}
	mayors, _ := register.Find(KindMayorsPermit, "")
	mayors.ExpiresOn = MustParseDate("2027-12-31")
	if _, err := register.Put(mayors); err != nil {
		t.Fatalf("Put: %v", err)
	}

	gaps := register.Gaps(Philippines, "")
	if len(gaps) == 0 {
		t.Fatal("with one permit on file there should be gaps to report")
	}
	verdict := register.CanTradeToday(MustParseDate("2027-01-10"))
	if !verdict.CanTrade {
		t.Fatalf("gaps must not block trading: %s", verdict.Headline)
	}
}

func TestAddPresetRefusesAnUnknownKind(t *testing.T) {
	register := New()
	if _, err := register.AddPreset("invented-permit", ""); err == nil {
		t.Fatal("AddPreset accepted a kind the tool ships no preset for")
	}
}

func TestAddPresetKeepsWhatIsAlreadyThere(t *testing.T) {
	register := New()
	if _, err := register.AddPreset(KindFSIC, ""); err != nil {
		t.Fatalf("AddPreset: %v", err)
	}
	existing, _ := register.Find(KindFSIC, "")
	existing.Number = "FSIC-11-2026-0001"
	existing.ExpiresOn = MustParseDate("2026-12-31")
	if _, err := register.Put(existing); err != nil {
		t.Fatalf("Put: %v", err)
	}
	again, err := register.AddPreset(KindFSIC, "")
	if err != nil {
		t.Fatalf("AddPreset again: %v", err)
	}
	if again.Number != "FSIC-11-2026-0001" {
		t.Fatalf("AddPreset overwrote the owner's record: %+v", again)
	}
}

func TestPresetToPermitCarriesTheWholeEntry(t *testing.T) {
	preset, ok := PresetFor(KindMayorsPermit)
	if !ok {
		t.Fatal("the mayor's permit preset is missing")
	}
	permit := preset.ToPermit()
	if permit.Label != preset.Label || permit.Authority != preset.Authority {
		t.Fatal("ToPermit lost the label or the authority")
	}
	if len(permit.Checklist) != len(preset.Checklist) {
		t.Fatalf("ToPermit gave %d checklist lines, want %d", len(permit.Checklist), len(preset.Checklist))
	}
	for _, item := range permit.Checklist {
		if item.Done {
			t.Fatal("a fresh preset's checklist must start unticked")
		}
	}
	if len(permit.Prereqs) != len(preset.Prereqs) {
		t.Fatalf("ToPermit gave %d prerequisites, want %d", len(permit.Prereqs), len(preset.Prereqs))
	}
	if !permit.FromPreset {
		t.Fatal("a permit made from a preset should say so")
	}
	if permit.ExpiresOn != (Date{}) || permit.IssuedOn != (Date{}) {
		t.Fatal("a preset must not invent dates")
	}
}

func TestPresetToPermitFallsBackToTheConventionalLeadTime(t *testing.T) {
	preset := Preset{Kind: "made-up", Label: "Made up", ValidityMonths: 12}
	if got := preset.ToPermit().LeadDays; got != DefaultLeadDays {
		t.Fatalf("lead days = %d, want the shipped convention %d", got, DefaultLeadDays)
	}
	noExpiry := Preset{Kind: "made-up", Label: "Made up", NoExpiry: true}
	if got := noExpiry.ToPermit().LeadDays; got != 0 {
		t.Fatalf("a permit that never expires needs no lead time, got %d", got)
	}
}

func TestSetChecklistItem(t *testing.T) {
	register := New()
	if _, err := register.AddPreset(KindMayorsPermit, ""); err != nil {
		t.Fatalf("AddPreset: %v", err)
	}
	if err := register.SetChecklistItem(KindMayorsPermit, "", 0, true); err != nil {
		t.Fatalf("SetChecklistItem: %v", err)
	}
	permit, _ := register.Find(KindMayorsPermit, "")
	if !permit.Checklist[0].Done {
		t.Fatal("the first checklist line did not tick")
	}
	if got := ReadinessOf(permit).Done; got != 1 {
		t.Fatalf("readiness counted %d ticked, want 1", got)
	}
	if err := register.SetChecklistItem(KindMayorsPermit, "", 0, false); err != nil {
		t.Fatalf("unticking: %v", err)
	}
	permit, _ = register.Find(KindMayorsPermit, "")
	if permit.Checklist[0].Done {
		t.Fatal("the line did not untick")
	}

	if err := register.SetChecklistItem(KindMayorsPermit, "", 999, true); err == nil {
		t.Fatal("a line that does not exist must be refused")
	}
	if err := register.SetChecklistItem(KindMayorsPermit, "", -1, true); err == nil {
		t.Fatal("a negative line must be refused")
	}
	if err := register.SetChecklistItem(KindFDALTO, "", 0, true); err == nil {
		t.Fatal("ticking a line on a permit that is not on the register must be refused")
	}
}

func TestSuggestExpiry(t *testing.T) {
	register := New()
	if _, err := register.AddJurisdictionPreset(Philippines, ""); err != nil {
		t.Fatalf("AddJurisdictionPreset: %v", err)
	}

	cases := []struct {
		name     string
		kind     Kind
		issuedOn string
		wantOK   bool
		want     string
		wantNote string
	}{
		{
			name: "a twelve-month sanitary permit issued in January",
			kind: KindSanitaryPermit, issuedOn: "2027-01-18", wantOK: true, want: "2028-01-17",
		},
		{
			name: "a five-year business name registration",
			kind: KindBusinessName, issuedOn: "2027-03-15", wantOK: true, want: "2032-03-14",
		},
		{
			name: "a month-end issue date clamps rather than overflowing",
			kind: KindSanitaryPermit, issuedOn: "2027-01-31", wantOK: true, want: "2028-01-30",
		},
		{
			name: "the FDA licence ships no period on purpose",
			kind: KindFDALTO, issuedOn: "2027-01-18", wantOK: false, wantNote: "ships no validity period",
		},
		{
			name: "a registration that does not expire has nothing to work out",
			kind: KindBIRRegistration, issuedOn: "2027-01-18", wantOK: false, wantNote: "does not expire",
		},
		{
			name: "no issue date, nothing to work from",
			kind: KindSanitaryPermit, issuedOn: "", wantOK: false, wantNote: "issue date first",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issued := Date{}
			if c.issuedOn != "" {
				issued = MustParseDate(c.issuedOn)
			}
			got, ok, note := register.SuggestExpiry(c.kind, "", issued)
			if ok != c.wantOK {
				t.Fatalf("SuggestExpiry ok = %v (note %q), want %v", ok, note, c.wantOK)
			}
			if c.wantOK && got.String() != c.want {
				t.Fatalf("SuggestExpiry = %s, want %s", got, c.want)
			}
			if !c.wantOK && !strings.Contains(note, c.wantNote) {
				t.Fatalf("note was %q, wanted it to mention %q", note, c.wantNote)
			}
		})
	}
}

func TestSuggestExpiryFollowsTheOwnersOverride(t *testing.T) {
	register := New()
	if _, err := register.AddPreset(KindFDALTO, ""); err != nil {
		t.Fatalf("AddPreset: %v", err)
	}
	// The tool ships no period for the FDA licence. Once the owner sets one
	// from their own paper, the suggestion has to follow it.
	permit, _ := register.Find(KindFDALTO, "")
	permit.ValidityMonths = 36
	if _, err := register.Put(permit); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, note := register.SuggestExpiry(KindFDALTO, "", MustParseDate("2027-02-01"))
	if !ok {
		t.Fatalf("with a period set the suggestion should work: %q", note)
	}
	if got.String() != "2030-01-31" {
		t.Fatalf("SuggestExpiry = %s, want 2030-01-31", got)
	}
}

func TestNormaliseRepairsAHandEditedFile(t *testing.T) {
	raw := `{
	  "business": {"name": "  Lola's Kitchen  ", "locality": "Quezon City"},
	  "tiers": {"noticeDays": 7, "warnDays": 30, "urgentDays": 60},
	  "permits": [
	    {"kind": "  MAYORS-PERMIT  ", "label": "", "expiresOn": "2027-01-20", "blocks": true,
	     "validityMonths": -4, "leadDays": 9999,
	     "prereqs": [{"kind": "mayors-permit"}, {"kind": "barangay-clearance"}, {"kind": "barangay-clearance"}]},
	    {"kind": "mayors-permit", "label": "A duplicate that must be dropped"},
	    {"kind": "", "label": "No kind at all"},
	    {"kind": "bir-registration", "noExpiry": true, "expiresOn": "2020-01-01"}
	  ]
	}`
	register := New()
	if err := json.Unmarshal([]byte(raw), register); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}
	register.Normalise()

	if register.Business.Name != "Lola's Kitchen" {
		t.Fatalf("the business name was not trimmed: %q", register.Business.Name)
	}
	if register.Tiers != (Tiers{60, 30, 7}) {
		t.Fatalf("the tiers were not repaired: %+v", register.Tiers)
	}
	if len(register.Permits) != 2 {
		t.Fatalf("Normalise kept %d permits, want 2 (the duplicate and the one with no kind dropped): %v",
			len(register.Permits), register.Permits)
	}

	mayors, ok := register.Find(KindMayorsPermit, "")
	if !ok {
		t.Fatal("the mayor's permit was lost")
	}
	if mayors.Label == "" {
		t.Fatal("an empty label should be filled in from the preset")
	}
	if mayors.ValidityMonths != 0 {
		t.Fatalf("a negative validity became %d, want 0", mayors.ValidityMonths)
	}
	if mayors.LeadDays != MaxLeadDays {
		t.Fatalf("an absurd lead time became %d, want it clamped to %d", mayors.LeadDays, MaxLeadDays)
	}
	if len(mayors.Prereqs) != 1 || mayors.Prereqs[0].Kind != KindBarangayClearance {
		t.Fatalf("prerequisites after repair = %+v, want one barangay clearance", mayors.Prereqs)
	}
	if mayors.Prereqs[0].Basis != FromOwner {
		t.Fatalf("an edge with no basis became %q, want %q", mayors.Prereqs[0].Basis, FromOwner)
	}

	bir, ok := register.Find(KindBIRRegistration, "")
	if !ok {
		t.Fatal("the BIR registration was lost")
	}
	if !bir.ExpiresOn.Zero() {
		t.Fatalf("a no-expiry permit kept an expiry date of %s", bir.ExpiresOn)
	}
}

func TestClipTrimsCollapsesAndCapsText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  spaced  out  ", "spaced out"},
		{"line\nbreak", "line break"},
		{"tab\tseparated", "tab separated"},
		{"", ""},
	}
	for _, c := range cases {
		if got := clip(c.in); got != c.want {
			t.Errorf("clip(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	long := strings.Repeat("x", MaxTextLen+100)
	if got := clip(long); len([]rune(got)) != MaxTextLen {
		t.Fatalf("clip left %d runes, want %d", len([]rune(got)), MaxTextLen)
	}
	if got := clip("drop\x00the\x07controls"); got != "dropthecontrols" {
		t.Fatalf("clip(%q) = %q", "drop\\x00the\\x07controls", got)
	}
}

func TestSites(t *testing.T) {
	register := New()
	for _, site := range []string{"Cubao branch", "", "Anonas branch"} {
		permit := samplePermit()
		permit.Site = site
		if _, err := register.Put(permit); err != nil {
			t.Fatalf("Put(%q): %v", site, err)
		}
	}
	got := register.Sites()
	want := []string{"", "Anonas branch", "Cubao branch"}
	if len(got) != len(want) {
		t.Fatalf("Sites() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Sites() = %v, want %v (the main premises first)", got, want)
		}
	}
}

func TestSiteBoard(t *testing.T) {
	register := New()
	for _, site := range []string{"Cubao branch", "Anonas branch"} {
		permit := samplePermit()
		permit.Site = site
		if _, err := register.Put(permit); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	rows := register.SiteBoard("Cubao branch", MustParseDate("2026-06-01"))
	if len(rows) != 1 || rows[0].Permit.Site != "Cubao branch" {
		t.Fatalf("SiteBoard returned %d rows: %v", len(rows), rows)
	}
}

func TestPutRefusesBeyondTheCap(t *testing.T) {
	register := New()
	// Fill the register to the cap directly, then try to add one more.
	register.Permits = make([]Permit, 0, MaxPermits)
	for i := 0; i < MaxPermits; i++ {
		register.Permits = append(register.Permits, Permit{Kind: Kind(letters(i)), Label: "x"})
	}
	if _, err := register.Put(samplePermit()); err == nil {
		t.Fatal("Put must refuse to go past the register cap")
	}
}

// letters turns a number into a distinct lower-case slug.
func letters(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	out := []byte{}
	for {
		out = append([]byte{alphabet[n%26]}, out...)
		n /= 26
		if n == 0 {
			break
		}
	}
	return string(out)
}

func TestDefaultLateRatesPerMarket(t *testing.T) {
	ph := DefaultLateRates(Philippines)
	if ph.SurchargeRate != PHSurchargeRateCeiling || ph.MonthlyInterest != PHMonthlyInterestCeiling {
		t.Fatalf("the Philippine defaults are %+v, want the section 168 ceilings", ph)
	}
	if ph.InterestCapMo != PHInterestMonthsCap {
		t.Fatalf("the interest cap is %d, want %d", ph.InterestCapMo, PHInterestMonthsCap)
	}
	in := DefaultLateRates(India)
	if in.SurchargeRate != 0 || in.MonthlyInterest != 0 || in.DailyLateFee != 0 {
		t.Fatalf("the India defaults are %+v; this tool asserts no India penalty figure", in)
	}
}
