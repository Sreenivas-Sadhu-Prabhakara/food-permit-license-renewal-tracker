package permits

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// The register and the season plan both come out as CSV, because a spreadsheet
// is where a small business already keeps everything and because a printed
// season plan is what somebody actually carries to the city hall.
//
// Import is the same shape as export, so a register can be edited in a
// spreadsheet and brought back.

var registerHeader = []string{
	"site", "market", "kind", "permit", "authority", "number",
	"issued_on", "expires_on", "no_expiry", "blocks",
	"validity_months", "lead_days", "season_anchor",
	"prerequisites", "document_reference", "notes",
	"checklist_done", "checklist_total",
}

// WriteRegisterCSV writes the whole register.
func WriteRegisterCSV(w io.Writer, r *Register) error {
	out := csv.NewWriter(w)
	if err := out.Write(registerHeader); err != nil {
		return err
	}
	for _, p := range r.Permits {
		readiness := ReadinessOf(p)
		prereqs := make([]string, 0, len(p.Prereqs))
		for _, dep := range p.Prereqs {
			prereqs = append(prereqs, fmt.Sprintf("%s (%s)", dep.Kind, dep.Basis))
		}
		row := []string{
			p.Site,
			string(p.Jurisdiction),
			string(p.Kind),
			p.Label,
			p.Authority,
			p.Number,
			p.IssuedOn.String(),
			p.ExpiresOn.String(),
			yesNo(p.NoExpiry),
			yesNo(p.Blocks),
			strconv.Itoa(p.ValidityMonths),
			strconv.Itoa(p.LeadDays),
			yesNo(p.SeasonAnchor),
			strings.Join(prereqs, "; "),
			p.DocRef,
			p.Notes,
			strconv.Itoa(readiness.Done),
			strconv.Itoa(readiness.Total),
		}
		if err := out.Write(row); err != nil {
			return err
		}
	}
	out.Flush()
	return out.Error()
}

var seasonHeader = []string{
	"order", "wave", "permit", "authority", "why_now", "status", "days_left",
	"start_by", "slack_days", "lead_days", "chain_behind_days", "critical_path",
	"waiting_on", "holds_up", "checklist_done", "checklist_total", "document_reference",
}

// WriteSeasonCSV writes one renewal season plan in the order it should be
// worked, with the deadline and its basis stated in the first lines so a
// printed sheet carries its own provenance.
func WriteSeasonCSV(w io.Writer, s Season) error {
	out := csv.NewWriter(w)
	preamble := [][]string{
		{s.Title},
		{"site", orNone(s.Site)},
		{"window", s.From.String(), "to", s.To.String()},
		{"deadline", s.DueBy.String(), string(s.DueByBasis), s.DueByWhy},
		{"as of", s.AsOf.String()},
		{"lead times", "a convention set in this tool, not a published service standard — confirm with each office"},
		{},
	}
	for _, line := range preamble {
		if err := out.Write(line); err != nil {
			return err
		}
	}
	if err := out.Write(seasonHeader); err != nil {
		return err
	}
	for _, row := range s.Rows {
		waiting := make([]string, 0, len(row.BlockedBy))
		for _, dep := range row.BlockedBy {
			waiting = append(waiting, fmt.Sprintf("%s (%s)", dep.Label, dep.Basis.Label()))
		}
		holds := make([]string, 0, len(row.Blocks))
		for _, kind := range row.Blocks {
			holds = append(holds, string(kind))
		}
		record := []string{
			strconv.Itoa(row.Order),
			strconv.Itoa(row.Wave),
			row.Permit.Label,
			row.Permit.Authority,
			string(row.Why),
			string(row.Status.Level),
			strconv.Itoa(row.Status.DaysLeft),
			row.StartBy.String(),
			strconv.Itoa(row.Slack),
			strconv.Itoa(row.LeadDays),
			strconv.Itoa(row.DownstreamLeadDays),
			yesNo(row.Critical),
			strings.Join(waiting, "; "),
			strings.Join(holds, "; "),
			strconv.Itoa(row.Readiness.Done),
			strconv.Itoa(row.Readiness.Total),
			row.Permit.DocRef,
		}
		if err := out.Write(record); err != nil {
			return err
		}
	}
	out.Flush()
	return out.Error()
}

