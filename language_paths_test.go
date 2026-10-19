package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestLanguageAcrossSections(t *testing.T) {
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
	cookies := map[string]*http.Cookie{}
	visit := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.RemoteAddr = "127.0.0.1:1234"
		for _, cookie := range cookies {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		for _, cookie := range w.Result().Cookies() {
			cookies[cookie.Name] = cookie
		}
		return w
	}
	visit("/ar/learn")
	if cookies["sn_language"] == nil || cookies["sn_language"].Path != "/" {
		t.Fatal("missing root language preference")
	}
	for _, path := range []string{"/community", "/community/new", "/community/login", "/community/register"} {
		w := visit(path)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `lang="ar" dir="rtl"`) || !strings.Contains(w.Body.String(), `href="/ar/articles"`) {
			t.Fatal("Arabic lost", path, w.Code)
		}
	}
	if w := visit("/"); w.Header().Get("Location") != "/ar/" {
		t.Fatal("home forgot language")
	}
	if w := visit("/ar/articles"); !strings.Contains(w.Body.String(), "اقرأ مقالات نافعة") {
		t.Fatal("articles language")
	}
	w := visit("/community?category=1&lang=en")
	if !strings.Contains(w.Body.String(), `lang="en" dir="ltr"`) {
		t.Fatal("cannot switch back")
	}
	if cookies["sn_language"].Value != "en" {
		t.Fatal("English not persisted")
	}
	w = visit("/community/login")
	if !strings.Contains(w.Body.String(), "Welcome back.") {
		t.Fatal("English lost")
	}
	// User-generated titles and bodies must not be translated; templates are request-local.
	c.bootstrapAdmin("owner", "owner-password-12345")
	c.db.Exec("INSERT INTO topics(id,category_id,member_id,title,body,created) VALUES(1,1,1,'Home','Forum','today')")
	c.db.Exec("INSERT INTO replies(id,topic_id,member_id,body,created) VALUES(1,1,1,'Learning','today')")
	w = visit("/community/t/1?lang=ar")
	if !strings.Contains(w.Body.String(), ">Home</h1>") || !strings.Contains(w.Body.String(), "صاحب الموضوع") {
		t.Fatal("thread content or translation incorrect")
	}
	w = visit("/community/u/owner")
	if !strings.Contains(w.Body.String(), "ملخص الملف الشخصي") {
		t.Fatal("profile not translated")
	}
	var wg sync.WaitGroup
	for _, lang := range []string{"ar", "en", "ar", "en"} {
		wg.Add(1)
		go func(lang string) {
			defer wg.Done()
			r := httptest.NewRequest("GET", "/community?lang="+lang, nil)
			w := httptest.NewRecorder()
			c.ServeHTTP(w, r)
			if !strings.Contains(w.Body.String(), `lang="`+lang+`"`) {
				t.Error("language leaked across requests")
			}
		}(lang)
	}
	wg.Wait()
}
func TestLearningResourceCollection(t *testing.T) {
	c, e := openCommunity(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer c.db.Close()
	c.bootstrapAdmin("owner", "owner-password-12345")
	f := &forumClient{c: c, cookies: map[string]*http.Cookie{}}
	f.request("GET", "/community/login", nil)
	f.request("POST", "/community/login", url.Values{"username": {"owner"}, "password": {"owner-password-12345"}})
	v := url.Values{"action": {"content-save"}, "kind": {"path"}, "topic": {"aqeedah"}, "title_en": {"Beginner collection"}, "title_ar": {"مسار للمبتدئين"}, "step_en": {"", "", ""}, "step_href": {"/articles/reading-with-context", "/learn/riyad-as-salihin", "/learn/ibn-baz"}, "step_body_en": {"Read this first.", "Then study the book.", "Consult the scholarly sources."}}
	w := f.request("POST", "/community/admin", v)
	if w.Code != 303 {
		t.Fatal(w.Body.String())
	}
	items, _ := c.catalogItems()
	var slug string
	for _, item := range items {
		if item.Title.EN == "Beginner collection" {
			slug = item.Slug
			if len(item.Steps) != 3 {
				t.Fatal("resources not saved")
			}
		}
	}
	a, _ := newApp("")
	a.community = c
	for _, lang := range []string{"en", "ar"} {
		out := httptest.NewRecorder()
		a.ServeHTTP(out, httptest.NewRequest("GET", "/"+lang+"/learn/"+slug, nil))
		body := out.Body.String()
		if out.Code != 200 || !strings.Contains(body, "resource-roadmap") {
			t.Fatal("missing collection")
		}
		for _, bad := range []string{"<progress", `type="checkbox"`, "localStorage", "Mark as complete", "catalog.js"} {
			if strings.Contains(body, bad) {
				t.Fatal("checklist remains", bad)
			}
		}
		x := strings.Index(body, "/articles/reading-with-context")
		y := strings.Index(body, "/learn/riyad-as-salihin")
		z := strings.Index(body, "/learn/ibn-baz")
		if x < 0 || y < x || z < y {
			t.Fatal("resource order changed")
		}
	}
	v.Set("step_href", "/learn/reading-plan")
	if w = f.request("POST", "/community/admin", v); w.Code != 400 {
		t.Fatal("nested path accepted")
	}
	v.Set("step_href", "/articles/missing")
	if w = f.request("POST", "/community/admin", v); w.Code != 400 {
		t.Fatal("missing resource accepted")
	}
}
