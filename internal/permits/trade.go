package permits

import (
	"fmt"
	"sort"
	"strings"
)

// BlockReason says why one permit stops the business trading.
type BlockReason string

const (
	// ReasonExpired is the straightforward case: the date has passed.
	ReasonExpired BlockReason = "expired"
	// ReasonNoDate is a permit marked as blocking whose expiry date nobody
	// recorded. It is not safe to call it valid — the tool has no way to know
	// whether it is — so it stops the answer from being yes. Entering the date
	// clears it.
	ReasonNoDate BlockReason = "noExpiryDateRecorded"
)

// Blocker is one permit that makes trading unlawful, named so the owner knows
// which door to go to.
type Blocker struct {
	Kind        Kind        `json:"kind"`
	Site        string      `json:"site"`
	Label       string      `json:"label"`
	Authority   string      `json:"authority"`
	Reason      BlockReason `json:"reason"`
	ExpiredOn   Date        `json:"expiredOn"`
	DaysOverdue int         `json:"daysOverdue"`
	Says        string      `json:"says"`
}

// AtRisk is a blocking permit that has not lapsed yet but is inside the final
// reminder tier — the ones that will be blockers next week if nobody moves.
type AtRisk struct {
	Kind      Kind   `json:"kind"`
	Site      string `json:"site"`
	Label     string `json:"label"`
	Authority string `json:"authority"`
	ExpiresOn Date   `json:"expiresOn"`
	DaysLeft  int    `json:"daysLeft"`
	Says      string `json:"says"`
}

// TradeVerdict is the headline answer: on this day, is any permit expired such
// that the business is operating unlawfully?
type TradeVerdict struct {
	AsOf     Date   `json:"asOf"`
	Site     string `json:"site"`
	CanTrade bool   `json:"canTrade"`
	// Checked is how many blocking permits the verdict looked at. A yes from
	// an empty register means nothing, and this number is how the interface
	// says so.
	Checked  int       `json:"checked"`
	Blockers []Blocker `json:"blockers"`
	AtRisk   []AtRisk  `json:"atRisk"`
	Headline string    `json:"headline"`
	Note     string    `json:"note"`
}

// CanTradeToday answers for the whole register, every site.
func (r *Register) CanTradeToday(asOf Date) TradeVerdict {
	return r.verdict(asOf, "", false)
}

// CanTradeTodayAt answers for one site, which is the question that matters when
// a business runs two outlets and only one of them has a lapsed permit.
func (r *Register) CanTradeTodayAt(site string, asOf Date) TradeVerdict {
	return r.verdict(asOf, site, true)
}

func (r *Register) verdict(asOf Date, site string, oneSite bool) TradeVerdict {
	out := TradeVerdict{
		AsOf:     asOf,
		Site:     site,
		Blockers: []Blocker{},
		AtRisk:   []AtRisk{},
	}

	for _, p := range r.Permits {
		if oneSite && !strings.EqualFold(p.Site, site) {
			continue
		}
		// A permit the owner has said does not stop trading is reported on the
		// board but never here. A legal metrology stamp on a set of scales and
		// a lapsed sanitary permit are not the same kind of problem.
		if !p.Blocks {
			continue
		}
		// A registration that does not lapse cannot be the thing that stopped
		// you.
		if p.NoExpiry {
			out.Checked++
			continue
		}
		out.Checked++

		status := StatusOf(p.ExpiresOn, asOf, p.NoExpiry, r.Tiers)
		switch {
		case status.Level == LevelUnknown:
			out.Blockers = append(out.Blockers, Blocker{
				Kind:      p.Kind,
				Site:      p.Site,
				Label:     p.Label,
				Authority: p.Authority,
				Reason:    ReasonNoDate,
				Says: fmt.Sprintf("%s%s has no expiry date on the register, so this tool cannot tell you it is valid. Enter the date printed on it.",
					p.Label, siteSuffix(p.Site)),
			})
		case status.Expired():
			out.Blockers = append(out.Blockers, Blocker{
				Kind:        p.Kind,
				Site:        p.Site,
				Label:       p.Label,
				Authority:   p.Authority,
				Reason:      ReasonExpired,
				ExpiredOn:   p.ExpiresOn,
				DaysOverdue: -status.DaysLeft,
				Says: fmt.Sprintf("%s%s expired on %s, %s ago. It is issued by %s.",
					p.Label, siteSuffix(p.Site), p.ExpiresOn, plural(-status.DaysLeft, "day", "days"), p.Authority),
			})
		case status.Level == LevelUrgent:
			out.AtRisk = append(out.AtRisk, AtRisk{
				Kind:      p.Kind,
				Site:      p.Site,
				Label:     p.Label,
				Authority: p.Authority,
				ExpiresOn: p.ExpiresOn,
				DaysLeft:  status.DaysLeft,
				Says: fmt.Sprintf("%s%s %s, on %s.",
					p.Label, siteSuffix(p.Site), status.Label, p.ExpiresOn),
			})
		}
	}

	// Worst first, and a stable order inside that so the same register always
	// reads the same way.
	sort.SliceStable(out.Blockers, func(i, j int) bool {
		a, b := out.Blockers[i], out.Blockers[j]
		if a.DaysOverdue != b.DaysOverdue {
			return a.DaysOverdue > b.DaysOverdue
		}
		return a.Label < b.Label
	})
	sort.SliceStable(out.AtRisk, func(i, j int) bool {
		a, b := out.AtRisk[i], out.AtRisk[j]
		if a.DaysLeft != b.DaysLeft {
			return a.DaysLeft < b.DaysLeft
		}
		return a.Label < b.Label
	})

	out.CanTrade = len(out.Blockers) == 0
	out.Headline = tradeHeadline(out)
	out.Note = "This answers from the register and from nothing else. A permit you have not added cannot appear here, and a date typed wrongly stays wrong — the permits you have not put on the register are listed separately as gaps. It is a prompt, not a legal opinion."
	return out
}

func tradeHeadline(v TradeVerdict) string {
	where := ""
	if v.Site != "" {
		where = " at " + v.Site
	}
	if !v.CanTrade {
		names := make([]string, 0, len(v.Blockers))
		for _, b := range v.Blockers {
			names = append(names, b.Label)
		}
		return fmt.Sprintf("No. On %s%s, %s out of order: %s.",
			v.AsOf, where, plural(len(v.Blockers), "permit is", "permits are"), strings.Join(names, "; "))
	}
	if v.Checked == 0 {
		return fmt.Sprintf("Nothing to answer from. No permit on the register%s is marked as one that stops you trading, so this is not a yes — add your permits first.", where)
	}
	if len(v.AtRisk) > 0 {
		verb := "is"
		if len(v.AtRisk) != 1 {
			verb = "are"
		}
		return fmt.Sprintf("Yes, on %s%s — all %s in order. But %d of them %s inside the final reminder.",
			v.AsOf, where, plural(v.Checked, "permit is", "permits are"), len(v.AtRisk), verb)
	}
	return fmt.Sprintf("Yes, on %s%s — all %s on the register in order.",
		v.AsOf, where, plural(v.Checked, "blocking permit is", "blocking permits are"))
}
