package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sreenivas-sadhu-prabhakara/food-permit-license-renewal-tracker/internal/permits"
)

// maxBodyBytes caps every request body. The interface never sends anything
// large, and a CSV of a few hundred permits is well inside this.
const maxBodyBytes = 4 << 20 // 4 MiB

// Server wires the local web interface and the JSON API over one Store.
// Everything is same-origin: the interface is served from the binary and talks
// to /api on the same loopback address. There is no other origin involved and
// no outbound request of any kind.
type Server struct {
	store *Store
	ui    fs.FS
	mux   *http.ServeMux
}

// NewServer builds the router.
func NewServer(store *Store, ui fs.FS) *Server {
	s := &Server{store: store, ui: ui, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A local tool still sets the headers that stop a stray page in another
	// tab from reading somebody's licence numbers.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.Handle("GET /", http.FileServerFS(s.ui))

	s.mux.HandleFunc("GET /api/state", s.handleState)
	s.mux.HandleFunc("GET /api/board", s.handleBoard)
	s.mux.HandleFunc("GET /api/can-trade", s.handleCanTrade)
	s.mux.HandleFunc("GET /api/season", s.handleSeason)
	s.mux.HandleFunc("GET /api/suggest-expiry", s.handleSuggestExpiry)
	s.mux.HandleFunc("GET /api/late-charge", s.handleLateCharge)
	s.mux.HandleFunc("GET /api/register.csv", s.handleExportRegister)
	s.mux.HandleFunc("GET /api/season.csv", s.handleExportSeason)

	s.mux.HandleFunc("PUT /api/permit", s.handlePutPermit)
	s.mux.HandleFunc("DELETE /api/permit", s.handleDeletePermit)
	s.mux.HandleFunc("POST /api/preset", s.handleAddPreset)
	s.mux.HandleFunc("PUT /api/checklist", s.handlePutChecklist)
	s.mux.HandleFunc("PUT /api/settings", s.handlePutSettings)
	s.mux.HandleFunc("POST /api/register.csv", s.handleImportRegister)
}

// ---------- reads ----------

type stateResponse struct {
	Today     permits.Date        `json:"today"`
	DataFile  string              `json:"dataFile"`
	Business  permits.Business    `json:"business"`
	Tiers     permits.Tiers       `json:"tiers"`
	LateRates permits.LateRates   `json:"lateRates"`
	Permits   []permits.Permit    `json:"permits"`
	Sites     []string            `json:"sites"`
	Presets   []permits.Preset    `json:"presets"`
	Markets   []marketInfo        `json:"markets"`
	Gaps      map[string][]string `json:"gaps"`
	Defaults  defaultsInfo        `json:"defaults"`
}

type marketInfo struct {
	Code  permits.Jurisdiction `json:"code"`
	Label string               `json:"label"`
}

type defaultsInfo struct {
	Tiers            []int   `json:"tiers"`
	LeadDays         int     `json:"leadDays"`
	SurchargeCeiling float64 `json:"surchargeCeiling"`
	InterestCeiling  float64 `json:"interestCeiling"`
	InterestCapMo    int     `json:"interestCapMonths"`
	PHDeadlineDay    int     `json:"phDeadlineDay"`
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	out := stateResponse{
		Today:    permits.Today(),
		DataFile: s.store.Path(),
		Presets:  permits.AllPresets(),
		Permits:  []permits.Permit{},
		Sites:    []string{},
		Gaps:     map[string][]string{},
		Defaults: defaultsInfo{
			Tiers:            permits.DefaultTiers().Days(),
			LeadDays:         permits.DefaultLeadDays,
			SurchargeCeiling: permits.PHSurchargeRateCeiling,
			InterestCeiling:  permits.PHMonthlyInterestCeiling,
			InterestCapMo:    permits.PHInterestMonthsCap,
			PHDeadlineDay:    permits.PHRenewalDeadlineDay,
		},
	}
	for _, j := range permits.AllJurisdictions() {
		out.Markets = append(out.Markets, marketInfo{Code: j, Label: j.Label()})
	}

	s.store.Read(func(register *permits.Register) {
		out.Business = register.Business
		out.Tiers = register.Tiers
		out.LateRates = register.LateRates
		out.Permits = append(out.Permits, register.Permits...)
		out.Sites = append(out.Sites, register.Sites()...)
		if len(out.Sites) == 0 {
			out.Sites = []string{""}
		}
		for _, site := range out.Sites {
			for _, j := range []permits.Jurisdiction{permits.India, permits.Philippines} {
				key := string(j) + "\n" + site
				for _, preset := range register.Gaps(j, site) {
					out.Gaps[key] = append(out.Gaps[key], string(preset.Kind))
				}
			}
		}
	})
	writeJSON(w, http.StatusOK, out)
}

type boardResponse struct {
	AsOf   permits.Date       `json:"asOf"`
	Site   string             `json:"site"`
	Rows   []permits.BoardRow `json:"rows"`
	Counts permits.Counts     `json:"counts"`
}

func (s *Server) handleBoard(w http.ResponseWriter, r *http.Request) {
	asOf, err := asOfParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	site, allSites := siteParam(r)
	out := boardResponse{AsOf: asOf, Site: site, Rows: []permits.BoardRow{}}
	s.store.Read(func(register *permits.Register) {
		if allSites {
			out.Rows = append(out.Rows, register.Board(asOf)...)
			return
		}
		out.Rows = append(out.Rows, register.SiteBoard(site, asOf)...)
	})
	out.Counts = permits.Summarise(out.Rows)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCanTrade(w http.ResponseWriter, r *http.Request) {
	asOf, err := asOfParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	site, allSites := siteParam(r)
	var out permits.TradeVerdict
	s.store.Read(func(register *permits.Register) {
		if allSites {
			out = register.CanTradeToday(asOf)
			return
		}
		out = register.CanTradeTodayAt(site, asOf)
	})
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSeason(w http.ResponseWriter, r *http.Request) {
	season, err := s.season(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, season)
}

// season reads the window from the query. Three shapes are accepted: the
// Philippine January rush for a year, the Indian financial-year end for a
// year, or a window the owner gave with their own dates.
func (s *Server) season(r *http.Request) (permits.Season, error) {
	query := r.URL.Query()
	asOf, err := asOfParam(r)
	if err != nil {
		return permits.Season{}, err
	}
	site, _ := siteParam(r)

	year := asOf.Year
	if raw := strings.TrimSpace(query.Get("year")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1900 || parsed > 2200 {
			return permits.Season{}, fmt.Errorf("%q is not a year this tool will plan for", raw)
		}
		year = parsed
	}

	var out permits.Season
	var planErr error
	s.store.Read(func(register *permits.Register) {
		switch strings.ToLower(strings.TrimSpace(query.Get("window"))) {
		case "", "ph", "january":
			out = register.PHSeason(site, year, asOf)
		case "in", "financial-year":
			out = register.INSeason(site, year, asOf)
		case "custom":
			from, err := permits.ParseDate(query.Get("from"))
			if err != nil {
				planErr = err
				return
			}
			to, err := permits.ParseDate(query.Get("to"))
			if err != nil {
				planErr = err
				return
			}
			if from.Zero() || to.Zero() {
				planErr = errors.New("a window needs a from date and a to date")
				return
			}
			if to.Before(from) {
				planErr = fmt.Errorf("the window ends on %s, before it starts on %s", to, from)
				return
			}
			dueBy, err := permits.ParseDate(query.Get("dueBy"))
			if err != nil {
				planErr = err
				return
			}
			out = register.SeasonWindow(site, from, to, dueBy, asOf)
		default:
			planErr = fmt.Errorf("%q is not a window: use ph, in or custom", query.Get("window"))
		}
	})
	return out, planErr
}

type suggestResponse struct {
	Suggested permits.Date `json:"suggested"`
	Found     bool         `json:"found"`
	Note      string       `json:"note"`
}

func (s *Server) handleSuggestExpiry(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	kind, err := permits.ParseKind(query.Get("kind"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	issuedOn, err := permits.ParseDate(query.Get("issuedOn"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	site, _ := siteParam(r)

	var out suggestResponse
	s.store.Read(func(register *permits.Register) {
		out.Suggested, out.Found, out.Note = register.SuggestExpiry(kind, site, issuedOn)
	})
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleLateCharge(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	base, err := strconv.ParseFloat(strings.TrimSpace(query.Get("base")), 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("%q is not an amount", query.Get("base")))
		return
	}
	dueOn, err := permits.ParseDate(query.Get("dueOn"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	paidOn, err := permits.ParseDate(query.Get("paidOn"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var rates permits.LateRates
	s.store.Read(func(register *permits.Register) { rates = register.LateRates })

	charge, err := permits.LateChargeOn(base, dueOn, paidOn, rates)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, charge)
}

func (s *Server) handleExportRegister(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="permit-register.csv"`)
	// The status line goes out with the first byte, so a write that fails
	// part-way can only stop. Nothing is lost: an export does not touch the
	// data file and the owner simply downloads it again.
	s.store.Read(func(register *permits.Register) {
		_ = permits.WriteRegisterCSV(w, register)
	})
}

func (s *Server) handleExportSeason(w http.ResponseWriter, r *http.Request) {
	season, err := s.season(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="renewal-season.csv"`)
	_ = permits.WriteSeasonCSV(w, season)
}

// ---------- writes ----------

func (s *Server) handlePutPermit(w http.ResponseWriter, r *http.Request) {
	var in permits.Permit
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var saved permits.Permit
	if err := s.store.Write(func(register *permits.Register) error {
		var err error
		saved, err = register.Put(in)
		return err
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleDeletePermit(w http.ResponseWriter, r *http.Request) {
	kind, err := permits.ParseKind(r.URL.Query().Get("kind"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	site, _ := siteParam(r)
	if err := s.store.Write(func(register *permits.Register) error {
		if !register.Remove(kind, site) {
			return fmt.Errorf("no %s is on the register for that site", kind)
		}
		return nil
	}); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"removed": string(kind), "site": site})
}

type addPresetResponse struct {
	Added []permits.Permit `json:"added"`
}

func (s *Server) handleAddPreset(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	site, _ := siteParam(r)
	out := addPresetResponse{Added: []permits.Permit{}}

	// Either one permit by kind, or a whole market's starting list.
	if raw := strings.TrimSpace(query.Get("kind")); raw != "" {
		kind, err := permits.ParseKind(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.store.Write(func(register *permits.Register) error {
			added, err := register.AddPreset(kind, site)
			if err != nil {
				return err
			}
			out.Added = append(out.Added, added)
			return nil
		}); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}

	market, err := permits.ParseJurisdiction(query.Get("market"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.Write(func(register *permits.Register) error {
		added, err := register.AddJurisdictionPreset(market, site)
		if err != nil {
			return err
		}
		out.Added = append(out.Added, added...)
		return nil
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type checklistRequest struct {
	Kind  permits.Kind `json:"kind"`
	Site  string       `json:"site"`
	Index int          `json:"index"`
	Done  bool         `json:"done"`
}

func (s *Server) handlePutChecklist(w http.ResponseWriter, r *http.Request) {
	var in checklistRequest
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var saved permits.Permit
	if err := s.store.Write(func(register *permits.Register) error {
		if err := register.SetChecklistItem(in.Kind, in.Site, in.Index, in.Done); err != nil {
			return err
		}
		saved, _ = register.Find(in.Kind, in.Site)
		return nil
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

type settingsRequest struct {
	Business  *permits.Business  `json:"business"`
	Tiers     *permits.Tiers     `json:"tiers"`
	LateRates *permits.LateRates `json:"lateRates"`
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var in settingsRequest
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.Write(func(register *permits.Register) error {
		if in.Tiers != nil {
			if err := in.Tiers.Validate(); err != nil {
				return err
			}
			register.Tiers = in.Tiers.Normalise()
		}
		if in.Business != nil {
			if _, err := permits.ParseJurisdiction(string(in.Business.Jurisdiction)); err != nil {
				return err
			}
			register.Business = *in.Business
		}
		if in.LateRates != nil {
			// Price a token charge to make the rates prove themselves before
			// they are saved, so a rate typed as 25 rather than 0.25 is
			// refused at the form rather than silently producing nonsense
			// later.
			if _, err := permits.LateChargeOn(1,
				permits.NewDate(2000, time.January, 1),
				permits.NewDate(2000, time.February, 1), *in.LateRates); err != nil {
				return err
			}
			register.LateRates = *in.LateRates
		}
		register.Normalise()
		return nil
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.handleState(w, r)
}

type importResponse struct {
	Mode     string         `json:"mode"`
	Permits  int            `json:"permits"`
	Warnings []string       `json:"warnings"`
	Counts   permits.Counts `json:"counts"`
}

func (s *Server) handleImportRegister(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Errorf("that file is too large to import: %w", err))
		return
	}
	imported, warnings, err := permits.ReadRegisterCSV(strings.NewReader(string(body)))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	mode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("mode")))
	if mode == "" {
		mode = "merge"
	}
	out := importResponse{Mode: mode, Warnings: warnings}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}

	switch mode {
	case "replace":
		if err := s.store.ReplacePermits(imported); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	case "merge":
		if err := s.store.Write(func(register *permits.Register) error {
			for _, permit := range imported {
				if _, err := register.Put(permit); err != nil {
					out.Warnings = append(out.Warnings, err.Error())
				}
			}
			return nil
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("mode %q is not \"merge\" or \"replace\"", mode))
		return
	}

	asOf := permits.Today()
	s.store.Read(func(register *permits.Register) {
		out.Permits = len(register.Permits)
		out.Counts = permits.Summarise(register.Board(asOf))
	})
	writeJSON(w, http.StatusOK, out)
}

// ---------- plumbing ----------

// asOfParam reads the day being asked about. The interface normally leaves it
// off and gets today, but it can be set to look ahead — "will we still be legal
// on the 25th?" — or back.
func asOfParam(r *http.Request) (permits.Date, error) {
	raw := r.URL.Query().Get("asOf")
	if strings.TrimSpace(raw) == "" {
		return permits.Today(), nil
	}
	parsed, err := permits.ParseDate(raw)
	if err != nil {
		return permits.Date{}, err
	}
	if parsed.Zero() {
		return permits.Today(), nil
	}
	return parsed, nil
}

// siteParam reads the branch being asked about. An absent site parameter means
// the whole business; an explicitly empty one means the main premises, which is
// a different question, so the two are distinguished by presence.
func siteParam(r *http.Request) (site string, allSites bool) {
	values := r.URL.Query()
	if !values.Has("site") {
		return "", true
	}
	return strings.TrimSpace(values.Get("site")), false
}

func decodeBody(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("could not read the request: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(true)
	_ = encoder.Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
