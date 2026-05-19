// challenges/runner/main.go
//
// Round-244 deliverable — Database submodule deep-doc + Challenge enrichment.
//
// Runtime probe driven by challenges/database_describe_challenge.sh. Exercises
// the SQLite adapter end-to-end against a real :memory: database file:
//
//   Leg 1 (always): Connect → migration Apply → INSERT → SELECT → assert
//                   payload round-tripped correctly. Emits a "describe"
//                   block in the locale loaded from
//                   challenges/fixtures/<LANG_FIXTURE>.yaml (default: en).
//
//   Leg 2 (PROBE_MODE=MUTATION): re-runs the SAME flow against a
//                   "swallowingDatabase" adapter whose Exec returns success
//                   but writes nothing. The subsequent SELECT MUST return
//                   sql.ErrNoRows; the probe exits 99 in that case (proves
//                   the assertion would catch a real regression in the
//                   adapter contract — CONST-035 anti-bluff posture).
//
// Anti-bluff invariants (Article XI §11.9):
//   - the SQLite leg uses the real modernc.org/sqlite driver via the
//     production pkg/sqlite adapter — no in-memory map, no mock, no fake.
//   - the mutation leg deliberately breaks the adapter contract and
//     asserts the probe FAILs, proving the assertions are real.
//
// Exit codes:
//   0  — normal probe passed.
//   2  — leg 1 setup failure (sqlite Connect / migration Init / migration Apply).
//   3  — leg 1 INSERT returned non-nil error in normal mode.
//   4  — leg 1 SELECT returned non-nil error in normal mode.
//   5  — leg 1 SELECT returned wrong name (payload scrambled) in normal mode.
//   6  — leg 1 COUNT returned wrong row count in normal mode.
//   7  — fixture file missing / malformed.
//   99 — MUTATION leg behaved like normal (probe assertions would NOT catch
//        a real regression — CONST-035 bluff).
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	db "digital.vasic.database/pkg/database"
	"digital.vasic.database/pkg/migration"
	"digital.vasic.database/pkg/sqlite"
)

// Fixture is the minimal YAML shape consumed by the probe.
// We avoid a YAML dependency by parsing the small key:"value" pairs manually
// — the fixtures are entirely under our control so the parser need only
// handle the two-level structure the round-244 fixtures use.
type Fixture struct {
	Locale   string
	Describe map[string]string
	Errors   map[string]string
}

func loadFixture(path string) (*Fixture, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture %s: %w", path, err)
	}
	f := &Fixture{
		Describe: map[string]string{},
		Errors:   map[string]string{},
	}
	section := ""
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(line, "locale:") {
			f.Locale = strings.Trim(strings.TrimPrefix(line, "locale:"), " \"")
			section = ""
			continue
		}
		if strings.HasPrefix(line, "describe:") {
			section = "describe"
			continue
		}
		if strings.HasPrefix(line, "errors:") {
			section = "errors"
			continue
		}
		// nested "  key: \"value\"" lines under the active section
		if (section == "describe" || section == "errors") && strings.HasPrefix(line, "  ") {
			kv := strings.SplitN(trimmed, ":", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.TrimSpace(kv[0])
			val := strings.Trim(strings.TrimSpace(kv[1]), "\"")
			if section == "describe" {
				f.Describe[key] = val
			} else {
				f.Errors[key] = val
			}
		}
	}
	if f.Locale == "" {
		return nil, fmt.Errorf("fixture %s missing 'locale' key", path)
	}
	if _, ok := f.Describe["banner"]; !ok {
		return nil, fmt.Errorf("fixture %s missing describe.banner", path)
	}
	return f, nil
}

// swallowingDatabase wraps a real db.Database but its Exec silently returns
// success without performing the write — the MUTATION leg uses this to
// simulate the "PASS-bluff at the adapter-contract layer" failure mode.
type swallowingDatabase struct {
	inner db.Database
}

type swallowedResult struct{}

func (swallowedResult) RowsAffected() (int64, error) { return 1, nil }

