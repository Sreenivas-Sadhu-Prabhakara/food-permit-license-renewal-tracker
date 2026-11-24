package permits

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    Date
		wantErr bool
	}{
		{"an ordinary day", "2026-11-24", NewDate(2026, time.November, 24), false},
		{"an empty string is no date, not an error", "", Date{}, false},
		{"whitespace is also no date", "   ", Date{}, false},
		{"surrounding whitespace is trimmed", " 2026-01-20 ", NewDate(2026, time.January, 20), false},
		{"29 February in a leap year", "2028-02-29", NewDate(2028, time.February, 29), false},
		{"29 February in a common year is refused", "2027-02-29", Date{}, true},
		{"30 February is refused", "2026-02-30", Date{}, true},
		{"31 April is refused", "2026-04-31", Date{}, true},
		{"a century year divisible by 400 is a leap year", "2000-02-29", NewDate(2000, time.February, 29), false},
		{"a century year not divisible by 400 is not", "1900-02-29", Date{}, true},
		{"day/month/year order is refused", "24-11-2026", Date{}, true},
		{"a slashed date is refused", "2026/11/24", Date{}, true},
		{"a month of 13 is refused", "2026-13-01", Date{}, true},
		{"a day of 00 is refused", "2026-11-00", Date{}, true},
		{"words are refused", "next tuesday", Date{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseDate(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseDate(%q) = %v, wanted an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDate(%q) returned %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("ParseDate(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestDateStringAndZero(t *testing.T) {
	if got := NewDate(2026, time.January, 5).String(); got != "2026-01-05" {
		t.Fatalf("String() = %q, want 2026-01-05 with the leading zeroes", got)
	}
	var unset Date
	if !unset.Zero() {
		t.Fatal("a Date that was never set must report Zero")
	}
	if got := unset.String(); got != "" {
		t.Fatalf("an unset date renders as %q, want the empty string", got)
	}
	if NewDate(2026, time.January, 1).Zero() {
		t.Fatal("a real date must not report Zero")
	}
}

func TestDaysInMonth(t *testing.T) {
	cases := []struct {
		year  int
		month time.Month
		want  int
	}{
		{2026, time.January, 31},
		{2026, time.February, 28},
		{2028, time.February, 29},
		{2027, time.February, 28},
		{2000, time.February, 29},
		{1900, time.February, 28},
		{2100, time.February, 28},
		{2026, time.April, 30},
		{2026, time.December, 31},
	}
	for _, c := range cases {
		if got := DaysInMonth(c.year, c.month); got != c.want {
			t.Errorf("DaysInMonth(%d, %v) = %d, want %d", c.year, c.month, got, c.want)
		}
	}
}

func TestAddMonthsClampsToMonthEnd(t *testing.T) {
	cases := []struct {
		name   string
		from   string
		months int
		want   string
	}{
		{"31 January plus one month is 28 February, not 3 March", "2026-01-31", 1, "2026-02-28"},
		{"the same in a leap year is the 29th", "2028-01-31", 1, "2028-02-29"},
		{"30 November plus three months", "2026-11-30", 3, "2027-02-28"},
		{"31 March plus one month is 30 April", "2026-03-31", 1, "2026-04-30"},
		{"29 February plus twelve months lands on the 28th", "2028-02-29", 12, "2029-02-28"},
		{"29 February plus forty-eight months is the 29th again", "2028-02-29", 48, "2032-02-29"},
		{"a day that exists everywhere is untouched", "2026-01-15", 1, "2026-02-15"},
		{"twelve months is the same date next year", "2026-06-10", 12, "2027-06-10"},
		{"sixty months is five years on", "2026-06-10", 60, "2031-06-10"},
		{"crossing a year boundary", "2026-11-30", 2, "2027-01-30"},
		{"nought months changes nothing", "2026-11-30", 0, "2026-11-30"},
		{"going back a month clamps too", "2026-03-31", -1, "2026-02-28"},
		{"going back across a year", "2026-01-15", -1, "2025-12-15"},
		{"going back twelve months", "2028-02-29", -12, "2027-02-28"},
		{"going back thirteen months from January", "2026-01-31", -13, "2024-12-31"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := AddMonths(MustParseDate(c.from), c.months)
			if got.String() != c.want {
				t.Fatalf("AddMonths(%s, %d) = %s, want %s", c.from, c.months, got, c.want)
			}
		})
	}
}

func TestAddMonthsLeavesTheZeroDateAlone(t *testing.T) {
	if got := AddMonths(Date{}, 12); !got.Zero() {
		t.Fatalf("AddMonths on an unset date gave %v, want it left unset", got)
	}
	if got := AddDays(Date{}, 5); !got.Zero() {
		t.Fatalf("AddDays on an unset date gave %v, want it left unset", got)
	}
}

func TestAddDays(t *testing.T) {
	cases := []struct {
		name string
		from string
		days int
		want string
	}{
		{"across the end of February in a leap year", "2028-02-28", 1, "2028-02-29"},
		{"and on into March", "2028-02-29", 1, "2028-03-01"},
		{"across the end of February in a common year", "2027-02-28", 1, "2027-03-01"},
		{"across a new year", "2026-12-31", 1, "2027-01-01"},
		{"backwards across a new year", "2027-01-01", -1, "2026-12-31"},
		{"the January renewal deadline less a fortnight", "2027-01-20", -14, "2027-01-06"},
		{"the deadline less thirty-four days reaches back into December", "2027-01-20", -34, "2026-12-17"},
		{"nought days changes nothing", "2026-05-05", 0, "2026-05-05"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AddDays(MustParseDate(c.from), c.days); got.String() != c.want {
				t.Fatalf("AddDays(%s, %d) = %s, want %s", c.from, c.days, got, c.want)
			}
		})
	}
}

func TestDaysBetween(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want int
	}{
		{"the same day", "2026-11-24", "2026-11-24", 0},
		{"one day forward", "2026-11-24", "2026-11-25", 1},
		{"one day back", "2026-11-25", "2026-11-24", -1},
		{"across 29 February", "2028-02-28", "2028-03-01", 2},
		{"across 28 February in a common year", "2027-02-28", "2027-03-01", 1},
		{"a leap year is 366 days", "2028-01-01", "2029-01-01", 366},
		{"a common year is 365", "2026-01-01", "2027-01-01", 365},
		{"a full quarter", "2027-01-20", "2027-04-20", 90},
		{"across a daylight-saving change in the owner's zone is still whole days", "2026-03-01", "2026-04-01", 31},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DaysBetween(MustParseDate(c.a), MustParseDate(c.b)); got != c.want {
				t.Fatalf("DaysBetween(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
			}
		})
	}
	if got := DaysBetween(Date{}, MustParseDate("2026-01-01")); got != 0 {
		t.Fatalf("DaysBetween with an unset date = %d, want 0", got)
	}
}

