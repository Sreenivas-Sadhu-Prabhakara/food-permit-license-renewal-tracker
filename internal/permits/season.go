package permits

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// The renewal season.
//
// Permits do not fall due one at a time. In the Philippines they fall due all
// at once: local taxes, fees and charges accrue on the first day of January and
// are payable within the first twenty days of January under sections 166 and
// 167 of the Local Government Code of 1991 (Republic Act No. 7160), so the
// barangay clearance, the sanitary permit, the fire certificate and the mayor's
// permit all come round together, in the same three weeks, at the same
// counters, behind everybody else in the city.
//
// Worse, they do not come round independently. The barangay clearance has to be
// in hand before the city will issue the business permit, and so does the fire
// safety inspection certificate. A business that queues for them in the wrong
// order spends January discovering it cannot do the next thing.
//
// So this view does three things a list of expiry dates cannot. It groups
// everything falling due in one window. It orders that group by what blocks
// what, saying for each link whether a law fixes the order or whether this tool
// is only assuming it. And it works backwards from the deadline through the
// chain to say when each one has to be started, which is how a food business
// finds out in December that the food handlers' medicals should already be
// under way.

// InclusionReason says why a permit is in the season.
type InclusionReason string

const (
	// DueInWindow is the ordinary case: it expires inside the window.
	DueInWindow InclusionReason = "dueInWindow"
	// AlreadyLapsed is a permit that expired before the window opened and is
	// still not renewed. It is pulled in because it has to be dealt with
	// first: everything downstream of it is stuck behind it.
	AlreadyLapsed InclusionReason = "alreadyLapsed"
	// DateNotRecorded is a permit that falls in this season by its nature —
	// a Philippine business permit renews in January whatever the register
	// says — but whose expiry date nobody has typed in.
	DateNotRecorded InclusionReason = "dateNotRecorded"
)

// PrereqRef is one thing a row is waiting on.
type PrereqRef struct {
	Kind  Kind   `json:"kind"`
	Label string `json:"label"`
	Basis Basis  `json:"basis"`
	Why   string `json:"why"`
	// InSeason is whether the prerequisite is itself one of the rows in this
	// plan. A prerequisite outside the plan is one that is not due — fine if
	// it is in order, a problem if it has lapsed.
	InSeason bool `json:"inSeason"`
	// Settled is whether the prerequisite is currently valid, as far as the
	// register knows.
	Settled bool `json:"settled"`
	// OnRegister is whether the prerequisite is on the register at all.
	OnRegister bool `json:"onRegister"`
}

// SeasonRow is one permit inside the plan, with its place in the order.
type SeasonRow struct {
	Permit    Permit          `json:"permit"`
	Status    Status          `json:"status"`
	Readiness Readiness       `json:"readiness"`
	Why       InclusionReason `json:"why"`

	// Wave is the step of the plan this permit belongs to, counting from one.
	// Everything in wave one can be started straight away; wave two waits on
	// wave one. Rows inside a wave are independent of each other and can be
	// run in parallel, which matters when two people are splitting the queue.
	Wave int `json:"wave"`
	// Order is the row's position in the whole plan, so a printed sheet can be
	// worked down the page.
	Order int `json:"order"`

	BlockedBy []PrereqRef `json:"blockedBy"`
	// Blocks names the permits in this plan that are waiting on this one.
	Blocks []Kind `json:"blocks"`

	LeadDays int `json:"leadDays"`
	// DownstreamLeadDays is the longest lead time of everything queued behind
	// this permit. It is why a five-day barangay clearance has to be started a
	// fortnight out: the business permit behind it needs its own ten days.
	DownstreamLeadDays int `json:"downstreamLeadDays"`
	// StartBy is the deadline less this permit's own lead time and the longest
	// chain behind it. Miss it and the deadline at the end of the chain goes
	// with it.
	StartBy Date `json:"startBy"`
	// Slack is days from the as-of date to StartBy. Negative means the start
	// date has already gone by.
	Slack int `json:"slack"`
	// Critical marks a row on the longest path to the deadline — the chain
	// with no room in it.
	Critical bool `json:"critical"`
}

