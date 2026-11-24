package permits

// Every figure this tool uses lives in this file, is labelled indicative, and
// can be overridden inside the tool. Nothing here is a published table: a
// validity period, a processing lead time and a surcharge rate are all set
// locally — by a state, by a city or municipal ordinance, by a barangay — and
// they are revised. The tool's arithmetic always follows the owner's own
// figure once they have set one, and the shipped number is only a starting
// point so that an empty register is not useless.
//
// Confirm every figure with the office named against the permit before you
// rely on it.

// ---------- reminder tiers ----------

// The default escalating reminder tiers, in days before expiry. Sixty days is
// about the point at which a renewal that needs an inspection can still be
// started calmly; thirty is the last comfortable moment; seven is an
// emergency. These are this tool's own convention and not a statutory
// requirement of any authority — the owner sets their own.
const (
	DefaultNoticeDays = 60
	DefaultWarnDays   = 30
	DefaultUrgentDays = 7
)

// MaxTierDays caps a tier so a mistyped figure cannot swallow the whole board.
const MaxTierDays = 3650

// ---------- processing lead times ----------

// DefaultLeadDays is how long the tool assumes one permit takes to come
// through once it has been applied for, used to work backwards from a deadline
// and say when to start. It is a convention, not a published service standard:
// no authority in either market publishes a figure this tool could quote, and
// the real answer depends on the office, the queue and the month. Every permit
// carries its own lead time in the preset, and every one of them is editable.
const DefaultLeadDays = 7

// MaxLeadDays caps a lead time at a year, which is already far beyond anything
// plausible and stops a typo producing a start date in the last decade.
const MaxLeadDays = 365

// ---------- the Philippine January renewal window ----------
//
// Local taxes, fees and charges accrue on the first day of January and are
// payable within the first twenty days of January, or of each subsequent
// quarter — Local Government Code of 1991 (Republic Act No. 7160), sections
// 166 and 167. That is the statutory shape of the January rush: every
// business permit in the country falls due in the same three weeks.
//
// A sanggunian may extend the deadline for a justifiable reason, and many
// cities do so by ordinance in a given year, so the window is a default and
// not a promise. Check your own city or municipal ordinance for the year.

const (
	// PHRenewalMonth is January: the month local business taxes accrue in.
	PHRenewalMonth = 1
	// PHRenewalDeadlineDay is the twentieth, the end of the first statutory
	// payment window under section 167.
	PHRenewalDeadlineDay = 20
)

// PHQuarterlyDeadlineDays are the first-twenty-days windows of each quarter,
// for a business paying its local business tax in instalments rather than in
// full in January. Section 167 again.
var PHQuarterlyDeadlineMonths = [4]int{1, 4, 7, 10}

// ---------- late payment of a Philippine local business tax ----------
//
// Local Government Code of 1991 (Republic Act No. 7160), section 168: a local
// government unit may impose a surcharge not exceeding twenty-five per cent of
// the amount of taxes, fees or charges not paid on time, and an interest not
// exceeding two per cent per month of the unpaid amount including the
// surcharge, until it is paid — provided that the total interest on the unpaid
// amount or portion of it does not exceed thirty-six months.
//
// These are the statutory CEILINGS. The rate that actually applies to you is
// whatever your own city or municipal revenue code sets at or below them, and
// some LGUs set less. The tool ships the ceilings because a ceiling is the only
// figure that is the same everywhere, and it labels the result as the worst
// case rather than as your bill.

const (
	// PHSurchargeRateCeiling is 25% of the unpaid amount, charged once.
	PHSurchargeRateCeiling = 0.25
	// PHMonthlyInterestCeiling is 2% a month, charged on the unpaid amount
	// including the surcharge.
	PHMonthlyInterestCeiling = 0.02
	// PHInterestMonthsCap is the thirty-six month limit on the interest.
	PHInterestMonthsCap = 36
)

// ---------- India ----------
//
// This tool ships no India late-fee figure, deliberately.
//
// The ₹100-per-day late fee for an FSSAI renewal filed inside the thirty days
// before expiry is widely quoted and was set out in regulation 2.1.7(4) of the
// Food Safety and Standards (Licensing and Registration of Food Businesses)
// Regulations, 2011. FSSAI's own 2025 stakeholder notice on fixed renewal
// cycles states that this late fee has been discontinued. We could not find a
// final order settling the position either way, so quoting ₹100 a day as
// current would be asserting something we cannot stand behind. The India late
// charge therefore starts at zero and the owner enters whatever FoSCoS
// actually charges them.
//
// The municipal trade licence, the fire NOC and the state Shops and
// Establishments renewal each carry their own penalty under their own local
// by-law, and there is no national figure to ship.

const (
	// INSurchargeRateDefault and INMonthlyInterestDefault are zero: the tool
	// asserts no India penalty rate. Set them in the tool to whatever your
	// corporation or FoSCoS charges you.
	INSurchargeRateDefault   = 0.0
	INMonthlyInterestDefault = 0.0
	// INDailyLateFeeDefault is likewise zero, for the same reason.
	INDailyLateFeeDefault = 0.0
)

// INFSSAIRenewalLeadDays is the thirty-day window before expiry inside which
// an FSSAI renewal was to be filed under regulation 2.1.7(2) of the 2011
// Licensing and Registration Regulations — file earlier than this and you are
// inside the ordinary window. It is used only to set the FSSAI permit's
// default reminder lead, and FSSAI consulted on changing the renewal mechanics
// in 2025, so confirm it on FoSCoS.
const INFSSAIRenewalLeadDays = 30

// ---------- sanity caps ----------

const (
	// MaxValidityMonths caps a validity period at sixty years, which is longer
	// than any permit either market issues and short enough that a typo is
	// caught.
	MaxValidityMonths = 720
	// MaxTextLen caps every free-text field the owner types, so a pasted
	// document cannot bloat the data file.
	MaxTextLen = 500
	// MaxPermits caps the register. A food business holds a handful of
	// permits, not thousands; the cap exists so a broken import cannot fill
	// the disk.
	MaxPermits = 2000
)
