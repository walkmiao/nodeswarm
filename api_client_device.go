package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/walkmiao/nodeswarm/internal/control"
	"github.com/walkmiao/nodeswarm/internal/manager"
)

// These device-scoped endpoints let the native workstation app manage only its
// own bindings. The device's long-lived credential is never an admin token.
func (a *app) clientDeviceRoutes(mux *http.ServeMux) {
	authenticate := func(r *http.Request, id string) (control.AgentView, bool) {
		if id == "" || len(id) > 96 || bearer(r) == "" || !a.registry.Authorize(id, bearer(r)) {
			return control.AgentView{}, false
		}
		view, ok := a.registry.Find(id)
		return view, ok && view.Kind == "client"
	}
	mux.HandleFunc("GET /api/agent/client/context", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		view, ok := authenticate(r, id)
		if !ok {
			fail(w, 401, errors.New("Windows Client credential rejected"))
			return
		}
		nodes, _, _ := a.store.Snapshot() // passwords are excluded by the store
		send(w, 200, map[string]any{"client": view, "nodes": nodes})
	})
	mux.HandleFunc("PUT /api/agent/client/bindings", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID               string                  `json:"id"`
			ExpectedRevision *int64                  `json:"expectedRevision"`
			Bindings         []control.ClientBinding `json:"bindings"`
		}
		if err := decode(r, &request); err != nil {
			fail(w, 400, err)
			return
		}
		view, ok := authenticate(r, request.ID)
		if !ok {
			fail(w, 401, errors.New("Windows Client credential rejected"))
			return
		}
		if request.ExpectedRevision == nil {
			fail(w, 400, errors.New("expectedRevision required for device update"))
			return
		}
		nodes, locals, _ := a.store.Snapshot()
		capacities := map[string]int{}
		now := time.Now()
		for _, n := range nodes {
			if manager.NodeAvailable(n, 0, now) && n.LastStatus != "error" && n.LastStatus != "failed" {
				capacities[n.ID] = n.Capacity
			}
		}
		for _, local := range locals {
			capacities[local.NodeID]--
		}
		updated, err := a.registry.UpdateClientBindingsCAS(request.ID, request.Bindings, capacities, *request.ExpectedRevision)
		if err != nil {
			fail(w, 409, err)
			return
		}
		_ = a.store.RecordEvent("info", fmt.Sprintf("Windows Client %s 本机操作更新绑定：%d 项（修订 %d）", view.Name, len(updated.ClientBindings), updated.Revision))
		send(w, 200, map[string]any{"client": updated})
	})
}
