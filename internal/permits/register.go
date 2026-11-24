package permits

import (
	"fmt"
	"sort"
	"strings"
)

// MaxChecklistItems caps one permit's readiness checklist.
const MaxChecklistItems = 60

// earliestYear and latestYear bound a date the owner types, so a slipped digit
// shows up as an error rather than as a permit that expired in the year 226.
const (
	earliestYear = 1900
	latestYear   = 2200
)

// ChecklistItem is one line of the renewal readiness checklist for a permit:
// a document to gather, an inspection to pass, a fee to pay.
type ChecklistItem struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

// Permit is one row of the register — one permit, from one authority, on one
// cycle, for one site.
//
// There is no document vault here and there never will be. DocRef holds a path
// or a note in the owner's own words, and the tool neither reads it, copies it,
// nor uploads it: it is a reminder of where the certificate is, so that the
// person renewing it can find the thing itself.
type Permit struct {
	Kind         Kind         `json:"kind"`
	Jurisdiction Jurisdiction `json:"jurisdiction"`

	// Site is the branch or outlet the permit belongs to, empty for a
	// business with one premises. A Philippine business permit is issued per
	// location, so a second outlet means a second set of permits on their own
	// dates; the sequencing graph is resolved inside one site.
	Site string `json:"site"`

	Label     string `json:"label"`
	Authority string `json:"authority"`
	Number    string `json:"number"`

	IssuedOn  Date `json:"issuedOn"`
	ExpiresOn Date `json:"expiresOn"`
	NoExpiry  bool `json:"noExpiry"`

	// Blocks says that trading with this one expired makes the business
	// unlawful, not merely untidy. It is what CanTradeToday reads.
	Blocks bool `json:"blocks"`

	ValidityMonths int  `json:"validityMonths"`
	LeadDays       int  `json:"leadDays"`
	SeasonAnchor   bool `json:"seasonAnchor"`

	DocRef string `json:"docRef"`
	Notes  string `json:"notes"`

	Prereqs   []Prereq        `json:"prereqs"`
	Checklist []ChecklistItem `json:"checklist"`

	// FromPreset records that the row started life as a shipped entry, so the
	// interface can offer to put its note and checklist back.
	FromPreset bool `json:"fromPreset"`
}

// Business is who the register belongs to. It is only ever shown back to the
// owner; nothing about it leaves the machine.
type Business struct {
	Name         string       `json:"name"`
	Jurisdiction Jurisdiction `json:"jurisdiction"`
	// Locality is the city, municipality or corporation whose by-laws actually
	// set most of the figures on this board — worth writing down, because the
	// answer to "what is my renewal deadline" is always "ask them".
	Locality string `json:"locality"`
	Note     string `json:"note"`
}

// LateRates are the surcharge and interest rates used to price a late renewal.
// They start at the Philippine statutory ceilings for a PH register and at zero
// for an Indian one, and the owner overrides them with whatever their own
// ordinance or authority charges. See params.go.
type LateRates struct {
	SurchargeRate   float64 `json:"surchargeRate"`
	MonthlyInterest float64 `json:"monthlyInterest"`
	InterestCapMo   int     `json:"interestCapMonths"`
	DailyLateFee    float64 `json:"dailyLateFee"`
	Currency        string  `json:"currency"`
}

// Register is the whole data file.
type Register struct {
	Business  Business  `json:"business"`
	Tiers     Tiers     `json:"tiers"`
	LateRates LateRates `json:"lateRates"`
	Permits   []Permit  `json:"permits"`
}

// New starts an empty register with the default reminder tiers.
func New() *Register {
	return &Register{
		Tiers:     DefaultTiers(),
		LateRates: DefaultLateRates(Other),
		Permits:   []Permit{},
	}
}

