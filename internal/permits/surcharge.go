package permits

import (
	"fmt"
	"math"
)

// What a late renewal costs.
//
// This is the one place in the tool that touches money, and it does so without
// shipping a single fee. The tool does not know what your business permit costs
// — that is assessed by your own city or municipality from your own gross
// receipts under its own revenue code, and no national figure exists. You type
// in the amount you were assessed; this works out what being late adds to it.
//
// The rates come from section 168 of the Local Government Code of 1991
// (Republic Act No. 7160): a local government unit may impose a surcharge not
// exceeding twenty-five per cent of the amount of taxes, fees or charges not
// paid on time, and an interest not exceeding two per cent per month of the
// unpaid amount including the surcharge, until the amount is paid in full,
// provided the total interest does not exceed thirty-six months.
//
// Those are CEILINGS. Your own revenue code sets the rate that applies to you,
// at or below them, and some set less. The rates are editable for exactly that
// reason, and the result is labelled as the worst case rather than as a bill.

// LateCharge is what a late payment adds, broken out so the owner can check
// each line against the notice they were handed.
type LateCharge struct {
	Base       float64 `json:"base"`
	Currency   string  `json:"currency"`
	DaysLate   int     `json:"daysLate"`
	MonthsLate int     `json:"monthsLate"`
	// InterestMonths is the months actually charged, which is MonthsLate
	// capped at the thirty-six month limit.
	InterestMonths int  `json:"interestMonths"`
	Capped         bool `json:"capped"`

	SurchargeRate   float64 `json:"surchargeRate"`
	MonthlyInterest float64 `json:"monthlyInterest"`

	Surcharge float64 `json:"surcharge"`
	Interest  float64 `json:"interest"`
	DailyFees float64 `json:"dailyFees"`
	Total     float64 `json:"total"`
	TotalDue  float64 `json:"totalDue"`
	Basis     string  `json:"basis"`
	Note      string  `json:"note"`
}

// LateChargeOn prices a payment made on paidOn for an amount that was due on
// dueOn. A payment on or before the due date costs nothing extra.
//
// Interest runs on the unpaid amount INCLUDING the surcharge, which is what
// section 168 says, and is charged per month or fraction of a month of delay —
// one day late is one month's interest, which is why the first day late is the
// expensive one.
func LateChargeOn(base float64, dueOn, paidOn Date, rates LateRates) (LateCharge, error) {
	if base < 0 {
		return LateCharge{}, fmt.Errorf("an amount of %.2f is not an amount: enter what you were assessed", base)
	}
	if math.IsNaN(base) || math.IsInf(base, 0) {
		return LateCharge{}, fmt.Errorf("that is not a number")
	}
	if dueOn.Zero() {
		return LateCharge{}, fmt.Errorf("when was it due? enter the deadline")
	}
	if paidOn.Zero() {
		return LateCharge{}, fmt.Errorf("when was it paid, or when will it be? enter a date")
	}
	if rates.SurchargeRate < 0 || rates.MonthlyInterest < 0 || rates.DailyLateFee < 0 {
		return LateCharge{}, fmt.Errorf("a negative rate is not a rate")
	}
	if rates.SurchargeRate > 1 || rates.MonthlyInterest > 1 {
		return LateCharge{}, fmt.Errorf("enter a rate as a fraction: 0.25 for twenty-five per cent, not 25")
	}

	capMonths := rates.InterestCapMo
	if capMonths <= 0 {
		capMonths = PHInterestMonthsCap
	}

	out := LateCharge{
		Base:            round2(base),
		Currency:        rates.Currency,
		DaysLate:        DaysBetween(dueOn, paidOn),
		MonthsLate:      MonthsBetween(dueOn, paidOn),
		SurchargeRate:   rates.SurchargeRate,
		MonthlyInterest: rates.MonthlyInterest,
		Basis: "Surcharge and interest rates under section 168 of the Local Government Code of 1991 (Republic Act No. 7160), " +
			"which sets them as ceilings a local government unit may not exceed. Your own city or municipal revenue code sets the rate that applies to you, at or below them.",
	}
	if out.DaysLate < 0 {
		out.DaysLate = 0
	}
	if out.MonthsLate == 0 {
		out.TotalDue = out.Base
		out.Note = "Paid on time, so nothing is added."
		return out, nil
	}

	out.InterestMonths = out.MonthsLate
	if out.InterestMonths > capMonths {
		out.InterestMonths = capMonths
		out.Capped = true
	}

	out.Surcharge = round2(base * rates.SurchargeRate)
	// Interest runs on the unpaid amount including the surcharge.
	out.Interest = round2((base + out.Surcharge) * rates.MonthlyInterest * float64(out.InterestMonths))
	out.DailyFees = round2(rates.DailyLateFee * float64(out.DaysLate))
	out.Total = round2(out.Surcharge + out.Interest + out.DailyFees)
	out.TotalDue = round2(out.Base + out.Total)

	out.Note = fmt.Sprintf("%s late, charged as %s of interest (or fraction thereof).",
		plural(out.DaysLate, "day", "days"), plural(out.InterestMonths, "month", "months"))
	if out.Capped {
		out.Note += fmt.Sprintf(" The interest is capped at %d months.", capMonths)
	}
	if rates.SurchargeRate == 0 && rates.MonthlyInterest == 0 && rates.DailyLateFee == 0 {
		out.Note += " Every rate here is nought, which is what this tool ships for India: it asserts no Indian penalty figure. Enter what your corporation or FoSCoS actually charges you."
	}
	return out, nil
}

// round2 rounds to the nearest minor unit. Money in both markets is quoted to
// two places, and an unrounded float printed into a summary is the sort of
// thing that makes an owner distrust everything else on the page.
func round2(v float64) float64 { return math.Round(v*100) / 100 }
