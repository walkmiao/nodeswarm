// Package monitor performs bounded, periodic SOCKS5 reachability checks.
package monitor

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/walkmiao/nodeswarm/internal/manager"
)

const (
	Interval   = 3 * time.Minute
	StartDelay = 20 * time.Second
	MaxPerPass = 64
	Workers    = 8
)

// ScanOnce only probes enabled proxy nodes, oldest-check-first. One round is
// bounded even if the inventory contains thousands of entries; later rounds
// will pick remaining nodes using their older LastCheckedAt timestamp.
func ScanOnce(ctx context.Context, store *manager.Store, max, workers int) (checked, successful int) {
	if max <= 0 || max > 64 {
		max = MaxPerPass
	}
	if workers <= 0 || workers > 8 {
		workers = Workers
	}
	nodes, _, _ := store.Snapshot()
	eligible := make([]manager.Node, 0, len(nodes))
	now := time.Now()
	for _, node := range nodes {
		if !node.Enabled {
			continue
		}
		if node.ExpiresAt != "" {
			if exp, err := time.Parse(time.RFC3339, node.ExpiresAt); err == nil && !exp.After(now) {
				continue
			}
		}
		eligible = append(eligible, node)
	}
	sort.SliceStable(eligible, func(i, j int) bool { return eligible[i].LastCheckedAt < eligible[j].LastCheckedAt })
	if len(eligible) > max {
		eligible = eligible[:max]
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	var mu sync.Mutex
schedule:
	for _, n := range eligible {
		select {
		case <-ctx.Done():
			break schedule
		case sem <- struct{}{}:
		}
		if ctx.Err() != nil {
			<-sem
			break schedule
		}
		wg.Add(1)
		go func(n manager.Node) {
			defer wg.Done()
			defer func() { <-sem }()
			source, exists := store.NodeByID(n.ID)
			if !exists {
				return
			}
			result := manager.ProbeSOCKS5(source, 4*time.Second)
			if err := store.SaveProbe(n.ID, result); err != nil {
				return
			}
			mu.Lock()
			checked++
			if result.OK {
				successful++
			}
			mu.Unlock()
		}(n)
	}
	wg.Wait()
	return checked, successful
}

func Run(ctx context.Context, store *manager.Store) {
	timer := time.NewTimer(StartDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	ticker := time.NewTicker(Interval)
	defer ticker.Stop()
	for {
		checked, success := ScanOnce(ctx, store, MaxPerPass, Workers)
		if checked > 0 {
			log.Printf("[monitor] SOCKS5 scan: %d checked, %d passed", checked, success)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
