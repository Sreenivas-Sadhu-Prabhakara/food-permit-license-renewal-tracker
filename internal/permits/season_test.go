package permits

import (
	"strings"
	"testing"
	"time"
)

func TestPHJanuarySeasonWindow(t *testing.T) {
	from, to, dueBy, title, basis, why := PHJanuarySeason(2027)
	if from.String() != "2026-12-01" {
		t.Errorf("the window opens on %s, want 2026-12-01 — the permits that fall due in January lapse in December", from)
	}
	if to.String() != "2027-01-31" {
		t.Errorf("the window closes on %s, want 2027-01-31", to)
	}
	if dueBy.String() != "2027-01-20" {
		t.Errorf("the deadline is %s, want 2027-01-20 under section 167", dueBy)
	}
	if basis != FromStatute {
		t.Errorf("the January deadline has a statute behind it; basis = %q", basis)
	}
	if !strings.Contains(why, "7160") || !strings.Contains(why, "167") {
		t.Errorf("the deadline must cite the instrument; got %q", why)
	}
	if !strings.Contains(why, "extend") {
		t.Errorf("the deadline note must say a city can extend it; got %q", why)
	}
	if !strings.Contains(title, "2027") {
		t.Errorf("title = %q, want it to name the year", title)
	}
}

func TestINFinancialYearSeasonIsLabelledAConvention(t *testing.T) {
	_, to, dueBy, _, basis, why := INFinancialYearSeason(2027)
	if to.String() != "2027-03-31" || dueBy.String() != "2027-03-31" {
		t.Fatalf("the window closes %s with a deadline of %s, want both 2027-03-31", to, dueBy)
	}
	if basis != FromConvention {
		t.Fatalf("India has no statutory single renewal date; basis = %q, want %q", basis, FromConvention)
	}
	if !strings.Contains(why, "no single renewal date") {
		t.Fatalf("the India note must say so plainly; got %q", why)
	}
}

// phSeasonRegister is a Quezon City carinderia whose January permits all ran to
// 31 December 2026, seen from 2 December 2026 — the moment the question "what
// do I start first" actually gets asked.
func phSeasonRegister(t *testing.T) *Register {
	t.Helper()
	register := phRegister(t)
	return register
}

func TestPHSeasonGroupsAndOrdersTheJanuaryRush(t *testing.T) {
	register := phSeasonRegister(t)
	season := register.PHSeason("", 2027, MustParseDate("2026-12-02"))

	if len(season.Rows) == 0 {
		t.Fatal("nothing landed in the January season")
	}
	byKind := map[Kind]SeasonRow{}
	for _, row := range season.Rows {
		byKind[row.Permit.Kind] = row
	}

	// The five permits that renew in January have to be in it.
	for _, kind := range []Kind{
		KindBarangayClearance, KindHealthCertificate, KindSanitaryPermit, KindFSIC, KindMayorsPermit,
	} {
		if _, ok := byKind[kind]; !ok {
			t.Errorf("%s expires on 31 December but is not in the January season", kind)
		}
	}
	// The BIR certificate does not expire, so it has no business in a season.
	if _, ok := byKind[KindBIRRegistration]; ok {
		t.Error("a registration that does not expire must not appear in a renewal season")
	}

	// The waves: the barangay clearance, the fire certificate and the food
	// handlers' certificates have nothing in front of them. The sanitary
	// permit waits on the health certificates. The mayor's permit waits on
	// all three of the others, so it is last.
	wants := []struct {
		kind Kind
		wave int
	}{
		{KindBarangayClearance, 1},
		{KindFSIC, 1},
		{KindHealthCertificate, 1},
		{KindSanitaryPermit, 2},
		{KindMayorsPermit, 3},
	}
	for _, w := range wants {
		if got := byKind[w.kind].Wave; got != w.wave {
			t.Errorf("%s is in wave %d, want wave %d", w.kind, got, w.wave)
		}
	}
	if season.Waves != 3 {
		t.Errorf("the plan has %d steps, want 3", season.Waves)
	}
	if len(season.Cycles) != 0 {
		t.Fatalf("the shipped presets must contain no prerequisite loop; found %v", season.Cycles)
	}

	// The mayor's permit must be ordered after everything it waits on.
	mayorsOrder := byKind[KindMayorsPermit].Order
	for _, kind := range []Kind{KindBarangayClearance, KindFSIC, KindSanitaryPermit, KindHealthCertificate} {
		if byKind[kind].Order >= mayorsOrder {
			t.Errorf("%s is at position %d and the mayor's permit at %d — it must come first",
				kind, byKind[kind].Order, mayorsOrder)
		}
	}
	// The order must be a straight 1..n with no gaps, so a printed sheet works.
	for i, row := range season.Rows {
		if row.Order != i+1 {
			t.Fatalf("row %d carries order %d", i, row.Order)
		}
	}
}

