package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type member struct {
	ID    int64
	Name  string
	Admin bool
}
type category struct {
	ID                int64
	Name, Description string
	Count             int
}
type topic struct {
	ID, CategoryID                         int64
	Title, Body, Author, Category, Created string
	Replies                                int
	Locked                                 bool
}
type reply struct {
	ID, ParentID                        int64
	Author, Body, Created, ParentAuthor string
}
type commentNode struct {
	reply
	Children    []*commentNode
	TopicID     int64
	TopicAuthor string
	Locked      bool
}
type communityPage struct {
	Page
	LanguageSwitch                                      string
	AdminTab, EditorMode, ContentLanguage, OriginalSlug string
	AdminSaved                                          bool
	Editor                                              catalogItem
	LibraryTopics                                       []catalogTopic

	CatalogItems                             []catalogItem
	AdminComments                            []profileComment
	Profile                                  memberProfile
	Members                                  []memberProfile
	AdminTopics                              []adminTopic
	ProfileComments                          []profileComment
	Tab                                      string
	TotalTopics, TotalComments, TotalMembers int
	CommentTree                              []*commentNode
	View, CSRF, Error, Query                 string
	User                                     *member
	Categories                               []category
	Topics                                   []topic
	Topic                                    topic
	Replies                                  []reply
	CategoryID                               int64
	ReplyTo                                  reply
}
type requestWindow struct {
	Count int
	Until time.Time
}
type Community struct {
	db     *sql.DB
	tmpl   *template.Template
	mu     sync.Mutex
	limits map[string]requestWindow
	secure bool
}

