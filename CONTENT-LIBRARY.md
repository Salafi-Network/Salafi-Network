# Articles and learning

Local URLs: `/en/articles`, `/en/learn`, and their `/ar/` equivalents.

Sign in as `network_admin` at `http://localhost:8095/community/admin` for Windows development. The dashboard has separate Articles, Learning, Categories, Members, Discussions and Comments pages.

To publish: choose Articles → Add article (or Learning → Add learning resource), choose a writing language and category, enter a title and text, and press Publish. Page addresses are generated automatically; short and Arabic titles work. Optional descriptions, section headings, authors and external sources can be left blank. Source links must use HTTPS.

Choose English, Arabic or both. Only translated entries appear in each public language library: Arabic-only entries do not appear in English, and vice versa. Edit an entry and choose both languages to add a translation later. Changing the writing language switches visible fields and does not erase an existing translation. Learning paths collect existing articles, books and scholars in a chosen order. Select resources, add optional introductions or notes, and use Move up or Move down to arrange them. Aqeedah and Quran are included as categories; add more under Categories. Forum categories and library categories have separate forms.

Use the content list to search, edit or delete entries. Delete opens an explicit confirmation. Failed validation keeps your submitted article text. Publishing and deletion take effect immediately without a restart. Only locally connected administrators can manage content; session and CSRF checks are required. Text is escaped on display.

With `COMMUNITY_DB` enabled, the library is stored in the SQLite database alongside the community. Include this database in backups. `content/catalog.json` seeds the library once; deleted entries do not reappear on restart. A server without a database uses the read-only embedded catalog.

The Members section deletes a regular member's account, profile and sessions, while preserving their posts under `deleted_member`. Administrator accounts cannot be deleted. Deleting a topic removes all its comments; deleting a comment preserves its child replies. Each deletion requires its confirmation checkbox. Content deletion clears links to that entry in learning paths, preserving their notes.

Starter content: four original study-habit guides, two practical reading paths,
two book references, and two official scholar website references. These are not
the sample articles or usage statistics in the design. Book graphics are
decorative covers, not representations of a particular printed edition.

Directory sources:
- https://sunnah.com/riyadussalihin
- https://sunnah.com/bulugh
- https://binbaz.org.sa/
- https://binothaimeen.net/

Search, filters, and sorting run on the server. Learning paths display ordered resource cards without completion checklists. The language choice persists across the site, including the Arabic forum.

Run `go test ./...` to check localized routes, resource links, filters, escaped
search input, unknown routes, and existing forum functionality.

## Illustration

Both sections use the existing simple illustration in `static/forum-skyline.svg`.


Article authors can link to a scholar profile using the Author section in the editor. Choose an existing scholar from Learning; the author name opens that profile and the profile lists associated articles in the current language. Keep the author name as plain text when no profile is needed. Deleting a scholar clears the link while preserving the attribution and articles.
