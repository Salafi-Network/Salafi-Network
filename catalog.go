package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

//go:embed content/catalog.json
var catalogJSON []byte

type localizedText struct {
	EN string `json:"en"`
	AR string `json:"ar"`
}

func (t localizedText) In(lang string) string {
	if lang == "ar" {
		return t.AR
	}
	return t.EN
}

type catalogSection struct {
	Heading localizedText
	Body    localizedText
}
type catalogStep struct {
	Title localizedText
	Body  localizedText
	Href  string
}
type catalogItem struct {
	AuthorScholar                            string
	Difficulty                               string
	Slug, Kind, Topic, Image, Author, Source string
	Title, Summary                           localizedText
	MetaDescription                          localizedText
	Featured, Beginner                       bool
	Sections                                 []catalogSection
	Steps                                    []catalogStep
}
type catalogTopic struct {
	Slug, Icon string
	Label      localizedText
	Count      int
}
type catalogPage struct {
	ScholarIndex    map[string]catalogItem
	ScholarArticles []catalogItem
	PathResources   []pathResource
	Page
	User                                     *member
	Query, TopicFilter, Format, Filter, Sort string
	Items, Books, Scholars, Paths, Recent    []catalogItem
	Topics                                   []catalogTopic
	Selected, Featured                       catalogItem
	Total                                    int
}

func (p catalogPage) KindLabel(k string) string {
	switch k {
	case "article":
		return p.Text("Article", "مقالة")
	case "book":
		return p.Text("Book", "كتاب")
	case "scholar":
		return p.Text("Scholar", "عالم")
	case "path":
		return p.Text("Learning path", "مسار تعلّم")
	}
	return ""
}
func (p catalogPage) TopicName(slug string) string {
	for _, t := range p.Topics {
		if t.Slug == slug {
			return t.Label.In(p.Lang)
		}
	}
	return slug
}
func (p catalogPage) URL(item catalogItem) string {
	section := "learn"
	if item.Kind == "article" {
		section = "articles"
	}
	return "/" + p.Lang + "/" + section + "/" + item.Slug
}
func (p catalogPage) Base() string { return "/" + p.Lang + "/" + p.Section }
func (p catalogPage) ReadTime(item catalogItem) int {
	words := len(strings.Fields(item.Summary.In(p.Lang)))
	for _, s := range item.Sections {
		words += len(strings.Fields(s.Body.In(p.Lang)))
	}
	return words/180 + 1
}
func (a *App) serveCatalog(w http.ResponseWriter, r *http.Request, page Page, parts []string) {
	p := catalogPage{Page: page, Query: strings.TrimSpace(r.URL.Query().Get("q")), TopicFilter: r.URL.Query().Get("topic"), Format: r.URL.Query().Get("format"), Filter: r.URL.Query().Get("filter"), Sort: r.URL.Query().Get("sort")}
	if a.community != nil {
		p.User = a.community.user(r)
	}
	p.Title = p.Text("Articles & reading", "المقالات والقراءة")
	if p.Section == "learn" {
		p.Title = p.Text("Learning", "التعلّم")
	}
	if len(p.Query) > 150 {
		p.Query = p.Query[:150]
	}
	p.Topics = defaultCatalogTopics()
	if a.community != nil {
		var err error
		p.Topics, err = a.community.libraryTopics()
		if err != nil {
			http.Error(w, "Unable to load topics", 500)
			return
		}
	}
	var all []catalogItem
	var err error
	if a.community != nil {
		all, err = a.community.catalogItems()
	} else {
		err = json.Unmarshal(catalogJSON, &all)
	}
	if err != nil {
		http.Error(w, "Unable to load the library", 500)
		return
	}
	for _, item := range all {
		belongs := (p.Section == "articles" && item.Kind == "article") || (p.Section == "learn" && item.Kind != "article")
		if !belongs || strings.TrimSpace(item.Title.In(p.Lang)) == "" {
			continue
		}
		p.Total++
		for i := range p.Topics {
			if p.Topics[i].Slug == item.Topic {
				p.Topics[i].Count++
			}
		}
		if len(parts) == 3 && parts[2] == item.Slug {
			p.Selected = item
			p.Title = item.Title.In(p.Lang)
		}
		if item.Featured && p.Featured.Slug == "" {
			p.Featured = item
		}
		if len(p.Recent) < 3 {
			p.Recent = append(p.Recent, item)
		}
		haystack := strings.ToLower(item.Title.EN + " " + item.Title.AR + " " + item.Summary.EN + " " + item.Summary.AR + " " + item.Author + " " + p.TopicName(item.Topic) + " " + item.Topic)
		if p.Query != "" && !strings.Contains(haystack, strings.ToLower(p.Query)) {
			continue
		}
		if p.TopicFilter != "" && p.TopicFilter != item.Topic {
			continue
		}
		if p.Format != "" && p.Format != item.Kind {
			continue
		}
		if p.Filter == "featured" && !item.Featured {
			continue
		}
		if p.Filter == "beginner" && !item.Beginner {
			continue
		}
		p.Items = append(p.Items, item)
	}
	if len(parts) > 3 || (len(parts) == 3 && p.Selected.Slug == "") {
		http.NotFound(w, r)
		return
	}
	if p.Sort == "title" {
		sort.SliceStable(p.Items, func(i, j int) bool { return p.Items[i].Title.In(p.Lang) < p.Items[j].Title.In(p.Lang) })
	}
	for _, item := range p.Items {
		switch item.Kind {
		case "book":
			p.Books = append(p.Books, item)
		case "scholar":
			p.Scholars = append(p.Scholars, item)
		case "path":
			p.Paths = append(p.Paths, item)
		}
	}
	p.resolveAuthors(all)
	p.resolvePath(all)
	p.SEO = catalogSEO(p, r)
	var b bytes.Buffer
	if err := a.tmpl.ExecuteTemplate(&b, "catalog.html", p); err != nil {
		http.Error(w, "Unable to render the library", 500)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b.Bytes())
}

// Old paths were displayed as Beginner before difficulty was configurable.
func (item catalogItem) PathDifficulty() string {
	if item.Difficulty == "" {
		return "beginner"
	}
	return item.Difficulty
}
func (item catalogItem) DifficultyLabel(lang string) string {
	switch item.PathDifficulty() {
	case "intermediate":
		return (localizedText{"Intermediate", "متوسط"}).In(lang)
	case "hard":
		return (localizedText{"Hard", "صعب"}).In(lang)
	case "expert":
		return (localizedText{"Expert", "خبير"}).In(lang)
	default:
		return (localizedText{"Beginner", "مبتدئ"}).In(lang)
	}
}
