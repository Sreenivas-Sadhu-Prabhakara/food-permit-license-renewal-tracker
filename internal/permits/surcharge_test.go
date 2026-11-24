package permits

import (
	"strings"
	"testing"
)

func TestLateChargeOnWithTheStatutoryCeilings(t *testing.T) {
	rates := DefaultLateRates(Philippines)
	due := MustParseDate("2027-01-20")

	cases := []struct {
		name           string
		base           float64
		paidOn         string
		wantMonths     int
		wantInterestMo int
		wantSurcharge  float64
		wantInterest   float64
		wantTotal      float64
		wantCapped     bool
	}{
		{
			name: "paid on the deadline costs nothing extra",
			base: 10000, paidOn: "2027-01-20",
			wantMonths: 0, wantInterestMo: 0, wantSurcharge: 0, wantInterest: 0, wantTotal: 0,
		},
		{
			name: "paid early costs nothing extra",
			base: 10000, paidOn: "2027-01-05",
			wantMonths: 0, wantInterestMo: 0, wantSurcharge: 0, wantInterest: 0, wantTotal: 0,
		},
		{
			// 2,500 surcharge, then 2% of 12,500 for one month = 250.
			name: "one day late is a full month of interest",
			base: 10000, paidOn: "2027-01-21",
			wantMonths: 1, wantInterestMo: 1, wantSurcharge: 2500, wantInterest: 250, wantTotal: 2750,
		},
		{
			name: "a fortnight late is still one month",
			base: 10000, paidOn: "2027-02-03",
			wantMonths: 1, wantInterestMo: 1, wantSurcharge: 2500, wantInterest: 250, wantTotal: 2750,
		},
		{
			name: "a month and a day is two months",
			base: 10000, paidOn: "2027-02-21",
			wantMonths: 2, wantInterestMo: 2, wantSurcharge: 2500, wantInterest: 500, wantTotal: 3000,
		},
		{
			name: "a year late",
			base: 10000, paidOn: "2028-01-20",
			wantMonths: 12, wantInterestMo: 12, wantSurcharge: 2500, wantInterest: 3000, wantTotal: 5500,
		},
		{
			name: "exactly three years is the thirty-six month limit, not yet capped",
			base: 10000, paidOn: "2030-01-20",
			wantMonths: 36, wantInterestMo: 36, wantSurcharge: 2500, wantInterest: 9000, wantTotal: 11500,
		},
		{
			name: "a day past three years is capped at thirty-six months of interest",
			base: 10000, paidOn: "2030-01-21",
			wantMonths: 37, wantInterestMo: 36, wantSurcharge: 2500, wantInterest: 9000, wantTotal: 11500,
			wantCapped: true,
		},
		{
			name: "ten years late is still capped at thirty-six months",
			base: 10000, paidOn: "2037-01-20",
			wantMonths: 120, wantInterestMo: 36, wantSurcharge: 2500, wantInterest: 9000, wantTotal: 11500,
			wantCapped: true,
		},
		{
			name: "nothing assessed means nothing to add",
			base: 0, paidOn: "2027-06-20",
			wantMonths: 5, wantInterestMo: 5, wantSurcharge: 0, wantInterest: 0, wantTotal: 0,
		},
		{
			// 1,234.56 x 0.25 = 308.64; (1234.56 + 308.64) x 0.02 = 30.864 -> 30.86.
			name: "an awkward amount rounds to the minor unit",
			base: 1234.56, paidOn: "2027-01-21",
			wantMonths: 1, wantInterestMo: 1, wantSurcharge: 308.64, wantInterest: 30.86, wantTotal: 339.5,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := LateChargeOn(c.base, due, MustParseDate(c.paidOn), rates)
			if err != nil {
				t.Fatalf("LateChargeOn: %v", err)
			}
			if got.MonthsLate != c.wantMonths {
				t.Errorf("monthsLate = %d, want %d", got.MonthsLate, c.wantMonths)
			}
			if got.InterestMonths != c.wantInterestMo {
				t.Errorf("interestMonths = %d, want %d", got.InterestMonths, c.wantInterestMo)
			}
			if got.Surcharge != c.wantSurcharge {
				t.Errorf("surcharge = %.2f, want %.2f", got.Surcharge, c.wantSurcharge)
			}
			if got.Interest != c.wantInterest {
				t.Errorf("interest = %.2f, want %.2f", got.Interest, c.wantInterest)
			}
			if got.Total != c.wantTotal {
				t.Errorf("total added = %.2f, want %.2f", got.Total, c.wantTotal)
			}
			if got.TotalDue != round2(c.base+c.wantTotal) {
				t.Errorf("total due = %.2f, want %.2f", got.TotalDue, round2(c.base+c.wantTotal))
			}
			if got.Capped != c.wantCapped {
				t.Errorf("capped = %v, want %v", got.Capped, c.wantCapped)
			}
			if !strings.Contains(got.Basis, "7160") || !strings.Contains(got.Basis, "168") {
				t.Errorf("the basis must cite the instrument; got %q", got.Basis)
			}
			if !strings.Contains(got.Basis, "ceiling") {
				t.Errorf("the basis must say these are ceilings; got %q", got.Basis)
			}
		})
	}
}

