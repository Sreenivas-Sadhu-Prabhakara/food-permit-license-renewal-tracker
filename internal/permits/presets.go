package permits

// The named jurisdiction presets.
//
// A preset is a starting list, not an authority. Every entry names the office
// that issues the permit, carries an indicative validity period and says in its
// own note how much that period varies locally. The owner adds the entries that
// apply to their business, deletes the ones that do not, and corrects every
// date and period from the paper in front of them.
//
// Two things are deliberately missing from every entry: a fee, and a tier. No
// fee is shipped anywhere in this tool, because a food business permit fee is
// set by a city or municipal ordinance, a state by-law or a barangay
// resolution, and a national figure does not exist. No tier logic is shipped
// either: which FSSAI tier an Indian food business needs is a separate question
// with its own rules, and the FSSAI License Navigator answers it.

// Prereq is one edge of the sequencing graph: this permit cannot be had, or is
// not worth applying for, until the named one is in hand.
type Prereq struct {
	Kind  Kind   `json:"kind"`
	Basis Basis  `json:"basis"`
	Why   string `json:"why"`
}

// Preset is one shipped permit entry for a market.
type Preset struct {
	Kind         Kind         `json:"kind"`
	Jurisdiction Jurisdiction `json:"jurisdiction"`
	Label        string       `json:"label"`
	Authority    string       `json:"authority"`
	AppliesTo    string       `json:"appliesTo"`

	// ValidityMonths is the indicative period, or zero where the period
	// genuinely varies so much that shipping one would be a guess. Zero means
	// the tool will not suggest an expiry date for this permit: enter the date
	// printed on the paper.
	ValidityMonths int `json:"validityMonths"`

	// NoExpiry marks a registration that does not lapse. It still sits on the
	// board, because an owner needs to see that it exists and is in order.
	NoExpiry bool `json:"noExpiry"`

	// Blocks says whether trading with this one expired makes the business
	// unlawful rather than merely untidy. It is the flag CanTradeToday reads,
	// and it is a default the owner should check against their own premises.
	Blocks bool `json:"blocks"`

	// LeadDays is how far ahead of the deadline to start, a convention rather
	// than a published service standard. See DefaultLeadDays.
	LeadDays int `json:"leadDays"`

	// SeasonAnchor marks a permit that falls in the Philippine January
	// renewal window with everything else, rather than running on its own
	// anniversary.
	SeasonAnchor bool `json:"seasonAnchor"`

	Prereqs   []Prereq `json:"prereqs"`
	Checklist []string `json:"checklist"`
	Note      string   `json:"note"`
}

