package main

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCatalogRoutesAndFilters(t *testing.T) {
	a, err := newApp("")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, contains string
		status         int
	}{
		{"/en/articles", "Read beneficial articles", 200}, {"/ar/articles", "اقرأ مقالات نافعة", 200}, {"/en/learn", "Learn with clarity", 200}, {"/ar/learn", "تعلّم بوضوح", 200},
		{"/en/articles/a-steady-study-routine", "Start with a manageable session", 200}, {"/en/learn/reading-plan", "class=\"resource-roadmap\"", 200},
		{"/en/learn?format=book", "Explore books", 200}, {"/en/learn?format=scholar", "Official resource directory", 200},
		{"/en/articles?q=zzzzzz", "No matching resources", 200}, {"/en/learn/not-a-resource", "", 404}, {"/en/articles/reading-plan", "", 404}, {"/en/learn/reading-plan/extra", "", 404},
	} {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) {
			t.Fatalf("%s: %d missing %q", tc.path, w.Code, tc.contains)
		}
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", "/en/articles?q="+url.QueryEscape("<script>alert(1)</script>"), nil))
	if strings.Contains(w.Body.String(), "<script>alert(1)</script>") {
		t.Fatal("Search was not escaped")
	}
	w = httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", "/en/learn?format=book", nil))
	if strings.Contains(w.Body.String(), `class="scholar-card"`) || strings.Contains(w.Body.String(), `class="path-card"`) {
		t.Fatal("Format filter ignored")
	}
}
func TestCatalogIntegrity(t *testing.T) {
	var items []catalogItem
	if err := json.Unmarshal(catalogJSON, &items); err != nil {
		t.Fatal(err)
	}
	a, err := newApp("")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range items {
		if seen[item.Slug] || item.Slug == "" || item.Title.EN == "" || item.Title.AR == "" {
			t.Fatal("Invalid or duplicate catalog entry", item.Slug)
		}
		seen[item.Slug] = true
		if item.Source != "" {
			u, e := url.Parse(item.Source)
			if e != nil || u.Scheme != "https" || u.Host == "" {
				t.Fatal("Invalid source", item.Source)
			}
		}
		section := "learn"
		if item.Kind == "article" {
			section = "articles"
		}
		for _, lang := range []string{"en", "ar"} {
			paths := []string{"/" + lang + "/" + section + "/" + item.Slug}
			for _, step := range item.Steps {
				paths = append(paths, "/"+lang+step.Href)
			}
			for _, path := range paths {
				w := httptest.NewRecorder()
				a.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
				if w.Code != 200 {
					t.Fatalf("Broken catalog link %s: %d", path, w.Code)
				}
			}
		}
	}
}