func (s *swallowingDatabase) Connect(ctx context.Context) error { return s.inner.Connect(ctx) }
func (s *swallowingDatabase) Close() error                      { return s.inner.Close() }
func (s *swallowingDatabase) Exec(ctx context.Context, query string, args ...any) (db.Result, error) {
	// Silent swallow: lie about success without writing anything.
	// CREATE TABLE / migration DDL is allowed through so the table exists
	// when the probe SELECTs; only INSERT / UPDATE / DELETE are swallowed.
	upper := strings.ToUpper(strings.TrimSpace(query))
	if strings.HasPrefix(upper, "INSERT") || strings.HasPrefix(upper, "UPDATE") || strings.HasPrefix(upper, "DELETE") {
		return swallowedResult{}, nil
	}
	return s.inner.Exec(ctx, query, args...)
}
func (s *swallowingDatabase) Query(ctx context.Context, query string, args ...any) (db.Rows, error) {
	return s.inner.Query(ctx, query, args...)
}
func (s *swallowingDatabase) QueryRow(ctx context.Context, query string, args ...any) db.Row {
	return s.inner.QueryRow(ctx, query, args...)
}
func (s *swallowingDatabase) Begin(ctx context.Context) (db.Tx, error)      { return s.inner.Begin(ctx) }
func (s *swallowingDatabase) HealthCheck(ctx context.Context) error         { return s.inner.HealthCheck(ctx) }

func main() {
	ctx := context.Background()
	mode := os.Getenv("PROBE_MODE")

	// Locate fixture (default en).
	lang := os.Getenv("LANG_FIXTURE")
	if lang == "" {
		lang = "en"
	}
	fixturePath := os.Getenv("FIXTURE_PATH")
	if fixturePath == "" {
		fixturePath = fmt.Sprintf("challenges/fixtures/%s.yaml", lang)
	}
	fx, err := loadFixture(fixturePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fixture load failed: %v\n", err)
		os.Exit(7)
	}

	fmt.Println(fx.Describe["banner"])
	fmt.Printf("locale: %s\n", fx.Locale)

	// Build the real SQLite adapter against an in-memory database.
	sqliteCfg := sqlite.DefaultConfig(":memory:")
	var d db.Database = sqlite.New(sqliteCfg)

	// In MUTATION mode, wrap the real adapter with the swallowing one so
	// Exec lies about writes. The migration / SELECT paths still touch the
	// real driver — proving the probe's assertion is on user-visible state.
	if mode == "MUTATION" {
		d = &swallowingDatabase{inner: d}
	}

	if err := d.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Connect failed: %v\n", err)
		os.Exit(2)
	}
	defer d.Close()
	fmt.Println(fx.Describe["connect_ok"])

	// Init + apply a single migration creating a users table.
	runner := migration.NewRunner(d, "")
	if err := runner.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "migration Init failed: %v\n", err)
		os.Exit(2)
	}
	migrations := []migration.Migration{
		{
			Version: 1,
			Name:    "create users",
			Up:      "CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL)",
			Down:    "DROP TABLE users",
		},
	}
	if err := runner.Apply(ctx, migrations); err != nil {
		fmt.Fprintf(os.Stderr, "migration Apply failed: %v\n", err)
		os.Exit(2)
	}
	fmt.Printf(fx.Describe["migrate_ok"]+"\n", len(migrations))

	// INSERT a known row.
	const wantName = "Ada Lovelace"
	if _, err := d.Exec(ctx, "INSERT INTO users (name) VALUES (?)", wantName); err != nil {
		fmt.Fprintf(os.Stderr, "INSERT failed: %v\n", err)
		os.Exit(3)
	}
	fmt.Printf(fx.Describe["insert_ok"]+"\n", wantName, 1)

	// SELECT it back and assert the round-trip.
	var gotName string
	err = d.QueryRow(ctx, "SELECT name FROM users WHERE id = ?", 1).Scan(&gotName)
	if err != nil {
		// In MUTATION mode the INSERT was swallowed; SELECT now sees no row.
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Fprintf(os.Stderr, "%s\n", fx.Errors["no_rows"])
			os.Exit(99) // anti-bluff mutation correctly detected
		}
		fmt.Fprintf(os.Stderr, "SELECT failed: %v\n", err)
		os.Exit(4)
	}
	if gotName != wantName {
		fmt.Fprintf(os.Stderr, "%s (want=%q got=%q)\n", fx.Errors["wrong_name"], wantName, gotName)
		os.Exit(5)
	}
	fmt.Printf(fx.Describe["query_ok"]+"\n", 1, gotName)

	// COUNT verifies the persistence cardinality (also catches partial-write
	// adapters that succeed on INSERT but lose rows on QueryRow).
	var count int
	err = d.QueryRow(ctx, "SELECT COUNT(*) FROM users", nil).Scan(&count)
	if err != nil {
		// QueryRow without args might fail on some adapters; try without nil.
		err = d.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "COUNT failed: %v\n", err)
		os.Exit(4)
	}
	if count != 1 {
		fmt.Fprintf(os.Stderr, "%s (want=1 got=%d)\n", fx.Errors["wrong_count"], count)
		os.Exit(6)
	}
	fmt.Printf(fx.Describe["count_ok"]+"\n", count)

	fmt.Println(fx.Describe["pass"])
}
