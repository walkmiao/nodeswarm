//go:build windows && amd64

package pidtcp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

func runWindows(ctx context.Context, routes []Route, driverDir string, ready chan<- error) (result error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// No routing is enabled until all listeners and the driver are available.
	// The caller has explicitly selected the experimental TCP mode; neither
	// this package nor WinDivert is a fail-closed operating-system firewall.
	r4, e := net.Listen("tcp4", "0.0.0.0:0")
	if e != nil {
		ready <- e
		return e
	}
	defer r4.Close()
	port := r4.Addr().(*net.TCPAddr).Port
	r6, e := net.Listen("tcp6", fmt.Sprintf("[::]:%d", port))
	if e != nil {
		ready <- fmt.Errorf("IPv6 TCP relay unavailable: %w", e)
		return e
	}
	defer r6.Close()
	driver, e := loadDriver(driverDir)
	if e != nil {
		ready <- e
		return e
	}
	defer driver.dispose()
	table := newFlowTable(uint16(port), 4096)
	routeByPID := make(map[int]Route, len(routes))
	for _, p := range routes {
		routeByPID[p.PID] = p
	}
	owner := func(key flowKey) (*Route, error) {
		pid, err := ownerWindows(key)
		if err != nil || pid == 0 {
			return nil, err
		}
		route, ok := routeByPID[int(pid)]
		if !ok {
			return nil, nil
		}
		if !lifetimeMatches(int(pid), route.StartTime, route.Name) {
			return nil, errors.New("PID identity changed or cannot be verified")
		}
		return &route, nil
	}
	router := newRouter(driver, table, owner)
	workerDone := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range router.jobs {
				router.resolve(job)
			}
		}()
	}
	go func() { workers.Wait(); close(workerDone) }()
	errorsCh := make(chan error, 1)
	fail := func(err error) {
		select {
		case errorsCh <- err:
		default:
		}
	}
	router.onFailure = func(err error) { fail(fmt.Errorf("WinDivert packet injection failed: %w", err)) }
	var handlerWG, acceptWG sync.WaitGroup
	for _, listener := range []net.Listener{r4, r6} {
		acceptWG.Add(1)
		go func(ln net.Listener) {
			defer acceptWG.Done()
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				conn, ok := c.(*net.TCPConn)
				if !ok {
					c.Close()
					continue
				}
				m := table.claim(conn)
				if m == nil {
					conn.Close()
					continue
				}
				handlerWG.Add(1)
				go func() {
					defer handlerWG.Done()
					defer conn.Close()
					defer table.release(m)
					if err := relayTCP(ctx, conn, m.route.Endpoint, m.original.remote); err != nil && ctx.Err() == nil {
						log.Printf("[pidtcp] PID %d TCP relay failed: %v", m.route.PID, err)
					}
				}()
			}
		}(listener)
	}
	captureDone := make(chan struct{})
	go func() {
		defer close(captureDone)
		buf := make([]byte, 65575)
		for {
			n, addr, err := driver.receive(buf)
			if err != nil {
				if ctx.Err() == nil {
					fail(fmt.Errorf("WinDivert capture: %w", err))
				}
				return
			}
			router.route(buf[:n], addr)
		}
	}()
	maintenanceDone := make(chan struct{})
	go func() {
		defer close(maintenanceDone)
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		last := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-tick.C:
				router.expire()
				if now.Sub(last) > time.Second {
					table.expire(now)
					last = now
				}
			}
		}
	}()
	ready <- nil
	log.Printf("[pidtcp] experimental Windows TCP redirector ready: %d PIDs, relay port %d; UDP/DNS are NOT proxied", len(routes), port)
	select {
	case <-ctx.Done():
	case result = <-errorsCh:
	}
	// Stop is coordinated. Existing diverted sockets are closed before
	// capture teardown, but new game traffic can go direct after teardown.
	cancel() // also shut down maintenance when the driver, not the user, fails.
	router.stop()
	table.stop()
	r4.Close()
	r6.Close()
	driver.stop()
	<-captureDone
	<-workerDone
	acceptWG.Wait()
	handlerWG.Wait()
	<-maintenanceDone
	return result
}
