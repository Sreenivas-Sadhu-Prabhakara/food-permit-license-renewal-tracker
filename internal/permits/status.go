package permits

import (
	"fmt"
	"sort"
)

// Level is how close one permit is to lapsing. The five live levels escalate:
// a permit crosses ok, then notice, then warn, then urgent, then expired, and
// never goes back without a renewal.
type Level string

const (
	// LevelNoExpiry is a registration that does not lapse — a regular GST
	// registration, a BIR certificate of registration. It is on the board so
	// the owner can see it is there, not because it needs watching.
	LevelNoExpiry Level = "noExpiry"
	// LevelUnknown is a permit with no expiry date recorded. It is not safe to
	// call it valid, so it reads amber and, if the permit blocks, it blocks.
	LevelUnknown Level = "unknown"
	LevelOK      Level = "ok"
	LevelNotice  Level = "notice"
	LevelWarn    Level = "warn"
	LevelUrgent  Level = "urgent"
	LevelExpired Level = "expired"
)

// Tiers are the escalating reminder thresholds in days before expiry. The
// defaults are sixty, thirty and seven; the owner sets their own, because a
// permit that needs an inspection and one that needs a form have very
// different useful warning times.
type Tiers struct {
	NoticeDays int `json:"noticeDays"`
	WarnDays   int `json:"warnDays"`
	UrgentDays int `json:"urgentDays"`
}

// DefaultTiers is 60, 30 and 7 days.
func DefaultTiers() Tiers {
	return Tiers{NoticeDays: DefaultNoticeDays, WarnDays: DefaultWarnDays, UrgentDays: DefaultUrgentDays}
}

// Validate rejects a tier set that cannot mean anything.
func (t Tiers) Validate() error {
	type named struct {
		label string
		days  int
	}
	for _, n := range []named{
		{"first reminder", t.NoticeDays},
		{"second reminder", t.WarnDays},
		{"final reminder", t.UrgentDays},
	} {
		if n.days < 0 {
			return fmt.Errorf("the %s cannot be a negative number of days", n.label)
		}
		if n.days > MaxTierDays {
			return fmt.Errorf("the %s at %d days is further ahead than this tool will look (%d days)", n.label, n.days, MaxTierDays)
		}
	}
	return nil
}

// Normalise sorts the three tiers so the earliest reminder is always the
// widest. An owner who types 7, 30, 60 into the three boxes in the order they
// read them meant the same thing as 60, 30, 7, and a board that silently
// never fired a reminder because of it would be the worst kind of bug.
func (t Tiers) Normalise() Tiers {
	days := []int{t.NoticeDays, t.WarnDays, t.UrgentDays}
	for i := range days {
		if days[i] < 0 {
			days[i] = 0
		}
		if days[i] > MaxTierDays {
			days[i] = MaxTierDays
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(days)))
	return Tiers{NoticeDays: days[0], WarnDays: days[1], UrgentDays: days[2]}
}

// Days lists the tiers from widest to narrowest, for the interface to print.
func (t Tiers) Days() []int {
	n := t.Normalise()
	return []int{n.NoticeDays, n.WarnDays, n.UrgentDays}
}

// Status is the verdict on one expiry date as of one day.
type Status struct {
	Level Level `json:"level"`
	// DaysLeft is whole days from asOf to the expiry date: zero on the expiry
	// day itself, and negative once it has passed.
	DaysLeft int `json:"daysLeft"`
	// TierDays is the reminder tier that fired, or zero if none did.
	TierDays int    `json:"tierDays"`
	Label    string `json:"label"`
	Colour   string `json:"colour"`
}

// StatusOf is the whole alert rule, and the only place it is written.
//
// A permit is valid through the whole of its expiry date — a licence that says
// "valid until 31 December" is good on the 31st — so the expiry day itself is
// nought days left and inside the final reminder tier, not expired. It becomes
// expired on the following day.
//
// The tiers are inclusive at their boundary: with the default tiers, thirty
// days left is the second reminder and not the first, and seven days left is
// the final reminder and not the second.
func StatusOf(expiresOn Date, asOf Date, noExpiry bool, tiers Tiers) Status {
	if noExpiry {
		return Status{
			Level:  LevelNoExpiry,
			Label:  "does not expire",
			Colour: "grey",
		}
	}
	if expiresOn.Zero() {
		return Status{
			Level:  LevelUnknown,
			Label:  "no expiry date recorded",
			Colour: "amber",
		}
	}

	t := tiers.Normalise()
	left := DaysBetween(asOf, expiresOn)

	switch {
	case left < 0:
		return Status{
			Level:    LevelExpired,
			DaysLeft: left,
			Label:    fmt.Sprintf("expired %s", plural(-left, "day", "days")+" ago"),
			Colour:   "red",
		}
	case left <= t.UrgentDays:
		return Status{
			Level:    LevelUrgent,
			DaysLeft: left,
			TierDays: t.UrgentDays,
			Label:    leftLabel(left),
			Colour:   "red",
		}
	case left <= t.WarnDays:
		return Status{
			Level:    LevelWarn,
			DaysLeft: left,
			TierDays: t.WarnDays,
			Label:    leftLabel(left),
			Colour:   "amber",
		}
	case left <= t.NoticeDays:
		return Status{
			Level:    LevelNotice,
			DaysLeft: left,
			TierDays: t.NoticeDays,
			Label:    leftLabel(left),
			Colour:   "amber",
		}
	default:
		return Status{
			Level:    LevelOK,
			DaysLeft: left,
			Label:    leftLabel(left),
			Colour:   "green",
		}
	}
}

func leftLabel(days int) string {
	if days == 0 {
		return "expires today"
	}
	return plural(days, "day", "days") + " left"
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// NeedsAction reports whether a status is one the owner has to do something
// about: anything from the first reminder onwards, plus a permit whose expiry
// nobody recorded.
func (s Status) NeedsAction() bool {
	switch s.Level {
	case LevelNotice, LevelWarn, LevelUrgent, LevelExpired, LevelUnknown:
		return true
	default:
		return false
	}
}

// Expired is the narrower question CanTradeToday asks.
func (s Status) Expired() bool { return s.Level == LevelExpired }

// rank orders the board: the worst thing first, then the soonest expiry.
func (s Status) rank() int {
	switch s.Level {
	case LevelExpired:
		return 0
	case LevelUrgent:
		return 1
	case LevelWarn:
		return 2
	case LevelUnknown:
		return 3
	case LevelNotice:
		return 4
	case LevelOK:
		return 5
	case LevelNoExpiry:
		return 6
	default:
		return 7
	}
}

// Counts summarises a board for the strip of figures above it.
type Counts struct {
	Expired  int `json:"expired"`
	Urgent   int `json:"urgent"`
	Warn     int `json:"warn"`
	Notice   int `json:"notice"`
	OK       int `json:"ok"`
	Unknown  int `json:"unknown"`
	NoExpiry int `json:"noExpiry"`
	Total    int `json:"total"`
}

// Summarise counts the rows by level.
func Summarise(rows []BoardRow) Counts {
	var c Counts
	for _, row := range rows {
		c.Total++
		switch row.Status.Level {
		case LevelExpired:
			c.Expired++
		case LevelUrgent:
			c.Urgent++
		case LevelWarn:
			c.Warn++
		case LevelNotice:
			c.Notice++
		case LevelOK:
			c.OK++
		case LevelUnknown:
			c.Unknown++
		case LevelNoExpiry:
			c.NoExpiry++
		}
	}
	return c
}
