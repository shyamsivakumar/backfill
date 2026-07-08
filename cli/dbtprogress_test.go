package main

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func testConfig() *Config {
	return &Config{APIBase: "https://example.test", DeviceID: "device"}
}

func testRotator(cmd string) *adRotator {
	return &adRotator{
		cfg:   testConfig(),
		cmd:   cmd,
		start: time.Now(),
		items: []Ad{{ID: "tip_fixture", Text: "fixture tip"}},
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = old
		r.Close()
	}()

	fn()
	w.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(b)
}

func TestDbtRunFamilyIgnoresLeadingFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"plain run", []string{"dbt", "run"}, true},
		{"path basename", []string{"/opt/homebrew/bin/dbt", "build"}, true},
		{"global value flags", []string{"dbt", "--profiles-dir", "profiles", "--target", "prod", "run"}, true},
		{"global equals flag", []string{"dbt", "--project-dir=warehouse", "test"}, true},
		{"unsupported docs", []string{"dbt", "docs", "generate"}, false},
		{"unsupported source", []string{"dbt", "source", "freshness"}, false},
		{"flag value is not subcommand", []string{"dbt", "--profiles-dir", "run", "docs", "generate"}, false},
		{"wrong binary", []string{"python", "-m", "dbt", "run"}, false},
	}
	for _, c := range cases {
		if got := isDbtRunFamily(c.args); got != c.want {
			t.Errorf("%s: isDbtRunFamily(%v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
}

func TestDbtRendererFixtureKeepsMeaningfulLinesAndCountsProgress(t *testing.T) {
	r := &dbtRenderer{cfg: testConfig(), rot: testRotator("dbt"), start: time.Now()}
	lines := []string{
		"\x1b[0m12:00:01  Running with dbt=1.8.7\x1b[0m",
		"12:00:02  Found 2 models, 1 seed, 1 test",
		"12:00:03  Concurrency: 4 threads (target='prod')",
		"12:00:04  9 of 10 WAIT sql model analytics.not_progress ........ [WAIT]",
		"\x1b[32m12:00:05  1 of 4 START sql model analytics.fct_orders_daily ........ [RUN]\x1b[0m",
		"12:00:06  1 of 4 OK created sql model analytics.fct_orders_daily ........ [SELECT 1 in 0.11s]",
		"12:00:07  2 of 4 PASS relationships fact_orders_customer_id__id ........ [PASS in 0.03s]",
		"\x1b[33m12:00:08  3 of 4 WARN accepted_values_stg_orders_status ........ [WARN 1 in 0.02s]\x1b[0m",
		"\x1b[31m12:00:09  4 of 4 FAIL unique_orders_order_id ........ [FAIL 1 in 0.04s]\x1b[0m",
		"12:00:10  Done. PASS=2 WARN=1 ERROR=1 SKIP=0 TOTAL=4",
	}

	out := captureStdout(t, func() {
		for _, line := range lines {
			r.handle(line)
		}
	})

	if r.total != 4 || r.done != 4 {
		t.Fatalf("dbt progress = %d/%d, want 4/4", r.done, r.total)
	}
	if r.current != "analytics.fct_orders_daily" {
		t.Fatalf("current model = %q, want analytics.fct_orders_daily", r.current)
	}
	for _, want := range []string{
		"Running with dbt=1.8.7",
		"Found 2 models",
		"WARN accepted_values_stg_orders_status",
		"FAIL unique_orders_order_id",
		"Done. PASS=2 WARN=1 ERROR=1 SKIP=0 TOTAL=4",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("visible dbt output missing %q in %q", want, out)
		}
	}
	for _, suppressed := range []string{"Concurrency: 4 threads", "WAIT sql model analytics.not_progress"} {
		if strings.Contains(out, suppressed) {
			t.Errorf("routine dbt noise was not suppressed: %q", suppressed)
		}
	}
}

func TestDbtNoiseFixture(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"", true},
		{"12:00:03  Concurrency: 4 threads (target='prod')", true},
		{"12:00:04  9 of 10 WAIT sql model analytics.not_progress ........ [WAIT]", true},
		{"\x1b[33mWarning: adapter emitted a warning\x1b[0m", false},
		{"Database Error in model analytics.fct_orders", false},
		{"12:00:10  Finished running 2 models, 1 seed, 1 test", false},
	}
	for _, c := range cases {
		if got := isDbtNoise(stripANSI(c.line)); got != c.want {
			t.Errorf("isDbtNoise(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}
