package permits

import (
	"strings"
	"testing"
)

// phRegister builds a Quezon City carinderia: the whole Philippine preset
// except the FDA licence to operate, which a carinderia does not hold and
// which the preset itself says to delete. The five January permits run to
// 31 December 2026, the business name registration to 2029, and the BIR
// certificate of registration does not expire at all.
func phRegister(t *testing.T) *Register {
	t.Helper()
	register := New()
	register.Business = Business{Name: "Lola's Carinderia", Jurisdiction: Philippines, Locality: "Quezon City"}
	register.LateRates = DefaultLateRates(Philippines)
	if _, err := register.AddJurisdictionPreset(Philippines, ""); err != nil {
		t.Fatalf("AddJurisdictionPreset: %v", err)
	}
	if !register.Remove(KindFDALTO, "") {
		t.Fatal("the FDA licence should have been on the register to remove")
	}
	for _, p := range append([]Permit(nil), register.Permits...) {
		if p.NoExpiry {
			continue
		}
		p.IssuedOn = MustParseDate("2026-01-20")
		if p.Kind == KindBusinessName {
			p.IssuedOn = MustParseDate("2024-07-01")
			p.ExpiresOn = MustParseDate("2029-06-30")
		} else {
			p.ExpiresOn = MustParseDate("2026-12-31")
		}
		if _, err := register.Put(p); err != nil {
			t.Fatalf("Put(%s): %v", p.Kind, err)
		}
	}
	return register
}

func TestCanTradeTodayWithEverythingInOrder(t *testing.T) {
	register := phRegister(t)
	verdict := register.CanTradeToday(MustParseDate("2026-06-01"))
	if !verdict.CanTrade {
		t.Fatalf("every permit is current but the verdict is no: %s", verdict.Headline)
	}
	if len(verdict.Blockers) != 0 {
		t.Fatalf("blockers with nothing expired: %+v", verdict.Blockers)
	}
	if verdict.Checked == 0 {
		t.Fatal("the verdict checked nothing, so a yes means nothing")
	}
	if !strings.HasPrefix(verdict.Headline, "Yes") {
		t.Fatalf("headline = %q, want it to start with a yes", verdict.Headline)
	}
	if verdict.Note == "" {
		t.Fatal("the verdict must carry its own caveat")
	}
}

func TestCanTradeTodayNamesEveryExpiredPermit(t *testing.T) {
	register := phRegister(t)
	// 2 February 2027: everything that ran to 31 December 2026 has lapsed and
	// nobody renewed in January.
	verdict := register.CanTradeToday(MustParseDate("2027-02-02"))
	if verdict.CanTrade {
		t.Fatalf("every blocking permit is 33 days overdue but the verdict is yes: %s", verdict.Headline)
	}

	// Each blocking permit must be named, with its authority and the overdue
	// count, because the owner has to know which door to go to.
	byKind := map[Kind]Blocker{}
	for _, b := range verdict.Blockers {
		byKind[b.Kind] = b
	}
	for _, kind := range []Kind{KindBarangayClearance, KindSanitaryPermit, KindFSIC, KindMayorsPermit} {
		blocker, ok := byKind[kind]
		if !ok {
			t.Fatalf("%s expired on 31 December but is not named as a blocker", kind)
		}
		if blocker.Reason != ReasonExpired {
			t.Errorf("%s blocked for %q, want %q", kind, blocker.Reason, ReasonExpired)
		}
		if blocker.DaysOverdue != 33 {
			t.Errorf("%s is %d days overdue, want 33", kind, blocker.DaysOverdue)
		}
		if blocker.Authority == "" {
			t.Errorf("%s names no authority, so the owner does not know where to go", kind)
		}
		if !strings.Contains(blocker.Says, blocker.Label) {
			t.Errorf("%s says %q, want it to name the permit", kind, blocker.Says)
		}
	}

	// A non-blocking permit must never be a blocker, however overdue.
	if _, named := byKind[KindHealthCertificate]; named {
		t.Error("the food handlers' health certificate is not marked as blocking and must not block")
	}
	if !strings.HasPrefix(verdict.Headline, "No.") {
		t.Fatalf("headline = %q, want it to start with No.", verdict.Headline)
	}
	if !strings.Contains(verdict.Headline, "Mayor's permit") {
		t.Fatalf("headline = %q, want it to name the permits", verdict.Headline)
	}
}