// indiaPresets is the India starting list: eight permits a food business
// commonly holds, from four different authorities on four different cycles.
func indiaPresets() []Preset {
	return []Preset{
		{
			Kind:           KindFSSAI,
			Jurisdiction:   India,
			Label:          "FSSAI licence or registration",
			Authority:      "Food Safety and Standards Authority of India — filed and renewed on FoSCoS",
			AppliesTo:      "Every food business: manufacturer, processor, trader, wholesaler, retailer, restaurant, caterer, transporter, cold store, home kitchen or cloud kitchen.",
			ValidityMonths: 12,
			Blocks:         true,
			LeadDays:       INFSSAIRenewalLeadDays,
			Note: "You chose the validity when you applied — anywhere from one to five years, priced per year — so the only right answer is the date printed on your licence. " +
				"The thirty-day lead comes from regulation 2.1.7(2) of the Food Safety and Standards (Licensing and Registration of Food Businesses) Regulations, 2011, which had a renewal filed no later than thirty days before expiry; FSSAI consulted in 2025 on fixed renewal cycles and longer validity, so confirm the current mechanics on FoSCoS. " +
				"Which of the three tiers you need — Basic Registration, State Licence or Central Licence — is a different question with its own thresholds, and this tool does not try to answer it.",
			Prereqs: []Prereq{{
				Kind:  KindTradeLicence,
				Basis: FromConvention,
				Why:   "Several states ask for a no-objection certificate or trade licence from the local municipality or panchayat among the FSSAI documents. It is not asked for everywhere, so delete this link if your state does not.",
			}},
			Checklist: []string{
				"Sign in to FoSCoS with the credentials tied to this licence",
				"Check that the Kind of Business list still matches what you actually do, and amend it if it does not",
				"Form IX nomination, if the nominated person has changed",
				"Food safety management system plan or certificate, where your tier asks for one",
				"Water analysis report from a recognised laboratory, where your tier asks for one",
				"Current proof of the premises: rent agreement or ownership document",
				"Photo identity and address proof of the proprietor, partners or directors",
				"Pay the renewal fee for the number of years you want, and save both the receipt and the new licence",
			},
		},
		{
			Kind:         KindGST,
			Jurisdiction: India,
			Label:        "GST registration",
			Authority:    "Central Board of Indirect Taxes and Customs with your state tax authority, on the GST portal",
			AppliesTo:    "Any food business registered for GST, whether by crossing the turnover threshold or voluntarily.",
			NoExpiry:     true,
			Blocks:       true,
			Note: "A regular GST registration does not expire, so this sits on the board as a no-expiry row that reminds you it exists. " +
				"A casual taxable person's or a non-resident taxable person's registration DOES expire, on the date in the certificate; if that is what you hold, clear the no-expiry box and enter that date. " +
				"The real way to lose a regular registration is cancellation by the officer for not filing returns, which no expiry date can warn you about — that risk lives in your filing calendar, not here.",
			Checklist: []string{
				"Confirm the registration still shows as Active on the GST portal",
				"Check the principal and additional places of business against where you actually operate",
				"File every return that is due — a registration cancelled for non-filing is the usual way this is lost",
				"Keep the registration certificate, Form REG-06, with the permit file",
			},
		},
		{
			Kind:           KindTradeLicence,
			Jurisdiction:   India,
			Label:          "Municipal trade licence",
			Authority:      "Your city corporation, municipality or panchayat — usually the health or licensing department",
			AppliesTo:      "Premises trading inside the local body's limits. Names differ: trade licence, health trade licence, general licence.",
			ValidityMonths: 12,
			Blocks:         true,
			LeadDays:       21,
			Note: "The most local permit on this board. The name, the fee, the validity and the renewal date are all set by your own corporation's by-laws — many run it to the financial year and want the renewal by 31 March, some to the calendar year, some issue it for longer than a year. " +
				"There is no national figure to ship, so confirm yours at the ward office and set the date you are actually given.",
			Prereqs: []Prereq{{
				Kind:  KindFireNOC,
				Basis: FromConvention,
				Why:   "Many corporations ask for a current fire safety no-objection certificate with the trade licence renewal, for premises that need one. Delete this link if yours does not.",
			}},
			Checklist: []string{
				"Last year's trade licence and the fee receipt",
				"Current property tax receipt for the premises, yours or the landlord's",
				"Rent agreement or ownership document",
				"Your FSSAI licence number, which most corporations now ask for",
				"Fire safety no-objection certificate, where the premises needs one",
				"Pay the renewal fee at the ward office or on the corporation's portal",
			},
		},
		{
			Kind:         KindShopsEstab,
			Jurisdiction: India,
			Label:        "Shops and establishments registration",
			Authority:    "Your state labour department, under the state Shops and Commercial Establishments Act",
			AppliesTo:    "Any shop or commercial establishment with employees, which includes most restaurants and retail food shops.",
			Blocks:       false,
			LeadDays:     14,
			Note: "Validity is a state matter and the states differ sharply: several now issue a one-time registration that never needs renewing, while others renew it every year or every five. " +
				"We ship no default period for exactly that reason — enter what your certificate says, or mark the row no-expiry if your state issues it for the life of the business.",
			Checklist: []string{
				"The current registration certificate",
				"Employee count and details, which the renewal form asks for",
				"Weekly off and working hours as you actually run them",
				"Any change of address or of the employer's name since the last renewal",
			},
		},
		{
			Kind:           KindFireNOC,
			Jurisdiction:   India,
			Label:          "Fire safety no-objection certificate",
			Authority:      "Your state fire and emergency services department",
			AppliesTo:      "Premises that cross the height, area or occupancy threshold the state's fire service rules set. Many small shops and kitchens are below it.",
			ValidityMonths: 12,
			Blocks:         false,
			LeadDays:       30,
			Note: "Required where the premises crosses your state's own threshold, and those thresholds, the validity and the fee are all state matters — many states renew yearly, some issue it for longer. Twelve months is a starting point and nothing more. " +
				"Leave this row as non-blocking if your premises does not need an NOC at all; set it to block if it does, because running premises that require one without a current one is an offence in its own right.",
			Checklist: []string{
				"Previous no-objection certificate and the inspection report",
				"Building plan approved by the local authority",
				"Service records for the extinguishers, hydrants, alarm and sprinkler system",
				"Fire drill or staff training record, where the rules ask for one",
				"Apply for the inspection and keep a note of the date you are given",
			},
		},
		{
			Kind:           KindEatingHouse,
			Jurisdiction:   India,
			Label:          "Police eating house licence",
			Authority:      "The Commissioner of Police, licensing branch, in the states that require one",
			AppliesTo:      "Eating houses in states that license them through the police — Delhi, Karnataka, Maharashtra and others. Delete this row if your state does not.",
			ValidityMonths: 12,
			Blocks:         false,
			LeadDays:       30,
			Note:           "Only some states require this at all, and both the validity and the fee are set by the state. Check with the licensing branch whether your premises needs one before you chase a renewal you may not owe.",
			Prereqs: []Prereq{
				{Kind: KindFSSAI, Basis: FromConvention, Why: "The police licensing branch asks for the FSSAI licence with the application. Delete this link if yours does not."},
				{Kind: KindTradeLicence, Basis: FromConvention, Why: "The trade licence is normally asked for alongside it. Delete this link if yours does not."},
			},
			Checklist: []string{
				"Previous eating house licence",
				"FSSAI licence",
				"Trade licence",
				"Fire safety no-objection certificate, where the premises needs one",
				"Proof of the premises and identity proof of the occupier",
				"Police verification of the staff, where the state asks for it",
			},
		},
		{
			Kind:           KindLegalMetrology,
			Jurisdiction:   India,
			Label:          "Legal metrology verification of weighing equipment",
			Authority:      "Your state legal metrology department",
			AppliesTo:      "Any weighing or measuring instrument used in trade — a counter scale, a platform scale, a measure.",
			ValidityMonths: 12,
			Blocks:         false,
			LeadDays:       14,
			Note:           "An instrument used in trade has to carry a current verification stamp. The re-verification cycle is set by state rules and differs by class of instrument, so check the period for the scales you actually use rather than taking the twelve months here.",
			Checklist: []string{
				"List of every scale, weight and measure used in trade",
				"Previous verification certificates and their stamping dates",
				"Book the inspector's visit, or take the instruments in to the office",
				"Packaged commodity declarations on your labels, if you pack for retail",
			},
		},
		{
			Kind:         KindPollutionNOC,
			Jurisdiction: India,
			Label:        "Pollution control board consent to operate",
			Authority:    "Your State Pollution Control Board or Pollution Control Committee",
			AppliesTo:    "Food processing units in the Board's categories. A small retailer or restaurant is often outside it altogether.",
			Blocks:       false,
			LeadDays:     60,
			Note: "Food processing falls in the Board's red, orange, green or white categories depending on what you make and how much, and the consent period follows the category — a few years to fifteen, with white-category units often outside the consent regime entirely. " +
				"There is no single period worth shipping, so enter the one on your consent order.",
			Checklist: []string{
				"Current consent to operate order, and the category it puts you in",
				"Water and electricity consumption figures the renewal form asks for",
				"Effluent and emission test reports from a recognised laboratory",
				"Evidence the effluent treatment plant works, where you have one",
				"Hazardous waste authorisation, if you hold one",
			},
		},
	}
}