func TestPHSeasonStatesWhichLinksAreLawAndWhichAreConvention(t *testing.T) {
	register := phSeasonRegister(t)
	season := register.PHSeason("", 2027, MustParseDate("2026-12-02"))

	var mayors SeasonRow
	for _, row := range season.Rows {
		if row.Permit.Kind == KindMayorsPermit {
			mayors = row
		}
	}
	if len(mayors.BlockedBy) != 3 {
		t.Fatalf("the mayor's permit waits on %d things, want 3", len(mayors.BlockedBy))
	}

	byKind := map[Kind]PrereqRef{}
	for _, dep := range mayors.BlockedBy {
		byKind[dep.Kind] = dep
	}
	// Two of the three are fixed by a named instrument.
	for kind, citation := range map[Kind]string{
		KindBarangayClearance: "152",
		KindFSIC:              "9514",
	} {
		dep, ok := byKind[kind]
		if !ok {
			t.Fatalf("%s is not listed as a prerequisite of the mayor's permit", kind)
		}
		if dep.Basis != FromStatute {
			t.Errorf("%s is fixed by law; basis = %q", kind, dep.Basis)
		}
		if !strings.Contains(dep.Why, citation) {
			t.Errorf("%s cites %q, want it to quote %s", kind, dep.Why, citation)
		}
	}
	// The third is this tool's own assumption and must say so.
	sanitary, ok := byKind[KindSanitaryPermit]
	if !ok {
		t.Fatal("the sanitary permit is not listed as a prerequisite of the mayor's permit")
	}
	if sanitary.Basis != FromConvention {
		t.Errorf("we could not source the sanitary permit's place in the order; basis = %q, want %q",
			sanitary.Basis, FromConvention)
	}
	if !strings.Contains(sanitary.Why, "reorder") {
		t.Errorf("a convention must invite the owner to change it; got %q", sanitary.Why)
	}
	if !strings.Contains(FromConvention.Label(), "check locally") {
		t.Errorf("the convention label is %q", FromConvention.Label())
	}

	// Every prerequisite inside the season must be marked as such, and every
	// one on the register must carry its real label.
	for _, dep := range mayors.BlockedBy {
		if !dep.InSeason {
			t.Errorf("%s is in this season but the reference says it is not", dep.Kind)
		}
		if !dep.OnRegister {
			t.Errorf("%s is on the register but the reference says it is not", dep.Kind)
		}
		if dep.Label == string(dep.Kind) {
			t.Errorf("%s shows its slug rather than its name", dep.Kind)
		}
	}
}

