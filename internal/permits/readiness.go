package permits

import "math"

// Readiness is how far along the renewal of one permit is: how much of its
// checklist is ticked, and whether the owner has said where the certificate
// itself is kept.
//
// It exists because "renew the business permit" is not one task. It is a
// barangay clearance, three inspections, a sworn declaration of gross receipts
// and a queue, and the useful question in the second week of January is not
// "is it due" but "how much of it is already done".
type Readiness struct {
	Done  int `json:"done"`
	Total int `json:"total"`
	// Percent is nought when there is no checklist at all, which the
	// interface shows as a dash rather than as nought per cent ready.
	Percent int `json:"percent"`
	// Outstanding lists the lines still to do, in checklist order.
	Outstanding []string `json:"outstanding"`
	// HasDocRef is whether the owner noted where the certificate is kept. No
	// file is ever read or stored: this is a path or a sentence they typed.
	HasDocRef bool `json:"hasDocRef"`
	Complete  bool `json:"complete"`
}

// ReadinessOf counts one permit's checklist.
func ReadinessOf(p Permit) Readiness {
	out := Readiness{
		Total:       len(p.Checklist),
		Outstanding: []string{},
		HasDocRef:   p.DocRef != "",
	}
	for _, item := range p.Checklist {
		if item.Done {
			out.Done++
			continue
		}
		out.Outstanding = append(out.Outstanding, item.Text)
	}
	if out.Total > 0 {
		out.Percent = int(math.Round(float64(out.Done) / float64(out.Total) * 100))
		out.Complete = out.Done == out.Total
	}
	return out
}
