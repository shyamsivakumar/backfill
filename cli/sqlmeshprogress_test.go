package main

import (
	"strings"
	"testing"
	"time"
)

func TestSqlmeshRunFamilyIgnoresLeadingFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"plain plan", []string{"sqlmesh", "plan"}, true},
		{"path basename", []string{"/usr/local/bin/sqlmesh", "run"}, true},
		{"global value flags", []string{"sqlmesh", "--gateway", "prod", "--config", "config.yaml", "plan"}, true},
		{"short path flag", []string{"sqlmesh", "-p", "warehouse", "run"}, true},
		{"unsupported migrate", []string{"sqlmesh", "migrate"}, false},
		{"unsupported ui", []string{"sqlmesh", "ui"}, false},
		{"flag value is not subcommand", []string{"sqlmesh", "--gateway", "run", "migrate"}, false},
		{"wrong binary", []string{"dbt", "run"}, false},
	}
	for _, c := range cases {
		if got := isSqlmeshRunFamily(c.args); got != c.want {
			t.Errorf("%s: isSqlmeshRunFamily(%v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
}

func TestSqlmeshRendererFixtureKeepsMeaningfulLinesAndCountsProgress(t *testing.T) {
	r := &sqlmeshRenderer{cfg: testConfig(), rot: testRotator("sqlmesh"), start: time.Now()}
	lines := []string{
		"Models needing backfill:",
		"  * `analytics.fct_orders_daily`: [2026-07-01, 2026-07-02]",
		"\x1b[36m  * `staging.stg_orders`: [2026-07-01, 2026-07-02]\x1b[0m",
		"Executing model batches ━━━━━━━━━━━━━━━━━ 50% • 0:00:01",
		"0.42s",
		"/site-packages/sqlmesh/core/foo.py:123: UserWarning: pandas behavior will change",
		"  warnings.warn('pandas behavior will change', UserWarning)",
		"WARNING: model `analytics.dim_customers` has no owner",
		"Error: failed while rendering 75% progress for model analytics.fct_orders_daily",
		"\x1b[32m[1/2] analytics.fct_orders_daily   [insert]\x1b[0m",
		"[2/2] staging.stg_orders   [insert]",
		"Model batches executed",
		"Virtual layer updated",
	}

	out := captureStdout(t, func() {
		for _, line := range lines {
			r.handle(line)
		}
	})

	if r.total != 2 || r.done != 2 {
		t.Fatalf("sqlmesh progress = %d/%d, want 2/2", r.done, r.total)
	}
	if r.current != "staging.stg_orders" {
		t.Fatalf("current model = %q, want staging.stg_orders", r.current)
	}
	for _, want := range []string{
		"Models needing backfill:",
		"`analytics.fct_orders_daily`",
		"`staging.stg_orders`",
		"WARNING: model `analytics.dim_customers` has no owner",
		"Error: failed while rendering 75% progress",
		"Model batches executed",
		"Virtual layer updated",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("visible sqlmesh output missing %q in %q", want, out)
		}
	}
	for _, suppressed := range []string{
		"Executing model batches",
		"0.42s",
		"UserWarning: pandas behavior will change",
		"warnings.warn",
	} {
		if strings.Contains(out, suppressed) {
			t.Errorf("routine sqlmesh noise was not suppressed: %q", suppressed)
		}
	}
}

func TestSqlmeshNoiseFixture(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"", true},
		{"Executing model batches ━━━━━━━━━━━━━━━━━ 50% • 0:00:01", true},
		{"0.42s", true},
		{"/site-packages/sqlmesh/core/foo.py:123: UserWarning: pandas behavior will change", true},
		{"  warnings.warn('pandas behavior will change', UserWarning)", true},
		{"WARNING: model `analytics.dim_customers` has no owner", false},
		{"Error: failed while rendering 75% progress for model analytics.fct_orders_daily", false},
		{"Model batches executed", false},
	}
	for _, c := range cases {
		if got := isSqlmeshNoise(stripANSI(c.line)); got != c.want {
			t.Errorf("isSqlmeshNoise(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}
