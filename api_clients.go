package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
)

func (a *app) listKind(kind string) []control.AgentView {
	all := a.registry.List()
	out := make([]control.AgentView, 0)
	for _, item := range all {
		if item.Kind == kind {
			out = append(out, item)
		}
	}
	return out
}

// The policy is returned only to the authenticated target client. Proxy
// passwords never appear in the administrative dashboard or client inventory.
func (a *app) clientPolicy(id string) *control.ClientPolicy {
	revision, limit, bindings, ok := a.registry.ClientSettings(id)
	if !ok {
		return nil
	}
	p := &control.ClientPolicy{Revision: revision, MaxProxyPIDs: limit, Bindings: bindings, Endpoints: []control.ClientEndpoint{}}
	known := map[string]bool{}
	for _, binding := range bindings {
		if known[binding.NodeID] {
			continue
		}
		known[binding.NodeID] = true
		node, found := a.store.NodeByID(binding.NodeID)
		if !found || !manager.NodeAvailable(node, 0, time.Now()) {
			continue
		}
		p.Endpoints = append(p.Endpoints, control.ClientEndpoint{
			NodeID: node.ID, Host: node.Host, Port: node.Port, Username: node.Username, Password: node.Password,
		})
	}
	return p
}

func (a *app) clientRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/clients", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, map[string]any{"clients": a.listKind("client")})
	})
	mux.HandleFunc("GET /api/clients/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, ok := a.registry.Find(r.PathValue("id"))
		if !ok || v.Kind != "client" {
			fail(w, 404, errors.New("Windows Client 不存在"))
			return
		}
		send(w, 200, v)
	})
	mux.HandleFunc("PUT /api/clients/{id}/limits", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MaxProxyPIDs     int    `json:"maxProxyPids"`
			ExpectedRevision *int64 `json:"expectedRevision"`
		}
		if err := decode(r, &req); err != nil {
			fail(w, 400, err)
			return
		}
		expected := int64(-1)
		if req.ExpectedRevision != nil {
			expected = *req.ExpectedRevision
		}
		v, err := a.registry.SetClientMaxProxyPIDs(r.PathValue("id"), req.MaxProxyPIDs, expected)
		if err != nil {
			fail(w, 409, err)
			return
		}
		_ = a.store.RecordEvent("info", fmt.Sprintf("Windows 客户端 %s PID 配额调整为 %d，修订 %d", v.Name, v.MaxProxyPIDs, v.Revision))
		send(w, 200, v)
	})
	mux.HandleFunc("PUT /api/clients/{id}/bindings", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Bindings         []control.ClientBinding `json:"bindings"`
			ExpectedRevision *int64                  `json:"expectedRevision"`
		}
		if err := decode(r, &req); err != nil {
			fail(w, 400, err)
			return
		}
		nodes, locals, _ := a.store.Snapshot()
		caps := map[string]int{}
		for _, n := range nodes {
			if !manager.NodeAvailable(n, 0, time.Now()) {
				continue
			}
			caps[n.ID] = n.Capacity
		}
		for _, local := range locals {
			caps[local.NodeID]--
		}
		expected := int64(-1)
		if req.ExpectedRevision != nil {
			expected = *req.ExpectedRevision
		}
		v, err := a.registry.UpdateClientBindingsCAS(r.PathValue("id"), req.Bindings, caps, expected)
		if err != nil {
			fail(w, 409, err)
			return
		}
		_ = a.store.RecordEvent("info", fmt.Sprintf("更新 Windows Client %s 绑定策略：%d 项，修订 %d", v.Name, len(v.ClientBindings), v.Revision))
		send(w, 200, v)
	})
	mux.HandleFunc("DELETE /api/clients/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, ok := a.registry.Find(r.PathValue("id"))
		if !ok || v.Kind != "client" {
			fail(w, 404, errors.New("Windows Client 不存在"))
			return
		}
		if err := a.registry.Delete(v.ID); err != nil {
			fail(w, 500, err)
			return
		}
		_ = a.store.RecordEvent("warning", "注销 Windows Client："+v.Name)
		send(w, 200, map[string]any{"ok": true})
	})
}