// DefaultLateRates are the starting rates for a market. The Philippine figures
// are the statutory ceilings of section 168 of the Local Government Code; the
// Indian ones are zero, because this tool ships no Indian penalty figure.
func DefaultLateRates(j Jurisdiction) LateRates {
	switch j {
	case Philippines:
		return LateRates{
			SurchargeRate:   PHSurchargeRateCeiling,
			MonthlyInterest: PHMonthlyInterestCeiling,
			InterestCapMo:   PHInterestMonthsCap,
			Currency:        "PHP",
		}
	case India:
		return LateRates{
			SurchargeRate:   INSurchargeRateDefault,
			MonthlyInterest: INMonthlyInterestDefault,
			InterestCapMo:   PHInterestMonthsCap,
			DailyLateFee:    INDailyLateFeeDefault,
			Currency:        "INR",
		}
	default:
		return LateRates{InterestCapMo: PHInterestMonthsCap}
	}
}

// Normalise tidies a register loaded from disk: it repairs the tiers, clips
// every text field, drops a permit with no kind, and sorts the rows. A data
// file edited by hand is still a data file, and the tool has to cope with one.
func (r *Register) Normalise() {
	r.Tiers = r.Tiers.Normalise()
	if r.LateRates.InterestCapMo <= 0 {
		r.LateRates.InterestCapMo = PHInterestMonthsCap
	}
	if r.LateRates.Currency == "" {
		r.LateRates.Currency = DefaultLateRates(r.Business.Jurisdiction).Currency
	}
	r.Business.Name = clip(r.Business.Name)
	r.Business.Locality = clip(r.Business.Locality)
	r.Business.Note = clip(r.Business.Note)
	if r.Business.Jurisdiction == "" {
		r.Business.Jurisdiction = Other
	}

	kept := make([]Permit, 0, len(r.Permits))
	seen := make(map[string]bool, len(r.Permits))
	for _, p := range r.Permits {
		p.canonicalise()
		p.clamp()
		if p.Kind == "" {
			continue
		}
		key := p.key()
		if seen[key] {
			continue
		}
		seen[key] = true
		kept = append(kept, p)
		if len(kept) >= MaxPermits {
			break
		}
	}
	r.Permits = kept
	r.sortPermits()
}

// canonicalise tidies one permit without losing anything the owner meant: it
// trims text, lowercases the kind, fills a missing label in from the preset and
// drops prerequisite edges that cannot mean anything (an edge to itself, the
// same edge twice). It is safe to run on the way in from a form, because it
// never changes a date, a flag or a figure.
func (p *Permit) canonicalise() {
	p.Kind = Kind(strings.ToLower(strings.TrimSpace(string(p.Kind))))
	p.Site = clip(p.Site)
	p.Label = clip(p.Label)
	p.Authority = clip(p.Authority)
	p.Number = clip(p.Number)
	p.DocRef = clip(p.DocRef)
	p.Notes = clip(p.Notes)
	if p.Jurisdiction == "" {
		p.Jurisdiction = Other
	}
	if p.Label == "" {
		if preset, ok := PresetFor(p.Kind); ok {
			p.Label = preset.Label
		} else {
			p.Label = string(p.Kind)
		}
	}
	for i := range p.Checklist {
		p.Checklist[i].Text = clip(p.Checklist[i].Text)
	}
	prereqs := make([]Prereq, 0, len(p.Prereqs))
	seen := map[Kind]bool{}
	for _, dep := range p.Prereqs {
		dep.Kind = Kind(strings.ToLower(strings.TrimSpace(string(dep.Kind))))
		dep.Why = clip(dep.Why)
		if dep.Kind == "" || dep.Kind == p.Kind || seen[dep.Kind] {
			continue
		}
		if dep.Basis == "" {
			dep.Basis = FromOwner
		}
		seen[dep.Kind] = true
		prereqs = append(prereqs, dep)
	}
	p.Prereqs = prereqs
}

