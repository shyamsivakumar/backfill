package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// adRotator holds a small pool of served items (ads + trending content + an
// earnings entry) and rotates which one is shown every collapseRotateSeconds.
// It is shared by the collapsed-command, dbt, and sqlmesh renderers so every
// surface cycles the same way. The pool is seeded with one item synchronously
// and topped up in the background, so starting a command never blocks on the
// network.
type adRotator struct {
	cfg   *Config
	cmd   string
	start time.Time

	mu    sync.Mutex
	items []Ad
}

func newAdRotator(cfg *Config, cmd string) *adRotator {
	r := &adRotator{cfg: cfg, cmd: cmd, start: time.Now()}
	first := fetchAd(cfg, cmd)
	r.mu.Lock()
	r.items = []Ad{first}
	earned := first.EarnedMicros
	r.mu.Unlock()

	go func() {
		ads := fetchAdsConcurrent(cfg, cmd, 4)
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, ad := range ads {
			if ad.EarnedMicros > earned {
				earned = ad.EarnedMicros
			}
			dup := false
			for _, existing := range r.items {
				if existing.ID == ad.ID {
					dup = true
					break
				}
			}
			if !dup {
				r.items = append(r.items, ad)
			}
		}
		_ = earned
	}()
	return r
}

// estCPMMicros is the nominal developer CPM ($20 per 1000 impressions, in
// micros) used to project the value of the wait. Real advertiser revenue and
// payouts replace this once they are live.
const estCPMMicros = 20_000_000

// meterText renders the live, always-on earnings meter shown on a wrapped run.
// 1 impression = 5 visible seconds; the publisher keeps USER_SHARE of the eCPM.
// It is an estimate at developer rates until real ads pay out, hence the "~".
func meterText(elapsed time.Duration) string {
	impressions := elapsed.Seconds() / 5.0
	micros := impressions * (estCPMMicros / 1000.0) * 0.5
	return fmt.Sprintf("~$%.4f", micros/1e6)
}

// slotLabel returns the prefix and ANSI color for a rotation item so the user
// can tell at a glance whether a slot is a paid ad or genuinely useful free
// content. Trending repos, HN stories, and tips are clearly NOT ads.
func slotLabel(id string) (label, color string) {
	switch {
	case strings.HasPrefix(id, "gh_"):
		return "trending", "36" // cyan
	case strings.HasPrefix(id, "hn_"):
		return "hn", "36"
	case strings.HasPrefix(id, "tip_"):
		return "tip", "32" // green
	default:
		return "ad", "33" // amber
	}
}

// composeLine builds the full collapsed line: a dim status segment, a green
// live earnings meter, and the rotating slot labeled by type (ad / trending /
// hn / tip) with its text truncated to fit the terminal width. The slot text is
// an OSC 8 hyperlink. leftPlain is the visible status text without ANSI.
func composeLine(leftPlain string, elapsed time.Duration, item Ad, link string) string {
	cols := 80
	if c, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && c > 0 {
		cols = c
	}
	meter := meterText(elapsed)
	statusMeter := fmt.Sprintf("\x1b[2m%s\x1b[0m · \x1b[32m%s\x1b[0m", leftPlain, meter)

	if item.ID == "" {
		return "\r\x1b[2K" + statusMeter
	}

	label, color := slotLabel(item.ID)
	prefix := label + " · "
	// reserved width: leftPlain + " · " + meter + "  " + prefix
	used := visibleLen(leftPlain) + 3 + visibleLen(meter) + 2 + len(prefix)
	runes := []rune(item.Text)
	budget := cols - used
	if budget < 8 {
		// too narrow for the slot: status + meter only
		return "\r\x1b[2K" + statusMeter
	}
	if len(runes) > budget {
		runes = append(runes[:budget-1], '…')
	}
	slot := fmt.Sprintf("\x1b]8;;%s\x07\x1b[%sm%s%s\x1b[0m\x1b]8;;\x07", link, color, prefix, string(runes))
	return "\r\x1b[2K" + statusMeter + "  " + slot
}

// current returns the item that should hold the line right now.
func (r *adRotator) current() Ad {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.items) == 0 {
		return Ad{}
	}
	slot := int(time.Since(r.start).Seconds()) / collapseRotateSeconds
	return r.items[slot%len(r.items)]
}

// link routes a real ad through the /r/ click tracker; the earnings entry links
// straight to the site so it is never billed as a click.
func (r *adRotator) link(item Ad) string {
	if item.ID == "earnings" {
		return item.URL
	}
	return fmt.Sprintf("%s/r/%s?d=%s", r.cfg.APIBase, item.ID, r.cfg.DeviceID)
}

// billable returns the first real ad in the pool (not trending content, not a
// tip, not the earnings entry) for honest single-impression accounting.
func (r *adRotator) billable() Ad {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ad := range r.items {
		if isHouseContentID(ad.ID) {
			continue
		}
		return ad
	}
	return Ad{}
}

// isHouseContentID is true for non-billable rotation entries: empty, the
// earnings tally, and the gh_/hn_/tip_ content items.
func isHouseContentID(id string) bool {
	return id == "" || id == "earnings" ||
		strings.HasPrefix(id, "gh_") || strings.HasPrefix(id, "hn_") || strings.HasPrefix(id, "tip_")
}
