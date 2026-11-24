package permits

import (
	"fmt"
	"strings"
	"unicode"
)

// Jurisdiction is the market a preset belongs to. The register may hold both:
// a food business can be registered in India and sell into the Philippines, or
// an owner can run the tool for two businesses out of one file.
type Jurisdiction string

const (
	India       Jurisdiction = "IN"
	Philippines Jurisdiction = "PH"
	// Other is for a permit the owner added that belongs to neither preset —
	// an export registration, a certification, a landlord's requirement.
	Other Jurisdiction = "OTHER"
)

// Label is how a jurisdiction is written in the interface.
func (j Jurisdiction) Label() string {
	switch j {
	case India:
		return "India"
	case Philippines:
		return "Philippines"
	case Other:
		return "Other"
	default:
		return string(j)
	}
}

// AllJurisdictions lists the markets in the order the interface shows them.
func AllJurisdictions() []Jurisdiction { return []Jurisdiction{India, Philippines, Other} }

// ParseJurisdiction reads a market code, case-insensitively, and treats an
// empty one as Other rather than failing — an imported row with a blank column
// is still a permit worth tracking.
func ParseJurisdiction(raw string) (Jurisdiction, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "IN", "INDIA":
		return India, nil
	case "PH", "PHL", "PHILIPPINES":
		return Philippines, nil
	case "", "OTHER":
		return Other, nil
	default:
		return "", fmt.Errorf("%q is not a market this tool knows: use IN, PH or OTHER", raw)
	}
}

// Kind identifies one permit within the register. The preset kinds below are
// the ones the tool ships with; the owner may add any other slug, because no
// list of permits is ever complete — a city can require something nobody else
// does, and a tracker that refuses to hold it is useless.
type Kind string

// The India preset kinds.
const (
	KindFSSAI          Kind = "fssai-licence"
	KindGST            Kind = "gst-registration"
	KindTradeLicence   Kind = "trade-licence"
	KindShopsEstab     Kind = "shops-establishments"
	KindFireNOC        Kind = "fire-noc"
	KindEatingHouse    Kind = "eating-house-licence"
	KindLegalMetrology Kind = "legal-metrology"
	KindPollutionNOC   Kind = "pollution-consent"
)

// The Philippines preset kinds.
const (
	KindBarangayClearance Kind = "barangay-clearance"
	KindMayorsPermit      Kind = "mayors-permit"
	KindSanitaryPermit    Kind = "sanitary-permit"
	KindHealthCertificate Kind = "health-certificate"
	KindFSIC              Kind = "fsic"
	KindBIRRegistration   Kind = "bir-registration"
	KindFDALTO            Kind = "fda-lto"
	KindBusinessName      Kind = "business-name"
)

// ParseKind validates a permit slug. It is deliberately permissive about which
// slugs exist and strict about their shape, because a slug ends up in a CSV
// column, a URL query and a JSON key.
func ParseKind(raw string) (Kind, error) {
	slug := strings.ToLower(strings.TrimSpace(raw))
	if slug == "" {
		return "", fmt.Errorf("which permit? a permit needs a short code such as %q", KindMayorsPermit)
	}
	if len(slug) > 60 {
		return "", fmt.Errorf("the permit code %q is too long: keep it under 60 characters", raw)
	}
	for _, r := range slug {
		if r == '-' || unicode.IsDigit(r) || (r >= 'a' && r <= 'z') {
			continue
		}
		return "", fmt.Errorf("the permit code %q may only use a-z, 0-9 and hyphens", raw)
	}
	return Kind(slug), nil
}

// Basis says where a rule in this tool came from, so the interface can show the
// difference between a sequence the law fixes and a sequence this tool merely
// assumes. The distinction matters: the owner may freely reorder a convention
// and should think twice before reordering a statute.
type Basis string

const (
	// FromStatute means a named law or regulation puts one permit before
	// another, and the citation is carried alongside it.
	FromStatute Basis = "statute"
	// FromConvention means this is the usual order, or the order most city
	// halls list the requirements in, but no instrument this tool could find
	// says so. Edit it freely.
	FromConvention Basis = "convention"
	// FromOwner means the owner set this edge themselves.
	FromOwner Basis = "owner"
)

// Label is how a basis is written in the interface.
func (b Basis) Label() string {
	switch b {
	case FromStatute:
		return "fixed by law"
	case FromConvention:
		return "convention — check locally"
	case FromOwner:
		return "you set this"
	default:
		return string(b)
	}
}

// ParseBasis reads a basis, defaulting a blank one to the owner's own, since
// anything arriving without a basis came from a form the owner filled in.
func ParseBasis(raw string) (Basis, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(FromStatute):
		return FromStatute, nil
	case string(FromConvention):
		return FromConvention, nil
	case "", string(FromOwner):
		return FromOwner, nil
	default:
		return "", fmt.Errorf("%q is not a basis: use statute, convention or owner", raw)
	}
}

// clip trims a free-text field and cuts it to the field cap, so no single
// pasted paragraph can distort the data file. It cuts on a rune boundary.
func clip(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Map(func(r rune) rune {
		// Keep ordinary text and drop control characters, which have no place
		// in a permit number and break a CSV.
		if r == '\n' || r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) <= MaxTextLen {
		return s
	}
	return string([]rune(s)[:MaxTextLen])
}