// clamp is the lossy half: it forces a figure into range and resolves a
// contradiction by picking one side. It belongs only on the load path, where a
// data file someone edited by hand has to be made usable and throwing the row
// away would lose the owner's register. A permit arriving from a form is
// validated and refused instead, so the owner is told rather than overruled.
func (p *Permit) clamp() {
	if p.NoExpiry {
		p.ExpiresOn = Date{}
	}
	if p.ValidityMonths < 0 {
		p.ValidityMonths = 0
	}
	if p.ValidityMonths > MaxValidityMonths {
		p.ValidityMonths = MaxValidityMonths
	}
	if p.LeadDays < 0 {
		p.LeadDays = 0
	}
	if p.LeadDays > MaxLeadDays {
		p.LeadDays = MaxLeadDays
	}
	if len(p.Checklist) > MaxChecklistItems {
		p.Checklist = p.Checklist[:MaxChecklistItems]
	}
	if !p.IssuedOn.Zero() && boundYear(p.IssuedOn, "issue date") != nil {
		p.IssuedOn = Date{}
	}
	if !p.ExpiresOn.Zero() && boundYear(p.ExpiresOn, "expiry date") != nil {
		p.ExpiresOn = Date{}
	}
}

// key is the identity of a permit inside the register: one of each kind per
// site.
func (p Permit) key() string { return strings.ToLower(p.Site) + "\x00" + string(p.Kind) }

// Validate checks a permit the owner is trying to save. Unlike tidy it refuses
// rather than repairs, because a form the owner is looking at should tell them
// what is wrong instead of quietly changing it.
func (p Permit) Validate() error {
	if _, err := ParseKind(string(p.Kind)); err != nil {
		return err
	}
	if _, err := ParseJurisdiction(string(p.Jurisdiction)); err != nil {
		return err
	}
	if strings.TrimSpace(p.Label) == "" {
		return fmt.Errorf("the permit needs a name — what does the paper call it?")
	}
	if p.NoExpiry && !p.ExpiresOn.Zero() {
		return fmt.Errorf("%s cannot both never expire and expire on %s — clear one of the two", p.Label, p.ExpiresOn)
	}
	if err := boundYear(p.IssuedOn, "issue date"); err != nil {
		return err
	}
	if err := boundYear(p.ExpiresOn, "expiry date"); err != nil {
		return err
	}
	if !p.IssuedOn.Zero() && !p.ExpiresOn.Zero() && p.ExpiresOn.Before(p.IssuedOn) {
		return fmt.Errorf("%s cannot expire on %s, before it was issued on %s", p.Label, p.ExpiresOn, p.IssuedOn)
	}
	if p.ValidityMonths < 0 || p.ValidityMonths > MaxValidityMonths {
		return fmt.Errorf("a validity of %d months is not a period this tool will take: use 0 to %d", p.ValidityMonths, MaxValidityMonths)
	}
	if p.LeadDays < 0 || p.LeadDays > MaxLeadDays {
		return fmt.Errorf("a lead time of %d days is not one this tool will take: use 0 to %d", p.LeadDays, MaxLeadDays)
	}
	if len(p.Checklist) > MaxChecklistItems {
		return fmt.Errorf("%d checklist lines is more than this tool keeps: the limit is %d", len(p.Checklist), MaxChecklistItems)
	}
	for _, dep := range p.Prereqs {
		if _, err := ParseKind(string(dep.Kind)); err != nil {
			return fmt.Errorf("prerequisite of %s: %w", p.Label, err)
		}
		if dep.Kind == p.Kind {
			return fmt.Errorf("%s cannot be its own prerequisite", p.Label)
		}
		if _, err := ParseBasis(string(dep.Basis)); err != nil {
			return fmt.Errorf("prerequisite %s of %s: %w", dep.Kind, p.Label, err)
		}
	}
	return nil
}

func boundYear(d Date, what string) error {
	if d.Zero() {
		return nil
	}
	if d.Year < earliestYear || d.Year > latestYear {
		return fmt.Errorf("the %s %s looks like a typing slip — this tool takes years %d to %d", what, d, earliestYear, latestYear)
	}
	return nil
}