// Season is the whole plan for one window at one site.
type Season struct {
	AsOf  Date   `json:"asOf"`
	From  Date   `json:"from"`
	To    Date   `json:"to"`
	Site  string `json:"site"`
	Title string `json:"title"`

	// DueBy is the date the plan is worked backwards from, and Basis says
	// where that date comes from — a statute, or a convention this tool is
	// assuming.
	DueBy      Date   `json:"dueBy"`
	DueByBasis Basis  `json:"dueByBasis"`
	DueByWhy   string `json:"dueByWhy"`

	Rows   []SeasonRow `json:"rows"`
	Waves  int         `json:"waves"`
	Counts Counts      `json:"counts"`

	// Cycles holds any prerequisite loop found in the graph. A loop cannot be
	// ordered, so the rows in it are put in a final wave and named here for
	// the owner to break. The shipped presets contain no loop; one can only
	// arrive from an edge somebody added.
	Cycles [][]Kind `json:"cycles"`

	Note string `json:"note"`
}

// PHJanuarySeason is the Philippine January rush for a calendar year.
//
// The window opens on 1 December of the previous year, because the permits that
// fall due in January are the ones that lapse on 31 December, and closes at the
// end of January. The plan is worked backwards from the twentieth, the end of
// the first statutory payment window under section 167.
func PHJanuarySeason(year int) (from, to, dueBy Date, title string, basis Basis, why string) {
	from = NewDate(year-1, time.December, 1)
	to = NewDate(year, time.January, 31)
	dueBy = NewDate(year, time.Month(PHRenewalMonth), PHRenewalDeadlineDay)
	title = fmt.Sprintf("Philippine January renewal season %d", year)
	basis = FromStatute
	why = "Local taxes, fees and charges accrue on the first day of January and are payable within the first twenty days of January, or of each subsequent quarter — Local Government Code of 1991 (Republic Act No. 7160), sections 166 and 167. " +
		"A sanggunian may extend the deadline for a justifiable reason and several cities do so by ordinance, so confirm the date your own city has set for the year."
	return
}

// INFinancialYearSeason is India's nearest equivalent, and it is a much weaker
// claim. There is no single Indian renewal date: FSSAI runs from whenever you
// last renewed, GST does not expire, and the municipal trade licence runs to
// whatever cycle the corporation's by-laws set. Many corporations do run it to
// the financial year and want the renewal by 31 March, which is why that is the
// default offered — but it is a convention and the tool says so.
func INFinancialYearSeason(year int) (from, to, dueBy Date, title string, basis Basis, why string) {
	from = NewDate(year, time.January, 1)
	to = NewDate(year, time.March, 31)
	dueBy = NewDate(year, time.March, 31)
	title = fmt.Sprintf("Financial year end, %d", year)
	basis = FromConvention
	why = "India has no single renewal date. Many municipal trade licences run to the financial year and are renewed by 31 March, which is the only date worth offering as a default — but your corporation sets its own, FSSAI runs from whenever you last renewed, and a regular GST registration does not expire at all. Set the window to whatever your own dates are."
	return
}

// SeasonWindow is a window the owner chose themselves, with the deadline they
// were given. A zero deadline is taken as the end of the window.
func (r *Register) SeasonWindow(site string, from, to, dueBy Date, asOf Date) Season {
	if dueBy.Zero() {
		dueBy = to
	}
	return r.planSeason(site, from, to, dueBy, asOf, "Renewals due "+from.String()+" to "+to.String(), FromOwner,
		"You set this window and this deadline.")
}

// PHSeason is the January plan for a site.
func (r *Register) PHSeason(site string, year int, asOf Date) Season {
	from, to, dueBy, title, basis, why := PHJanuarySeason(year)
	return r.planSeason(site, from, to, dueBy, asOf, title, basis, why)
}

// INSeason is the financial-year plan for a site.
func (r *Register) INSeason(site string, year int, asOf Date) Season {
	from, to, dueBy, title, basis, why := INFinancialYearSeason(year)
	return r.planSeason(site, from, to, dueBy, asOf, title, basis, why)
}