// philippinesPresets is the Philippines starting list. Five of these eight fall
// due together in January, which is what the renewal season view exists for,
// and three of the January five have to happen in a particular order.
func philippinesPresets() []Preset {
	return []Preset{
		{
			Kind:           KindBarangayClearance,
			Jurisdiction:   Philippines,
			Label:          "Barangay business clearance",
			Authority:      "The barangay where the business operates",
			AppliesTo:      "Every business, at the barangay of its address.",
			ValidityMonths: 12,
			Blocks:         true,
			LeadDays:       5,
			SeasonAnchor:   true,
			Note: "Renewed for the calendar year alongside the business permit, and it has to be first: section 152 of the Local Government Code of 1991 (Republic Act No. 7160) bars a city or municipality from issuing a business permit until a clearance is obtained from the barangay where the business is located. " +
				"The fee and the validity are set by barangay ordinance, so they differ from one barangay to the next inside the same city — confirm both at your own barangay hall.",
			Checklist: []string{
				"Last year's barangay clearance",
				"The barangay's own application form, filled in",
				"Proof of the business address: lease contract or title",
				"Pay the barangay fee and keep the receipt — the city hall will ask for it",
			},
		},
		{
			Kind:           KindHealthCertificate,
			Jurisdiction:   Philippines,
			Label:          "Health certificates for food handlers",
			Authority:      "The city or municipal health office",
			AppliesTo:      "Every person who handles food, individually. A new hire needs one before they start.",
			ValidityMonths: 12,
			Blocks:         false,
			LeadDays:       14,
			SeasonAnchor:   true,
			Note: "This one belongs to a person and not to the business. Under the Code on Sanitation of the Philippines (Presidential Decree No. 856) a food handler holds a health certificate issued by the local health officer, and the examinations behind it are the health office's own list. " +
				"Track the earliest expiry among your staff in this row and keep the roster in the document note, because the staff member whose certificate lapses first is the one who stops you passing an inspection. " +
				"Confirm the validity and the tests required with your own health office, and enter the date printed on the certificates.",
			Checklist: []string{
				"Roster of everyone who handles food, with each certificate's expiry date",
				"Medical examination at the city health office or an accredited clinic",
				"Food handler's seminar certificate, where the city requires one",
				"Collect the certificates and keep copies on the premises where an inspector can see them",
			},
		},
		{
			Kind:           KindSanitaryPermit,
			Jurisdiction:   Philippines,
			Label:          "Sanitary permit",
			Authority:      "The city or municipal health office, through the local health officer",
			AppliesTo:      "Every food establishment: restaurant, carinderia, bakery, store, commissary, canteen.",
			ValidityMonths: 12,
			Blocks:         true,
			LeadDays:       10,
			SeasonAnchor:   true,
			Note: "A food establishment operates on a sanitary permit from the local health officer under the Code on Sanitation of the Philippines (Presidential Decree No. 856) and its implementing rules. " +
				"The permit is issued and renewed locally, and the fee, the validity, the inspection routine and the supporting tests are the city's or municipality's own — confirm the period and the renewal date with your health office, and enter the date printed on the permit.",
			Prereqs: []Prereq{{
				Kind:  KindHealthCertificate,
				Basis: FromConvention,
				Why:   "Health offices commonly want the food handlers' certificates in hand before releasing the establishment's sanitary permit. We found no instrument that fixes this order, so it is a convention — reorder it to match your own health office.",
			}},
			Checklist: []string{
				"Last year's sanitary permit",
				"Health certificates for every food handler on the payroll",
				"Water potability test for your source, where the health office asks for it",
				"Pest control service record",
				"Pass the sanitary inspection of the premises",
			},
		},
		{
			Kind:           KindFSIC,
			Jurisdiction:   Philippines,
			Label:          "Fire safety inspection certificate",
			Authority:      "The Bureau of Fire Protection station with jurisdiction over the premises",
			AppliesTo:      "Every business premises, inspected again each time the business permit is renewed.",
			ValidityMonths: 12,
			Blocks:         true,
			LeadDays:       10,
			SeasonAnchor:   true,
			Note: "The Fire Code of the Philippines of 2008 (Republic Act No. 9514) makes the issuance or renewal of a business permit without a fire safety inspection certificate a prohibited act, and has the fire safety inspector inspect the premises every time the business permit is renewed. " +
				"The fire code fees are assessed by the Bureau under the Code's own schedule against your own figures, so there is no fee for this tool to quote — ask the station for the assessment, and enter the date printed on the certificate.",
			Checklist: []string{
				"Last year's fire safety inspection certificate",
				"Fire insurance policy, where you hold one",
				"Service receipts for the extinguishers and any sprinkler or alarm system",
				"Clear the exits and check the exit signage before the inspector arrives",
				"Fire code fee assessment and the payment receipt",
				"Pass the Bureau's inspection",
			},
		},
		{
			Kind:           KindMayorsPermit,
			Jurisdiction:   Philippines,
			Label:          "Mayor's permit (business permit)",
			Authority:      "The Business Permits and Licensing Office of your city or municipality",
			AppliesTo:      "Every business, at every location it operates from.",
			ValidityMonths: 12,
			Blocks:         true,
			LeadDays:       10,
			SeasonAnchor:   true,
			Note: "The one everybody means by 'renewal', and the reason January is the way it is. Local taxes, fees and charges accrue on the first day of January and are payable within the first twenty days of January, or of each subsequent quarter, under sections 166 and 167 of the Local Government Code of 1991 (Republic Act No. 7160). " +
				"A sanggunian may extend the deadline for a justifiable reason and several cities do so by ordinance, so confirm the date your own city has set for the year before you trust the twentieth.",
			Prereqs: []Prereq{
				{
					Kind:  KindBarangayClearance,
					Basis: FromStatute,
					Why:   "Local Government Code of 1991 (Republic Act No. 7160), section 152: no city or municipality may issue a business permit unless a clearance is first obtained from the barangay where the business is located.",
				},
				{
					Kind:  KindFSIC,
					Basis: FromStatute,
					Why:   "Fire Code of the Philippines of 2008 (Republic Act No. 9514): the issuance or renewal of a business permit without a fire safety inspection certificate is a prohibited act.",
				},
				{
					Kind:  KindSanitaryPermit,
					Basis: FromConvention,
					Why:   "Every city and municipal renewal checklist we read lists the sanitary permit among the requirements for a food establishment's business permit, but we found nothing that fixes it before or after the fire certificate. Many cities now run all of them through one one-stop shop in January, in which case the order stops mattering — reorder these to match your own city hall.",
				},
			},
			Checklist: []string{
				"Barangay business clearance for the year",
				"Fire safety inspection certificate",
				"Sanitary permit",
				"Last year's business permit and the official receipt",
				"Sworn declaration of last year's gross receipts — the business tax is assessed on it",
				"Audited financial statement or the BIR-received income tax return, where the city asks for one",
				"Community tax certificate (cedula)",
				"Lease contract or title for the premises",
				"Pay the assessed business tax and the permit and regulatory fees, then claim the permit, the plate and the sticker",
			},
		},
		{
			Kind:         KindBusinessName,
			Jurisdiction: Philippines,
			Label:        "Business name or company registration",
			Authority:    "The Department of Trade and Industry for a sole proprietorship; the Securities and Exchange Commission for a corporation or partnership; the Cooperative Development Authority for a cooperative",
			AppliesTo:    "Every business, in the form it is organised as.",
			// A DTI business name registration runs five years from
			// registration. Indicative and overridable like everything else.
			ValidityMonths: 60,
			Blocks:         true,
			LeadDays:       5,
			Note: "A DTI business name registration runs five years from registration and is renewed with the DTI, not with the city. " +
				"A corporation's or partnership's SEC registration does not expire — mark this row no-expiry if that is what you hold — though it carries annual reportorial filings that are a different job from this one. " +
				"The five years is indicative: confirm the dates on your own certificate.",
			Checklist: []string{
				"Business name certificate and its expiry date",
				"Renew with the DTI inside the window it allows around expiry",
				"Check that the name, the scope and the address still match the business",
				"Keep the certificate with the permit file — the city hall and the BIR both ask for it",
			},
		},
		{
			Kind:         KindBIRRegistration,
			Jurisdiction: Philippines,
			Label:        "BIR certificate of registration (Form 2303)",
			Authority:    "The Bureau of Internal Revenue revenue district office the business is registered with",
			AppliesTo:    "Every business.",
			NoExpiry:     true,
			Blocks:       true,
			Note: "The certificate of registration does not expire, so this is a no-expiry row. " +
				"The ₱500 annual registration fee and the BIR Form 0605 that went with it every January were removed by the Ease of Paying Taxes Act (Republic Act No. 11976); the Bureau stopped collecting the fee on 22 January 2024. A certificate printed before then that still shows the fee stays valid, and replacing it is optional. " +
				"Your monthly, quarterly and annual BIR filing deadlines are a different calendar and this tool does not keep it.",
			Prereqs: []Prereq{{
				Kind:  KindMayorsPermit,
				Basis: FromConvention,
				Why:   "A new BIR registration asks for the mayor's permit, or proof that it has been applied for. Nothing needs to happen here at renewal time, because the certificate does not expire — the link matters only when you first register or when you move.",
			}},
			Checklist: []string{
				"Certificate of registration, Form 2303, filed where you can find it",
				"Ask your revenue district office for a replacement certificate if yours still shows the annual registration fee and you would rather it did not",
				"Check that the registered line of business and the address still match reality, and file Form 1905 if they do not",
				"Keep the books of accounts registered and the authority to print receipts current",
			},
		},
		{
			Kind:         KindFDALTO,
			Jurisdiction: Philippines,
			Label:        "FDA licence to operate",
			Authority:    "The Food and Drug Administration, Center for Food Regulation and Research",
			AppliesTo:    "Food manufacturers, repackers, importers, exporters, distributors, wholesalers and traders. NOT an ordinary restaurant, carinderia or store, which operates on the local sanitary permit instead — delete this row if it does not apply to you.",
			Blocks:       true,
			LeadDays:     45,
			Note: "We ship no default validity for this one on purpose. The FDA has revised the licence to operate's validity more than once and the figures in circulation conflict with each other — two years initial and five on renewal in one account, three and six by enterprise size in another, five and ten in a third. " +
				"We could not confirm which is current, so asserting any of them here would be guessing. Enter the dates printed on your own licence and confirm the renewal window with the FDA before you plan around it.",
			Prereqs: []Prereq{{
				Kind:  KindMayorsPermit,
				Basis: FromConvention,
				Why:   "The FDA's application checklist asks for a valid business permit, so the city hall renewal has to be through first. Reorder or remove this link if your own category is handled differently.",
			}},
			Checklist: []string{
				"Current licence to operate and its validity dates",
				"Apply through the FDA's own portal before expiry — a lapsed licence can be pushed back to an initial application, which is slower and dearer",
				"Proof of business registration: DTI, SEC or CDA",
				"Valid mayor's or business permit",
				"The establishment documents the FDA checklist asks for in your category",
				"Food safety or risk management plan, where your category needs one",
				"Pay the fee assessed for your category and keep the order of payment",
			},
		},
	}
}

