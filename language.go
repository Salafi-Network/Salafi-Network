package main

import (
	"html/template"
	"net/http"
	"strings"
)

// A root-scoped preference connects the unprefixed forum to localized pages.
func preferredLanguage(r *http.Request) string {
	if lang := r.URL.Query().Get("lang"); lang == "ar" || lang == "en" {
		return lang
	}
	if c, err := r.Cookie("sn_language"); err == nil && c.Value == "ar" {
		return "ar"
	}
	return "en"
}
func rememberLanguage(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{Name: "sn_language", Value: lang, Path: "/", MaxAge: 31536000, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	w.Header().Add("Vary", "Cookie")
}
func languagePage(lang string) Page {
	p := Page{Lang: lang, Dir: "ltr", Arabic: lang == "ar"}
	if p.Arabic {
		p.Dir = "rtl"
	}
	return p
}
func (p communityPage) SwitchLanguage() string {
	lang := "ar"
	if p.Arabic {
		lang = "en"
	}
	return p.LanguageSwitch + lang
}
func forumTranslator(lang string) func(string) string {
	return func(s string) string {
		if lang == "ar" {
			if value, ok := forumArabic[s]; ok {
				return value
			}
		}
		return s
	}
}
func baseTemplateFuncs() template.FuncMap { return template.FuncMap{"tr": forumTranslator("en")} }
func languageSwitchURL(r *http.Request) string {
	q := r.URL.Query()
	q.Del("lang")
	base := r.URL.Path + "?"
	if len(q) > 0 {
		base += q.Encode() + "&"
	}
	return base + "lang="
}
func localizedResourceHref(href, lang string) string {
	if strings.HasPrefix(href, "/articles/") || strings.HasPrefix(href, "/learn/") {
		return "/" + lang + href
	}
	return ""
}
