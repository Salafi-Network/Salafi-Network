package main

import "strings"

type pathResource struct {
	Number                                            int
	Title, Summary, Note, Href, Kind, Image, Language string
}

func (p catalogPage) resourceLabel(kind string) string {
	if kind == "introduction" {
		return p.Text("Introduction", "مقدمة")
	}
	return p.KindLabel(kind)
}
func (p catalogPage) ResourceLabel(kind string) string { return p.resourceLabel(kind) }
func (p *catalogPage) resolvePath(all []catalogItem) {
	if p.Selected.Kind != "path" {
		return
	}
	byLink := map[string]catalogItem{}
	for _, item := range all {
		section := "learn"
		if item.Kind == "article" {
			section = "articles"
		}
		byLink["/"+section+"/"+item.Slug] = item
	}
	for _, entry := range p.Selected.Steps {
		x := pathResource{Title: entry.Title.In(p.Lang), Note: entry.Body.In(p.Lang), Kind: "introduction"}
		if linked, ok := byLink[entry.Href]; ok && linked.Kind != "path" {
			lang := p.Lang
			if strings.TrimSpace(linked.Title.In(lang)) == "" {
				continue
			}
			x.Title = linked.Title.In(lang)
			x.Summary = linked.Summary.In(lang)
			x.Kind = linked.Kind
			x.Image = linked.Image
			x.Href = localizedResourceHref(entry.Href, lang)
			if lang != p.Lang {
				if lang == "ar" {
					x.Language = "العربية"
				} else {
					x.Language = "English"
				}
			}
		}
		if strings.TrimSpace(x.Title+x.Note) == "" {
			continue
		}
		x.Number = len(p.PathResources) + 1
		p.PathResources = append(p.PathResources, x)
	}
}
