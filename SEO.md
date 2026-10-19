# Search visibility

Public origin: https://salafinetwork.com (seo.go). Articles, learning resources, home/about, and public forum discussions are eligible for indexing. Account pages, member profiles, administration, and search/filter results use noindex. Hidden/deleted threads are excluded from the sitemap. Robots must be allowed to crawl pages to read their noindex instruction; robots.txt is not an access-control mechanism.

/sitemap.xml is generated from the current database on every request. Only existing language versions are included. Article descriptions use their summary, or the first body section if the summary is empty. Canonical links keep search/filter and forum interface-language variants from becoming competing URLs. Article microdata uses the real visible title, description and author. No publication dates, ratings, or images are invented. Headers and hero art remain unchanged.

After deployment, verify ownership of salafinetwork.com in Google Search Console, submit https://salafinetwork.com/sitemap.xml, and inspect/request indexing for priority article URLs. This requires the owner's Google account and possibly a DNS verification record. Indexing and rankings are Google's decision; deployment alone does not submit or verify Search Console.

Publish descriptive titles, useful summaries, accurate author attribution and original, helpful content. Add translations only when they exist. Existing article URLs are preserved so shared links continue working.

Localhost pages use noindex and local robots.txt disallows crawling. Changing the public domain requires updating publicOrigin in seo.go.