// Put saves a permit, replacing the one of the same kind at the same site. It
// validates first, so a rejected edit changes nothing.
func (r *Register) Put(in Permit) (Permit, error) {
	in.canonicalise()
	if err := in.Validate(); err != nil {
		return Permit{}, err
	}
	for i, existing := range r.Permits {
		if existing.key() == in.key() {
			r.Permits[i] = in
			r.sortPermits()
			return in, nil
		}
	}
	if len(r.Permits) >= MaxPermits {
		return Permit{}, fmt.Errorf("this register already holds %d permits, which is as many as the tool keeps", MaxPermits)
	}
	r.Permits = append(r.Permits, in)
	r.sortPermits()
	return in, nil
}

// Remove drops one permit. It reports whether there was one to drop.
func (r *Register) Remove(kind Kind, site string) bool {
	target := Permit{Kind: kind, Site: site}
	target.canonicalise()
	for i, existing := range r.Permits {
		if existing.key() == target.key() {
			r.Permits = append(r.Permits[:i], r.Permits[i+1:]...)
			return true
		}
	}
	return false
}

// Find looks one permit up.
func (r *Register) Find(kind Kind, site string) (Permit, bool) {
	target := Permit{Kind: kind, Site: site}
	target.canonicalise()
	for _, existing := range r.Permits {
		if existing.key() == target.key() {
			return existing, true
		}
	}
	return Permit{}, false
}

// SetChecklistItem ticks or unticks one line of a permit's checklist.
func (r *Register) SetChecklistItem(kind Kind, site string, index int, done bool) error {
	target := Permit{Kind: kind, Site: site}
	target.canonicalise()
	for i := range r.Permits {
		if r.Permits[i].key() != target.key() {
			continue
		}
		if index < 0 || index >= len(r.Permits[i].Checklist) {
			return fmt.Errorf("there is no checklist line %d on %s", index+1, r.Permits[i].Label)
		}
		r.Permits[i].Checklist[index].Done = done
		return nil
	}
	return fmt.Errorf("no %s is on file%s", kind, siteSuffix(site))
}

// AddPreset puts one shipped entry on the register for a site, leaving the one
// already there alone — adding a preset must never wipe dates the owner typed.
func (r *Register) AddPreset(kind Kind, site string) (Permit, error) {
	preset, ok := PresetFor(kind)
	if !ok {
		return Permit{}, fmt.Errorf("%q is not one of the permits this tool ships a preset for; add it as your own instead", kind)
	}
	permit := preset.ToPermit()
	permit.Site = site
	if existing, found := r.Find(kind, site); found {
		return existing, nil
	}
	return r.Put(permit)
}

// AddJurisdictionPreset puts a whole market's starting list on the register,
// skipping anything already there. It returns what it added, so the interface
// can say so.
func (r *Register) AddJurisdictionPreset(j Jurisdiction, site string) ([]Permit, error) {
	presets := PresetsFor(j)
	if len(presets) == 0 {
		return nil, fmt.Errorf("there is no shipped preset for %q", j)
	}
	added := make([]Permit, 0, len(presets))
	for _, preset := range presets {
		if _, found := r.Find(preset.Kind, site); found {
			continue
		}
		permit := preset.ToPermit()
		permit.Site = site
		saved, err := r.Put(permit)
		if err != nil {
			return added, err
		}
		added = append(added, saved)
	}
	return added, nil
}

// Gaps lists the shipped entries for a market that are not on the register for
// a site.
//
// A gap is NOT a finding that the business is operating unlawfully, and the
// tool never treats it as one: whether a food business needs an FDA licence to
// operate, a police eating house licence or a pollution consent depends on what
// it does and where, and only the owner knows. A gap is a prompt to decide.
func (r *Register) Gaps(j Jurisdiction, site string) []Preset {
	var out []Preset
	for _, preset := range PresetsFor(j) {
		if _, found := r.Find(preset.Kind, site); !found {
			out = append(out, preset)
		}
	}
	return out
}

