package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Never derive public canonical URLs from an untrusted request Host header.
const publicOrigin = "https://salafinetwork.com"

type seoAlternate struct{ Lang, URL string }
type seoMetadata struct {
	Title, Description, Canonical, Kind, Language string
	Alternates                                    []seoAlternate
	NoIndex                                       bool
	Author                                        string
}

func excerpt(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > 170 {
		return string(r[:167]) + "…"
	}
	return s
}
func localSEO(r *http.Request) bool {
	host := strings.Split(r.Host, ":")[0]
	return host == "localhost" || host == "127.0.0.1" || strings.HasPrefix(r.Host, "[::1]")
}
func pageSEO(p Page, path string, r *http.Request) seoMetadata {
	s := seoMetadata{Title: p.Title + " · Salafi Network", Description: p.Text("A home for beneficial reading, structured learning, and thoughtful discussion.", "مساحة للقراءة النافعة والتعلّم المنظّم والحوار الهادف."), Canonical: publicOrigin + path, Kind: "website", Language: p.Lang, NoIndex: localSEO(r)}
	for _, lang := range []string{"en", "ar"} {
		s.Alternates = append(s.Alternates, seoAlternate{lang, publicOrigin + "/" + lang + strings.TrimPrefix(path, "/"+p.Lang)})
	}
	return s
}
func catalogSEO(p catalogPage, r *http.Request) seoMetadata {
	path := p.Base()
	if p.Selected.Slug != "" {
		path = p.URL(p.Selected)
	}
	s := pageSEO(p.Page, path, r)
	if p.Section == "articles" {
		s.Description = p.Text("Browse beneficial articles and organized knowledge from Salafi Network.", "تصفّح المقالات النافعة والمعرفة المنظّمة في شبكة سلفي.")
	} else {
		s.Description = p.Text("Explore learning paths, books, and scholarly resources.", "استكشف مسارات التعلّم والكتب ومصادر أهل العلم.")
	}
	if p.Selected.Slug != "" {
		s.Description = excerpt(p.Selected.Summary.In(p.Lang))
		if s.Description == "" {
			for _, section := range p.Selected.Sections {
				if section.Body.In(p.Lang) != "" {
					s.Description = excerpt(section.Body.In(p.Lang))
					break
				}
			}
		}
		if s.Description == "" {
			s.Description = p.Selected.Title.In(p.Lang)
		}
		s.Alternates = nil
		for _, lang := range []string{"en", "ar"} {
			if strings.TrimSpace(p.Selected.Title.In(lang)) != "" {
				s.Alternates = append(s.Alternates, seoAlternate{lang, publicOrigin + "/" + lang + "/" + p.Section + "/" + p.Selected.Slug})
			}
		}
		if p.Selected.Kind == "article" {
			s.Kind = "article"
			s.Author = p.Selected.Author
		}
	}
	if p.Query != "" || p.TopicFilter != "" || p.Format != "" || p.Filter != "" || p.Sort != "" {
		s.NoIndex = true
	}
	return s
}
func communitySEO(p communityPage, r *http.Request) seoMetadata {
	s := pageSEO(p.Page, "/community", r)
	s.Title = forumTranslator(p.Lang)("Forum") + " · Salafi Network"
	s.Description = p.Text("Ask questions, share knowledge, and join thoughtful discussions.", "اطرح أسئلتك وشارك المعرفة وانضم إلى نقاشات هادفة.")
	s.Alternates = nil
	if p.Topic.ID > 0 {
		s.Title = p.Topic.Title + " · Salafi Network"
		s.Description = excerpt(p.Topic.Body)
		s.Canonical = publicOrigin + "/community/t/" + strconv.FormatInt(p.Topic.ID, 10)
	} else if p.View != "" {
		s.NoIndex = true
	}
	// Discussion content is shared; the language choice changes its interface.
	for key := range r.URL.Query() {
		if key != "lang" {
			s.NoIndex = true
		}
	}
	return s
}

type sitemapURL struct {
	Loc string `xml:"loc"`
}
type sitemapDocument struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

func (a *App) serveDiscovery(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/robots.txt" && r.URL.Path != "/sitemap.xml" {
		return false
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", 405)
		return true
	}
	if r.URL.Path == "/robots.txt" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if localSEO(r) {
			fmt.Fprint(w, "User-agent: *\nDisallow: /\n")
		} else {
			fmt.Fprint(w, "User-agent: *\nAllow: /\nSitemap: "+publicOrigin+"/sitemap.xml\n")
		}
		return true
	}
	var items []catalogItem
	var err error
	if a.community != nil {
		items, err = a.community.catalogItems()
	} else {
		err = json.Unmarshal(catalogJSON, &items)
	}
	if err != nil {
		http.Error(w, "Unable to load sitemap", 503)
		return true
	}
	doc := sitemapDocument{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	add := func(path string) { doc.URLs = append(doc.URLs, sitemapURL{publicOrigin + path}) }
	for _, lang := range []string{"en", "ar"} {
		for _, path := range []string{"/", "/about", "/articles", "/learn"} {
			add("/" + lang + path)
		}
		for _, item := range items {
			if strings.TrimSpace(item.Title.In(lang)) != "" {
				p := catalogPage{Page: Page{Lang: lang}}
				add(p.URL(item))
			}
		}
	}
	add("/community")
	if a.community != nil {
		rows, e := a.community.db.Query("SELECT id FROM topics WHERE hidden=0 ORDER BY id")
		if e != nil {
			http.Error(w, "Unable to load sitemap", 503)
			return true
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if e = rows.Scan(&id); e != nil {
				http.Error(w, "Unable to load sitemap", 503)
				return true
			}
			add("/community/t/" + strconv.FormatInt(id, 10))
		}
		if rows.Err() != nil {
			http.Error(w, "Unable to load sitemap", 503)
			return true
		}
	}
	data, e := xml.Marshal(doc)
	if e != nil {
		http.Error(w, "Unable to build sitemap", 500)
		return true
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprint(w, xml.Header+string(data))
	return true
}
