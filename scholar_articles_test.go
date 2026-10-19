package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestScholarArticleAssociation(t *testing.T) {
	c, e := openCommunity(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer c.db.Close()
	if e = c.bootstrapAdmin("owner", "owner-password-12345"); e != nil {
		t.Fatal(e)
	}
	f := &forumClient{c: c, cookies: map[string]*http.Cookie{}}
	f.request("GET", "/community/login", nil)
	f.request("POST", "/community/login", url.Values{"username": {"owner"}, "password": {"owner-password-12345"}})
	v := url.Values{"action": {"content-save"}, "kind": {"article"}, "topic": {"aqeedah"}, "title_en": {"Linked article"}, "title_ar": {"مقال مرتبط"}, "author_scholar": {"ibn-baz"}, "author": {"Named author"}}
	if w := f.request("POST", "/community/admin", v); w.Code != 303 {
		t.Fatal(w.Body.String())
	}
	items, _ := c.catalogItems()
	var linked catalogItem
	for _, item := range items {
		if item.Title.EN == "Linked article" {
			linked = item
		}
	}
	if linked.AuthorScholar != "ibn-baz" {
		t.Fatal("Association not stored")
	}
	a, _ := newApp("")
	a.community = c
	for _, lang := range []string{"en", "ar"} {
		get := func(path string) string {
			w := httptest.NewRecorder()
			a.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			if w.Code != 200 {
				t.Fatal(w.Code)
			}
			return w.Body.String()
		}
		body := get("/" + lang + "/articles/" + linked.Slug)
		if !strings.Contains(body, `itemprop="url" href="/`+lang+`/learn/ibn-baz"`) {
			t.Fatal("Author is not linked")
		}
		body = get("/" + lang + "/learn/ibn-baz")
		if !strings.Contains(body, "/"+lang+"/articles/"+linked.Slug) {
			t.Fatal("Scholar articles missing")
		}
	}
	v.Set("author_scholar", "reading-plan")
	if w := f.request("POST", "/community/admin", v); w.Code != 400 {
		t.Fatal("Non-scholar accepted")
	}
	v.Set("author_scholar", "missing")
	if w := f.request("POST", "/community/admin", v); w.Code != 400 {
		t.Fatal("Missing scholar accepted")
	}
	if w := f.request("POST", "/community/admin", url.Values{"action": {"content-delete"}, "slug": {"ibn-baz"}, "confirm": {"yes"}}); w.Code != 303 {
		t.Fatal(w.Body.String())
	}
	items, _ = c.catalogItems()
	for _, item := range items {
		if item.Slug == linked.Slug && (item.AuthorScholar != "" || item.Author != "Named author") {
			t.Fatal("Deletion did not preserve author attribution and clear link")
		}
	}
}

func TestScholarLanguageFallback(t *testing.T) {
	p := catalogPage{Page: Page{Lang: "en"}}
	p.resolveAuthors([]catalogItem{{Slug: "scholar", Kind: "scholar", Title: localizedText{AR: "عالم"}}})
	item := catalogItem{AuthorScholar: "scholar"}
	if p.AuthorURL(item) != "/ar/learn/scholar" || p.AuthorName(item) != "عالم" {
		t.Fatal("Missing language fallback")
	}
}