func TestLateChargeInterestRunsOnTheSurchargeToo(t *testing.T) {
	// Section 168 charges interest on "the unpaid amount including the
	// surcharge", which is more than interest on the principal alone. Prove
	// the difference is actually there rather than quietly dropped.
	rates := DefaultLateRates(Philippines)
	got, err := LateChargeOn(10000, MustParseDate("2027-01-20"), MustParseDate("2027-02-21"), rates)
	if err != nil {
		t.Fatalf("LateChargeOn: %v", err)
	}
	onPrincipalOnly := round2(10000 * rates.MonthlyInterest * 2)
	if got.Interest == onPrincipalOnly {
		t.Fatalf("interest of %.2f is interest on the principal alone; it must include the surcharge", got.Interest)
	}
	wantIncludingSurcharge := round2((10000 + 2500) * rates.MonthlyInterest * 2)
	if got.Interest != wantIncludingSurcharge {
		t.Fatalf("interest = %.2f, want %.2f", got.Interest, wantIncludingSurcharge)
	}
}

func TestLateChargeWithIndiasZeroRates(t *testing.T) {
	// The tool ships no India penalty figure, so the arithmetic produces
	// nothing and says why.
	got, err := LateChargeOn(5000, MustParseDate("2027-03-31"), MustParseDate("2027-06-30"), DefaultLateRates(India))
	if err != nil {
		t.Fatalf("LateChargeOn: %v", err)
	}
	if got.Total != 0 {
		t.Fatalf("total added = %.2f with every rate at nought", got.Total)
	}
	if got.TotalDue != 5000 {
		t.Fatalf("total due = %.2f, want the base unchanged", got.TotalDue)
	}
	if !strings.Contains(got.Note, "asserts no Indian penalty figure") {
		t.Fatalf("the note must explain the zeroes; got %q", got.Note)
	}
}

func TestLateChargeWithAnOwnerSuppliedDailyFee(t *testing.T) {
	// An owner told a daily charge by their own authority enters it, and the
	// arithmetic follows their figure.
	rates := LateRates{DailyLateFee: 100, InterestCapMo: PHInterestMonthsCap, Currency: "INR"}
	got, err := LateChargeOn(2000, MustParseDate("2027-03-31"), MustParseDate("2027-04-10"), rates)
	if err != nil {
		t.Fatalf("LateChargeOn: %v", err)
	}
	if got.DaysLate != 10 {
		t.Fatalf("daysLate = %d, want 10", got.DaysLate)
	}
	if got.DailyFees != 1000 {
		t.Fatalf("daily fees = %.2f, want 1000", got.DailyFees)
	}
	if got.Total != 1000 || got.TotalDue != 3000 {
		t.Fatalf("total added %.2f, due %.2f; want 1000 and 3000", got.Total, got.TotalDue)
	}
}

func TestLateChargeOnACustomRateBelowTheCeiling(t *testing.T) {
	// A city whose revenue code sets ten per cent and one per cent, which it
	// may, must produce that and not the ceiling.
	rates := LateRates{SurchargeRate: 0.10, MonthlyInterest: 0.01, InterestCapMo: 36, Currency: "PHP"}
	got, err := LateChargeOn(10000, MustParseDate("2027-01-20"), MustParseDate("2027-01-21"), rates)
	if err != nil {
		t.Fatalf("LateChargeOn: %v", err)
	}
	if got.Surcharge != 1000 {
		t.Fatalf("surcharge = %.2f, want 1000", got.Surcharge)
	}
	if got.Interest != 110 {
		t.Fatalf("interest = %.2f, want 110", got.Interest)
	}
}