// planSeason is the whole sequencing rule, and the only place it is written.
func (r *Register) planSeason(site string, from, to, dueBy, asOf Date, title string, basis Basis, why string) Season {
	season := Season{
		AsOf:       asOf,
		From:       from,
		To:         to,
		Site:       site,
		Title:      title,
		DueBy:      dueBy,
		DueByBasis: basis,
		DueByWhy:   why,
		Rows:       []SeasonRow{},
		Cycles:     [][]Kind{},
	}

	// ---- 1. which permits are in this season ----

	selected := map[Kind]Permit{}
	reasons := map[Kind]InclusionReason{}
	for _, p := range r.Permits {
		if !strings.EqualFold(p.Site, site) || p.NoExpiry {
			continue
		}
		switch {
		case !p.ExpiresOn.Zero() && !p.ExpiresOn.Before(from) && !p.ExpiresOn.After(to):
			selected[p.Kind], reasons[p.Kind] = p, DueInWindow
		case !p.ExpiresOn.Zero() && p.ExpiresOn.Before(from) && p.ExpiresOn.Before(asOf):
			// Still unrenewed from a previous cycle. It has to be dealt with
			// inside this season whether the owner planned to or not, because
			// everything that needs it is stuck behind it.
			selected[p.Kind], reasons[p.Kind] = p, AlreadyLapsed
		case p.ExpiresOn.Zero() && p.SeasonAnchor:
			selected[p.Kind], reasons[p.Kind] = p, DateNotRecorded
		}
	}
	if len(selected) == 0 {
		season.Note = seasonNote(season, 0)
		return season
	}

	// ---- 2. the graph, restricted to the selected permits ----

	// prereqsOf holds only the edges between permits that are both in the
	// season, which are the ones that constrain the order of this plan.
	prereqsOf := map[Kind][]Kind{}
	dependents := map[Kind][]Kind{}
	for kind, p := range selected {
		for _, dep := range p.Prereqs {
			if _, inSeason := selected[dep.Kind]; !inSeason {
				continue
			}
			prereqsOf[kind] = append(prereqsOf[kind], dep.Kind)
			dependents[dep.Kind] = append(dependents[dep.Kind], kind)
		}
	}

	// ---- 3. waves, by Kahn's algorithm ----
	//
	// A wave is one step of the plan. Wave one is everything with nothing in
	// front of it; a permit's wave is one more than the deepest of its
	// prerequisites. Anything left over when the queue empties is in a
	// prerequisite loop, which cannot be ordered — those go in a final wave
	// and are reported so the owner can break the loop.

	indegree := map[Kind]int{}
	for kind := range selected {
		indegree[kind] = len(prereqsOf[kind])
	}
	wave := map[Kind]int{}
	ready := make([]Kind, 0, len(selected))
	for kind := range selected {
		if indegree[kind] == 0 {
			ready = append(ready, kind)
			wave[kind] = 1
		}
	}
	sortKinds(ready)

	settled := 0
	for len(ready) > 0 {
		kind := ready[0]
		ready = ready[1:]
		settled++
		next := append([]Kind(nil), dependents[kind]...)
		sortKinds(next)
		for _, dependent := range next {
			if wave[kind]+1 > wave[dependent] {
				wave[dependent] = wave[kind] + 1
			}
			indegree[dependent]--
			if indegree[dependent] == 0 {
				ready = append(ready, dependent)
				sortKinds(ready)
			}
		}
	}

	if settled < len(selected) {
		stuck := make([]Kind, 0, len(selected)-settled)
		maxWave := 0
		for kind := range selected {
			if wave[kind] > maxWave {
				maxWave = wave[kind]
			}
		}
		for kind := range selected {
			if _, ok := wave[kind]; !ok || indegree[kind] > 0 {
				stuck = append(stuck, kind)
			}
		}
		sortKinds(stuck)
		for _, kind := range stuck {
			wave[kind] = maxWave + 1
		}
		season.Cycles = findCycles(stuck, prereqsOf)
	}

	// ---- 4. backwards from the deadline ----
	//
	// The longest chain behind a permit is what decides when it has to be
	// started. A barangay clearance that takes five days is not a five-day job
	// if a ten-day business permit is queued behind it: it is a fifteen-day
	// job counted from the deadline.

	downstream := map[Kind]int{}
	var longestBehind func(Kind, map[Kind]bool) int
	longestBehind = func(kind Kind, onPath map[Kind]bool) int {
		if value, done := downstream[kind]; done {
			return value
		}
		if onPath[kind] {
			// A loop was already reported; stop walking it.
			return 0
		}
		onPath[kind] = true
		best := 0
		for _, dependent := range dependents[kind] {
			if chain := leadOf(selected[dependent]) + longestBehind(dependent, onPath); chain > best {
				best = chain
			}
		}
		delete(onPath, kind)
		downstream[kind] = best
		return best
	}

	// The matching forward pass: the longest chain AHEAD of a permit. A permit
	// sits on the critical path when the chain ahead of it, its own lead time
	// and the chain behind it add up to the longest chain in the whole plan —
	// which is to say, when there is no room anywhere around it.
	upstream := map[Kind]int{}
	var longestAhead func(Kind, map[Kind]bool) int
	longestAhead = func(kind Kind, onPath map[Kind]bool) int {
		if value, done := upstream[kind]; done {
			return value
		}
		if onPath[kind] {
			return 0
		}
		onPath[kind] = true
		best := 0
		for _, dep := range prereqsOf[kind] {
			if chain := leadOf(selected[dep]) + longestAhead(dep, onPath); chain > best {
				best = chain
			}
		}
		delete(onPath, kind)
		upstream[kind] = best
		return best
	}

	criticalLength := 0
	for kind := range selected {
		total := longestAhead(kind, map[Kind]bool{}) + leadOf(selected[kind]) + longestBehind(kind, map[Kind]bool{})
		if total > criticalLength {
			criticalLength = total
		}
	}

	// ---- 5. the rows ----

	for kind, p := range selected {
		lead := leadOf(p)
		behind := downstream[kind]
		row := SeasonRow{
			Permit:             p,
			Status:             StatusOf(p.ExpiresOn, asOf, p.NoExpiry, r.Tiers),
			Readiness:          ReadinessOf(p),
			Why:                reasons[kind],
			Wave:               wave[kind],
			LeadDays:           lead,
			DownstreamLeadDays: behind,
			StartBy:            AddDays(dueBy, -(lead + behind)),
			BlockedBy:          []PrereqRef{},
			Blocks:             append([]Kind(nil), dependents[kind]...),
			Critical:           criticalLength > 0 && upstream[kind]+lead+behind == criticalLength,
		}
		row.Slack = DaysBetween(asOf, row.StartBy)
		sortKinds(row.Blocks)

		for _, dep := range p.Prereqs {
			ref := PrereqRef{Kind: dep.Kind, Basis: dep.Basis, Why: dep.Why, Label: string(dep.Kind)}
			if _, inSeason := selected[dep.Kind]; inSeason {
				ref.InSeason = true
			}
			if onRegister, found := r.Find(dep.Kind, site); found {
				ref.OnRegister = true
				ref.Label = onRegister.Label
				depStatus := StatusOf(onRegister.ExpiresOn, asOf, onRegister.NoExpiry, r.Tiers)
				ref.Settled = !depStatus.Expired() && depStatus.Level != LevelUnknown
			} else if preset, ok := PresetFor(dep.Kind); ok {
				ref.Label = preset.Label
			}
			row.BlockedBy = append(row.BlockedBy, ref)
		}
		sort.SliceStable(row.BlockedBy, func(i, j int) bool {
			return row.BlockedBy[i].Label < row.BlockedBy[j].Label
		})
		season.Rows = append(season.Rows, row)
	}

	// Wave first, then the tightest start date, then the worst status, so the
	// sheet reads top to bottom as the order to work in.
	sort.SliceStable(season.Rows, func(i, j int) bool {
		a, b := season.Rows[i], season.Rows[j]
		if a.Wave != b.Wave {
			return a.Wave < b.Wave
		}
		if a.Slack != b.Slack {
			return a.Slack < b.Slack
		}
		if ra, rb := a.Status.rank(), b.Status.rank(); ra != rb {
			return ra < rb
		}
		return a.Permit.Label < b.Permit.Label
	})
	for i := range season.Rows {
		season.Rows[i].Order = i + 1
		if season.Rows[i].Wave > season.Waves {
			season.Waves = season.Rows[i].Wave
		}
	}

	boardRows := make([]BoardRow, 0, len(season.Rows))
	for _, row := range season.Rows {
		boardRows = append(boardRows, BoardRow{Permit: row.Permit, Status: row.Status, Readiness: row.Readiness})
	}
	season.Counts = Summarise(boardRows)
	season.Note = seasonNote(season, criticalLength)
	return season
}