func openCommunity(path string) (*Community, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	schema := `PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS members(id INTEGER PRIMARY KEY, name TEXT NOT NULL COLLATE NOCASE UNIQUE, password TEXT NOT NULL, admin INTEGER NOT NULL DEFAULT 0);
 CREATE TABLE IF NOT EXISTS sessions(token TEXT PRIMARY KEY, member_id INTEGER NOT NULL REFERENCES members(id), expires INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS categories(id INTEGER PRIMARY KEY,name TEXT NOT NULL COLLATE NOCASE UNIQUE,description TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS topics(id INTEGER PRIMARY KEY,category_id INTEGER NOT NULL REFERENCES categories(id),member_id INTEGER NOT NULL REFERENCES members(id),title TEXT NOT NULL,body TEXT NOT NULL,created TEXT NOT NULL,locked INTEGER NOT NULL DEFAULT 0,hidden INTEGER NOT NULL DEFAULT 0);
 CREATE TABLE IF NOT EXISTS replies(id INTEGER PRIMARY KEY,topic_id INTEGER NOT NULL REFERENCES topics(id),member_id INTEGER NOT NULL REFERENCES members(id),body TEXT NOT NULL,created TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS replies_topic ON replies(topic_id);
 CREATE INDEX IF NOT EXISTS topics_category ON topics(category_id,id);
 INSERT OR IGNORE INTO categories(id,name,description) VALUES(1,'General Discussion','Meet the community, ask questions, and share beneficial ideas.');`
	if _, err = db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	var hasParent int
	if err = db.QueryRow("SELECT count(*) FROM pragma_table_info('replies') WHERE name='parent_id'").Scan(&hasParent); err != nil {
		db.Close()
		return nil, err
	}
	if hasParent == 0 {
		if _, err = db.Exec("ALTER TABLE replies ADD COLUMN parent_id INTEGER REFERENCES replies(id)"); err != nil {
			db.Close()
			return nil, err
		}
	}
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS profiles(member_id INTEGER PRIMARY KEY REFERENCES members(id), display_name TEXT NOT NULL DEFAULT '', bio TEXT NOT NULL DEFAULT '', interests TEXT NOT NULL DEFAULT '', goals TEXT NOT NULL DEFAULT '', joined TEXT NOT NULL DEFAULT '');`); err != nil {
		db.Close()
		return nil, err
	}
	if err = initCatalog(db); err != nil {
		db.Close()
		return nil, err
	}
	tmpl, err := template.New("").Funcs(baseTemplateFuncs()).ParseFS(assets, "templates/community.html", "templates/members.html")
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Community{db: db, tmpl: tmpl, limits: map[string]requestWindow{}, secure: os.Getenv("COMMUNITY_INSECURE_COOKIES") != "1"}, nil
}
func (c *Community) bootstrapAdmin(name, password string) error {
	var count int
	if err := c.db.QueryRow("SELECT count(*) FROM members WHERE admin=1").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if len(password) < 12 || len(password) > 72 {
		return errors.New("admin password must be 12–72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = c.db.Exec("INSERT INTO members(name,password,admin) VALUES(?,?,1)", name, string(hash))
	return err
}
func token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func digest(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func (c *Community) cookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/community", HttpOnly: true, Secure: c.secure, SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func (c *Community) user(r *http.Request) *member {
	cookie, err := r.Cookie("sn_session")
	if err != nil {
		return nil
	}
	u := new(member)
	err = c.db.QueryRow(`SELECT m.id,m.name,m.admin FROM members m JOIN sessions s ON s.member_id=m.id WHERE s.token=? AND s.expires>?`, digest(cookie.Value), time.Now().Unix()).Scan(&u.ID, &u.Name, &u.Admin)
	if err != nil {
		return nil
	}
	return u
}
func (c *Community) allow(r *http.Request) bool {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if net.ParseIP(ip).IsLoopback() {
		if p := net.ParseIP(r.Header.Get("CF-Connecting-IP")); p != nil {
			ip = p.String()
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.limits {
		if now.After(v.Until) {
			delete(c.limits, k)
		}
	}
	key := ip
	v := c.limits[key]
	if v.Until.IsZero() {
		if len(c.limits) > 10000 {
			return false
		}
		v.Until = now.Add(5 * time.Minute)
	}
	v.Count++
	c.limits[key] = v
	return v.Count <= 40
}
func (c *Community) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	lang := preferredLanguage(r)
	rememberLanguage(w, lang)
	p := communityPage{Page: languagePage(lang), LanguageSwitch: languageSwitchURL(r), User: c.user(r), View: strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/"), "/community")}
	if !localAdminRequest(r) {
		if p.View == "/admin" || strings.HasPrefix(p.View, "/admin/") {
			http.Error(w, "Administration is available only on localhost.", http.StatusForbidden)
			return
		}
		if p.User != nil {
			p.User.Admin = false
		}
	}

	if p.View == "/admin" && (r.Method == "GET" || r.Method == "HEAD") {
		if p.User == nil {
			http.Redirect(w, r, "/community/login?next=admin", http.StatusSeeOther)
			return
		}
		if !p.User.Admin {
			http.Error(w, "Administrator account required. You are signed in as a regular member. Return to /community, log out, then sign in as network_admin.", http.StatusForbidden)
			return
		}
	}
	csrf, err := r.Cookie("sn_csrf")
	if err != nil || len(csrf.Value) != 64 {
		p.CSRF = token()
		c.cookie(w, "sn_csrf", p.CSRF, 86400)
	} else {
		p.CSRF = csrf.Value
	}
	if r.Method == http.MethodPost {
		limit := int64(32768)
		if p.View == "/admin" && p.User != nil && p.User.Admin {
			limit = 262144
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		if err = r.ParseForm(); err != nil {
			http.Error(w, "Form is too large", 400)
			return
		}
		if csrf == nil || subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf")), []byte(p.CSRF)) != 1 {
			http.Error(w, forumTranslator(lang)("Please reload the page and try again."), 403)
			return
		}
		if !c.allow(r) {
			http.Error(w, forumTranslator(lang)("Too many requests. Please try again in five minutes."), 429)
			return
		}
		if err = c.post(w, r, &p); err == nil {
			return
		} else {
			p.Error = forumTranslator(lang)(err.Error())
			w.WriteHeader(http.StatusBadRequest)
		}
	} else if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	if err = c.load(r, &p); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
		} else {
			log.Print(err)
			http.Error(w, "Unable to load the community", 500)
		}
		return
	}
	var out bytes.Buffer
	render, cloneErr := c.tmpl.Clone()
	if cloneErr != nil {
		http.Error(w, "Unable to render community", 500)
		return
	}
	render = render.Funcs(template.FuncMap{"tr": forumTranslator(lang)})
	if err = render.ExecuteTemplate(&out, "community.html", p); err != nil {
		log.Print(err)
		http.Error(w, "Unable to render community", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(out.Bytes())
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,24}$`)