// AllPresets is every shipped entry, India first then the Philippines, in the
// order the interface lists them.
func AllPresets() []Preset {
	out := indiaPresets()
	return append(out, philippinesPresets()...)
}

// PresetsFor is the starting list for one market.
func PresetsFor(j Jurisdiction) []Preset {
	var out []Preset
	for _, p := range AllPresets() {
		if p.Jurisdiction == j {
			out = append(out, p)
		}
	}
	return out
}

// PresetFor looks up one shipped entry. A kind the owner invented is not in
// the presets, which is not an error — it just has no shipped defaults.
func PresetFor(kind Kind) (Preset, bool) {
	for _, p := range AllPresets() {
		if p.Kind == kind {
			return p, true
		}
	}
	return Preset{}, false
}

// ToPermit turns a preset entry into a register row ready to be saved, with the
// checklist unticked and the dates empty. The owner then types in the number
// and the dates from the paper.
func (p Preset) ToPermit() Permit {
	permit := Permit{
		Kind:           p.Kind,
		Jurisdiction:   p.Jurisdiction,
		Label:          p.Label,
		Authority:      p.Authority,
		ValidityMonths: p.ValidityMonths,
		NoExpiry:       p.NoExpiry,
		Blocks:         p.Blocks,
		LeadDays:       p.LeadDays,
		SeasonAnchor:   p.SeasonAnchor,
		FromPreset:     true,
	}
	if permit.LeadDays == 0 && !permit.NoExpiry {
		permit.LeadDays = DefaultLeadDays
	}
	permit.Prereqs = append(permit.Prereqs, p.Prereqs...)
	for _, item := range p.Checklist {
		permit.Checklist = append(permit.Checklist, ChecklistItem{Text: item})
	}
	return permit
}
