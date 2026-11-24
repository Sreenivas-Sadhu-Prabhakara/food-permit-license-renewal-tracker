package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sreenivas-sadhu-prabhakara/food-permit-license-renewal-tracker/internal/permits"
)

func newTestServer(t *testing.T) (*Server, *Store) {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "register.json"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!DOCTYPE html><title>t</title>")}}
	return NewServer(store, ui), store
}

func do(t *testing.T, srv *Server, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshalling the request: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, target, reader)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body.String(), err)
	}
	return out
}

func TestServerServesTheEmbeddedInterface(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := do(t, srv, http.MethodGet, "/", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<!DOCTYPE html>") {
		t.Fatalf("GET / served %q", rec.Body.String())
	}
	for _, header := range []string{"X-Content-Type-Options", "Referrer-Policy", "Cache-Control"} {
		if rec.Header().Get(header) == "" {
			t.Errorf("%s is not set", header)
		}
	}
}

func TestStateOnAnEmptyRegister(t *testing.T) {
	srv, store := newTestServer(t)
	rec := do(t, srv, http.MethodGet, "/api/state", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/state = %d: %s", rec.Code, rec.Body)
	}
	state := decode[stateResponse](t, rec)

	if state.DataFile != store.Path() {
		t.Errorf("dataFile = %q, want %q", state.DataFile, store.Path())
	}
	if state.Today.Zero() {
		t.Error("state carries no today")
	}
	if len(state.Permits) != 0 {
		t.Errorf("an empty register reported %d permits", len(state.Permits))
	}
	if len(state.Presets) != len(permits.AllPresets()) {
		t.Errorf("state carries %d presets, want %d", len(state.Presets), len(permits.AllPresets()))
	}
	if state.Tiers != permits.DefaultTiers() {
		t.Errorf("tiers = %+v, want the defaults", state.Tiers)
	}
	if state.Defaults.PHDeadlineDay != permits.PHRenewalDeadlineDay {
		t.Errorf("the Philippine deadline day is not stated")
	}
	if len(state.Markets) == 0 {
		t.Error("state names no markets")
	}
	// Even an empty register reports the gaps for both markets, so the
	// interface can offer the starting lists.
	if len(state.Gaps) == 0 {
		t.Error("state reports no gaps on an empty register")
	}

	// Running the tool and looking at it must leave nothing behind: the data
	// file is not created until something is saved.
	if _, err := os.Stat(store.Path()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("reading the state created %s", store.Path())
	}
}

func TestPutAndDeleteAPermit(t *testing.T) {
	srv, store := newTestServer(t)

	permit := permits.Permit{
		Kind:         permits.KindMayorsPermit,
		Jurisdiction: permits.Philippines,
		Label:        "Mayor's permit",
		Authority:    "Quezon City BPLO",
		Number:       "QC-2026-99",
		IssuedOn:     permits.MustParseDate("2026-01-18"),
		ExpiresOn:    permits.MustParseDate("2026-12-31"),
		Blocks:       true,
		LeadDays:     10,
	}
	rec := do(t, srv, http.MethodPut, "/api/permit", permit)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/permit = %d: %s", rec.Code, rec.Body)
	}
	saved := decode[permits.Permit](t, rec)
	if saved.Number != "QC-2026-99" {
		t.Fatalf("saved %+v", saved)
	}

	// It must be on disk, not only in memory.
	reopened, err := OpenStore(store.Path())
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	var count int
	reopened.Read(func(r *permits.Register) { count = len(r.Permits) })
	if count != 1 {
		t.Fatalf("the reopened file holds %d permits, want 1", count)
	}

	// A bad edit is refused and changes nothing.
	bad := permit
	bad.ExpiresOn = permits.MustParseDate("2020-01-01")
	rec = do(t, srv, http.MethodPut, "/api/permit", bad)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT with an expiry before the issue date = %d, want 400", rec.Code)
	}

	rec = do(t, srv, http.MethodDelete, "/api/permit?kind=mayors-permit&site=", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE = %d: %s", rec.Code, rec.Body)
	}
	rec = do(t, srv, http.MethodDelete, "/api/permit?kind=mayors-permit&site=", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("a second DELETE = %d, want 404", rec.Code)
	}
	rec = do(t, srv, http.MethodDelete, "/api/permit?kind=not%20a%20slug&site=", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("DELETE with a bad kind = %d, want 400", rec.Code)
	}
}