// Sites lists every site on the register, the unnamed one first.
func (r *Register) Sites() []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range r.Permits {
		if seen[p.Site] {
			continue
		}
		seen[p.Site] = true
		out = append(out, p.Site)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i] == "") != (out[j] == "") {
			return out[i] == ""
		}
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

// sortPermits puts the register in a stable, readable order: by site, then by
// the order the presets list the kinds in, then alphabetically for anything the
// owner added.
func (r *Register) sortPermits() {
	order := presetOrder()
	rank := func(p Permit) int {
		if i, ok := order[p.Kind]; ok {
			return i
		}
		return len(order)
	}
	sort.SliceStable(r.Permits, func(i, j int) bool {
		a, b := r.Permits[i], r.Permits[j]
		if (a.Site == "") != (b.Site == "") {
			return a.Site == ""
		}
		if !strings.EqualFold(a.Site, b.Site) {
			return strings.ToLower(a.Site) < strings.ToLower(b.Site)
		}
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra < rb
		}
		return a.Kind < b.Kind
	})
}

func presetOrder() map[Kind]int {
	all := AllPresets()
	out := make(map[Kind]int, len(all))
	for i, p := range all {
		out[p.Kind] = i
	}
	return out
}

// BoardRow is one line of the register board: the permit, how close it is to
// lapsing, and how ready the renewal is.
type BoardRow struct {
	Permit    Permit    `json:"permit"`
	Status    Status    `json:"status"`
	Readiness Readiness `json:"readiness"`
}

// Board is every permit, worst first. This is the view that answers "what do I
// have to deal with".
func (r *Register) Board(asOf Date) []BoardRow {
	rows := make([]BoardRow, 0, len(r.Permits))
	for _, p := range r.Permits {
		rows = append(rows, BoardRow{
			Permit:    p,
			Status:    StatusOf(p.ExpiresOn, asOf, p.NoExpiry, r.Tiers),
			Readiness: ReadinessOf(p),
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if ra, rb := a.Status.rank(), b.Status.rank(); ra != rb {
			return ra < rb
		}
		if a.Status.DaysLeft != b.Status.DaysLeft {
			return a.Status.DaysLeft < b.Status.DaysLeft
		}
		if a.Permit.Site != b.Permit.Site {
			return a.Permit.Site < b.Permit.Site
		}
		return a.Permit.Label < b.Permit.Label
	})
	return rows
}

// SiteBoard is the board for one site.
func (r *Register) SiteBoard(site string, asOf Date) []BoardRow {
	var out []BoardRow
	for _, row := range r.Board(asOf) {
		if strings.EqualFold(row.Permit.Site, site) {
			out = append(out, row)
		}
	}
	return out
}

// SuggestExpiry works an expiry date out from an issue date and the permit's
// indicative validity period, so the owner does not have to count months.
//
// It returns false, with a reason, where there is no period to work from —
// either because the permit does not expire, or because the tool deliberately
// ships no period for it. The date on the paper always wins over this.
func (r *Register) SuggestExpiry(kind Kind, site string, issuedOn Date) (Date, bool, string) {
	months := 0
	if existing, found := r.Find(kind, site); found {
		months = existing.ValidityMonths
		if existing.NoExpiry {
			return Date{}, false, "this permit does not expire, so there is nothing to work out"
		}
	} else if preset, ok := PresetFor(kind); ok {
		months = preset.ValidityMonths
		if preset.NoExpiry {
			return Date{}, false, "this permit does not expire, so there is nothing to work out"
		}
	}
	if issuedOn.Zero() {
		return Date{}, false, "enter the issue date first"
	}
	if months <= 0 {
		return Date{}, false, "this tool ships no validity period for this permit, because the period is set locally and varies — enter the date printed on the paper"
	}
	return ExpiryFrom(issuedOn, months), true, ""
}

func siteSuffix(site string) string {
	if strings.TrimSpace(site) == "" {
		return ""
	}
	return " for " + site
}
