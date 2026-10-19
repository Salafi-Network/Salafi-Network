package main

import "strings"

func (p *catalogPage) resolveAuthors(items []catalogItem) {
	p.ScholarIndex = make(map[string]catalogItem)
	for _, item := range items {
		if item.Kind == "scholar" {
			p.ScholarIndex[item.Slug] = item
		}
		if p.Selected.Kind == "scholar" && item.Kind == "article" && item.AuthorScholar == p.Selected.Slug && strings.TrimSpace(item.Title.In(p.Lang)) != "" {
			p.ScholarArticles = append(p.ScholarArticles, item)
		}
	}
}
func (p catalogPage) AuthorURL(item catalogItem) string {
	scholar, ok := p.ScholarIndex[item.AuthorScholar]
	if !ok {
		return ""
	}
	lang := p.Lang
	if strings.TrimSpace(scholar.Title.In(lang)) == "" {
		if lang == "ar" {
			lang = "en"
		} else {
			lang = "ar"
		}
	}
	if strings.TrimSpace(scholar.Title.In(lang)) == "" {
		return ""
	}
	return "/" + lang + "/learn/" + scholar.Slug
}
func (p catalogPage) AuthorName(item catalogItem) string {
	if item.Author != "" {
		return item.Author
	}
	if scholar, ok := p.ScholarIndex[item.AuthorScholar]; ok {
		if name := scholar.Title.In(p.Lang); name != "" {
			return name
		}
		return scholar.AdminTitle()
	}
	return ""
}