func TestPutPermitRefusesUnknownFields(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPut, "/api/permit",
		strings.NewReader(`{"kind":"mayors-permit","label":"x","surpriseField":1}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("a body with an unknown field = %d, want 400", rec.Code)
	}
}

func TestAddPresetByMarketAndByKind(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := do(t, srv, http.MethodPost, "/api/preset?market=PH&site=", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/preset = %d: %s", rec.Code, rec.Body)
	}
	added := decode[addPresetResponse](t, rec)
	if len(added.Added) != len(permits.PresetsFor(permits.Philippines)) {
		t.Fatalf("added %d permits, want the whole Philippine list", len(added.Added))
	}

	// A second pass adds nothing.
	rec = do(t, srv, http.MethodPost, "/api/preset?market=PH&site=", nil)
	if again := decode[addPresetResponse](t, rec); len(again.Added) != 0 {
		t.Fatalf("a second pass added %d permits", len(again.Added))
	}

	// One permit by kind, onto a named branch.
	rec = do(t, srv, http.MethodPost, "/api/preset?kind=fssai-licence&site=Bengaluru%20kitchen", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST a single preset = %d: %s", rec.Code, rec.Body)
	}
	one := decode[addPresetResponse](t, rec)
	if len(one.Added) != 1 || one.Added[0].Site != "Bengaluru kitchen" {
		t.Fatalf("added %+v", one.Added)
	}

	for _, target := range []string{
		"/api/preset?kind=invented-permit&site=",
		"/api/preset?market=US&site=",
		"/api/preset?kind=not+a+slug&site=",
	} {
		if rec := do(t, srv, http.MethodPost, target, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("POST %s = %d, want 400", target, rec.Code)
		}
	}
}

func TestBoardAndCanTradeEndToEnd(t *testing.T) {
	srv, _ := newTestServer(t)
	if rec := do(t, srv, http.MethodPost, "/api/preset?market=PH&site=", nil); rec.Code != http.StatusOK {
		t.Fatalf("seeding the preset: %s", rec.Body)
	}
	// Give every expiring permit a date of 31 December 2026.
	state := decode[stateResponse](t, do(t, srv, http.MethodGet, "/api/state", nil))
	for _, permit := range state.Permits {
		if permit.NoExpiry {
			continue
		}
		permit.ExpiresOn = permits.MustParseDate("2026-12-31")
		if rec := do(t, srv, http.MethodPut, "/api/permit", permit); rec.Code != http.StatusOK {
			t.Fatalf("PUT %s: %s", permit.Kind, rec.Body)
		}
	}

	board := decode[boardResponse](t, do(t, srv, http.MethodGet, "/api/board?asOf=2026-12-15", nil))
	if board.Counts.Total != len(state.Permits) {
		t.Fatalf("the board counts %d of %d permits", board.Counts.Total, len(state.Permits))
	}
	if board.Counts.Warn == 0 && board.Counts.Urgent == 0 {
		t.Fatalf("a fortnight out nothing is in a reminder tier: %+v", board.Counts)
	}
	if board.AsOf.String() != "2026-12-15" {
		t.Fatalf("asOf = %s", board.AsOf)
	}

	before := decode[permits.TradeVerdict](t, do(t, srv, http.MethodGet, "/api/can-trade?asOf=2026-12-31", nil))
	if !before.CanTrade {
		t.Fatalf("on the expiry day the business can still trade: %s", before.Headline)
	}
	after := decode[permits.TradeVerdict](t, do(t, srv, http.MethodGet, "/api/can-trade?asOf=2027-01-01", nil))
	if after.CanTrade {
		t.Fatalf("the day after expiry: %s", after.Headline)
	}
	if len(after.Blockers) == 0 {
		t.Fatal("no blockers were named")
	}
	for _, blocker := range after.Blockers {
		if blocker.Authority == "" || blocker.Says == "" {
			t.Errorf("blocker %+v does not say enough to act on", blocker)
		}
	}

	if rec := do(t, srv, http.MethodGet, "/api/board?asOf=not-a-date", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("GET /api/board with a bad date = %d, want 400", rec.Code)
	}
	if rec := do(t, srv, http.MethodGet, "/api/can-trade?asOf=2027-02-30", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("GET /api/can-trade with an impossible date = %d, want 400", rec.Code)
	}
}

func TestSeasonEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	if rec := do(t, srv, http.MethodPost, "/api/preset?market=PH&site=", nil); rec.Code != http.StatusOK {
		t.Fatalf("seeding: %s", rec.Body)
	}
	state := decode[stateResponse](t, do(t, srv, http.MethodGet, "/api/state", nil))
	for _, permit := range state.Permits {
		if permit.NoExpiry || permit.Kind == permits.KindFDALTO || permit.Kind == permits.KindBusinessName {
			continue
		}
		permit.ExpiresOn = permits.MustParseDate("2026-12-31")
		if rec := do(t, srv, http.MethodPut, "/api/permit", permit); rec.Code != http.StatusOK {
			t.Fatalf("PUT %s: %s", permit.Kind, rec.Body)
		}
	}

	rec := do(t, srv, http.MethodGet, "/api/season?window=ph&year=2027&asOf=2026-12-02", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/season = %d: %s", rec.Code, rec.Body)
	}
	season := decode[permits.Season](t, rec)
	if season.DueBy.String() != "2027-01-20" {
		t.Fatalf("deadline = %s, want 2027-01-20", season.DueBy)
	}
	if season.DueByBasis != permits.FromStatute {
		t.Fatalf("basis = %q", season.DueByBasis)
	}
	if season.Waves < 2 {
		t.Fatalf("the plan has %d steps; the Philippine January set has prerequisites", season.Waves)
	}
	last := season.Rows[len(season.Rows)-1]
	if last.Permit.Kind != permits.KindMayorsPermit {
		t.Fatalf("the last row of the plan is %s, want the mayor's permit", last.Permit.Kind)
	}

	// The Indian window is offered too, and labelled a convention.
	inSeason := decode[permits.Season](t,
		do(t, srv, http.MethodGet, "/api/season?window=in&year=2027&asOf=2027-01-02", nil))
	if inSeason.DueByBasis != permits.FromConvention {
		t.Fatalf("the India window basis = %q, want %q", inSeason.DueByBasis, permits.FromConvention)
	}

	// A window of the owner's own.
	custom := decode[permits.Season](t, do(t, srv, http.MethodGet,
		"/api/season?window=custom&from=2026-12-01&to=2026-12-31&dueBy=2026-12-20&asOf=2026-11-01", nil))
	if custom.DueBy.String() != "2026-12-20" || custom.DueByBasis != permits.FromOwner {
		t.Fatalf("the custom window came back as %s / %q", custom.DueBy, custom.DueByBasis)
	}

	for _, target := range []string{
		"/api/season?window=nonsense",
		"/api/season?window=custom&from=2026-12-01",
		"/api/season?window=custom&from=2026-12-31&to=2026-12-01",
		"/api/season?window=custom&from=bad&to=2026-12-01",
		"/api/season?year=90000",
		"/api/season?year=abc",
	} {
		if rec := do(t, srv, http.MethodGet, target, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400", target, rec.Code)
		}
	}
}

func TestSuggestExpiryEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	if rec := do(t, srv, http.MethodPost, "/api/preset?market=PH&site=", nil); rec.Code != http.StatusOK {
		t.Fatalf("seeding: %s", rec.Body)
	}

	rec := do(t, srv, http.MethodGet, "/api/suggest-expiry?kind=sanitary-permit&site=&issuedOn=2027-01-18", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("= %d: %s", rec.Code, rec.Body)
	}
	got := decode[suggestResponse](t, rec)
	if !got.Found || got.Suggested.String() != "2028-01-17" {
		t.Fatalf("suggested %s (found %v, note %q)", got.Suggested, got.Found, got.Note)
	}

	// The FDA licence ships no period, and the endpoint says why rather than
	// guessing.
	rec = do(t, srv, http.MethodGet, "/api/suggest-expiry?kind=fda-lto&site=&issuedOn=2027-01-18", nil)
	none := decode[suggestResponse](t, rec)
	if none.Found {
		t.Fatal("the FDA licence must not get a suggested date")
	}
	if !strings.Contains(none.Note, "ships no validity period") {
		t.Fatalf("note = %q", none.Note)
	}

	for _, target := range []string{
		"/api/suggest-expiry?kind=&issuedOn=2027-01-18",
		"/api/suggest-expiry?kind=sanitary-permit&issuedOn=2027-13-01",
	} {
		if rec := do(t, srv, http.MethodGet, target, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400", target, rec.Code)
		}
	}
}

func TestLateChargeEndpointUsesTheSavedRates(t *testing.T) {
	srv, _ := newTestServer(t)
	rates := permits.DefaultLateRates(permits.Philippines)
	if rec := do(t, srv, http.MethodPut, "/api/settings", settingsRequest{LateRates: &rates}); rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/settings = %d: %s", rec.Code, rec.Body)
	}

	rec := do(t, srv, http.MethodGet, "/api/late-charge?base=10000&dueOn=2027-01-20&paidOn=2027-01-21", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("= %d: %s", rec.Code, rec.Body)
	}
	charge := decode[permits.LateCharge](t, rec)
	if charge.Surcharge != 2500 || charge.Interest != 250 {
		t.Fatalf("charge = %+v", charge)
	}
	if !strings.Contains(charge.Basis, "7160") {
		t.Errorf("the basis does not cite the instrument: %q", charge.Basis)
	}

	for _, target := range []string{
		"/api/late-charge?base=abc&dueOn=2027-01-20&paidOn=2027-01-21",
		"/api/late-charge?base=10000&dueOn=bad&paidOn=2027-01-21",
		"/api/late-charge?base=10000&dueOn=2027-01-20&paidOn=bad",
		"/api/late-charge?base=-5&dueOn=2027-01-20&paidOn=2027-01-21",
		"/api/late-charge?base=10000&paidOn=2027-01-21",
	} {
		if rec := do(t, srv, http.MethodGet, target, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400", target, rec.Code)
		}
	}
}

func TestSettingsEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)

	// Tiers typed in the order they read must be repaired, not rejected.
	tiers := permits.Tiers{NoticeDays: 7, WarnDays: 45, UrgentDays: 90}
	business := permits.Business{Name: "Lola's Carinderia", Jurisdiction: permits.Philippines, Locality: "Quezon City"}
	rec := do(t, srv, http.MethodPut, "/api/settings", settingsRequest{Tiers: &tiers, Business: &business})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/settings = %d: %s", rec.Code, rec.Body)
	}
	state := decode[stateResponse](t, rec)
	if state.Tiers != (permits.Tiers{NoticeDays: 90, WarnDays: 45, UrgentDays: 7}) {
		t.Fatalf("tiers = %+v, want them sorted widest first", state.Tiers)
	}
	if state.Business.Name != "Lola's Carinderia" {
		t.Fatalf("business = %+v", state.Business)
	}

	bad := permits.Tiers{NoticeDays: -1}
	if rec := do(t, srv, http.MethodPut, "/api/settings", settingsRequest{Tiers: &bad}); rec.Code != http.StatusBadRequest {
		t.Errorf("a negative tier = %d, want 400", rec.Code)
	}
	// A rate typed as a percentage rather than a fraction is caught at the
	// form, not left to produce nonsense later.
	percentage := permits.LateRates{SurchargeRate: 25, InterestCapMo: 36}
	if rec := do(t, srv, http.MethodPut, "/api/settings", settingsRequest{LateRates: &percentage}); rec.Code != http.StatusBadRequest {
		t.Errorf("a rate of 25 = %d, want 400", rec.Code)
	}
	badMarket := permits.Business{Jurisdiction: "US"}
	if rec := do(t, srv, http.MethodPut, "/api/settings", settingsRequest{Business: &badMarket}); rec.Code != http.StatusBadRequest {
		t.Errorf("an unknown market = %d, want 400", rec.Code)
	}
}

func TestChecklistEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	if rec := do(t, srv, http.MethodPost, "/api/preset?kind=mayors-permit&site=", nil); rec.Code != http.StatusOK {
		t.Fatalf("seeding: %s", rec.Body)
	}

	rec := do(t, srv, http.MethodPut, "/api/checklist",
		checklistRequest{Kind: permits.KindMayorsPermit, Index: 0, Done: true})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/checklist = %d: %s", rec.Code, rec.Body)
	}
	permit := decode[permits.Permit](t, rec)
	if !permit.Checklist[0].Done {
		t.Fatal("the line did not tick")
	}

	for _, body := range []checklistRequest{
		{Kind: permits.KindMayorsPermit, Index: 999, Done: true},
		{Kind: permits.KindFDALTO, Index: 0, Done: true},
		{Kind: "", Index: 0, Done: true},
	} {
		if rec := do(t, srv, http.MethodPut, "/api/checklist", body); rec.Code != http.StatusBadRequest {
			t.Errorf("PUT /api/checklist %+v = %d, want 400", body, rec.Code)
		}
	}
}

func TestExportAndImportTheRegister(t *testing.T) {
	srv, _ := newTestServer(t)
	if rec := do(t, srv, http.MethodPost, "/api/preset?market=IN&site=", nil); rec.Code != http.StatusOK {
		t.Fatalf("seeding: %s", rec.Body)
	}

	rec := do(t, srv, http.MethodGet, "/api/register.csv", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/register.csv = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/csv") {
		t.Errorf("Content-Type = %q", got)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "permit-register.csv") {
		t.Errorf("Content-Disposition = %q", rec.Header().Get("Content-Disposition"))
	}
	exported := rec.Body.String()
	if !strings.Contains(exported, "fssai-licence") {
		t.Fatalf("the export does not hold the FSSAI row: %s", exported)
	}

	// Replace the register from the export.
	req := httptest.NewRequest(http.MethodPost, "/api/register.csv?mode=replace", strings.NewReader(exported))
	replaceRec := httptest.NewRecorder()
	srv.ServeHTTP(replaceRec, req)
	if replaceRec.Code != http.StatusOK {
		t.Fatalf("POST /api/register.csv = %d: %s", replaceRec.Code, replaceRec.Body)
	}
	report := decode[importResponse](t, replaceRec)
	if report.Permits != len(permits.PresetsFor(permits.India)) {
		t.Fatalf("after a replace the register holds %d permits", report.Permits)
	}
	if report.Mode != "replace" {
		t.Fatalf("mode = %q", report.Mode)
	}

	// The owner's settings survive a replace.
	state := decode[stateResponse](t, do(t, srv, http.MethodGet, "/api/state", nil))
	if state.Tiers != permits.DefaultTiers() {
		t.Fatalf("a replace changed the tiers: %+v", state.Tiers)
	}

	// Merge mode keeps what is there.
	req = httptest.NewRequest(http.MethodPost, "/api/register.csv?mode=merge", strings.NewReader(exported))
	mergeRec := httptest.NewRecorder()
	srv.ServeHTTP(mergeRec, req)
	if mergeRec.Code != http.StatusOK {
		t.Fatalf("merge = %d: %s", mergeRec.Code, mergeRec.Body)
	}
	if merged := decode[importResponse](t, mergeRec); merged.Permits != report.Permits {
		t.Fatalf("a merge of the same file changed the count to %d", merged.Permits)
	}

	for _, c := range []struct{ target, body string }{
		{"/api/register.csv?mode=sideways", exported},
		{"/api/register.csv", "not,a,register\n1,2,3\n"},
		{"/api/register.csv", ""},
	} {
		req := httptest.NewRequest(http.MethodPost, c.target, strings.NewReader(c.body))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("POST %s with %q = %d, want 400", c.target, truncate(c.body), rec.Code)
		}
	}
}

func TestExportTheSeasonSheet(t *testing.T) {
	srv, _ := newTestServer(t)
	if rec := do(t, srv, http.MethodPost, "/api/preset?market=PH&site=", nil); rec.Code != http.StatusOK {
		t.Fatalf("seeding: %s", rec.Body)
	}
	state := decode[stateResponse](t, do(t, srv, http.MethodGet, "/api/state", nil))
	for _, permit := range state.Permits {
		if permit.NoExpiry {
			continue
		}
		permit.ExpiresOn = permits.MustParseDate("2026-12-31")
		if rec := do(t, srv, http.MethodPut, "/api/permit", permit); rec.Code != http.StatusOK {
			t.Fatalf("PUT: %s", rec.Body)
		}
	}

	rec := do(t, srv, http.MethodGet, "/api/season.csv?window=ph&year=2027&asOf=2026-12-02", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/season.csv = %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "renewal-season.csv") {
		t.Errorf("Content-Disposition = %q", rec.Header().Get("Content-Disposition"))
	}
	sheet := rec.Body.String()
	for _, want := range []string{"2027-01-20", "7160", "Mayor's permit", "convention"} {
		if !strings.Contains(sheet, want) {
			t.Errorf("the season sheet does not mention %q", want)
		}
	}
	if rec := do(t, srv, http.MethodGet, "/api/season.csv?window=nonsense", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("GET /api/season.csv with a bad window = %d, want 400", rec.Code)
	}
}

func TestSitesAreKeptApartThroughTheAPI(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, site := range []string{"", "Cubao"} {
		target := "/api/preset?market=PH&site=" + strings.ReplaceAll(site, " ", "%20")
		if rec := do(t, srv, http.MethodPost, target, nil); rec.Code != http.StatusOK {
			t.Fatalf("seeding %q: %s", site, rec.Body)
		}
	}
	state := decode[stateResponse](t, do(t, srv, http.MethodGet, "/api/state", nil))
	if len(state.Sites) != 2 {
		t.Fatalf("sites = %v, want two", state.Sites)
	}

	// Let one Cubao permit lapse and leave the main premises in order.
	for _, permit := range state.Permits {
		if permit.NoExpiry {
			continue
		}
		permit.ExpiresOn = permits.MustParseDate("2027-12-31")
		if permit.Site == "Cubao" && permit.Kind == permits.KindFSIC {
			permit.ExpiresOn = permits.MustParseDate("2026-12-31")
		}
		if rec := do(t, srv, http.MethodPut, "/api/permit", permit); rec.Code != http.StatusOK {
			t.Fatalf("PUT: %s", rec.Body)
		}
	}

	main := decode[permits.TradeVerdict](t, do(t, srv, http.MethodGet, "/api/can-trade?asOf=2027-03-01&site=", nil))
	if !main.CanTrade {
		t.Fatalf("the main premises is in order: %s", main.Headline)
	}
	cubao := decode[permits.TradeVerdict](t, do(t, srv, http.MethodGet, "/api/can-trade?asOf=2027-03-01&site=Cubao", nil))
	if cubao.CanTrade {
		t.Fatal("Cubao's fire certificate has lapsed")
	}
	// With no site parameter at all the question is about the whole business.
	whole := decode[permits.TradeVerdict](t, do(t, srv, http.MethodGet, "/api/can-trade?asOf=2027-03-01", nil))
	if whole.CanTrade {
		t.Fatal("one branch with a lapsed permit must show on the whole-business verdict")
	}

	cubaoBoard := decode[boardResponse](t, do(t, srv, http.MethodGet, "/api/board?asOf=2027-03-01&site=Cubao", nil))
	for _, row := range cubaoBoard.Rows {
		if row.Permit.Site != "Cubao" {
			t.Fatalf("the Cubao board holds a %q row", row.Permit.Site)
		}
	}
}

func TestAsOfDefaultsToToday(t *testing.T) {
	srv, _ := newTestServer(t)
	board := decode[boardResponse](t, do(t, srv, http.MethodGet, "/api/board", nil))
	if board.AsOf != permits.Today() {
		t.Fatalf("asOf = %s with no parameter, want today", board.AsOf)
	}
	// An empty asOf is the same as none, because that is what an untouched
	// date input sends.
	blank := decode[boardResponse](t, do(t, srv, http.MethodGet, "/api/board?asOf=", nil))
	if blank.AsOf != permits.Today() {
		t.Fatalf("asOf = %s with an empty parameter, want today", blank.AsOf)
	}
}

func truncate(s string) string {
	if len(s) <= 24 {
		return s
	}
	return s[:24] + "..."
}