func TestCanTradeTodayOnTheExpiryDayItself(t *testing.T) {
	register := phRegister(t)
	onTheDay := register.CanTradeToday(MustParseDate("2026-12-31"))
	if !onTheDay.CanTrade {
		t.Fatalf("a permit is valid through the whole of its expiry day: %s", onTheDay.Headline)
	}
	if len(onTheDay.AtRisk) == 0 {
		t.Fatal("on the expiry day every blocking permit should be flagged as at risk")
	}
	nextDay := register.CanTradeToday(MustParseDate("2027-01-01"))
	if nextDay.CanTrade {
		t.Fatal("the day after expiry the business is trading unlawfully")
	}
	if len(nextDay.Blockers) == 0 {
		t.Fatal("the day after expiry there must be blockers")
	}
}

func TestCanTradeTodayWithNoExpiryDateRecorded(t *testing.T) {
	register := New()
	if _, err := register.AddPreset(KindSanitaryPermit, ""); err != nil {
		t.Fatalf("AddPreset: %v", err)
	}
	verdict := register.CanTradeToday(MustParseDate("2027-01-10"))
	if verdict.CanTrade {
		t.Fatal("a blocking permit with no expiry date recorded cannot be called valid")
	}
	if len(verdict.Blockers) != 1 || verdict.Blockers[0].Reason != ReasonNoDate {
		t.Fatalf("blockers = %+v, want one with reason %q", verdict.Blockers, ReasonNoDate)
	}
	if !strings.Contains(verdict.Blockers[0].Says, "Enter the date") {
		t.Fatalf("the blocker says %q; it should tell the owner what to do", verdict.Blockers[0].Says)
	}

	// Entering the date clears it.
	permit, _ := register.Find(KindSanitaryPermit, "")
	permit.ExpiresOn = MustParseDate("2027-12-31")
	if _, err := register.Put(permit); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if verdict := register.CanTradeToday(MustParseDate("2027-01-10")); !verdict.CanTrade {
		t.Fatalf("after entering the date the answer should be yes: %s", verdict.Headline)
	}
}

func TestCanTradeTodayWithARegistrationThatDoesNotLapse(t *testing.T) {
	register := New()
	if _, err := register.AddPreset(KindBIRRegistration, ""); err != nil {
		t.Fatalf("AddPreset: %v", err)
	}
	verdict := register.CanTradeToday(MustParseDate("2030-01-01"))
	if !verdict.CanTrade {
		t.Fatalf("a registration that does not expire cannot be the thing that stopped you: %s", verdict.Headline)
	}
	if verdict.Checked != 1 {
		t.Fatalf("checked %d permits, want 1 — a no-expiry registration is still checked", verdict.Checked)
	}
}

func TestCanTradeTodayOnAnEmptyRegisterIsNotAYes(t *testing.T) {
	register := New()
	verdict := register.CanTradeToday(MustParseDate("2027-01-10"))
	if verdict.Checked != 0 {
		t.Fatalf("checked %d on an empty register", verdict.Checked)
	}
	if strings.HasPrefix(verdict.Headline, "Yes") {
		t.Fatalf("an empty register said %q; a yes from nothing is a lie", verdict.Headline)
	}
	if !strings.Contains(verdict.Headline, "add your permits") {
		t.Fatalf("headline = %q, want it to tell the owner to add their permits", verdict.Headline)
	}
}

func TestCanTradeTodayPerSite(t *testing.T) {
	register := New()
	for _, site := range []string{"Kamuning", "Cubao"} {
		if _, err := register.AddJurisdictionPreset(Philippines, site); err != nil {
			t.Fatalf("AddJurisdictionPreset(%s): %v", site, err)
		}
		for _, p := range append([]Permit(nil), register.Permits...) {
			if p.Site != site || p.NoExpiry {
				continue
			}
			p.ExpiresOn = MustParseDate("2027-12-31")
			if _, err := register.Put(p); err != nil {
				t.Fatalf("Put: %v", err)
			}
		}
	}
	// Let one Cubao permit lapse.
	cubao, _ := register.Find(KindFSIC, "Cubao")
	cubao.ExpiresOn = MustParseDate("2026-12-31")
	if _, err := register.Put(cubao); err != nil {
		t.Fatalf("Put: %v", err)
	}

	asOf := MustParseDate("2027-03-01")
	if v := register.CanTradeTodayAt("Kamuning", asOf); !v.CanTrade {
		t.Fatalf("Kamuning is in order but the verdict is no: %s", v.Headline)
	}
	cubaoVerdict := register.CanTradeTodayAt("Cubao", asOf)
	if cubaoVerdict.CanTrade {
		t.Fatal("Cubao's fire certificate has lapsed; that branch cannot trade")
	}
	if len(cubaoVerdict.Blockers) != 1 || cubaoVerdict.Blockers[0].Kind != KindFSIC {
		t.Fatalf("Cubao blockers = %+v, want just the fire certificate", cubaoVerdict.Blockers)
	}
	if !strings.Contains(cubaoVerdict.Headline, "at Cubao") {
		t.Fatalf("headline = %q, want it to name the branch", cubaoVerdict.Headline)
	}

	// Across the whole business the lapse still shows, because one branch
	// trading unlawfully is the business's problem.
	whole := register.CanTradeToday(asOf)
	if whole.CanTrade {
		t.Fatal("one branch with a lapsed permit must show on the whole-business verdict")
	}
}

