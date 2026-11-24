package permits

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Date is a calendar day with no time and no zone. Every date in this tool is
// a day printed on a piece of paper — "valid until 31 December 2026" — and a
// clock or a zone has no business anywhere near it. Comparing two time.Time
// values that were parsed in different zones is the classic way to be a day
// out on an expiry, so the type carries only the three numbers.
type Date struct {
	Year  int        `json:"year"`
	Month time.Month `json:"month"`
	Day   int        `json:"day"`
}

// ISO is the only format read or written: YYYY-MM-DD, which is what an
// <input type="date"> sends and what a spreadsheet sorts correctly.
const ISO = "2006-01-02"

// NewDate normalises as it builds, so NewDate(2026, 13, 1) is January 2027 and
// NewDate(2026, 2, 30) is 2 March. Callers that need the stricter behaviour use
// ParseDate, which rejects a day that is not in its month.
func NewDate(year int, month time.Month, day int) Date {
	return fromTime(time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
}

func fromTime(t time.Time) Date {
	return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}
}

// Time is the midnight UTC instant of the day, used only as the engine for
// date arithmetic. It never escapes this package as a time.
func (d Date) Time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// Zero reports whether the date was never set. A zero date means "not
// recorded", which is a meaningful state: a permit whose expiry nobody typed
// in is not a permit anyone can rely on.
func (d Date) Zero() bool { return d.Year == 0 && d.Month == 0 && d.Day == 0 }

// String renders YYYY-MM-DD, or an empty string for a date that was never set.
func (d Date) String() string {
	if d.Zero() {
		return ""
	}
	return d.Time().Format(ISO)
}

// Before, After and Equal compare calendar days.
func (d Date) Before(o Date) bool { return d.Time().Before(o.Time()) }
func (d Date) After(o Date) bool  { return d.Time().After(o.Time()) }
func (d Date) Equal(o Date) bool  { return d == o }

// ParseDate reads YYYY-MM-DD and rejects a day that does not exist in its
// month, so 2026-02-30 and 2027-02-29 are errors rather than quietly becoming
// March. An empty string parses to the zero date without complaint, because
// "no expiry recorded" is a legitimate answer from a form.
func ParseDate(raw string) (Date, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Date{}, nil
	}
	t, err := time.ParseInLocation(ISO, raw, time.UTC)
	if err != nil {
		return Date{}, fmt.Errorf("%q is not a date in YYYY-MM-DD form", raw)
	}
	// time.Parse normalises an impossible day instead of refusing it, so the
	// only reliable check is to format it back and compare.
	if t.Format(ISO) != raw {
		return Date{}, fmt.Errorf("%q is not a real date", raw)
	}
	return fromTime(t), nil
}

// MustParseDate is for tests and for the presets, where the date is a literal
// in this repository and a bad one is a bug rather than bad input.
func MustParseDate(raw string) Date {
	d, err := ParseDate(raw)
	if err != nil {
		panic(err)
	}
	return d
}

// MarshalJSON writes the date as the ISO string the UI expects, and the zero
// date as "" rather than a 0000-00-00 that no date input will accept.
func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

// UnmarshalJSON accepts the ISO string, and also an object with year, month and
// day for a data file written by an older build of the tool.
func (d *Date) UnmarshalJSON(raw []byte) error {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		parsed, err := ParseDate(s)
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	}
	var shape struct {
		Year  int        `json:"year"`
		Month time.Month `json:"month"`
		Day   int        `json:"day"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		return fmt.Errorf("could not read a date from %s", string(raw))
	}
	if shape.Year == 0 && shape.Month == 0 && shape.Day == 0 {
		*d = Date{}
		return nil
	}
	*d = NewDate(shape.Year, shape.Month, shape.Day)
	return nil
}

// Today is the day on this machine, in this machine's own zone — the owner's
// local day is the one that matters for a deadline at the city hall.
func Today() Date {
	now := time.Now()
	return NewDate(now.Year(), now.Month(), now.Day())
}

// DaysBetween counts whole days from a to b, negative when b is earlier. It is
// exact across February in a leap year because it works in days of UTC
// midnight, where every day is the same length.
func DaysBetween(a, b Date) int {
	if a.Zero() || b.Zero() {
		return 0
	}
	return int(b.Time().Sub(a.Time()).Hours() / 24)
}

// AddDays moves a date by whole days.
func AddDays(d Date, days int) Date {
	if d.Zero() {
		return d
	}
	return fromTime(d.Time().AddDate(0, 0, days))
}

// AddMonths moves a date by whole months and clamps to the end of the target
// month instead of overflowing into the next one. Go's own AddDate would turn
// 31 January plus one month into 3 March, which is wrong for a permit: a
// licence issued on the 31st and valid for one month expires on the last day
// of February, not in March.
//
// The clamp is what makes a leap year behave: 31 January 2028 plus one month is
// 29 February 2028, and the same date in 2027 is 28 February 2027.
func AddMonths(d Date, months int) Date {
	if d.Zero() {
		return d
	}
	year := d.Year
	month := int(d.Month) + months
	// Normalise the month into 1..12, carrying into the year. Go's integer
	// division truncates towards zero, so a negative month needs the floor.
	year += (month - 1) / 12
	month = (month - 1) % 12
	if month < 0 {
		month += 12
		year--
	}
	month++

	day := d.Day
	if last := DaysInMonth(year, time.Month(month)); day > last {
		day = last
	}
	return Date{Year: year, Month: time.Month(month), Day: day}
}

// DaysInMonth is the length of one month, 29 for February in a leap year.
func DaysInMonth(year int, month time.Month) int {
	// The zeroth day of the next month is the last day of this one.
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// EndOfMonth is the last day of the month the date falls in.
func EndOfMonth(d Date) Date {
	if d.Zero() {
		return d
	}
	return Date{Year: d.Year, Month: d.Month, Day: DaysInMonth(d.Year, d.Month)}
}

// ExpiryFrom is the last day a permit issued on a day and valid for a whole
// number of months covers.
//
// The ordinary rule is the day before the anniversary: issued 1 January and
// valid twelve months means valid through 31 December. Where the anniversary
// has been clamped back because the target month is shorter, the clamped date
// is already the last day of that month and is the end of the period as it
// stands — a permit issued on 31 January and valid one month runs through 28
// February, or through the 29th in a leap year, and taking a further day off
// would short the holder.
func ExpiryFrom(issuedOn Date, months int) Date {
	if issuedOn.Zero() || months <= 0 {
		return Date{}
	}
	anniversary := AddMonths(issuedOn, months)
	if anniversary.Day != issuedOn.Day {
		return anniversary
	}
	return AddDays(anniversary, -1)
}

// MonthsBetween counts whole calendar months from a to b, used to price a late
// charge that runs per month of delay. A part month counts as a whole one,
// which is how a surcharge ordinance reads: "per month or fraction thereof".
func MonthsBetween(a, b Date) int {
	if a.Zero() || b.Zero() || !b.After(a) {
		return 0
	}
	months := (b.Year-a.Year)*12 + int(b.Month) - int(a.Month)
	if b.Day > a.Day {
		months++
	}
	if months < 1 {
		months = 1
	}
	return months
}
