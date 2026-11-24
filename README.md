# Food Permit &amp; Licence Renewal Tracker

A free, single-binary tool that keeps **every** statutory permit a food business holds in one register — across the authorities that issue them and the different cycles they run on — escalates the reminders, and answers the question that matters before the shutters go up: **is anything expired such that we are trading unlawfully today?**

It runs on your own machine, serves a small web interface from the binary itself, and stores everything in one local JSON file. No certificate is ever uploaded.

## What it does

Nobody loses the licence they think about. They lose the one nobody was watching. A restaurant in Quezon City holds a barangay clearance, a sanitary permit, health certificates for every food handler, a fire safety inspection certificate, a mayor's permit, a DTI business name registration and a BIR certificate of registration — seven permits, six offices, and only some of them on the same clock. A processor in Bengaluru holds an FSSAI licence, a GST registration, a municipal trade licence, a Shops and Establishments registration, a fire NOC and a pollution consent.

Put them all in, and the tool does five things:

- **`Can we trade today?`** — on the date you ask about, which permits have lapsed such that the business is operating unlawfully, naming each one, the office that issues it and how many days past it is. Only permits you have marked as ones you cannot trade without can make the answer no, so a lapsed weights-and-measures stamp is reported without closing the shop. A blocking permit whose expiry date nobody recorded also stops the answer being yes — the tool will not call a date it was never given valid.
- **Escalating reminders** at three tiers, 60, 30 and 7 days before expiry by default, all of them yours to change.
- **A renewal season plan.** Everything falling due in a window, grouped into steps by what blocks what, and worked backwards from the deadline along the longest chain to say when each permit has to be *started*. See below — this is the part a list of dates cannot do.
- **A renewal readiness checklist per permit**, drawn from what the offices actually ask for, ticked off as you gather things and saved as you go.
- **The cost of being late**, on an amount you type in, at rates you set.

### The Philippine January renewal rush

Permits in the Philippines do not fall due one at a time. They fall due all at once: local taxes, fees and charges accrue on the first day of January and are payable within the first twenty days of January under **sections 166 and 167 of the Local Government Code of 1991 (Republic Act No. 7160)**. So the barangay clearance, the sanitary permit, the fire certificate and the mayor's permit all come round in the same three weeks, at the same counters, behind everybody else in the city — and a business that misses the window pays a surcharge.

Worse, they do not come round independently. The barangay clearance has to be in hand before the city will issue the business permit, and so does the fire safety inspection certificate. A business that queues for them in the wrong order spends January discovering it cannot do the next thing.

The renewal season view groups the window, orders it, and works backwards. On a typical Philippine register that produces a fact the owner would not otherwise have:

```
Philippine January renewal season 2027
window 2026-12-01 to 2027-01-31 | deadline 2027-01-20 (fixed by law)

step 1  Health certificates for food handlers    start by 2026-12-17   [longest chain]
step 1  Fire safety inspection certificate       start by 2026-12-31
step 1  Barangay business clearance              start by 2027-01-05
step 2  Sanitary permit                          start by 2026-12-31   [longest chain]
          waits on  Health certificates for food handlers   (convention — check locally)
step 3  Mayor's permit (business permit)          start by 2027-01-10   [longest chain]
          waits on  Barangay business clearance             (fixed by law)
          waits on  Fire safety inspection certificate      (fixed by law)
          waits on  Sanitary permit                         (convention — check locally)
```

The food handlers' medicals have to be under way by the middle of December — not because anything expires then, but because the sanitary permit waits on them and the business permit waits on that.

### Which FSSAI tier do I need? A different tool answers that

Working out whether an Indian food business needs a **Basic Registration, a State Licence or a Central Licence** has its own thresholds and its own reasoning, and this tracker deliberately does not reimplement it — none of those turnover, capacity or milk-volume figures appear anywhere in this repository. This tool watches the licence you already hold, alongside everything else you hold. For the tier question:

<https://sreenivas-sadhu-prabhakara.github.io/fssai-license-navigator/>

## Who it's for

Small food businesses in **India** and the **Philippines**: restaurants, carinderias, bakeries, sari-sari stores, canteens, caterers, commissaries, cloud kitchens, small processors and packers — the operators without a compliance officer, where the person who renews the permits is also the person who runs the kitchen.

It is just as useful with two or three outlets. A Philippine business permit is issued per location, so each branch carries its own permits on its own dates; the tool keeps them apart and will tell you that one branch cannot trade while the others can.

## The named regulations

| Rule the tool implements | Instrument |
| --- | --- |
| Local taxes, fees and charges accrue 1 January and are payable in the first twenty days of January, or of each subsequent quarter | Local Government Code of 1991 (Republic Act No. 7160), ss. 166–167 |
| No city or municipality may issue a business permit until a barangay clearance is obtained from the barangay where the business is located | Republic Act No. 7160, s. 152 |
| Surcharge not exceeding 25% of the amount not paid on time, interest not exceeding 2% a month on the unpaid amount including the surcharge, total interest capped at 36 months | Republic Act No. 7160, s. 168 |
| Issuing or renewing a business permit without a fire safety inspection certificate is a prohibited act; the premises is inspected each time the business permit is renewed | Fire Code of the Philippines of 2008 (Republic Act No. 9514) |
| A food establishment operates on a sanitary permit from the local health officer; a food handler holds a health certificate from the local health officer | Code on Sanitation of the Philippines (Presidential Decree No. 856) |
| The BIR annual registration fee and its January Form 0605 filing were removed; the Bureau ceased collection on 22 January 2024 | Ease of Paying Taxes Act (Republic Act No. 11976) |
| FSSAI licensing and the thirty-day renewal lead | Food Safety and Standards Act, 2006; Food Safety and Standards (Licensing and Registration of Food Businesses) Regulations, 2011, reg. 2.1.7(2) |