func (c *Community) post(w http.ResponseWriter, r *http.Request, p *communityPage) error {
	get := func(k string) string { return strings.TrimSpace(r.PostForm.Get(k)) }
	redirect := "/community"
	if strings.HasPrefix(p.View, "/u/") {
		return c.saveProfile(w, r, p)
	}
	if p.View == "/admin" && get("action") != "" {
		return c.adminAction(w, r, p)
	}
	switch p.View {
	case "/register", "/login":
		name, password := get("username"), r.PostForm.Get("password")
		if strings.EqualFold(name, "deleted_member") || !usernamePattern.MatchString(name) || len(password) < 12 || len(password) > 72 {
			return errors.New("Use a 3–24 character username (letters, numbers, underscore) and a password of 12–72 bytes.")
		}
		var id int64
		var hash string
		if p.View == "/register" {
			h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			result, err := c.db.Exec("INSERT INTO members(name,password) VALUES(?,?)", name, string(h))
			if err != nil {
				return errors.New("That username is unavailable. Choose another.")
			}
			id, _ = result.LastInsertId()
			if _, err := c.db.Exec("INSERT INTO profiles(member_id,joined) VALUES(?,?)", id, time.Now().UTC().Format("January 2006")); err != nil {
				return err
			}
		} else {
			err := c.db.QueryRow("SELECT id,password FROM members WHERE name=?", name).Scan(&id, &hash)
			if err != nil {
				bcrypt.CompareHashAndPassword([]byte("$2a$10$7EqJtq98hPqEX7fNZaFWoO5oaSzzLB.hqCiZMZN.xEiRdqxBhUhNe"), []byte(password))
				return errors.New("Username or password is incorrect.")
			}
			if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
				return errors.New("Username or password is incorrect.")
			}
		}
		t := token()
		_, err := c.db.Exec("INSERT INTO sessions(token,member_id,expires) VALUES(?,?,?)", digest(t), id, time.Now().Add(7*24*time.Hour).Unix())
		if err != nil {
			return err
		}
		c.db.Exec("DELETE FROM sessions WHERE expires<=?", time.Now().Unix())
		c.cookie(w, "sn_session", t, 7*86400)
		c.cookie(w, "sn_csrf", token(), 86400)
		if r.URL.Query().Get("next") == "admin" && localAdminRequest(r) {
			redirect = "/community/admin"
		}
	case "/logout":
		if s, err := r.Cookie("sn_session"); err == nil {
			c.db.Exec("DELETE FROM sessions WHERE token=?", digest(s.Value))
		}
		c.cookie(w, "sn_session", "", -1)
	case "/new":
		if p.User == nil {
			return errors.New("Please sign in to start a topic.")
		}
		title, body := get("title"), get("body")
		cat, _ := strconv.ParseInt(get("category"), 10, 64)
		if len([]rune(title)) < 5 || len([]rune(title)) > 160 || len([]rune(body)) < 5 || len(body) > 20000 {
			return errors.New("Add a title of 5–160 characters and a message of 5–20,000 bytes.")
		}
		result, err := c.db.Exec("INSERT INTO topics(category_id,member_id,title,body,created) VALUES(?,?,?,?,?)", cat, p.User.ID, title, body, time.Now().UTC().Format("02 Jan 2006 · 15:04 UTC"))
		if err != nil {
			return errors.New("Please select a valid category.")
		}
		id, _ := result.LastInsertId()
		redirect = fmt.Sprintf("/community/t/%d", id)
	case "/admin":
		if p.User == nil || !p.User.Admin {
			return errors.New("Only an administrator can manage categories.")
		}
		name, description := get("name"), get("description")
		if len([]rune(name)) < 1 || len([]rune(name)) > 60 || len(description) > 500 {
			return errors.New("Enter a category name (up to 60 characters) and a shorter description.")
		}
		if _, err := c.db.Exec("INSERT INTO categories(name,description) VALUES(?,?)", name, description); err != nil {
			return errors.New("That category already exists.")
		}
		redirect = "/community/admin?tab=categories&saved=1"
	default:
		if !strings.HasPrefix(p.View, "/t/") {
			return errors.New("Unknown action.")
		}
		if p.User == nil {
			return errors.New("Please sign in to reply.")
		}
		id, _ := strconv.ParseInt(strings.TrimPrefix(p.View, "/t/"), 10, 64)
		if action := get("action"); action == "hide" || action == "lock" {
			if !p.User.Admin {
				return errors.New("Only an administrator can moderate topics.")
			}
			if action == "hide" {
				_, err := c.db.Exec("UPDATE topics SET hidden=1 WHERE id=?", id)
				if err != nil {
					return err
				}
			} else {
				_, err := c.db.Exec("UPDATE topics SET locked=1-locked WHERE id=? AND hidden=0", id)
				if err != nil {
					return err
				}
				redirect = r.URL.Path
			}
		} else {
			body := get("body")
			if len([]rune(body)) < 2 || len(body) > 20000 {
				return errors.New("Replies must contain 2–20,000 bytes.")
			}
			var parent any
			if raw := get("parent_id"); raw != "" {
				parentID, parseErr := strconv.ParseInt(raw, 10, 64)
				var exists int
				if parseErr != nil || parentID <= 0 {
					return errors.New("Choose a valid comment to reply to.")
				}
				if err := c.db.QueryRow("SELECT count(*) FROM replies WHERE id=? AND topic_id=?", parentID, id).Scan(&exists); err != nil || exists != 1 {
					return errors.New("That comment is not part of this topic.")
				}
				parent = parentID
			}
			result, err := c.db.Exec("INSERT INTO replies(topic_id,member_id,body,created,parent_id) SELECT id,?,?,?,? FROM topics WHERE id=? AND locked=0 AND hidden=0", p.User.ID, body, time.Now().UTC().Format("02 Jan 2006 · 15:04 UTC"), parent, id)
			if err != nil {
				return err
			}
			n, _ := result.RowsAffected()
			if n == 0 {
				return errors.New("This topic is closed or unavailable.")
			}
			replyID, _ := result.LastInsertId()
			redirect = fmt.Sprintf("%s#reply-%d", r.URL.Path, replyID)
		}
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}
func (c *Community) load(r *http.Request, p *communityPage) error {
	if err := c.db.QueryRow(`SELECT (SELECT count(*) FROM topics WHERE hidden=0), (SELECT count(*) FROM replies r JOIN topics t ON t.id=r.topic_id WHERE t.hidden=0), (SELECT count(*) FROM members WHERE name != 'deleted_member')`).Scan(&p.TotalTopics, &p.TotalComments, &p.TotalMembers); err != nil {
		return err
	}
	rows, err := c.db.Query("SELECT c.id,c.name,c.description,(SELECT count(*) FROM topics t WHERE t.category_id=c.id AND t.hidden=0) FROM categories c ORDER BY c.id")
	if err != nil {
		return err
	}
	for rows.Next() {
		var cat category
		if err = rows.Scan(&cat.ID, &cat.Name, &cat.Description, &cat.Count); err != nil {
			rows.Close()
			return err
		}
		p.Categories = append(p.Categories, cat)
	}
	rows.Close()
	switch {
	case strings.HasPrefix(p.View, "/u/"):
		return c.loadProfile(r, p)
	case p.View == "/admin":
		return c.loadAdmin(r, p)
	case p.View == "":
		if p.View == "/admin" && (p.User == nil || !p.User.Admin) {
			return sql.ErrNoRows
		}
		p.Query = strings.TrimSpace(r.URL.Query().Get("q"))
		if len(p.Query) > 150 {
			p.Query = p.Query[:150]
		}
		p.CategoryID, _ = strconv.ParseInt(r.URL.Query().Get("category"), 10, 64)
		rows, err = c.db.Query(`SELECT t.id,t.title,m.name,c.name,t.created,(SELECT count(*) FROM replies WHERE topic_id=t.id),t.locked FROM topics t JOIN members m ON m.id=t.member_id JOIN categories c ON c.id=t.category_id WHERE t.hidden=0 AND (?=0 OR t.category_id=?) AND (?='' OR instr(lower(t.title),lower(?))>0) ORDER BY t.id DESC LIMIT 100`, p.CategoryID, p.CategoryID, p.Query, p.Query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t topic
			if err = rows.Scan(&t.ID, &t.Title, &t.Author, &t.Category, &t.Created, &t.Replies, &t.Locked); err != nil {
				return err
			}
			p.Topics = append(p.Topics, t)
		}
		return rows.Err()
	case strings.HasPrefix(p.View, "/t/"):
		id, _ := strconv.ParseInt(strings.TrimPrefix(p.View, "/t/"), 10, 64)
		err = c.db.QueryRow(`SELECT t.id,t.title,t.body,m.name,c.name,t.created,t.locked FROM topics t JOIN members m ON m.id=t.member_id JOIN categories c ON c.id=t.category_id WHERE t.id=? AND t.hidden=0`, id).Scan(&p.Topic.ID, &p.Topic.Title, &p.Topic.Body, &p.Topic.Author, &p.Topic.Category, &p.Topic.Created, &p.Topic.Locked)
		if err != nil {
			return err
		}
		rows, err = c.db.Query("SELECT r.id,m.name,r.body,r.created,COALESCE(r.parent_id,0),COALESCE(pm.name,'') FROM replies r JOIN members m ON m.id=r.member_id LEFT JOIN replies parent ON parent.id=r.parent_id LEFT JOIN members pm ON pm.id=parent.member_id WHERE r.topic_id=? ORDER BY r.id LIMIT 500", id)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var reply reply
			if err = rows.Scan(&reply.ID, &reply.Author, &reply.Body, &reply.Created, &reply.ParentID, &reply.ParentAuthor); err != nil {
				return err
			}
			p.Replies = append(p.Replies, reply)
			if strconv.FormatInt(reply.ID, 10) == r.URL.Query().Get("reply_to") {
				p.ReplyTo = reply
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		nodes := make(map[int64]*commentNode)
		for _, item := range p.Replies {
			node := &commentNode{reply: item, TopicID: p.Topic.ID, TopicAuthor: p.Topic.Author, Locked: p.Topic.Locked}
			nodes[item.ID] = node
			if parent := nodes[item.ParentID]; parent != nil {
				parent.Children = append(parent.Children, node)
			} else {
				p.CommentTree = append(p.CommentTree, node)
			}
		}
		return nil
	case p.View == "/login", p.View == "/register", p.View == "/new":
		return nil
	default:
		return sql.ErrNoRows
	}
}

// Require a direct loopback connection and a localhost Host. Proxy headers
// disqualify requests even when a local tunnel is the connecting peer.
func localAdminRequest(r *http.Request) bool {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || !net.ParseIP(peer).IsLoopback() {
		return false
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return false
	}
	for key := range r.Header {
		k := strings.ToLower(key)
		if k == "forwarded" || k == "x-real-ip" || strings.HasPrefix(k, "x-forwarded-") || strings.HasPrefix(k, "cf-") {
			return false
		}
	}
	return true
}
