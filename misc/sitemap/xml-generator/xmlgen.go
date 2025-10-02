package xmlgen

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sitemap-generator/dynamo"
)

type UrlSet struct {
	XMLName xml.Name `xml:"urlset"`
	Xmlns   string   `xml:"xmlns,attr"`
	Xhtml   string   `xml:"xmlns:xhtml,attr"`
	Urls    []Url    `xml:"url"`
}

type Url struct {
	Loc   string `xml:"loc"`
	Links []Link `xml:"xhtml:link"`
}

type Link struct {
	Rel      string `xml:"rel,attr"`
	Hreflang string `xml:"hreflang,attr"`
	Href     string `xml:"href,attr"`
}


func BuildSitemap(metas []dynamo.Metadata, lang string) (string, error) {
	urlset := UrlSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		Xhtml: "http://www.w3.org/1999/xhtml",
	}

	for _, m := range metas {
		// Skip if no path for this lang
		path, ok := m.Path[lang]
		if !ok {
			continue
		}

		loc := fmt.Sprintf("https://sumtube.io/%s/%s/%s", lang, m.Vid, path)

		var links []Link
		for altLang := range m.LanguagesFound {
			href := fmt.Sprintf("https://sumtube.io/%s/%s", altLang, m.Vid)
			links = append(links, Link{
				Rel:      "alternate",
				Hreflang: altLang,
				Href:     href,
			})
		}

		urlset.Urls = append(urlset.Urls, Url{
			Loc:   loc,
			Links: links,
		})
	}

	// Use Encoder to avoid self-closing tags
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(urlset); err != nil {
		return "", err
	}
	// Ensure all tokens flushed
	enc.Flush()

	return buf.String(), nil
}
