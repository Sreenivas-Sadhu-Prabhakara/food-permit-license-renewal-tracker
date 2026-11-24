package permits

import (
	"testing"
	"time"
)

// TestStatusOfEveryTierBoundary walks the default 60/30/7 tiers one day at a
// time across every boundary. These are the cases that matter: a reminder that
// fires a day late is a reminder that did not fire.
func TestStatusOfEveryTierBoundary(t *testing.T) {
	asOf := MustParseDate("2027-01-01")
	tiers := DefaultTiers()

	cases := []struct {
		name      string
		expiresOn string
		wantLevel Level
		wantDays  int
		wantTier  int
	}{
		{"well clear", "2027-12-31", LevelOK, 364, 0},
		{"a day outside the first reminder", "2027-03-03", LevelOK, 61, 0},
		{"exactly the first reminder", "2027-03-02", LevelNotice, 60, 60},
		{"inside the first reminder", "2027-03-01", LevelNotice, 59, 60},
		{"a day outside the second", "2027-02-01", LevelNotice, 31, 60},
		{"exactly the second reminder", "2027-01-31", LevelWarn, 30, 30},
		{"inside the second", "2027-01-30", LevelWarn, 29, 30},
		{"a day outside the final", "2027-01-09", LevelWarn, 8, 30},
		{"exactly the final reminder", "2027-01-08", LevelUrgent, 7, 7},
		{"inside the final", "2027-01-05", LevelUrgent, 4, 7},
		{"tomorrow", "2027-01-02", LevelUrgent, 1, 7},
		{"the expiry day itself is still valid", "2027-01-01", LevelUrgent, 0, 7},
		{"the day after is expired", "2026-12-31", LevelExpired, -1, 0},
		{"a week overdue", "2026-12-25", LevelExpired, -7, 0},
		{"a year overdue", "2026-01-01", LevelExpired, -365, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := StatusOf(MustParseDate(c.expiresOn), asOf, false, tiers)
			if got.Level != c.wantLevel {
				t.Errorf("level = %q, want %q", got.Level, c.wantLevel)
			}
			if got.DaysLeft != c.wantDays {
				t.Errorf("daysLeft = %d, want %d", got.DaysLeft, c.wantDays)
			}
			if got.TierDays != c.wantTier {
				t.Errorf("tierDays = %d, want %d", got.TierDays, c.wantTier)
			}
			if got.Label == "" {
				t.Error("every status needs a label the owner can read")
			}
		})
	}
}

// TestStatusOfBoundariesAcrossALeapDay repeats the boundary walk where the
// arithmetic has to cross 29 February, because an off-by-one in the day count
// hides there and nowhere else.
func TestStatusOfBoundariesAcrossALeapDay(t *testing.T) {
	// 2028 is a leap year, so 1 January to 29 February is 59 days.
	asOf := MustParseDate("2028-01-01")
	tiers := DefaultTiers()

	cases := []struct {
		name      string
		expiresOn string
		wantLevel Level
		wantDays  int
	}{
		{"29 February is 59 days from 1 January in a leap year", "2028-02-29", LevelNotice, 59},
		{"1 March is 60 days away, exactly the first reminder", "2028-03-01", LevelNotice, 60},
		{"2 March is 61 days away and clear", "2028-03-02", LevelOK, 61},
		{"the same dates in a common year are a day nearer", "2028-01-31", LevelWarn, 30},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := StatusOf(MustParseDate(c.expiresOn), asOf, false, tiers)
			if got.Level != c.wantLevel || got.DaysLeft != c.wantDays {
				t.Fatalf("StatusOf(%s) = %s at %d days, want %s at %d",
					c.expiresOn, got.Level, got.DaysLeft, c.wantLevel, c.wantDays)
			}
		})
	}

	// In a common year the same span is one day shorter, which is the whole
	// point of counting in days rather than in months.
	common := StatusOf(MustParseDate("2027-03-01"), MustParseDate("2027-01-01"), false, tiers)
	if common.DaysLeft != 59 {
		t.Fatalf("1 January to 1 March in a common year = %d days, want 59", common.DaysLeft)
	}
	if common.Level != LevelNotice {
		t.Fatalf("level = %s, want %s", common.Level, LevelNotice)
	}
}