// ReadRegisterCSV reads a register back from the export shape. It returns the
// permits it could read and a warning for every line it could not, rather than
// refusing the whole file for one bad date — a spreadsheet round trip almost
// always has one.
//
// A checklist does not survive the round trip as ticks: the export carries the
// counts for reading, and an imported permit gets the shipped checklist back,
// unticked, where the tool ships one for that kind.
func ReadRegisterCSV(src io.Reader) ([]Permit, []string, error) {
	in := csv.NewReader(src)
	in.FieldsPerRecord = -1
	in.TrimLeadingSpace = true

	records, err := in.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("that does not read as a CSV file: %w", err)
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("the file is empty")
	}

	index := map[string]int{}
	for i, name := range records[0] {
		index[strings.ToLower(strings.TrimSpace(name))] = i
	}
	if _, ok := index["kind"]; !ok {
		return nil, nil, fmt.Errorf("the first line has no \"kind\" column, so this is not a register export")
	}

	field := func(row []string, name string) string {
		i, ok := index[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	var out []Permit
	var warnings []string
	for n, row := range records[1:] {
		line := n + 2
		if isBlank(row) {
			continue
		}
		kind, err := ParseKind(field(row, "kind"))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("line %d skipped: %v", line, err))
			continue
		}
		jurisdiction, err := ParseJurisdiction(field(row, "market"))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("line %d: %v — filed under Other", line, err))
			jurisdiction = Other
		}
		issuedOn, err := ParseDate(field(row, "issued_on"))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("line %d: %v — issue date left empty", line, err))
		}
		expiresOn, err := ParseDate(field(row, "expires_on"))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("line %d: %v — expiry date left empty", line, err))
		}

		permit := Permit{
			Kind:           kind,
			Jurisdiction:   jurisdiction,
			Site:           field(row, "site"),
			Label:          field(row, "permit"),
			Authority:      field(row, "authority"),
			Number:         field(row, "number"),
			IssuedOn:       issuedOn,
			ExpiresOn:      expiresOn,
			NoExpiry:       truthy(field(row, "no_expiry")),
			Blocks:         truthy(field(row, "blocks")),
			ValidityMonths: number(field(row, "validity_months")),
			LeadDays:       number(field(row, "lead_days")),
			SeasonAnchor:   truthy(field(row, "season_anchor")),
			DocRef:         field(row, "document_reference"),
			Notes:          field(row, "notes"),
		}
		// Put the shipped checklist and prerequisites back where the tool has
		// them, since the CSV carries neither in a form it can replay.
		if preset, ok := PresetFor(kind); ok {
			permit.FromPreset = true
			permit.Prereqs = append(permit.Prereqs, preset.Prereqs...)
			for _, item := range preset.Checklist {
				permit.Checklist = append(permit.Checklist, ChecklistItem{Text: item})
			}
			if permit.Label == "" {
				permit.Label = preset.Label
			}
			if permit.Authority == "" {
				permit.Authority = preset.Authority
			}
		}
		permit.canonicalise()
		permit.clamp()
		if err := permit.Validate(); err != nil {
			warnings = append(warnings, fmt.Sprintf("line %d skipped: %v", line, err))
			continue
		}
		out = append(out, permit)
	}
	if len(out) == 0 {
		return nil, warnings, fmt.Errorf("no permit on that file could be read")
	}
	return out, warnings, nil
}

func isBlank(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "yes", "y", "true", "1":
		return true
	default:
		return false
	}
}

func number(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(the main premises)"
	}
	return s
}
