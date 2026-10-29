package main

import (
	"encoding/xml"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestSEOPublicMetadata(t *testing.T) {
	a, e := newApp("")
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct{ path, want string }{
		{"/en/articles/a-steady-study-routine", `itemtype="https://schema.org/Article"`},
		{"/ar/articles/a-steady-study-routine", `hreflang="en"`},
		{"/en/articles?q=study", `content="noindex, follow"`},
		{"/en/articles", `content="index, follow"`},
	} {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("GET", publicOrigin+tc.path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), tc.want) {
			t.Fatalf("%s: %d missing %s", tc.path, w.Code, tc.want)
		}
		if !strings.Contains(w.Body.String(), `rel="canonical" href="https://salafinetwork.com/`) {
			t.Fatal("Missing canonical")
		}
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost:8095/en/articles", nil))
	if !strings.Contains(w.Body.String(), `content="noindex, follow"`) {
		t.Fatal("Development should not be indexed")
	}
	p := catalogPage{Page: Page{Lang: "ar", Arabic: true, Section: "articles", Title: `عنوان <script>`}, Selected: catalogItem{Slug: "arabic-only", Kind: "article", Title: localizedText{AR: "عنوان"}, Summary: localizedText{AR: "ملخص"}, MetaDescription: localizedText{EN: "English meta description", AR: "وصف ميتا عربي"}}}
	s := catalogSEO(p, httptest.NewRequest("GET", publicOrigin+"/ar/articles/arabic-only", nil))
	if len(s.Alternates) != 1 || s.Alternates[0].Lang != "ar" || s.Description != "وصف ميتا عربي" {
		t.Fatalf("Wrong localized metadata: %+v", s)
	}
	p.Selected.MetaDescription = localizedText{}
	s = catalogSEO(p, httptest.NewRequest("GET", publicOrigin+"/ar/articles/arabic-only", nil))
	if s.Description != "ملخص" {
		t.Fatalf("Summary fallback should be preserved: %+v", s)
	}
}

func TestDiscoveryTracksPublicContent(t *testing.T) {
	c, e := openCommunity(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer c.db.Close()
	a, e := newApp("")
	if e != nil {
		t.Fatal(e)
	}
	a.community = c
	_, e = c.db.Exec(`INSERT INTO members(id,name,password) VALUES(500,'seo_member','unused'); INSERT INTO topics(id,category_id,member_id,title,body,created,hidden) VALUES(800,1,500,'Public title','Public description','now',0),(801,1,500,'Hidden title','Hidden body','now',1)`)
	if e != nil {
		t.Fatal(e)
	}
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("GET", publicOrigin+path, nil))
		return w
	}
	w := get("/sitemap.xml")
	var doc sitemapDocument
	if w.Code != 200 || xml.Unmarshal(w.Body.Bytes(), &doc) != nil {
		t.Fatal("Invalid sitemap", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "/community/t/800") || strings.Contains(w.Body.String(), "/community/t/801") || strings.Contains(w.Body.String(), "/admin") {
		t.Fatal("Sitemap leaked hidden or private URLs")
	}
	for _, entry := range doc.URLs {
		resp := get(strings.TrimPrefix(entry.Loc, publicOrigin))
		if resp.Code != 200 {
			t.Fatalf("Sitemap URL %s returned %d", entry.Loc, resp.Code)
		}
	}
	w = get("/community/t/800")
	if !strings.Contains(w.Body.String(), "Public title · Salafi Network") || strings.Contains(w.Body.String(), `content="noindex, follow"`) {
		t.Fatal("Public discussion metadata missing")
	}
	for _, path := range []string{"/community/login", "/community/register", "/community?q=test"} {
		w = get(path)
		if w.Header().Get("X-Robots-Tag") != "noindex, follow" {
			t.Fatal("Private/search indexing: " + path)
		}
	}
	if _, e = c.db.Exec("DELETE FROM topics WHERE id=800"); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(get("/sitemap.xml").Body.String(), "/community/t/800") {
		t.Fatal("Deleted topic retained")
	}
	if !strings.Contains(get("/robots.txt").Body.String(), "Sitemap: "+publicOrigin+"/sitemap.xml") {
		t.Fatal("Missing sitemap discovery")
	}
}
