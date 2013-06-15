package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"github.com/bmizerany/pat"
	"github.com/moovweb/gokogiri"
	"github.com/moovweb/gokogiri/css"
	"github.com/moovweb/gokogiri/xml"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"syscall"
	"time"
)

type Item struct {
	Id   string
	Href string
	Type string
}

type Epub struct {
	Zip *zip.Writer
	// TODO identifier / rights
	Title        string
	Subject      string
	Date         string
	Creator      string
	Contributors []string
	Items        []Item
}

const (
	Host        = "linuxfr.org"
	ContentType = "application/epub+zip"
	Container   = `<?xml version="1.0" encoding="utf-8"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles>
    <rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`
)

var PackageTemplate = template.Must(template.New("package").Parse(
	`<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" unique-identifier="bookid" xml:lang="fr">
	<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
		<dc:language id="pub-language">fr</dc:language>
		<dc:identifier id="pub-identifier">xxx</dc:identifier>
		{{if .Title}}<dc:title id="pub-title">{{.Title}}</dc:title>{{end}}
		{{if .Date}}<dc:date>{{.Date}}</dc:date>{{end}}
		{{if .Creator}}<dc:creator id="pub-creator">{{.Creator}}</dc:creator>{{end}}
		{{range .Contributors}}<dc:contributor>{{.}}</dc:contributor>{{end}}
		<dc:rights>xxx</dc:rights>
	</metadata>
	<manifest>
		{{range .Items}}<item id="{{.Id}}" href="{{.Href}}" media-type="{{.Type}}"/>{{end}}
	</manifest>
	<spine>
		{{range .Items}}<itemref idref="{{.Id}}"/>{{end}}
	</spine>
</package>`))

func NewEpub(w io.Writer) (epub *Epub) {
	z := zip.NewWriter(w)
	epub = &Epub{Zip: z}
	epub.AddFile("mimetype", ContentType)
	epub.AddFile("META-INF/container.xml", Container)
	return
}

func (epub *Epub) FindMeta(article xml.Node, selector string) string {
	xpath := css.Convert(selector, css.LOCAL)
	nodes, err := article.Search(xpath)
	if err != nil || len(nodes) == 0 {
		return ""
	}
	return nodes[0].Content()
}

func (epub *Epub) FindMetas(article xml.Node, selector string) []string {
	xpath := css.Convert(selector, css.LOCAL)
	nodes, err := article.Search(xpath)
	if err != nil {
		return nil
	}
	metas := make([]string, len(nodes))
	for i, node := range nodes {
		metas[i] = node.Content()
	}
	return metas
}

func (epub *Epub) FillMeta(article xml.Node) {
	epub.Title = epub.FindMeta(article, "header h1 a:last-child")
	epub.Subject = epub.FindMeta(article, "header h1 a.topic")
	meta := epub.FindMeta(article, "header time.updated")
	// FIXME ParseInLocation
	date, err := time.Parse("le 02/01/06 à 15:04", meta)
	if err == nil {
		epub.Date = date.String()
	}
	epub.Creator = epub.FindMeta(article, "header .meta a[rel=\"author\"]")
	epub.Contributors = epub.FindMetas(article, "header .meta .edited_by a")
}

func (epub *Epub) AddFile(filename, content string) (err error) {
	f, err := epub.Zip.Create(filename)
	if err != nil {
		log.Println(err)
		return
	}

	_, err = f.Write([]byte(content))
	if err != nil {
		log.Println(err)
		return
	}

	return
}

func (epub *Epub) Close() {
	var opf bytes.Buffer
	err := PackageTemplate.Execute(&opf, epub)
	if err != nil {
		log.Println(err)
		return
	}

	epub.AddFile("EPUB/package.opf", opf.String())
	err = epub.Zip.Close()
	if err != nil {
		log.Println(err)
	}
}

func fetchArticle(uri string) (article xml.Node, err error) {
	resp, err := http.Get(uri)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error on ioutil.ReadAll for %s: %s\n", uri, err)
		return
	}

	doc, err := gokogiri.ParseHtml(body)
	if err != nil {
		log.Printf("Gokogiri error: %s\n", err)
		return
	}

	xpath := css.Convert("#contents article", css.LOCAL)
	articles, err := doc.Root().Search(xpath)
	if err != nil {
		log.Printf("Gokogiri error: %s\n", err)
		return
	}

	if len(articles) == 0 {
		err = errors.New("No article found in the page")
		return
	}

	article = articles[0]
	return
}

// Create an epub for a news
func News(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", ContentType)

	slug := r.URL.Query().Get(":slug")
	uri := fmt.Sprintf("http://%s/news/%s", Host, slug)
	article, err := fetchArticle(uri)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// 	buffer := make([]byte, 4096)
	// 	buffer, _ = doc.Root().ToHtml(xml.DefaultEncodingBytes, buffer)

	epub := NewEpub(w)
	epub.FillMeta(article)
	epub.Close()
}

// Returns 200 OK if the server is running (for monitoring)
func Status(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func main() {
	// Parse the command-line
	var addr string
	var logs string
	flag.StringVar(&addr, "a", "127.0.0.1:8000", "Bind to this address:port")
	flag.StringVar(&logs, "l", "-", "Use this file for logs")
	flag.Parse()

	// Logging
	if logs != "-" {
		f, err := os.OpenFile(logs, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			log.Fatal("OpenFile: ", err)
		}
		syscall.Dup2(int(f.Fd()), int(os.Stdout.Fd()))
		syscall.Dup2(int(f.Fd()), int(os.Stderr.Fd()))
	}

	// Routing
	m := pat.New()
	m.Get("/status", http.HandlerFunc(Status))
	m.Get("/news/:slug.epub", http.HandlerFunc(News))
	http.Handle("/", m)

	// Start the HTTP server
	log.Printf("Listening on http://%s/\n", addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
