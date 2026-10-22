package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdminPathDifficulty(t *testing.T) {
	c, err := openCommunity(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.db.Close()
	c.bootstrapAdmin("owner", "owner-password-12345")
	f := &forumClient{c: c, cookies: map[string]*http.Cookie{}}
	f.request("GET", "/community/login", nil)
	f.request("POST", "/community/login", url.Values{"username": {"owner"}, "password": {"owner-password-12345"}})
	v := url.Values{"action": {"content-save"}, "kind": {"path"}, "topic": {"aqeedah"}, "title_en": {"A study collection"}, "title_ar": {"مسار الدراسة"}, "slug": {"difficulty-test"}}
	a, err := newApp("")
	if err != nil {
		t.Fatal(err)
	}
	a.community = c
	for _, level := range []string{"intermediate", "hard", "expert", "beginner"} {
		v.Set("difficulty", level)
		w := f.request("POST", "/community/admin", v)
		if w.Code != http.StatusSeeOther {
			t.Fatalf("save %s: %d %s", level, w.Code, w.Body.String())
		}
		items, err := c.catalogItems()
		if err != nil {
			t.Fatal(err)
		}
		var saved catalogItem
		for _, item := range items {
			if item.Slug == "difficulty-test" {
				saved = item
			}
		}
		if saved.Difficulty != level || saved.Beginner != (level == "beginner") {
			t.Fatalf("incorrect persisted difficulty: %+v", saved)
		}
		editor := f.request("GET", "/community/admin?tab=learning&edit=difficulty-test", nil)
		if !strings.Contains(editor.Body.String(), `value="`+level+`" selected`) {
			t.Fatalf("editor lost %s", level)
		}
		for _, lang := range []string{"en", "ar"} {
			page := httptest.NewRecorder()
			a.ServeHTTP(page, httptest.NewRequest("GET", "/"+lang+"/learn/difficulty-test", nil))
			if page.Code != 200 || !strings.Contains(page.Body.String(), saved.DifficultyLabel(lang)) {
				t.Fatalf("missing %s public difficulty", lang)
			}
		}
		v.Set("original", "difficulty-test")
	}
	v.Set("difficulty", "invalid")
	if w := f.request("POST", "/community/admin", v); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid difficulty accepted: %d", w.Code)
	}
	if (catalogItem{}).PathDifficulty() != "beginner" {
		t.Fatal("legacy default changed")
	}
}
