package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/walkmiao/nodeswarm/internal/agent"
	"github.com/walkmiao/nodeswarm/internal/control"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/walkmiao/nodeswarm/internal/manager"
	"github.com/walkmiao/nodeswarm/internal/monitor"
	"github.com/walkmiao/nodeswarm/internal/ops"
)

//go:embed web/*
var assets embed.FS

type app struct {
	store      *manager.Store
	registry   *control.Registry
	alerts     *ops.Store
	adminToken string
	enrollKey  string
}

func send(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, code int, err error) {
	send(w, code, map[string]any{"error": err.Error()})
}
func decode(r *http.Request, out any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return errors.New("请求必须使用 application/json")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 256*1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var more any
	if err := dec.Decode(&more); err != io.EOF {
		return errors.New("只允许一个 JSON 对象")
	}
	return nil
}
func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, map[string]any{"status": "ok", "mode": "controller", "version": "1.2.0"})
	})
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		nodes, as, ev := a.store.Snapshot()
		send(w, 200, map[string]any{"nodes": nodes, "assignments": as, "events": ev, "proxyEngine": "client_transparent_proxy_not_integrated", "note": "进程绑定仅为规划；云端 Agent SOCKS5 是实际运行的远程代理服务"})
	})
	mux.HandleFunc("GET /api/auth/check", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, map[string]any{"ok": true, "role": "administrator"})
	})
	a.agentsRoutes(mux)
	a.clientRoutes(mux)
	mux.HandleFunc("GET /api/processes", func(w http.ResponseWriter, r *http.Request) {
		procs, err := manager.ListProcesses()
		if err != nil {
			fail(w, 500, err)
			return
		}
		a.store.PruneProcesses(procs)
		filter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("name")))
		if filter != "" {
			var filtered []manager.Process
			for _, p := range procs {
				if strings.Contains(strings.ToLower(p.Name), filter) || strings.Contains(strings.ToLower(p.Path), filter) {
					filtered = append(filtered, p)
				}
			}
			procs = filtered
		}
		send(w, 200, map[string]any{"processes": procs, "count": len(procs)})
	})
	mux.HandleFunc("POST /api/nodes", func(w http.ResponseWriter, r *http.Request) {
		var in manager.NodeInput
		if err := decode(r, &in); err != nil {
			fail(w, 400, err)
			return
		}
		node, err := a.store.Create(in)
		if err != nil {
			fail(w, 400, err)
			return
		}
		send(w, 201, node)
	})
	mux.HandleFunc("PUT /api/nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in manager.NodeInput
		if err := decode(r, &in); err != nil {
			fail(w, 400, err)
			return
		}
		node, err := a.store.Update(r.PathValue("id"), in)
		if err != nil {
			fail(w, 400, err)
			return
		}
		send(w, 200, node)
	})
	mux.HandleFunc("DELETE /api/nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := a.store.Delete(r.PathValue("id")); err != nil {
			fail(w, 404, err)
			return
		}
		send(w, 200, map[string]any{"ok": true})
	})
	mux.HandleFunc("POST /api/nodes/import", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text string `json:"text"`
		}
		if err := decode(r, &body); err != nil {
			fail(w, 400, err)
			return
		}
		inputs, err := manager.ParseImport(body.Text)
		if err != nil {
			fail(w, 400, err)
			return
		}
		count, err := a.store.Import(inputs)
		if err != nil {
			fail(w, 400, err)
			return
		}
		send(w, 200, map[string]any{"imported": count, "skipped": len(inputs) - count})
	})
	// Bounded concurrent probes prevent one slow external proxy from freezing
	// the controller API or exhausting sockets.
	mux.HandleFunc("POST /api/nodes/probe-batch", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			IDs []string `json:"ids"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, 400, err)
			return
		}
		nodes, _, _ := a.store.Snapshot()
		wanted := make(map[string]bool, len(in.IDs))
		for _, id := range in.IDs {
			wanted[id] = true
		}
		var selected []manager.Node
		for _, n := range nodes {
			if len(in.IDs) == 0 || wanted[n.ID] {
				selected = append(selected, n)
			}
		}
		if len(selected) > 32 || len(in.IDs) > 32 {
			fail(w, 400, errors.New("一次最多检测 32 个节点"))
			return
		}
		type item struct {
			ID    string        `json:"id"`
			Name  string        `json:"name"`
			Probe manager.Probe `json:"probe"`
			Error string        `json:"error,omitempty"`
		}
		results := make([]item, len(selected))
		sem := make(chan struct{}, 8)
		var wg sync.WaitGroup
		for i, n := range selected {
			i, n := i, n
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				source, exists := a.store.NodeByID(n.ID)
				if !exists {
					results[i] = item{ID: n.ID, Name: n.Name, Error: "node was deleted"}
					return
				}
				probe := manager.ProbeSOCKS5(source, 4*time.Second)
				results[i] = item{ID: n.ID, Name: n.Name, Probe: probe}
				if err := a.store.SaveProbe(n.ID, probe); err != nil {
					results[i].Error = err.Error()
				}
			}()
		}
		wg.Wait()
		send(w, 200, map[string]any{"results": results, "count": len(results)})
	})
	mux.HandleFunc("POST /api/nodes/{id}/probe", func(w http.ResponseWriter, r *http.Request) {
		n, ok := a.store.NodeByID(r.PathValue("id"))
		if !ok {
			fail(w, 404, errors.New("节点不存在"))
			return
		}
		p := manager.ProbeSOCKS5(n, 5*time.Second)
		if err := a.store.SaveProbe(n.ID, p); err != nil {
			fail(w, 500, err)
			return
		}
		send(w, 200, p)
	})
	mux.HandleFunc("POST /api/assignments", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			PID       int    `json:"pid"`
			StartTime string `json:"startTime"`
			NodeID    string `json:"nodeId"`
			Auto      bool   `json:"auto"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, 400, err)
			return
		}
		procs, err := manager.ListProcesses()
		if err != nil {
			fail(w, 500, err)
			return
		}
		a.store.PruneProcesses(procs)
		var match manager.Process
		for _, p := range procs {
			if p.PID == in.PID && p.StartTime == in.StartTime {
				match = p
				break
			}
		}
		if match.PID == 0 {
			fail(w, 409, errors.New("进程不存在或已重启，请刷新列表"))
			return
		}
		a1, err := a.store.Assign(match, in.NodeID, in.Auto)
		if err != nil {
			fail(w, 400, err)
			return
		}
		send(w, 200, a1)
	})
	mux.HandleFunc("DELETE /api/assignments", func(w http.ResponseWriter, r *http.Request) {
		pid, err := strconv.Atoi(r.URL.Query().Get("pid"))
		if err != nil {
			fail(w, 400, errors.New("pid 无效"))
			return
		}
		if err := a.store.Unassign(pid, r.URL.Query().Get("startTime")); err != nil {
			fail(w, 404, err)
			return
		}
		send(w, 200, map[string]any{"ok": true})
	})
	web, err := fs.Sub(assets, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /", http.FileServer(http.FS(web)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin := r.Header.Get("Origin"); origin != "" {
				expected := "http://" + r.Host
				if origin != expected && origin != "https://"+r.Host {
					fail(w, 403, errors.New("不允许跨站写入"))
					return
				}
			}
		}
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/health" && !strings.HasPrefix(r.URL.Path, "/api/agent/") {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if a.adminToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(a.adminToken)) != 1 {
				fail(w, http.StatusUnauthorized, errors.New("管理员访问令牌无效"))
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}