The two statutory sequencing links — barangay clearance and fire certificate before the business permit — are shipped marked **fixed by law** with the instrument quoted. **Every other link in the default ordering is shipped marked as a convention**, because we could not source it. See *Scope and accuracy*.

## Install and run

One binary, nothing outside the Go standard library. With Go 1.25 or later:

```sh
# build it once, then run the binary anywhere
go build -o food-permit-license-renewal-tracker .
./food-permit-license-renewal-tracker

# or run it straight from the source directory
go run .

# choose a different port or data file
./food-permit-license-renewal-tracker -addr 127.0.0.1:8137 -data ~/permits.json
```

Then open <http://127.0.0.1:8080/>.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-addr` | `127.0.0.1:8080` | Address to listen on. Loopback only, so the tool is not reachable from your network. |
| `-data` | `./food-permit-license-renewal-tracker-data.json` | Path to the local data file. |

Run the tests with `go test ./...`.

Start by adding a jurisdiction preset under **Data &amp; settings** — a named starting list for India or the Philippines, each entry carrying the office that issues it and a note on how much its validity varies locally. Then delete the entries that do not apply to you and type in the dates from your own papers.

## Where your data is stored

In the single JSON file named by `-data`, holding your permits, your dates, your checklist ticks and your settings. The path is printed when the tool starts and shown in the interface. Back it up by copying that one file.

It is written with owner-only permissions, and written atomically — to a temporary file in the same directory, then renamed into place — so an interrupted write cannot corrupt what you already had.

**No certificate is ever uploaded or stored.** Each permit holds a *reference* to where the paper is kept: a path or a sentence in your own words. The tool never reads it, copies it or sends it anywhere. There is no document vault here and there is not going to be one.

Nothing else leaves your machine. The interface is embedded in the binary, the tool binds to the loopback address, and it makes **no outbound network call of any kind** — no account, no telemetry, no analytics, no update check, no third-party script or font, and no connection to any government portal. The portal addresses in the interface are plain text for you to copy into your own browser.

## Indicative figures

Every figure is a named constant in one place — `internal/permits/params.go` — labelled indicative, stated in the interface, and overridable inside the tool. The arithmetic follows your figure once you have set one.

**There is no fee table anywhere in this tool.** That is deliberate: a food permit fee is set by a city or municipal ordinance, a state by-law or a barangay resolution, and assessed against your own gross receipts. A national figure does not exist, so you type in what you were assessed and the tool prices the delay on it.

| Figure | Value used | Basis |
| --- | --- | --- |
| Reminder tiers | 60, 30 and 7 days before expiry | This tool's own convention |
| Processing lead time | 7 days, and per permit in the presets | A convention — no authority publishes one |
| Philippine renewal deadline | 20 January | RA 7160 s. 167; a city may extend it |
| Surcharge on a late local payment | 25% of the unpaid amount, once | RA 7160 s. 168 — a **ceiling** |
| Interest on a late local payment | 2% a month on the unpaid amount including the surcharge | RA 7160 s. 168 — a **ceiling** |
| Interest months cap | 36 months | RA 7160 s. 168 |
| FSSAI renewal lead | 30 days before expiry | FSS (Licensing and Registration) Regulations, 2011, reg. 2.1.7(2) |
| Preset validity periods | 12 months for most; 60 months for a DTI business name | Indicative starting points |
| India late fee | **nothing — this tool asserts no India penalty figure** | See below |
| Permit fees | **none shipped, anywhere** | Set locally, assessed on your own figures |

A permit is valid through the whole of its expiry date, so the day of expiry is nought days remaining and not yet overdue.

## Scope and accuracy

**Validity periods, fees and deadlines vary by city, municipality, barangay and state, and they change.** This is a planning aid. It is not legal advice, it is not a filing service, it submits nothing on your behalf, and it is connected to no government system. It knows only what you typed in: a permit you did not add cannot appear, and a date typed wrongly stays wrong.

Three places where we stopped short rather than assert something we could not stand behind:

- **The FDA licence to operate ships no validity period.** The FDA has revised it more than once and the figures in circulation conflict — two years initial and five on renewal in one account, three and six by enterprise size in another, five and ten in a third. We could not confirm which is current, so the tool asserts none of them and asks for the dates printed on your own licence.
- **The FSSAI ₹100-a-day late fee is not encoded.** It is widely quoted and was set out in regulation 2.1.7(4) of the 2011 regulations. FSSAI's own 2025 stakeholder notice on fixed renewal cycles states that it has been discontinued, and we found no final order settling the position. The India late-payment rates therefore start at nought and you enter what you are actually charged.
- **Most of the prerequisite ordering is a convention, and is labelled one.** Only two links are backed by a named instrument. Every city and municipal checklist we read lists the sanitary permit among the requirements for a food establishment's business permit, but nothing we found fixes it before or after the fire certificate, or fixes the food handlers' health certificates before the establishment's sanitary permit. Those links ship marked *check locally* and you reorder or delete them. Many cities now run the whole set through one one-stop shop in January, in which case the order stops mattering.

Permits from the starting lists that you have not added are shown as **gaps** — a prompt to decide whether you need that permit, never a finding that you are breaking the law. Whether a food business needs an FDA licence to operate, a police eating house licence or a pollution control consent depends on what it does and where, and only you know that.

Confirm every date, period, fee and requirement with the authority named against the permit — your barangay hall, city or municipal hall, health office, fire station, revenue district office, FoSCoS, or your state's department — before you act on anything here.

MIT licensed.