func TestExpiryFrom(t *testing.T) {
	cases := []struct {
		name   string
		issued string
		months int
		want   string
	}{
		{"a year from 1 January runs through 31 December", "2026-01-01", 12, "2026-12-31"},
		{"a year from 21 January", "2026-01-21", 12, "2027-01-20"},
		{"five years, as a DTI business name runs", "2026-03-15", 60, "2031-03-14"},
		{"one month from 31 January ends at the end of February", "2026-01-31", 1, "2026-02-28"},
		{"and on the 29th in a leap year", "2028-01-31", 1, "2028-02-29"},
		{"a year from 29 February ends on 28 February", "2028-02-29", 12, "2029-02-28"},
		{"four years from 29 February ends on the 28th, a day short of the leap day", "2028-02-29", 48, "2032-02-28"},
		{"a year from 1 March", "2026-03-01", 12, "2027-02-28"},
		{"a year from 1 March into a leap year", "2027-03-01", 12, "2028-02-29"},
		{"three months from 30 November", "2026-11-30", 3, "2027-02-28"},
		{"two years", "2026-07-01", 24, "2028-06-30"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExpiryFrom(MustParseDate(c.issued), c.months)
			if got.String() != c.want {
				t.Fatalf("ExpiryFrom(%s, %d) = %s, want %s", c.issued, c.months, got, c.want)
			}
		})
	}
}

func TestExpiryFromRefusesNothingToWorkWith(t *testing.T) {
	if got := ExpiryFrom(Date{}, 12); !got.Zero() {
		t.Fatalf("ExpiryFrom with no issue date = %v, want the zero date", got)
	}
	if got := ExpiryFrom(MustParseDate("2026-01-01"), 0); !got.Zero() {
		t.Fatalf("ExpiryFrom with no period = %v, want the zero date", got)
	}
	if got := ExpiryFrom(MustParseDate("2026-01-01"), -6); !got.Zero() {
		t.Fatalf("ExpiryFrom with a negative period = %v, want the zero date", got)
	}
}

func TestEndOfMonth(t *testing.T) {
	cases := []struct{ from, want string }{
		{"2026-02-01", "2026-02-28"},
		{"2028-02-01", "2028-02-29"},
		{"2026-12-15", "2026-12-31"},
		{"2026-04-30", "2026-04-30"},
	}
	for _, c := range cases {
		if got := EndOfMonth(MustParseDate(c.from)); got.String() != c.want {
			t.Errorf("EndOfMonth(%s) = %s, want %s", c.from, got, c.want)
		}
	}
	if got := EndOfMonth(Date{}); !got.Zero() {
		t.Errorf("EndOfMonth of an unset date = %v, want it left unset", got)
	}
}