func readOrCreateSecret(dir, filename, envName string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(envName)); v != "" {
		if len(v) < 24 {
			return "", fmt.Errorf("%s must be at least 24 characters", envName)
		}
		return v, nil
	}
	path := filepath.Join(dir, filename)
	if data, err := os.ReadFile(path); err == nil {
		v := strings.TrimSpace(string(data))
		if len(v) < 24 {
			return "", fmt.Errorf("invalid secret file %s", path)
		}
		return v, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	_, err = f.WriteString(token + "\n")
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return token, err
}
func main() {
	mode := flag.String("mode", "controller", "controller, agent (cloud SOCKS5), or client (Windows process monitor)")
	controller := flag.String("controller", "", "controller HTTPS URL in agent mode")
	name := flag.String("name", "", "agent display name")
	listen := flag.String("listen", "", "controller bind address (default 127.0.0.1:8088)")
	flag.Parse()
	if v := os.Getenv("NODE_MANAGER_MODE"); v != "" && *mode == "controller" {
		*mode = v
	}
	dataDir := os.Getenv("NODE_MANAGER_DATA")
	if dataDir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			log.Fatal(err)
		}
		dataDir = filepath.Join(base, "NodeConsole")
	}
	if *mode == "agent" || *mode == "client" {
		url := *controller
		if url == "" {
			url = os.Getenv("NODE_MANAGER_CONTROLLER")
		}
		if url == "" {
			log.Fatal("agent requires -controller or NODE_MANAGER_CONTROLLER")
		}
		display := *name
		if display == "" {
			display = os.Getenv("NODE_MANAGER_AGENT_NAME")
		}
		if display == "" {
			display, _ = os.Hostname()
		}
		agentDir := os.Getenv("NODE_MANAGER_AGENT_DATA")
		if agentDir == "" {
			agentDir = filepath.Join(dataDir, "agent")
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		log.Printf("Starting %s %q for %s", *mode, display, url)
		clientEngine := os.Getenv("NODE_MANAGER_CLIENT_ENGINE")
		if err := agent.Run(ctx, agent.Config{Controller: url, Name: display, EnrollKey: os.Getenv("NODE_MANAGER_ENROLL_KEY"), DataDir: agentDir,
			AllowInsecureHTTP: os.Getenv("NODE_MANAGER_ALLOW_INSECURE_HTTP") == "1", Role: *mode,
			ClientEngine: clientEngine, ProxyBridgeCLI: os.Getenv("NODE_MANAGER_PROXYBRIDGE_CLI"),
			ClientTargets: os.Getenv("NODE_MANAGER_CLIENT_TARGETS")}); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *mode != "controller" {
		log.Fatalf("unknown mode: %s", *mode)
	}
	store, err := manager.OpenStore(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	registry, err := control.NewRegistry(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	alerts, err := ops.Open(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	adminToken, err := readOrCreateSecret(dataDir, "admin.token", "NODE_MANAGER_ADMIN_TOKEN")
	if err != nil {
		log.Fatal(err)
	}
	enrollKey, err := readOrCreateSecret(dataDir, "enroll.token", "NODE_MANAGER_ENROLL_KEY")
	if err != nil {
		log.Fatal(err)
	}
	addr := *listen
	if addr == "" {
		addr = os.Getenv("NODE_MANAGER_LISTEN")
	}
	if addr == "" {
		port := os.Getenv("NODE_MANAGER_PORT")
		if port == "" {
			port = "8088"
		}
		if _, err := strconv.Atoi(port); err != nil {
			log.Fatal("NODE_MANAGER_PORT must be numeric")
		}
		addr = net.JoinHostPort("127.0.0.1", port)
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		log.Fatal("invalid listen address: ", err)
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) && os.Getenv("NODE_MANAGER_ALLOW_PUBLIC_LISTEN") != "1" {
		log.Fatal("refusing public HTTP binding; use Caddy/Nginx HTTPS reverse proxy to 127.0.0.1:8088, or explicitly set NODE_MANAGER_ALLOW_PUBLIC_LISTEN=1 for secure private networks")
	}
	fmt.Printf("NodeSwarm v1.2: http://%s\nData directory: %s\nAdmin token: %s\nEnrollment token: %s\n", addr, dataDir, filepath.Join(dataDir, "admin.token"), filepath.Join(dataDir, "enroll.token"))
	srv := &http.Server{Addr: addr, Handler: (&app{store: store, registry: registry, alerts: alerts, adminToken: adminToken, enrollKey: enrollKey}).routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 75 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go monitor.Run(ctx, store)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