// leadOf is a permit's lead time, falling back to the shipped convention when
// the owner has not set one.
func leadOf(p Permit) int {
	if p.LeadDays > 0 {
		return p.LeadDays
	}
	return DefaultLeadDays
}

func sortKinds(kinds []Kind) {
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
}

// findCycles walks the stuck nodes and pulls out one loop per starting point,
// so the interface can name the loop rather than just saying one exists.
func findCycles(stuck []Kind, prereqsOf map[Kind][]Kind) [][]Kind {
	inStuck := map[Kind]bool{}
	for _, kind := range stuck {
		inStuck[kind] = true
	}
	seen := map[string]bool{}
	out := [][]Kind{}

	for _, start := range stuck {
		var path []Kind
		onPath := map[Kind]bool{}
		var walk func(Kind) []Kind
		walk = func(kind Kind) []Kind {
			if onPath[kind] {
				// Cut the path down to the loop itself.
				for i, step := range path {
					if step == kind {
						return append([]Kind(nil), path[i:]...)
					}
				}
				return nil
			}
			onPath[kind] = true
			path = append(path, kind)
			for _, dep := range sortedCopy(prereqsOf[kind]) {
				if !inStuck[dep] {
					continue
				}
				if loop := walk(dep); loop != nil {
					return loop
				}
			}
			path = path[:len(path)-1]
			delete(onPath, kind)
			return nil
		}
		if loop := walk(start); len(loop) > 0 {
			key := canonicalLoopKey(loop)
			if !seen[key] {
				seen[key] = true
				out = append(out, loop)
			}
		}
	}
	return out
}