func TestPHSeasonWorksBackwardsFromTheDeadline(t *testing.T) {
	register := phSeasonRegister(t)
	asOf := MustParseDate("2026-12-02")
	season := register.PHSeason("", 2027, asOf)

	byKind := map[Kind]SeasonRow{}
	for _, row := range season.Rows {
		byKind[row.Permit.Kind] = row
	}

	// The shipped lead times: health certificate 14, sanitary 10, fire 10,
	// barangay 5, mayor's 10, against a deadline of 20 January 2027.
	//
	// The mayor's permit has nothing behind it, so it starts 10 days out.
	// The sanitary permit has the mayor's permit behind it: 10 + 10 = 20 days.
	// The health certificates have the sanitary permit and then the mayor's
	// permit behind them: 14 + 10 + 10 = 34 days, which reaches back to
	// 17 December — the fact that makes this view worth having.
	cases := []struct {
		kind           Kind
		wantLead       int
		wantDownstream int
		wantStartBy    string
		wantCritical   bool
	}{
		{KindMayorsPermit, 10, 0, "2027-01-10", true},
		{KindSanitaryPermit, 10, 10, "2026-12-31", true},
		{KindHealthCertificate, 14, 20, "2026-12-17", true},
		{KindBarangayClearance, 5, 10, "2027-01-05", false},
		{KindFSIC, 10, 10, "2026-12-31", false},
	}
	for _, c := range cases {
		row, ok := byKind[c.kind]
		if !ok {
			t.Fatalf("%s is missing from the season", c.kind)
		}
		if row.LeadDays != c.wantLead {
			t.Errorf("%s lead = %d days, want %d", c.kind, row.LeadDays, c.wantLead)
		}
		if row.DownstreamLeadDays != c.wantDownstream {
			t.Errorf("%s chain behind = %d days, want %d", c.kind, row.DownstreamLeadDays, c.wantDownstream)
		}
		if row.StartBy.String() != c.wantStartBy {
			t.Errorf("%s has to start by %s, want %s", c.kind, row.StartBy, c.wantStartBy)
		}
		if row.Critical != c.wantCritical {
			t.Errorf("%s critical = %v, want %v", c.kind, row.Critical, c.wantCritical)
		}
	}

	// Slack is counted from the day being asked about.
	if got := byKind[KindHealthCertificate].Slack; got != 15 {
		t.Errorf("on 2 December the health certificates have %d days of slack, want 15", got)
	}
	// Asked a month later, the same start date has already gone by.
	late := register.PHSeason("", 2027, MustParseDate("2027-01-02"))
	for _, row := range late.Rows {
		if row.Permit.Kind != KindHealthCertificate {
			continue
		}
		if row.Slack != -16 {
			t.Errorf("on 2 January the health certificates are %d days past their start date, want -16", row.Slack)
		}
	}
	if !strings.Contains(season.Note, "longest chain") {
		t.Errorf("the season note should state the critical chain; got %q", season.Note)
	}
	if !strings.Contains(season.Note, "convention") {
		t.Errorf("the season note must say the lead times are a convention; got %q", season.Note)
	}
}

func TestPHSeasonLeadTimeOverridesMoveTheStartDates(t *testing.T) {
	register := phSeasonRegister(t)
	// The owner's own city health office takes a month over the medicals.
	health, _ := register.Find(KindHealthCertificate, "")
	health.LeadDays = 30
	if _, err := register.Put(health); err != nil {
		t.Fatalf("Put: %v", err)
	}
	season := register.PHSeason("", 2027, MustParseDate("2026-12-02"))
	for _, row := range season.Rows {
		if row.Permit.Kind != KindHealthCertificate {
			continue
		}
		// 30 + 10 + 10 = 50 days back from 20 January 2027 is 1 December 2026.
		if row.StartBy.String() != "2026-12-01" {
			t.Fatalf("with a 30-day lead the medicals start by %s, want 2026-12-01", row.StartBy)
		}
		if row.Slack != -1 {
			t.Fatalf("slack = %d, want -1: the start date was yesterday", row.Slack)
		}
		return
	}
	t.Fatal("the health certificate row is missing")
}