// TestStatusOfOnAMonthEndExpiry covers the most common real expiry date in both
// markets: a permit that runs to the last day of a month.
func TestStatusOfOnAMonthEndExpiry(t *testing.T) {
	cases := []struct {
		name      string
		asOf      string
		expiresOn string
		wantLevel Level
		wantDays  int
	}{
		{"a Philippine permit on 31 December, seen on 1 December", "2026-12-01", "2026-12-31", LevelWarn, 30},
		{"the same, seen on 24 December", "2026-12-24", "2026-12-31", LevelUrgent, 7},
		{"the same, seen on 31 December — still valid", "2026-12-31", "2026-12-31", LevelUrgent, 0},
		{"the same, seen on 1 January — lapsed", "2027-01-01", "2026-12-31", LevelExpired, -1},
		{"an Indian trade licence to 31 March, seen on 1 February", "2027-02-01", "2027-03-31", LevelNotice, 58},
		{"a licence to 29 February seen on the 29th", "2028-02-29", "2028-02-29", LevelUrgent, 0},
		{"a licence to 29 February seen on 1 March", "2028-03-01", "2028-02-29", LevelExpired, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := StatusOf(MustParseDate(c.expiresOn), MustParseDate(c.asOf), false, DefaultTiers())
			if got.Level != c.wantLevel || got.DaysLeft != c.wantDays {
				t.Fatalf("= %s at %d days, want %s at %d", got.Level, got.DaysLeft, c.wantLevel, c.wantDays)
			}
		})
	}
}

func TestStatusOfTheTwoNonDateLevels(t *testing.T) {
	asOf := MustParseDate("2027-01-01")

	noExpiry := StatusOf(Date{}, asOf, true, DefaultTiers())
	if noExpiry.Level != LevelNoExpiry {
		t.Fatalf("a registration that does not lapse = %s, want %s", noExpiry.Level, LevelNoExpiry)
	}
	if noExpiry.Colour != "grey" || noExpiry.NeedsAction() {
		t.Fatal("a no-expiry registration needs no action and should not read as a warning")
	}
	// A no-expiry flag wins even if a date somehow survived on the record.
	stillNoExpiry := StatusOf(MustParseDate("2020-01-01"), asOf, true, DefaultTiers())
	if stillNoExpiry.Level != LevelNoExpiry {
		t.Fatalf("the no-expiry flag must win: got %s", stillNoExpiry.Level)
	}

	unknown := StatusOf(Date{}, asOf, false, DefaultTiers())
	if unknown.Level != LevelUnknown {
		t.Fatalf("a permit with no date recorded = %s, want %s", unknown.Level, LevelUnknown)
	}
	if !unknown.NeedsAction() {
		t.Fatal("a permit with no expiry date recorded needs action: nobody knows if it is valid")
	}
	if unknown.Expired() {
		t.Fatal("not knowing the date is not the same as being expired")
	}
}

func TestStatusNeedsAction(t *testing.T) {
	cases := []struct {
		level Level
		want  bool
	}{
		{LevelExpired, true},
		{LevelUrgent, true},
		{LevelWarn, true},
		{LevelNotice, true},
		{LevelUnknown, true},
		{LevelOK, false},
		{LevelNoExpiry, false},
	}
	for _, c := range cases {
		if got := (Status{Level: c.level}).NeedsAction(); got != c.want {
			t.Errorf("NeedsAction for %s = %v, want %v", c.level, got, c.want)
		}
	}
}

func TestTiersNormaliseSortsThemWidestFirst(t *testing.T) {
	cases := []struct {
		name string
		in   Tiers
		want Tiers
	}{
		{"already in order", Tiers{60, 30, 7}, Tiers{60, 30, 7}},
		{
			"typed in the order they read — this must not silently never fire",
			Tiers{NoticeDays: 7, WarnDays: 30, UrgentDays: 60},
			Tiers{NoticeDays: 60, WarnDays: 30, UrgentDays: 7},
		},
		{"jumbled", Tiers{30, 7, 60}, Tiers{60, 30, 7}},
		{"all the same is allowed: one reminder, three times", Tiers{14, 14, 14}, Tiers{14, 14, 14}},
		{"a negative tier is clamped to nought", Tiers{60, 30, -5}, Tiers{60, 30, 0}},
		{"an absurd tier is clamped to the ceiling", Tiers{99999, 30, 7}, Tiers{MaxTierDays, 30, 7}},
		{"all nought means only an expiry-day warning", Tiers{0, 0, 0}, Tiers{0, 0, 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.in.Normalise(); got != c.want {
				t.Fatalf("Normalise(%+v) = %+v, want %+v", c.in, got, c.want)
			}
		})
	}
}

func TestStatusOfUsesNormalisedTiers(t *testing.T) {
	// Tiers typed in the wrong order must still produce the right level.
	jumbled := Tiers{NoticeDays: 7, WarnDays: 60, UrgentDays: 30}
	got := StatusOf(MustParseDate("2027-03-02"), MustParseDate("2027-01-01"), false, jumbled)
	if got.Level != LevelNotice || got.TierDays != 60 {
		t.Fatalf("60 days out with jumbled tiers = %s at tier %d, want %s at 60", got.Level, got.TierDays, LevelNotice)
	}
}

