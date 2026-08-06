package server

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"pac-server/internal/config"
	"pac-server/internal/domainlist"
	"pac-server/internal/netiface"
	"pac-server/internal/pac"
)

//go:embed web/templates/*.html web/static/*
var content embed.FS

type Server struct {
	cfgPath  string
	logger   *slog.Logger
	indexTpl *template.Template

	mu  sync.RWMutex
	cfg config.Config
}

type pageData struct {
	Config            config.Config
	Profiles          []profileView
	Error             string
	Message           string
	DefaultPAC        string
	ProfileCount      int
	ProxyTypes        []string
	Lists             []listEntry
	Loaded            loadedList
	Interfaces        []netiface.Option
	SelectedListenIPs map[string]bool
}

type profileView struct {
	config.PACProfile
	DirectText string
	ProxyText  string
	PACURL     string
}

type listEntry struct {
	Name        string
	Description string
	RawURL      string
}

type loadedList struct {
	Name    string
	RawURL  string
	Count   int
	Preview []string
}

func New(cfg config.Config, cfgPath string, logger *slog.Logger) *Server {
	return &Server{
		cfg:      cfg,
		cfgPath:  cfgPath,
		logger:   logger,
		indexTpl: template.Must(template.ParseFS(content, "web/templates/*.html")),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	staticFS, err := fs.Sub(content, "web/static")
	if err != nil {
		s.logger.Error("load static fs", "error", err)
		staticFS = content
	}

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/settings", s.handleSettings)
	mux.HandleFunc("/lists", s.handleLists)
	mux.HandleFunc("/lists/load", s.handleLoadList)
	mux.HandleFunc("/lists/import", s.handleImportList)
	mux.HandleFunc("/profiles", s.handleCreateProfile)
	mux.HandleFunc("/profiles/", s.handleProfileAction)
	mux.HandleFunc("/pac/", s.handleNamedPAC)
	mux.HandleFunc("/proxy.pac", s.handleDefaultPAC)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))

	return loggingMiddleware(s.logger, securityHeaders(mux))
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "index.html", pageData{})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		cfg := s.snapshot()
		selected := make(map[string]bool, len(cfg.ListenIPs))
		for _, ip := range cfg.ListenIPs {
			selected[ip] = true
		}
		s.render(w, r, "settings.html", pageData{
			Interfaces:        netiface.WithCurrent(netiface.List(), cfg.ListenIPs),
			SelectedListenIPs: selected,
		})
		return
	}
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		redirectError(w, r, "/settings", err)
		return
	}

	ips, port := config.NewSettings(r.Form["listen_ip"], r.FormValue("listen_port"))
	if err := s.update(func(cfg *config.Config) error {
		cfg.ListenIPs = ips
		cfg.ListenPort = port
		return nil
	}); err != nil {
		redirectError(w, r, "/settings", err)
		return
	}

	redirectMessage(w, r, "/settings", "settings saved; restart server to apply listen address")
}

func (s *Server) handleLists(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/lists" {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "lists.html", pageData{})
}

func (s *Server) handleLoadList(w http.ResponseWriter, r *http.Request) {
	listName := r.URL.Query().Get("list")
	if listName == "" {
		listName = r.FormValue("v2fly_list")
	}
	if strings.TrimSpace(listName) == "" {
		s.render(w, r, "lists.html", pageData{})
		return
	}

	domains, err := domainlist.NewClient().Fetch(listName)
	if err != nil {
		s.render(w, r, "lists.html", pageData{Error: err.Error()})
		return
	}
	if len(domains) == 0 {
		s.render(w, r, "lists.html", pageData{Error: errNoImportedDomains.Error()})
		return
	}

	preview := domains
	if len(preview) > 300 {
		preview = preview[:300]
	}

	s.render(w, r, "lists.html", pageData{
		Loaded: loadedList{
			Name:    listName,
			RawURL:  domainlist.RawURL(listName),
			Count:   len(domains),
			Preview: preview,
		},
	})
}

func (s *Server) handleImportList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		redirectError(w, r, "/lists", err)
		return
	}

	listName := r.FormValue("v2fly_list")
	target := r.FormValue("import_target")
	if target != "direct" {
		target = "proxy"
	}
	profileSlug := config.Slugify(r.FormValue("profile_slug"))

	domains, err := domainlist.NewClient().Fetch(listName)
	if err != nil {
		redirectError(w, r, "/lists", err)
		return
	}
	if len(domains) == 0 {
		redirectError(w, r, "/lists", errNoImportedDomains)
		return
	}

	if err := s.update(func(cfg *config.Config) error {
		for i := range cfg.Profiles {
			if cfg.Profiles[i].Slug == profileSlug {
				if target == "direct" {
					cfg.Profiles[i].DirectDomains = mergeDomains(cfg.Profiles[i].DirectDomains, domains)
				} else {
					cfg.Profiles[i].ProxyDomains = mergeDomains(cfg.Profiles[i].ProxyDomains, domains)
				}
				return nil
			}
		}
		return errProfileNotFound
	}); err != nil {
		redirectError(w, r, "/lists", err)
		return
	}

	redirectMessage(w, r, "/lists", "imported "+listName)
}