func TestPHSeasonFallsBackToTheConventionalLeadTime(t *testing.T) {
	register := New()
	if _, err := register.Put(Permit{
		Kind: "one-off-permit", Label: "A permit with no lead time set",
		Authority: "Somebody", ExpiresOn: MustParseDate("2026-12-31"),
	}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	season := register.PHSeason("", 2027, MustParseDate("2026-12-02"))
	if len(season.Rows) != 1 {
		t.Fatalf("%d rows, want 1", len(season.Rows))
	}
	if season.Rows[0].LeadDays != DefaultLeadDays {
		t.Fatalf("lead = %d, want the shipped convention of %d", season.Rows[0].LeadDays, DefaultLeadDays)
	}
	if season.Rows[0].StartBy.String() != "2027-01-13" {
		t.Fatalf("start by %s, want 2027-01-13 (the deadline less %d days)", season.Rows[0].StartBy, DefaultLeadDays)
	}
}

func TestSeasonInclusionReasons(t *testing.T) {
	register := New()
	if _, err := register.AddJurisdictionPreset(Philippines, ""); err != nil {
		t.Fatalf("AddJurisdictionPreset: %v", err)
	}
	set := func(kind Kind, expires string) {
		permit, ok := register.Find(kind, "")
		if !ok {
			t.Fatalf("%s is missing", kind)
		}
		permit.NoExpiry = false
		if expires == "" {
			permit.ExpiresOn = Date{}
		} else {
			permit.ExpiresOn = MustParseDate(expires)
		}
		if _, err := register.Put(permit); err != nil {
			t.Fatalf("Put(%s): %v", kind, err)
		}
	}
	set(KindMayorsPermit, "2026-12-31")   // due in the window
	set(KindFSIC, "2026-08-15")           // lapsed back in August and never renewed
	set(KindBarangayClearance, "")        // a January permit with no date typed in
	set(KindBusinessName, "2029-06-30")   // nowhere near this window
	set(KindSanitaryPermit, "2027-06-30") // outside the window, still valid

	season := register.PHSeason("", 2027, MustParseDate("2026-12-02"))
	byKind := map[Kind]SeasonRow{}
	for _, row := range season.Rows {
		byKind[row.Permit.Kind] = row
	}

	cases := []struct {
		kind Kind
		want InclusionReason
		in   bool
	}{
		{KindMayorsPermit, DueInWindow, true},
		{KindFSIC, AlreadyLapsed, true},
		{KindBarangayClearance, DateNotRecorded, true},
		{KindBusinessName, "", false},
		{KindSanitaryPermit, "", false},
	}
	for _, c := range cases {
		row, present := byKind[c.kind]
		if present != c.in {
			t.Errorf("%s in the season = %v, want %v", c.kind, present, c.in)
			continue
		}
		if c.in && row.Why != c.want {
			t.Errorf("%s is in the season because %q, want %q", c.kind, row.Why, c.want)
		}
	}

	// A prerequisite that is outside the season but has lapsed must still be
	// flagged on the row that needs it.
	mayors := byKind[KindMayorsPermit]
	for _, dep := range mayors.BlockedBy {
		switch dep.Kind {
		case KindFSIC:
			if dep.Settled {
				t.Error("the fire certificate has lapsed; it cannot read as settled")
			}
		case KindSanitaryPermit:
			if dep.InSeason {
				t.Error("the sanitary permit is not due in this window")
			}
			if !dep.Settled {
				t.Error("the sanitary permit runs to June 2027, so it is settled")
			}
		}
	}
}

func TestSeasonWithNothingDue(t *testing.T) {
	register := phSeasonRegister(t)
	season := register.PHSeason("", 2030, MustParseDate("2026-12-02"))
	if len(season.Rows) != 0 {
		t.Fatalf("%d rows fell in the 2030 window, want none", len(season.Rows))
	}
	if season.Waves != 0 {
		t.Fatalf("waves = %d on an empty plan", season.Waves)
	}
	if !strings.Contains(season.Note, "Nothing on the register falls due") {
		t.Fatalf("note = %q, want it to say nothing is due", season.Note)
	}
	if !strings.Contains(season.Note, "not been entered") {
		t.Fatalf("note = %q, want it to allow that the dates may be missing", season.Note)
	}
}

func TestSeasonDetectsAPrerequisiteLoop(t *testing.T) {
	// A loop can only arrive from an edge somebody added. It cannot be
	// ordered, so the plan must say so rather than drop the permits or hang.
	register := New()
	build := func(kind Kind, prereq Kind) {
		permit := Permit{
			Kind: kind, Label: string(kind), Authority: "test",
			ExpiresOn: MustParseDate("2026-12-31"), LeadDays: 5,
			Prereqs: []Prereq{{Kind: prereq, Basis: FromOwner, Why: "the owner set this"}},
		}
		if _, err := register.Put(permit); err != nil {
			t.Fatalf("Put(%s): %v", kind, err)
		}
	}
	build("alpha", "beta")
	build("beta", "gamma")
	build("gamma", "alpha")
	// Plus one permit outside the loop, which must still be planned.
	if _, err := register.Put(Permit{
		Kind: "delta", Label: "delta", Authority: "test",
		ExpiresOn: MustParseDate("2026-12-31"), LeadDays: 5,
	}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	season := register.PHSeason("", 2027, MustParseDate("2026-12-02"))
	if len(season.Rows) != 4 {
		t.Fatalf("%d rows, want all 4 — a loop must not lose permits", len(season.Rows))
	}
	if len(season.Cycles) == 0 {
		t.Fatal("the loop was not reported")
	}
	loop := season.Cycles[0]
	if len(loop) != 3 {
		t.Fatalf("the reported loop is %v, want the three permits in it", loop)
	}
	inLoop := map[Kind]bool{}
	for _, kind := range loop {
		inLoop[kind] = true
	}
	for _, kind := range []Kind{"alpha", "beta", "gamma"} {
		if !inLoop[kind] {
			t.Errorf("%s is in the loop but was not reported", kind)
		}
	}
	if !strings.Contains(season.Note, "loop") {
		t.Errorf("the note must mention the loop; got %q", season.Note)
	}
	// Every row still gets a wave, so the interface has something to draw.
	for _, row := range season.Rows {
		if row.Wave < 1 {
			t.Errorf("%s has wave %d", row.Permit.Kind, row.Wave)
		}
	}
}

func TestSeasonTwoLoopsAreReportedOnce(t *testing.T) {
	register := New()
	for _, pair := range [][2]Kind{{"aa", "bb"}, {"bb", "aa"}, {"cc", "dd"}, {"dd", "cc"}} {
		if _, err := register.Put(Permit{
			Kind: pair[0], Label: string(pair[0]), Authority: "test",
			ExpiresOn: MustParseDate("2026-12-31"), LeadDays: 3,
			Prereqs: []Prereq{{Kind: pair[1], Basis: FromOwner}},
		}); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	season := register.PHSeason("", 2027, MustParseDate("2026-12-02"))
	if len(season.Cycles) != 2 {
		t.Fatalf("found %d loops (%v), want 2 distinct ones", len(season.Cycles), season.Cycles)
	}
}

func TestSeasonPrerequisitesOutsideTheSeasonDoNotConstrainTheOrder(t *testing.T) {
	// The FDA licence waits on the mayor's permit. If only the FDA licence is
	// due, there is nothing to sequence and it belongs in wave one.
	register := New()
	if _, err := register.AddJurisdictionPreset(Philippines, ""); err != nil {
		t.Fatalf("AddJurisdictionPreset: %v", err)
	}
	fda, _ := register.Find(KindFDALTO, "")
	fda.ExpiresOn = MustParseDate("2027-01-15")
	if _, err := register.Put(fda); err != nil {
		t.Fatalf("Put: %v", err)
	}
	mayors, _ := register.Find(KindMayorsPermit, "")
	mayors.ExpiresOn = MustParseDate("2029-12-31")
	if _, err := register.Put(mayors); err != nil {
		t.Fatalf("Put: %v", err)
	}

	season := register.PHSeason("", 2027, MustParseDate("2026-12-02"))
	for _, row := range season.Rows {
		if row.Permit.Kind != KindFDALTO {
			continue
		}
		if row.Wave != 1 {
			t.Fatalf("the FDA licence is in wave %d; nothing else is due, so it is wave 1", row.Wave)
		}
		if len(row.BlockedBy) != 1 {
			t.Fatalf("blockedBy = %+v, want the mayor's permit listed for information", row.BlockedBy)
		}
		if row.BlockedBy[0].InSeason {
			t.Error("the mayor's permit is not due in this window")
		}
		if !row.BlockedBy[0].Settled {
			t.Error("the mayor's permit runs to 2029, so it is settled")
		}
		return
	}
	t.Fatal("the FDA licence is missing from the season")
}

func TestSeasonWindowChosenByTheOwner(t *testing.T) {
	register := phSeasonRegister(t)
	from := NewDate(2026, time.December, 1)
	to := NewDate(2026, time.December, 31)
	season := register.SeasonWindow("", from, to, MustParseDate("2026-12-20"), MustParseDate("2026-11-01"))
	if season.DueByBasis != FromOwner {
		t.Fatalf("basis = %q, want %q", season.DueByBasis, FromOwner)
	}
	if season.DueBy.String() != "2026-12-20" {
		t.Fatalf("deadline = %s, want the one the owner gave", season.DueBy)
	}
	if len(season.Rows) == 0 {
		t.Fatal("the December window should hold the permits that lapse on 31 December")
	}

	// With no deadline given, the end of the window is the deadline.
	noDeadline := register.SeasonWindow("", from, to, Date{}, MustParseDate("2026-11-01"))
	if noDeadline.DueBy.String() != to.String() {
		t.Fatalf("with no deadline the plan uses %s, want the end of the window %s", noDeadline.DueBy, to)
	}
}

func TestSeasonIsPerSite(t *testing.T) {
	register := New()
	for _, site := range []string{"Kamuning", "Cubao"} {
		if _, err := register.AddJurisdictionPreset(Philippines, site); err != nil {
			t.Fatalf("AddJurisdictionPreset: %v", err)
		}
	}
	// Only the Kamuning permits have dates.
	for _, p := range append([]Permit(nil), register.Permits...) {
		if p.Site != "Kamuning" || p.NoExpiry {
			continue
		}
		p.ExpiresOn = MustParseDate("2026-12-31")
		if _, err := register.Put(p); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	asOf := MustParseDate("2026-12-02")
	kamuning := register.PHSeason("Kamuning", 2027, asOf)
	cubao := register.PHSeason("Cubao", 2027, asOf)

	if len(kamuning.Rows) == 0 {
		t.Fatal("Kamuning has permits due in January")
	}
	for _, row := range kamuning.Rows {
		if row.Permit.Site != "Kamuning" {
			t.Fatalf("the Kamuning plan holds a %s row", row.Permit.Site)
		}
	}
	// Cubao's permits have no dates, so only the January-anchored ones appear,
	// each flagged as having no date recorded.
	for _, row := range cubao.Rows {
		if row.Why != DateNotRecorded {
			t.Errorf("Cubao's %s is in the plan because %q, want %q", row.Permit.Kind, row.Why, DateNotRecorded)
		}
		if !row.Permit.SeasonAnchor {
			t.Errorf("Cubao's %s has no date and does not renew in January, so it should not be here", row.Permit.Kind)
		}
	}
}

func TestSeasonCountsItsOwnRows(t *testing.T) {
	register := phSeasonRegister(t)
	season := register.PHSeason("", 2027, MustParseDate("2027-02-02"))
	if season.Counts.Total != len(season.Rows) {
		t.Fatalf("counts total %d, rows %d", season.Counts.Total, len(season.Rows))
	}
	if season.Counts.Expired != len(season.Rows) {
		t.Fatalf("on 2 February every January permit has lapsed: expired = %d of %d",
			season.Counts.Expired, len(season.Rows))
	}
}

func TestSeasonRowsCarryReadiness(t *testing.T) {
	register := phSeasonRegister(t)
	if err := register.SetChecklistItem(KindMayorsPermit, "", 0, true); err != nil {
		t.Fatalf("SetChecklistItem: %v", err)
	}
	season := register.PHSeason("", 2027, MustParseDate("2026-12-02"))
	for _, row := range season.Rows {
		if row.Permit.Kind != KindMayorsPermit {
			continue
		}
		if row.Readiness.Done != 1 {
			t.Fatalf("readiness shows %d done, want 1", row.Readiness.Done)
		}
		if row.Readiness.Total < 5 {
			t.Fatalf("the mayor's permit checklist has %d lines, which is too few to be useful", row.Readiness.Total)
		}
		return
	}
	t.Fatal("the mayor's permit is missing from the season")
}