func TestTiersWithAllTiersAtNought(t *testing.T) {
	zero := Tiers{}
	onTheDay := StatusOf(MustParseDate("2027-01-01"), MustParseDate("2027-01-01"), false, zero)
	if onTheDay.Level != LevelUrgent {
		t.Fatalf("with every tier at nought the expiry day itself = %s, want %s", onTheDay.Level, LevelUrgent)
	}
	dayBefore := StatusOf(MustParseDate("2027-01-02"), MustParseDate("2027-01-01"), false, zero)
	if dayBefore.Level != LevelOK {
		t.Fatalf("with every tier at nought the day before = %s, want %s", dayBefore.Level, LevelOK)
	}
}

func TestTiersValidate(t *testing.T) {
	cases := []struct {
		name    string
		in      Tiers
		wantErr bool
	}{
		{"the defaults", DefaultTiers(), false},
		{"all nought", Tiers{}, false},
		{"one day", Tiers{1, 1, 1}, false},
		{"a negative first reminder", Tiers{-1, 30, 7}, true},
		{"a negative final reminder", Tiers{60, 30, -1}, true},
		{"beyond the ceiling", Tiers{MaxTierDays + 1, 30, 7}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.in.Validate()
			if c.wantErr != (err != nil) {
				t.Fatalf("Validate(%+v) returned %v, wantErr %v", c.in, err, c.wantErr)
			}
		})
	}
}

func TestTiersDays(t *testing.T) {
	got := Tiers{NoticeDays: 7, WarnDays: 60, UrgentDays: 30}.Days()
	want := []int{60, 30, 7}
	if len(got) != len(want) {
		t.Fatalf("Days() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Days() = %v, want %v", got, want)
		}
	}
}

func TestDefaultTiersAreTheDocumentedOnes(t *testing.T) {
	got := DefaultTiers()
	if got.NoticeDays != 60 || got.WarnDays != 30 || got.UrgentDays != 7 {
		t.Fatalf("the shipped tiers are %+v; the documentation says 60, 30 and 7", got)
	}
}

func TestSummariseCountsEveryLevel(t *testing.T) {
	asOf := MustParseDate("2027-01-10")
	register := New()
	add := func(kind Kind, expires string, noExpiry bool) {
		permit := Permit{
			Kind:      kind,
			Label:     string(kind),
			Authority: "test",
			NoExpiry:  noExpiry,
		}
		if expires != "" {
			permit.ExpiresOn = MustParseDate(expires)
		}
		if _, err := register.Put(permit); err != nil {
			t.Fatalf("Put(%s): %v", kind, err)
		}
	}
	add("a-expired", "2026-12-01", false)
	add("b-urgent", "2027-01-12", false)
	add("c-warn", "2027-02-01", false)
	add("d-notice", "2027-03-01", false)
	add("e-ok", "2028-01-01", false)
	add("f-unknown", "", false)
	add("g-no-expiry", "", true)

	counts := Summarise(register.Board(asOf))
	want := Counts{Expired: 1, Urgent: 1, Warn: 1, Notice: 1, OK: 1, Unknown: 1, NoExpiry: 1, Total: 7}
	if counts != want {
		t.Fatalf("Summarise = %+v, want %+v", counts, want)
	}
}

func TestBoardOrdersWorstFirst(t *testing.T) {
	asOf := MustParseDate("2027-01-10")
	register := New()
	for _, c := range []struct {
		kind    Kind
		expires string
	}{
		{"clear", "2028-06-01"},
		{"badly-overdue", "2026-06-01"},
		{"just-overdue", "2027-01-09"},
		{"urgent", "2027-01-12"},
	} {
		if _, err := register.Put(Permit{
			Kind: c.kind, Label: string(c.kind), Authority: "test",
			ExpiresOn: MustParseDate(c.expires),
		}); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	rows := register.Board(asOf)
	want := []Kind{"badly-overdue", "just-overdue", "urgent", "clear"}
	if len(rows) != len(want) {
		t.Fatalf("board has %d rows, want %d", len(rows), len(want))
	}
	for i, kind := range want {
		if rows[i].Permit.Kind != kind {
			t.Fatalf("board row %d is %s, want %s (board: %v)", i, rows[i].Permit.Kind, kind, kindsOf(rows))
		}
	}
}

func kindsOf(rows []BoardRow) []Kind {
	out := make([]Kind, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Permit.Kind)
	}
	return out
}

func TestStatusLabelsReadLikeEnglish(t *testing.T) {
	asOf := NewDate(2027, time.January, 1)
	cases := []struct {
		expiresOn string
		want      string
	}{
		{"2027-01-01", "expires today"},
		{"2027-01-02", "1 day left"},
		{"2027-01-03", "2 days left"},
		{"2026-12-31", "expired 1 day ago"},
		{"2026-12-30", "expired 2 days ago"},
	}
	for _, c := range cases {
		got := StatusOf(MustParseDate(c.expiresOn), asOf, false, DefaultTiers())
		if got.Label != c.want {
			t.Errorf("label for %s = %q, want %q", c.expiresOn, got.Label, c.want)
		}
	}
}
