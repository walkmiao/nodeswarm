package main

import (
	"bytes"
	"encoding/json"
	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
	"github.com/walkmiao/nodeswarm/internal/ops"
	"io"
	"net"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBatchProbeWithEncryptedCredentials(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_ = c.SetDeadline(time.Now().Add(3 * time.Second))
		greet := make([]byte, 3)
		if _, err := io.ReadFull(c, greet); err != nil {
			return
		}
		if !bytes.Equal(greet, []byte{5, 1, 2}) {
			done <- "invalid method"
			return
		}
		c.Write([]byte{5, 2})
		first := make([]byte, 2)
		if _, err := io.ReadFull(c, first); err != nil {
			return
		}
		username := make([]byte, int(first[1]))
		io.ReadFull(c, username)
		size := make([]byte, 1)
		io.ReadFull(c, size)
		password := make([]byte, int(size[0]))
		io.ReadFull(c, password)
		if string(username) != "alice" || string(password) != "secret-value" {
			done <- "credentials were redacted by Snapshot"
			return
		}
		c.Write([]byte{1, 0})
		request := make([]byte, 10)
		if _, err = io.ReadFull(c, request); err != nil {
			return
		}
		c.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 1})
		done <- "ok"
	}()
	dir := t.TempDir()
	store, err := manager.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	node, err := store.Create(manager.NodeInput{Name: "Auth Proxy", Host: "127.0.0.1", Port: addr.Port, Username: "alice", Password: "secret-value", Capacity: 2})
	if err != nil {
		t.Fatal(err)
	}
	reg, _ := control.NewRegistry(dir)
	alerts, _ := ops.Open(dir)
	a := (&app{store: store, registry: reg, alerts: alerts, adminToken: "admin-testing"}).routes()
	payload, _ := json.Marshal(map[string]any{"ids": []string{node.ID}})
	req := httptest.NewRequest("POST", "http://localhost/api/nodes/probe-batch", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-testing")
	resp := httptest.NewRecorder()
	a.ServeHTTP(resp, req)
	if resp.Code != 200 {
		t.Fatalf("bad status %d: %s", resp.Code, resp.Body.String())
	}
	var body struct {
		Results []struct {
			Probe manager.Probe `json:"probe"`
		} `json:"results"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Results) != 1 || !body.Results[0].Probe.OK {
		t.Fatalf("batch failed: %s", resp.Body.String())
	}
	select {
	case v := <-done:
		if v != "ok" {
			t.Fatal(v)
		}
	case <-time.After(time.Second):
		t.Fatal("mock proxy did not finish")
	}
}