func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	profile, err := profileFromRequest(r)
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}

	if err := s.update(func(cfg *config.Config) error {
		if hasSlug(*cfg, profile.Slug, "") {
			return errDuplicateSlug
		}
		cfg.Profiles = append(cfg.Profiles, profile)
		return nil
	}); err != nil {
		redirectError(w, r, "/", err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleProfileAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/profiles/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	slug := config.Slugify(parts[0])
	action := parts[1]

	switch action {
	case "update":
		s.handleUpdateProfile(w, r, slug)
	case "delete":
		s.handleDeleteProfile(w, r, slug)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request, oldSlug string) {
	profile, err := profileFromRequest(r)
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}

	if err := s.update(func(cfg *config.Config) error {
		if hasSlug(*cfg, profile.Slug, oldSlug) {
			return errDuplicateSlug
		}

		for i := range cfg.Profiles {
			if cfg.Profiles[i].Slug == oldSlug {
				cfg.Profiles[i] = profile
				return nil
			}
		}
		return errProfileNotFound
	}); err != nil {
		redirectError(w, r, "/", err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request, slug string) {
	if err := s.update(func(cfg *config.Config) error {
		if len(cfg.Profiles) == 1 {
			return errLastProfile
		}

		for i := range cfg.Profiles {
			if cfg.Profiles[i].Slug == slug {
				cfg.Profiles = append(cfg.Profiles[:i], cfg.Profiles[i+1:]...)
				return nil
			}
		}
		return errProfileNotFound
	}); err != nil {
		redirectError(w, r, "/", err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleDefaultPAC(w http.ResponseWriter, r *http.Request) {
	cfg := s.snapshot()
	s.writePAC(w, cfg.DefaultProfile())
}

func (s *Server) handleNamedPAC(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/pac/")
	slug = strings.TrimSuffix(slug, ".pac")
	slug = config.Slugify(slug)

	cfg := s.snapshot()
	profile, ok := cfg.FindProfile(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}

	s.writePAC(w, profile)
}

func (s *Server) writePAC(w http.ResponseWriter, profile config.PACProfile) {
	pacFile, err := pac.Generate(profile)
	if err != nil {
		s.logger.Error("generate pac", "profile", profile.Slug, "error", err)
		http.Error(w, "pac generation error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(pacFile))
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(s.snapshot())
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, templateName string, data pageData) {
	cfg := s.snapshot()
	if data.Config.Profiles == nil {
		data.Config = cfg
	}
	if data.Error == "" {
		data.Error = r.URL.Query().Get("error")
	}
	if data.Message == "" {
		data.Message = r.URL.Query().Get("message")
	}
	data.DefaultPAC = "/proxy.pac"
	data.ProfileCount = len(cfg.Profiles)
	data.ProxyTypes = []string{"SOCKS5", "SOCKS", "PROXY", "HTTPS", "DIRECT"}
	data.Lists = commonLists()
	data.Profiles = make([]profileView, 0, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		data.Profiles = append(data.Profiles, profileView{
			PACProfile: profile,
			DirectText: config.JoinList(profile.DirectDomains),
			ProxyText:  config.JoinList(profile.ProxyDomains),
			PACURL:     "/pac/" + profile.Slug + ".pac",
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.indexTpl.ExecuteTemplate(w, templateName, data); err != nil {
		s.logger.Error("render template", "template", templateName, "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) snapshot() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *Server) update(fn func(*config.Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := cloneConfig(s.cfg)
	if err := fn(&next); err != nil {
		return err
	}
	if err := config.Save(s.cfgPath, next); err != nil {
		return err
	}

	s.cfg = next
	return nil
}

func profileFromRequest(r *http.Request) (config.PACProfile, error) {
	if err := r.ParseForm(); err != nil {
		return config.PACProfile{}, err
	}

	profile := config.NewProfile(
		r.FormValue("name"),
		r.FormValue("slug"),
		r.FormValue("proxy_type"),
		r.FormValue("proxy_host"),
		r.FormValue("proxy_port"),
		r.FormValue("fallback"),
		r.FormValue("direct_domains"),
		r.FormValue("proxy_domains"),
	)
	if profile.Name == "" {
		return config.PACProfile{}, errEmptyName
	}
	if profile.ProxyType != "DIRECT" && profile.ProxyHost == "" {
		return config.PACProfile{}, errEmptyProxy
	}

	return profile, nil
}

func hasSlug(cfg config.Config, slug string, except string) bool {
	for _, profile := range cfg.Profiles {
		if profile.Slug == slug && profile.Slug != except {
			return true
		}
	}
	return false
}

func redirectError(w http.ResponseWriter, r *http.Request, path string, err error) {
	http.Redirect(w, r, path+"?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
}

func redirectMessage(w http.ResponseWriter, r *http.Request, path string, message string) {
	http.Redirect(w, r, path+"?message="+url.QueryEscape(message), http.StatusSeeOther)
}

func mergeDomains(base []string, extra []string) []string {
	joined := strings.Join(base, "\n") + "\n" + strings.Join(extra, "\n")
	return config.SplitList(joined)
}

func cloneConfig(cfg config.Config) config.Config {
	next := cfg
	next.Profiles = append([]config.PACProfile(nil), cfg.Profiles...)
	for i := range next.Profiles {
		next.Profiles[i].DirectDomains = append([]string(nil), next.Profiles[i].DirectDomains...)
		next.Profiles[i].ProxyDomains = append([]string(nil), next.Profiles[i].ProxyDomains...)
	}
	return next
}

func commonLists() []listEntry {
	names := []struct {
		name string
		desc string
	}{
		{"google", "Google domains"},
		{"telegram", "Telegram domains"},
		{"youtube", "YouTube domains"},
		{"twitter", "X / Twitter domains"},
		{"facebook", "Facebook domains"},
		{"category-ads-all", "Advertising domains"},
		{"geolocation-!cn", "Outside China geolocation list"},
		{"private", "Private and local domains"},
	}

	lists := make([]listEntry, 0, len(names))
	for _, item := range names {
		lists = append(lists, listEntry{
			Name:        item.name,
			Description: item.desc,
			RawURL:      domainlist.RawURL(item.name),
		})
	}
	return lists
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start).String(),
		)
	})
}
