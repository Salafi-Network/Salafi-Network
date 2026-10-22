package main

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

func defaultCatalogTopics() []catalogTopic {
	return []catalogTopic{
		{Slug: "aqeedah", Icon: "shield-check", Label: localizedText{"Aqeedah", "العقيدة"}},
		{Slug: "quran", Icon: "book-open", Label: localizedText{"Quran", "القرآن"}},
		{Slug: "study", Icon: "book-open", Label: localizedText{"Students & Learning", "الطلاب والتعلّم"}},
		{Slug: "hadith", Icon: "book-open", Label: localizedText{"Hadith", "الحديث"}},
		{Slug: "fiqh", Icon: "book-open", Label: localizedText{"Fiqh", "الفقه"}},
		{Slug: "manners", Icon: "users", Label: localizedText{"Manners & Character", "الآداب والأخلاق"}},
		{Slug: "resources", Icon: "folder", Label: localizedText{"Resources & References", "المصادر والمراجع"}},
	}
}
func (c *Community) libraryTopics() ([]catalogTopic, error) {
	rows, e := c.db.Query("SELECT slug,en,ar FROM library_topics ORDER BY rowid")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []catalogTopic
	for rows.Next() {
		var t catalogTopic
		t.Icon = "book-open"
		if e = rows.Scan(&t.Slug, &t.Label.EN, &t.Label.AR); e != nil {
			return nil, e
		}
		if t.Label.EN == "" {
			t.Label.EN = t.Label.AR
		}
		if t.Label.AR == "" {
			t.Label.AR = t.Label.EN
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func automaticSlug(title, kind string) string {
	base := strings.Trim(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(title), "-"), "-")
	if len(base) > 50 {
		base = base[:50]
	}
	if base == "" {
		base = kind
	}
	return base + "-" + token()[:12]
}
func adminTab(s string) string {
	switch s {
	case "learning", "categories", "members", "discussions", "comments":
		return s
	}
	return "articles"
}
func (i catalogItem) AdminTitle() string {
	if i.Title.EN != "" {
		return i.Title.EN
	}
	return i.Title.AR
}
func (i catalogItem) Languages() string {
	if i.Title.EN != "" && i.Title.AR != "" {
		return "English + العربية"
	}
	if i.Title.AR != "" {
		return "العربية"
	}
	return "English"
}
func (c *Community) prepareEditor(r *http.Request, p *communityPage) error {
	p.AdminTab = adminTab(r.FormValue("tab"))
	p.AdminSaved = r.URL.Query().Get("saved") == "1"
	var err error
	p.LibraryTopics, err = c.libraryTopics()
	if err != nil {
		return err
	}
	if p.Error == "" {
		p.EditorMode = r.URL.Query().Get("new")
		if p.EditorMode != "" {
			p.Editor = catalogItem{Kind: "article", Topic: "aqeedah"}
			if p.AdminTab == "learning" {
				p.Editor.Kind = "book"
				switch r.URL.Query().Get("kind") {
				case "path", "scholar":
					p.Editor.Kind = r.URL.Query().Get("kind")
				}
			}
			p.ContentLanguage = "en"
		}
		if slug := r.URL.Query().Get("edit"); slug != "" {
			var raw string
			if err = c.db.QueryRow("SELECT payload FROM library WHERE slug=?", slug).Scan(&raw); err != nil {
				return err
			}
			if err = json.Unmarshal([]byte(raw), &p.Editor); err != nil {
				return err
			}
			p.OriginalSlug = slug
			p.EditorMode = "edit"
			p.ContentLanguage = "en"
			if p.Editor.Title.AR != "" {
				p.ContentLanguage = "ar"
			}
			if p.Editor.Title.EN != "" && p.Editor.Title.AR != "" {
				p.ContentLanguage = "both"
			}
		}
	}
	if p.EditorMode != "" {
		if len(p.Editor.Sections) == 0 {
			p.Editor.Sections = []catalogSection{{}}
		}
		if len(p.Editor.Steps) == 0 {
			p.Editor.Steps = []catalogStep{{}}
		}
	}
	return nil
}

type adminLearningGroup struct {
	Kind, Title, Description, Icon, CSRF, AdminTab string
	CatalogItems                                   []catalogItem
}

func (p communityPage) LearningGroups() []adminLearningGroup {
	groups := []adminLearningGroup{
		{Kind: "book", Title: "Books", Description: "Manage books and reading resources.", Icon: "book-open"},
		{Kind: "scholar", Title: "Scholars", Description: "Manage scholar profiles and their linked articles.", Icon: "users"},
		{Kind: "path", Title: "Learning paths", Description: "Organize resources into a study order and set their difficulty.", Icon: "folder"},
	}
	for i := range groups {
		groups[i].CSRF, groups[i].AdminTab = p.CSRF, "learning"
		for _, item := range p.CatalogItems {
			if item.Kind == groups[i].Kind {
				groups[i].CatalogItems = append(groups[i].CatalogItems, item)
			}
		}
	}
	return groups
}
