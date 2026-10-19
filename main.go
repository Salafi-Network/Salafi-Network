package main

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

//go:embed templates/*.html static/*
var assets embed.FS

type App struct {
	tmpl      *template.Template
	forum     string
	community *Community
}
type Page struct {
	SEO                       seoMetadata
	Lang, Dir, Section, Title string
	Arabic                    bool
}

func (p Page) Text(en, ar string) string {
	if p.Arabic {
		return ar
	}
	return en
}
func newApp(_ string) (*App, error) {
	forum := os.Getenv("FORUM_URL")
	if forum == "" {
		forum = "http://localhost:8095/community"
	}
	u, err := url.Parse(forum)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, &url.Error{Op: "parse FORUM_URL", URL: forum, Err: errInvalidForum{}}
	}
	t, err := template.New("").Funcs(baseTemplateFuncs()).ParseFS(assets, "templates/*.html")
	return &App{tmpl: t, forum: strings.TrimRight(forum, "/")}, err
}

type errInvalidForum struct{}

func (errInvalidForum) Error() string { return "must be an absolute HTTP or HTTPS URL" }
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; img-src 'self' data:; form-action 'self'; frame-ancestors 'none'; base-uri 'self'")
	if a.serveDiscovery(w, r) {
		return
	}
	if strings.HasPrefix(r.URL.Path, "/static/") {
		sub, _ := fs.Sub(assets, "static")
		http.StripPrefix("/static/", http.FileServer(http.FS(sub))).ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/community" || strings.HasPrefix(r.URL.Path, "/community/") {
		if a.community == nil {
			http.Error(w, "Community preview is not enabled", 503)
			return
		}
		a.community.ServeHTTP(w, r)
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", 405)
		return
	}
	if r.URL.Path == "/" {
		http.Redirect(w, r, "/"+preferredLanguage(r)+"/", 302)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 1 || (parts[0] != "en" && parts[0] != "ar") {
		http.NotFound(w, r)
		return
	}
	rememberLanguage(w, parts[0])
	p := Page{Lang: parts[0], Dir: "ltr", Section: "home"}
	p.Arabic = p.Lang == "ar"
	if p.Arabic {
		p.Dir = "rtl"
	}
	if len(parts) > 1 {
		p.Section = parts[1]
	}
	switch p.Section {
	case "forum":
		http.Redirect(w, r, a.forum+"/?lang="+p.Lang, http.StatusFound)
		return
	case "search":
		http.Redirect(w, r, a.forum+"?lang="+p.Lang+"&q="+url.QueryEscape(r.URL.Query().Get("q")), http.StatusFound)
		return
	case "articles", "learn":
		a.serveCatalog(w, r, p, parts)
		return
	case "home":
		p.Title = p.Text("A home for knowledge, learning, and community", "بيت للسلفيين")
	case "about":
		p.Title = p.Text("About the project", "عن المشروع")
	case "guidelines":
		http.Redirect(w, r, a.forum, 302)
		return
	default:
		http.NotFound(w, r)
		return
	}
	if len(parts) > 2 {
		http.NotFound(w, r)
		return
	}
	p.SEO = pageSEO(p, "/"+p.Lang+"/", r)
	if p.Section == "about" {
		p.SEO = pageSEO(p, "/"+p.Lang+"/about", r)
	}
	var b bytes.Buffer
	if err := a.tmpl.ExecuteTemplate(&b, "page.html", p); err != nil {
		log.Print(err)
		http.Error(w, "Unable to render page", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b.Bytes())
}
func main() {
	if len(os.Args) == 3 && os.Args[1] == "backup-community" {
		path := os.Getenv("COMMUNITY_DB")
		if path == "" {
			log.Fatal("COMMUNITY_DB is required")
		}
		c, err := openCommunity(path)
		if err != nil {
			log.Fatal(err)
		}
		defer c.db.Close()
		if _, err := c.db.Exec("VACUUM INTO ?", os.Args[2]); err != nil {
			log.Fatal(err)
		}
		log.Print("Community database backup completed")
		return
	}
	a, err := newApp("")
	if err != nil {
		log.Fatal(err)
	}
	if path := os.Getenv("COMMUNITY_DB"); path != "" {
		a.community, err = openCommunity(path)
		if err != nil {
			log.Fatal(err)
		}
		defer a.community.db.Close()
		if file := os.Getenv("COMMUNITY_ADMIN_PASSWORD_FILE"); file != "" {
			password, readErr := os.ReadFile(file)
			if readErr != nil {
				log.Fatal(readErr)
			}
			if err = a.community.bootstrapAdmin("akram", strings.TrimSpace(string(password))); err != nil {
				log.Fatal(err)
			}
		}
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8094"
	}
	log.Printf("Salafi Network: http://%s", addr)
	log.Fatal((&http.Server{Addr: addr, Handler: a, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}).ListenAndServe())
}