func TestCanTradeTodayOrdersTheWorstFirst(t *testing.T) {
	register := New()
	for _, c := range []struct {
		kind    Kind
		expires string
	}{
		{"a-slightly-late", "2027-01-05"},
		{"b-very-late", "2025-01-05"},
		{"c-middling", "2026-07-05"},
	} {
		if _, err := register.Put(Permit{
			Kind: c.kind, Label: string(c.kind), Authority: "test",
			Blocks: true, ExpiresOn: MustParseDate(c.expires),
		}); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	verdict := register.CanTradeToday(MustParseDate("2027-02-01"))
	want := []Kind{"b-very-late", "c-middling", "a-slightly-late"}
	if len(verdict.Blockers) != len(want) {
		t.Fatalf("%d blockers, want %d", len(verdict.Blockers), len(want))
	}
	for i, kind := range want {
		if verdict.Blockers[i].Kind != kind {
			t.Fatalf("blocker %d is %s, want %s", i, verdict.Blockers[i].Kind, kind)
		}
	}
}

func TestCanTradeTodayAtRiskIsOnlyTheFinalTier(t *testing.T) {
	register := New()
	for _, c := range []struct {
		kind    Kind
		expires string
	}{
		{"final-tier", "2027-01-15"},  // 7 days out
		{"second-tier", "2027-02-01"}, // 24 days out
		{"clear", "2028-01-01"},
	} {
		if _, err := register.Put(Permit{
			Kind: c.kind, Label: string(c.kind), Authority: "test",
			Blocks: true, ExpiresOn: MustParseDate(c.expires),
		}); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	verdict := register.CanTradeToday(MustParseDate("2027-01-08"))
	if !verdict.CanTrade {
		t.Fatalf("nothing has lapsed yet: %s", verdict.Headline)
	}
	if len(verdict.AtRisk) != 1 || verdict.AtRisk[0].Kind != "final-tier" {
		t.Fatalf("at risk = %+v, want only the permit inside the final tier", verdict.AtRisk)
	}
	if verdict.AtRisk[0].DaysLeft != 7 {
		t.Fatalf("days left = %d, want 7", verdict.AtRisk[0].DaysLeft)
	}
	if !strings.Contains(verdict.Headline, "final reminder") {
		t.Fatalf("headline = %q, want it to mention the final reminder", verdict.Headline)
	}
}

func TestCanTradeTodayRespectsTheOwnersBlockingChoice(t *testing.T) {
	// The India fire NOC ships as non-blocking, because many small premises do
	// not need one at all. An owner whose premises does need one sets it to
	// block, and then a lapse is a straight offence.
	register := New()
	if _, err := register.AddPreset(KindFireNOC, ""); err != nil {
		t.Fatalf("AddPreset: %v", err)
	}
	permit, _ := register.Find(KindFireNOC, "")
	permit.ExpiresOn = MustParseDate("2026-03-31")
	if _, err := register.Put(permit); err != nil {
		t.Fatalf("Put: %v", err)
	}
	asOf := MustParseDate("2027-01-10")
	if v := register.CanTradeToday(asOf); !v.CanTrade {
		t.Fatalf("as shipped the fire NOC does not block: %s", v.Headline)
	}

	permit.Blocks = true
	if _, err := register.Put(permit); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if v := register.CanTradeToday(asOf); v.CanTrade {
		t.Fatal("once the owner marks it blocking, a lapsed fire NOC must block")
	}
}