func TestLateChargeRefusesNonsense(t *testing.T) {
	due := MustParseDate("2027-01-20")
	paid := MustParseDate("2027-02-20")
	rates := DefaultLateRates(Philippines)

	cases := []struct {
		name    string
		base    float64
		due     Date
		paid    Date
		rates   LateRates
		wantErr string
	}{
		{"a negative amount", -1, due, paid, rates, "not an amount"},
		{"no deadline", 100, Date{}, paid, rates, "when was it due"},
		{"no payment date", 100, due, Date{}, rates, "when was it paid"},
		{
			"a rate entered as a percentage rather than a fraction",
			100, due, paid, LateRates{SurchargeRate: 25, InterestCapMo: 36}, "as a fraction",
		},
		{
			"a negative rate",
			100, due, paid, LateRates{SurchargeRate: -0.25, InterestCapMo: 36}, "not a rate",
		},
		{
			"a negative daily fee",
			100, due, paid, LateRates{DailyLateFee: -5, InterestCapMo: 36}, "not a rate",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := LateChargeOn(c.base, c.due, c.paid, c.rates)
			if err == nil {
				t.Fatalf("LateChargeOn accepted %s", c.name)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error = %q, want it to mention %q", err, c.wantErr)
			}
		})
	}
}

func TestLateChargeFallsBackToTheStatutoryCapWhenNoneIsSet(t *testing.T) {
	rates := LateRates{SurchargeRate: 0.25, MonthlyInterest: 0.02}
	got, err := LateChargeOn(1000, MustParseDate("2020-01-20"), MustParseDate("2027-01-20"), rates)
	if err != nil {
		t.Fatalf("LateChargeOn: %v", err)
	}
	if got.InterestMonths != PHInterestMonthsCap {
		t.Fatalf("interestMonths = %d, want the statutory cap of %d", got.InterestMonths, PHInterestMonthsCap)
	}
}

func TestLateChargePaidBeforeDueDoesNotReportNegativeDays(t *testing.T) {
	got, err := LateChargeOn(1000, MustParseDate("2027-01-20"), MustParseDate("2026-12-01"), DefaultLateRates(Philippines))
	if err != nil {
		t.Fatalf("LateChargeOn: %v", err)
	}
	if got.DaysLate != 0 {
		t.Fatalf("daysLate = %d for a payment made early, want 0", got.DaysLate)
	}
	if !strings.Contains(got.Note, "on time") {
		t.Fatalf("note = %q, want it to say the payment was on time", got.Note)
	}
}

func TestRound2(t *testing.T) {
	// A literal that looks like an exact half is often not one once it is a
	// float64: 1.005 is stored as a shade under, so it rounds down. That is
	// binary floating point rather than a rule of this tool, and the cases
	// below assert what actually happens rather than what the decimal digits
	// suggest. It does not matter in practice — a tie at half a centavo on a
	// figure the owner typed is not a number anyone is going to dispute — and
	// it is written down here so nobody later reads it as a bug.
	cases := []struct {
		in   float64
		want float64
	}{
		{0, 0},
		{1.005, 1.0},
		{1.004, 1.0},
		{1.006, 1.01},
		{30.864, 30.86},
		{30.865, 30.87},
		{-1.005, -1.0},
		{2500, 2500},
		{0.005, 0.01},
	}
	for _, c := range cases {
		if got := round2(c.in); got != c.want {
			t.Errorf("round2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestTheStatutoryCeilingsAreTheDocumentedOnes(t *testing.T) {
	if PHSurchargeRateCeiling != 0.25 {
		t.Errorf("the surcharge ceiling is %v; section 168 says twenty-five per cent", PHSurchargeRateCeiling)
	}
	if PHMonthlyInterestCeiling != 0.02 {
		t.Errorf("the interest ceiling is %v; section 168 says two per cent a month", PHMonthlyInterestCeiling)
	}
	if PHInterestMonthsCap != 36 {
		t.Errorf("the interest cap is %d months; section 168 says thirty-six", PHInterestMonthsCap)
	}
	if PHRenewalDeadlineDay != 20 || PHRenewalMonth != 1 {
		t.Errorf("the renewal deadline is day %d of month %d; section 167 says the first twenty days of January",
			PHRenewalDeadlineDay, PHRenewalMonth)
	}
	if INFSSAIRenewalLeadDays != 30 {
		t.Errorf("the FSSAI renewal lead is %d days; regulation 2.1.7(2) had thirty", INFSSAIRenewalLeadDays)
	}
	if INSurchargeRateDefault != 0 || INMonthlyInterestDefault != 0 || INDailyLateFeeDefault != 0 {
		t.Error("this tool must assert no India penalty figure")
	}
}