func sortedCopy(kinds []Kind) []Kind {
	out := append([]Kind(nil), kinds...)
	sortKinds(out)
	return out
}

// canonicalLoopKey rotates a loop so the same cycle found from two different
// starting points is recognised as one cycle.
func canonicalLoopKey(loop []Kind) string {
	if len(loop) == 0 {
		return ""
	}
	lowest := 0
	for i, kind := range loop {
		if kind < loop[lowest] {
			lowest = i
		}
	}
	rotated := make([]string, 0, len(loop))
	for i := range loop {
		rotated = append(rotated, string(loop[(lowest+i)%len(loop)]))
	}
	return strings.Join(rotated, ">")
}

func seasonNote(s Season, criticalLength int) string {
	if len(s.Rows) == 0 {
		return fmt.Sprintf("Nothing on the register falls due between %s and %s. That is either good news or a sign the dates have not been entered yet.", s.From, s.To)
	}
	parts := []string{
		fmt.Sprintf("%s fall due in this window, in %s.",
			plural(len(s.Rows), "permit", "permits"), plural(s.Waves, "step", "steps")),
	}
	if criticalLength > 0 {
		parts = append(parts, fmt.Sprintf("The longest chain in it runs %s of lead time, so the first thing on it has to start by %s to make %s.",
			plural(criticalLength, "day", "days"), AddDays(s.DueBy, -criticalLength), s.DueBy))
	}
	parts = append(parts, "Lead times are this tool's own convention, not a published service standard — set each one to what your own offices actually take, and the start dates move with them.")
	if len(s.Cycles) > 0 {
		parts = append(parts, "A prerequisite loop was found and those permits could not be ordered. Break the loop by removing one of the links.")
	}
	return strings.Join(parts, " ")
}
