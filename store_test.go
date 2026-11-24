package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sreenivas-sadhu-prabhakara/food-permit-license-renewal-tracker/internal/permits"
)

func TestOpenStoreOnAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "register.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if !filepath.IsAbs(store.Path()) {
		t.Errorf("Path() = %q, want an absolute path the owner can find", store.Path())
	}
	var count int
	store.Read(func(r *permits.Register) { count = len(r.Permits) })
	if count != 0 {
		t.Errorf("a missing file loaded %d permits", count)
	}
	// Nothing is created until something is saved.
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("OpenStore created %s before anything was saved", path)
	}
}

func TestOpenStoreOnAnEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("writing an empty file: %v", err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("an empty file should load as an empty register: %v", err)
	}
	var tiers permits.Tiers
	store.Read(func(r *permits.Register) { tiers = r.Tiers })
	if tiers != permits.DefaultTiers() {
		t.Errorf("tiers = %+v, want the defaults", tiers)
	}
}

func TestOpenStoreRefusesRubbish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	if err := os.WriteFile(path, []byte("this is not JSON"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}
	_, err := OpenStore(path)
	if err == nil {
		t.Fatal("OpenStore accepted a file that is not JSON")
	}
	// The message has to name the file, because the owner has to know which
	// one to look at.
	if !strings.Contains(err.Error(), path) {
		t.Errorf("the error does not name the file: %v", err)
	}
}

func TestOpenStoreNormalisesWhatItLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	raw := `{
	  "tiers": {"noticeDays": 7, "warnDays": 30, "urgentDays": 60},
	  "permits": [
	    {"kind": "MAYORS-PERMIT", "label": "", "expiresOn": "2027-01-20", "leadDays": -5},
	    {"kind": "mayors-permit", "label": "a duplicate"}
	  ]
	}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	store.Read(func(r *permits.Register) {
		if r.Tiers != (permits.Tiers{NoticeDays: 60, WarnDays: 30, UrgentDays: 7}) {
			t.Errorf("tiers were not repaired: %+v", r.Tiers)
		}
		if len(r.Permits) != 1 {
			t.Fatalf("%d permits loaded, want the duplicate dropped", len(r.Permits))
		}
		if r.Permits[0].Label == "" {
			t.Error("an empty label was not filled in from the preset")
		}
		if r.Permits[0].LeadDays != 0 {
			t.Errorf("a negative lead time loaded as %d", r.Permits[0].LeadDays)
		}
	})
}

func TestWriteSavesAtomicallyAndRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "register.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}

	if err := store.Write(func(r *permits.Register) error {
		r.Business = permits.Business{Name: "Lola's Carinderia", Jurisdiction: permits.Philippines}
		_, err := r.AddJurisdictionPreset(permits.Philippines, "")
		return err
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the data file was not created: %v", err)
	}
	// The register holds somebody's licence numbers. Only its owner reads it.
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("the data file is mode %o, want 600", perm)
	}

	// No temporary file is left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the directory: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Errorf("a temporary file was left behind: %s", entry.Name())
		}
	}

	// It must be valid, re-readable JSON.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	var check map[string]any
	if err := json.Unmarshal(content, &check); err != nil {
		t.Fatalf("the saved file is not valid JSON: %v", err)
	}

	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	reopened.Read(func(r *permits.Register) {
		if r.Business.Name != "Lola's Carinderia" {
			t.Errorf("the business did not survive: %+v", r.Business)
		}
		if len(r.Permits) != len(permits.PresetsFor(permits.Philippines)) {
			t.Errorf("%d permits survived, want the whole Philippine list", len(r.Permits))
		}
		// Dates have to survive as dates, which is the thing most likely to
		// break across a save.
		for _, permit := range r.Permits {
			if permit.NoExpiry {
				continue
			}
			if !permit.ExpiresOn.Zero() && permit.ExpiresOn.Year < 1900 {
				t.Errorf("%s came back with the expiry %s", permit.Kind, permit.ExpiresOn)
			}
		}
	})
}

func TestWriteSavesNothingWhenTheEditFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	sentinel := errors.New("no")
	if err := store.Write(func(r *permits.Register) error {
		r.Business.Name = "should not be saved"
		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("Write returned %v, want the edit's own error", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("a failed edit wrote the data file")
	}
}

func TestReplacePermitsKeepsTheOwnersSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	custom := permits.Tiers{NoticeDays: 90, WarnDays: 45, UrgentDays: 14}
	rates := permits.DefaultLateRates(permits.Philippines)
	if err := store.Write(func(r *permits.Register) error {
		r.Tiers = custom
		r.LateRates = rates
		r.Business = permits.Business{Name: "Lola's", Jurisdiction: permits.Philippines, Locality: "Quezon City"}
		_, err := r.AddJurisdictionPreset(permits.Philippines, "")
		return err
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	replacement := []permits.Permit{{
		Kind: permits.KindFSSAI, Jurisdiction: permits.India,
		Label: "FSSAI licence", Authority: "FSSAI", Blocks: true,
		ExpiresOn: permits.MustParseDate("2028-03-31"),
	}}
	if err := store.ReplacePermits(replacement); err != nil {
		t.Fatalf("ReplacePermits: %v", err)
	}

	store.Read(func(r *permits.Register) {
		if len(r.Permits) != 1 || r.Permits[0].Kind != permits.KindFSSAI {
			t.Fatalf("after a replace the register holds %+v", r.Permits)
		}
		if r.Tiers != custom {
			t.Errorf("a replace changed the tiers to %+v", r.Tiers)
		}
		if r.LateRates != rates {
			t.Errorf("a replace changed the late rates to %+v", r.LateRates)
		}
		if r.Business.Name != "Lola's" {
			t.Errorf("a replace changed the business to %+v", r.Business)
		}
	})
}

func TestStoreIsSafeUnderConcurrentUse(t *testing.T) {
	// The interface fires several requests at once on load, so reads and
	// writes overlap. Run with -race to make this mean something.
	path := filepath.Join(t.TempDir(), "register.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			_ = store.Write(func(r *permits.Register) error {
				_, err := r.Put(permits.Permit{
					Kind:      permits.Kind("permit-" + string(rune('a'+n%26))),
					Label:     "concurrent",
					ExpiresOn: permits.MustParseDate("2027-12-31"),
				})
				return err
			})
		}(i)
		go func() {
			defer wg.Done()
			store.Read(func(r *permits.Register) {
				_ = r.Board(permits.Today())
				_ = r.CanTradeToday(permits.Today())
			})
		}()
	}
	wg.Wait()

	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatalf("the file did not survive concurrent writes: %v", err)
	}
	reopened.Read(func(r *permits.Register) {
		if len(r.Permits) == 0 {
			t.Error("nothing was saved")
		}
	})
}

func TestOpenStoreRefusesAPathThatCannotBeWritten(t *testing.T) {
	// A data file inside a directory that is actually a file cannot be saved,
	// and the owner should be told rather than losing the edit silently.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}
	store, err := OpenStore(filepath.Join(blocker, "register.json"))
	if err != nil {
		// Refusing at open time is also acceptable.
		return
	}
	if err := store.Write(func(r *permits.Register) error {
		r.Business.Name = "x"
		return nil
	}); err == nil {
		t.Fatal("Write reported success for a path it cannot write")
	}
}
