package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
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
		if earned > 0 {
			r.items = append(r.items, Ad{
				ID:   "earnings",
				Text: fmt.Sprintf("$%.2f earned · backfill", float64(earned)/1e6),
				URL:  cfg.APIBase,
			})
		}
	}()
	return r
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
