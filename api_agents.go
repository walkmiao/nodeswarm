package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"nodeconsole.local/node-console/internal/control"
	"nodeconsole.local/node-console/internal/manager"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func remoteIP(r *http.Request) string {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		return ""
	}
	return host
}
func bearer(r *http.Request) string {
	v := r.Header.Get("Authorization")
	if !strings.HasPrefix(v, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(v, "Bearer ")
}
func (a *app) syncManaged(v control.AgentView) error {
	c, ok := a.registry.Credentials(v.ID)
	if !ok {
		return errors.New("agent not found")
	}
	if c.PublicHost == "" {
		return a.store.DeleteManaged(v.ID)
	}
	returnManaged := func() error {
		_, err := a.store.UpsertManaged(v.ID, manager.NodeInput{Name: v.Name, Host: c.PublicHost, Port: c.Port, Username: c.Username, Password: c.Password, Provider: c.Provider, Region: c.Region, Capacity: c.Capacity, Enabled: &c.Enabled})
		return err
	}
	return returnManaged()
}
func (a *app) agentsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/agent/enroll", func(w http.ResponseWriter, r *http.Request) {
		if a.registry == nil || a.enrollKey == "" || subtle.ConstantTimeCompare([]byte(bearer(r)), []byte(a.enrollKey)) != 1 {
			fail(w, 401, errors.New("enrollment key rejected"))
			return
		}
		var in control.EnrollInput
		if err := decode(r, &in); err != nil {
			fail(w, 400, err)
			return
		}
		id, token, err := a.registry.Enroll(in, remoteIP(r))
		if err != nil {
			fail(w, 400, err)
			return
		}
		send(w, 201, map[string]any{"id": id, "token": token})
	})
	mux.HandleFunc("POST /api/agent/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		var hb control.Heartbeat
		if err := decode(r, &hb); err != nil {
			fail(w, 400, err)
			return
		}
		if a.registry == nil || !a.registry.Authorize(hb.ID, bearer(r)) {
			fail(w, 401, errors.New("agent credentials rejected"))
			return
		}
		result, err := a.registry.Report(hb, remoteIP(r))
		if err != nil {
			fail(w, 404, err)
			return
		}
		send(w, 200, result)
	})
	mux.HandleFunc("GET /api/dashboard", func(w http.ResponseWriter, r *http.Request) {
		nodes, assign, events := a.store.Snapshot()
		agents := a.registry.List()
		send(w, 200, map[string]any{"nodes": nodes, "assignments": assign, "events": events, "agents": agents, "proxyEngine": "client_transparent_proxy_not_integrated", "serverTime": time.Now().UTC().Format(time.RFC3339)})
	})
	mux.HandleFunc("GET /api/agents", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, map[string]any{"agents": a.registry.List()})
	})
	mux.HandleFunc("GET /api/agents/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, ok := a.registry.Find(r.PathValue("id"))
		if !ok {
			fail(w, 404, errors.New("agent not found"))
			return
		}
		send(w, 200, v)
	})
	mux.HandleFunc("PUT /api/agents/{id}/config", func(w http.ResponseWriter, r *http.Request) {
		var in control.ConfigureInput
		if err := decode(r, &in); err != nil {
			fail(w, 400, err)
			return
		}
		v, err := a.registry.Configure(r.PathValue("id"), in)
		if err != nil {
			fail(w, 400, err)
			return
		}
		if err := a.syncManaged(v); err != nil {
			fail(w, 500, fmt.Errorf("agent saved but proxy inventory sync failed: %w", err))
			return
		}
		send(w, 200, v)
	})
	mux.HandleFunc("POST /api/agents/{id}/action", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Action string `json:"action"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, 400, err)
			return
		}
		v, err := a.registry.Action(r.PathValue("id"), in.Action)
		if err != nil {
			fail(w, 400, err)
			return
		}
		if err := a.syncManaged(v); err != nil {
			fail(w, 500, err)
			return
		}
		send(w, 200, v)
	})
	mux.HandleFunc("POST /api/agents/batch", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			IDs    []string `json:"ids"`
			Action string   `json:"action"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, 400, err)
			return
		}
		if len(in.IDs) == 0 || len(in.IDs) > 100 {
			fail(w, 400, errors.New("select 1-100 agents"))
			return
		}
		results := make([]map[string]string, 0, len(in.IDs))
		for _, id := range in.IDs {
			v, e := a.registry.Action(id, in.Action)
			if e == nil {
				e = a.syncManaged(v)
			}
			status := "ok"
			detail := ""
			if e != nil {
				status = "error"
				detail = e.Error()
			}
			results = append(results, map[string]string{"id": id, "status": status, "detail": detail})
		}
		send(w, 200, map[string]any{"results": results})
	})
	mux.HandleFunc("DELETE /api/agents/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, ok := a.registry.Find(r.PathValue("id"))
		if !ok {
			fail(w, 404, errors.New("agent not found"))
			return
		}
		if v.Desired.Enabled || v.Runtime.Status == "running" {
			fail(w, 409, errors.New("请先停止 Agent 上的代理并等待状态确认，再撤销注册；离线机器请先在主机上手动停止 Agent"))
			return
		}
		if err := a.registry.Delete(v.ID); err != nil {
			fail(w, 500, err)
			return
		}
		_ = a.store.DeleteManaged(v.ID)
		send(w, 200, map[string]any{"ok": true})
	})
	mux.HandleFunc("GET /api/agents/{id}/profile", func(w http.ResponseWriter, r *http.Request) {
		c, ok := a.registry.Credentials(r.PathValue("id"))
		if !ok {
			fail(w, 404, errors.New("agent not found"))
			return
		}
		if c.PublicHost == "" || c.Port == 0 || c.Username == "" || c.Password == "" {
			fail(w, 409, errors.New("please configure public host and credentials first"))
			return
		}
		process := r.URL.Query().Get("process")
		if process == "" {
			process = "Client.exe"
		}
		if !regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,96}$`).MatchString(process) {
			fail(w, 400, errors.New("invalid process name"))
			return
		}
		profile := map[string]any{"Version": "1.0", "IsTrafficLoggingEnabled": true, "ProxyConfigs": []any{map[string]any{"Id": 1, "Type": "socks5", "Host": c.PublicHost, "Port": strconv.Itoa(c.Port), "Username": c.Username, "Password": c.Password}}, "ProxyRules": []any{map[string]any{"ProcessName": process, "TargetHosts": "*", "TargetPorts": "*", "Protocol": "BOTH", "Action": "PROXY", "IsEnabled": true, "LocalhostViaProxy": false, "ProxyConfigId": 1}}}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="proxybridge-`+r.PathValue("id")+`.pbprofile"`)
		_ = json.NewEncoder(w).Encode(profile)
	})
}