func TestMonthsBetweenCountsAFractionAsAWholeMonth(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want int
	}{
		{"paid on the day it was due is not late at all", "2027-01-20", "2027-01-20", 0},
		{"paid before it was due is not late", "2027-01-20", "2027-01-05", 0},
		{"one day late is one month", "2027-01-20", "2027-01-21", 1},
		{"a fortnight late is still one month", "2027-01-20", "2027-02-03", 1},
		{"exactly one month late is one month", "2027-01-20", "2027-02-20", 1},
		{"a month and a day is two months", "2027-01-20", "2027-02-21", 2},
		{"a year late is twelve months", "2027-01-20", "2028-01-20", 12},
		{"a year and a day is thirteen", "2027-01-20", "2028-01-21", 13},
		{"three years late is thirty-six months", "2027-01-20", "2030-01-20", 36},
		{"beyond the cap is still counted here; the cap is applied when charging", "2027-01-20", "2031-01-20", 48},
		{"a month-end start does not over-count", "2026-01-31", "2026-02-28", 1},
		{"and the day after the month end does", "2026-01-31", "2026-03-01", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MonthsBetween(MustParseDate(c.a), MustParseDate(c.b)); got != c.want {
				t.Fatalf("MonthsBetween(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestDateJSONRoundTrip(t *testing.T) {
	type holder struct {
		On Date `json:"on"`
	}
	raw, err := json.Marshal(holder{On: NewDate(2028, time.February, 29)})
	if err != nil {
		t.Fatalf("marshalling a date: %v", err)
	}
	if string(raw) != `{"on":"2028-02-29"}` {
		t.Fatalf("a date marshalled as %s, want the ISO string", raw)
	}

	var back holder
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshalling a date: %v", err)
	}
	if back.On != NewDate(2028, time.February, 29) {
		t.Fatalf("round trip gave %v", back.On)
	}

	// An unset date has to survive the trip as unset, so a permit with no
	// expiry recorded does not come back as the year nought.
	raw, err = json.Marshal(holder{})
	if err != nil {
		t.Fatalf("marshalling an unset date: %v", err)
	}
	if string(raw) != `{"on":""}` {
		t.Fatalf("an unset date marshalled as %s, want an empty string", raw)
	}
	back = holder{On: NewDate(2026, time.January, 1)}
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshalling an empty date: %v", err)
	}
	if !back.On.Zero() {
		t.Fatalf("an empty date came back as %v, want unset", back.On)
	}
}

func TestDateJSONReadsTheOlderObjectShape(t *testing.T) {
	// A data file written by an earlier build held the three numbers. It must
	// still load, because the owner's register is the point of the tool.
	var d Date
	if err := json.Unmarshal([]byte(`{"year":2027,"month":3,"day":31}`), &d); err != nil {
		t.Fatalf("unmarshalling the object shape: %v", err)
	}
	if d.String() != "2027-03-31" {
		t.Fatalf("the object shape loaded as %s", d)
	}
	if err := json.Unmarshal([]byte(`{"year":0,"month":0,"day":0}`), &d); err != nil {
		t.Fatalf("unmarshalling a zero object: %v", err)
	}
	if !d.Zero() {
		t.Fatalf("a zero object loaded as %s, want unset", d)
	}
	if err := json.Unmarshal([]byte(`"not-a-date"`), &d); err == nil {
		t.Fatal("a bad date string must be refused")
	}
	if err := json.Unmarshal([]byte(`42`), &d); err == nil {
		t.Fatal("a number is not a date and must be refused")
	}
}

func TestDateOrdering(t *testing.T) {
	early := MustParseDate("2026-01-20")
	late := MustParseDate("2027-01-20")
	if !early.Before(late) || early.After(late) {
		t.Fatal("Before/After disagree on two dates a year apart")
	}
	if !early.Equal(MustParseDate("2026-01-20")) {
		t.Fatal("Equal must hold for the same calendar day")
	}
	if early.Before(early) || early.After(early) {
		t.Fatal("a date is neither before nor after itself")
	}
}

func TestTodayIsAToday(t *testing.T) {
	now := time.Now()
	got := Today()
	if got.Year != now.Year() || got.Month != now.Month() || got.Day != now.Day() {
		t.Fatalf("Today() = %v, want the machine's own day %04d-%02d-%02d", got, now.Year(), now.Month(), now.Day())
	}
}

func TestMustParseDatePanicsOnABadLiteral(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustParseDate must panic on a date that does not exist")
		}
	}()
	MustParseDate("2027-02-29")
}
