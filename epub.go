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
	"runtime"
	"strings"
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
	ContentType    = "application/epub+zip"
	XmlDeclaration = `<?xml version="1.0" encoding="utf-8"?>`

	Container = XmlDeclaration + `
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles>
    <rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

	Nav = XmlDeclaration + `
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="fr" xml:lang="fr">
  <head>
    <title>LinuxFr.org</title>
    <meta charset="utf-8" />
  </head>
  <body>
    <section class="frontmatter TableOfContents" epub:type="frontmatter toc">
      <h1>Sommaire</h1>
      <nav xmlns:epub="http://www.idpf.org/2007/ops" epub:type="toc" id="toc">
        <ol>
          <li><a href="content.html">Aller au contenu</a></li>
        </ol>
      </nav>
    </section>
  </body>
</html>`

	HeaderHtml = XmlDeclaration + `
<html xmlns="http://www.w3.org/1999/xhtml" lang="fr" xml:lang="fr">
  <head>
    <title>LinuxFr.org</title>
    <meta charset="utf-8" />
  </head>
  <body>`

	FooterHtml = "</body></html>"
)

// TODO cover
var PackageTemplate = template.Must(template.New("package").Parse(`
<package xmlns="http://www.idpf.org/2007/opf" unique-identifier="pub-identifier" xml:lang="fr" version="3.0">
	<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
		<dc:language id="pub-language">fr</dc:language>
		<dc:identifier id="pub-identifier">xxx</dc:identifier>
		<dc:date>{{.Date}}</dc:date>
		<meta property="dcterms:modified">{{.Date}}</meta>
		{{if .Title}}<dc:title id="pub-title">{{.Title}}</dc:title>{{end}}
		{{if .Creator}}<dc:creator id="pub-creator">{{.Creator}}</dc:creator>{{end}}
		{{range .Contributors}}<dc:contributor>{{.}}</dc:contributor>
		{{end}}
		<dc:rights>xxx</dc:rights>
	</metadata>
	<manifest>
		<item id="nav" href="nav.html" media-type="application/xhtml+xml" properties="nav"/>
		{{range .Items}}<item id="{{.Id}}" href="{{.Href}}" media-type="{{.Type}}"/>
		{{end}}
	</manifest>
	<spine>
		{{range .Items}}<itemref idref="{{.Id}}"/>
		{{end}}
	</spine>
</package>`))

var Host string

func toHtml(node xml.Node) string {
	// TODO fix links to LinuxFr.org
	// TODO embed CSS & images
	return HeaderHtml + node.InnerHtml() + FooterHtml
}

func NewEpub(w io.Writer) (epub *Epub) {
	z := zip.NewWriter(w)
	epub = &Epub{Zip: z, Items: []Item{}}
	epub.AddMimetype()
	epub.AddFile("META-INF/container.xml", Container)
	epub.AddFile("EPUB/nav.html", Nav)
	return
}

func (epub *Epub) AddContent(article xml.Node) {
	xpath := css.Convert(".content", css.LOCAL)
	nodes, err := article.Search(xpath)
	if err != nil || len(nodes) == 0 {
		return
	}
	html := toHtml(nodes[0])
	filename := "content.html"
	epub.Items = append(epub.Items, Item{"item-content", filename, "application/xhtml+xml"})
	epub.AddFile("EPUB/"+filename, html)
}

func (epub *Epub) AddComments(article xml.Node) {
	list := article.NextSibling()
	xpath := css.Convert(".threads>li", css.LOCAL)
	threads, err := list.Search(xpath)
	if err != nil {
		return
	}
	for _, thread := range threads {
		html := toHtml(thread)
		id := thread.Attr("id")
		filename := id + ".html"
		epub.Items = append(epub.Items, Item{id, filename, "application/xhtml+xml"})
		epub.AddFile("EPUB/"+filename, html)
	}
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
	loc, _ := time.LoadLocation("Europe/Paris")
	date, err := time.ParseInLocation("le 02/01/06 à 15:04", meta, loc)
	if err != nil {
		date = time.Now()
	}
	epub.Date = date.Format(time.RFC3339)
	epub.Creator = epub.FindMeta(article, "header .meta a[rel=\"author\"]")
	epub.Contributors = epub.FindMetas(article, "header .meta .edited_by a")
}

func (epub *Epub) AddMimetype() (err error) {
	header := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	f, err := epub.Zip.CreateHeader(header)
	if err != nil {
		log.Println(err)
		return
	}

	_, err = f.Write([]byte(ContentType))
	if err != nil {
		log.Println(err)
		return
	}

	return
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

	epub.AddFile("EPUB/package.opf", XmlDeclaration+opf.String())
	err = epub.Zip.Close()
	if err != nil {
		log.Println(err)
	}
}

func FetchArticle(uri string) (article xml.Node, err error) {
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

func Content(w http.ResponseWriter, r *http.Request) {
	uri := "http://" + Host + strings.Replace(r.URL.Path, ".epub", "", 1)
	article, err := FetchArticle(uri)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Add("Content-Type", ContentType)

	epub := NewEpub(w)
	epub.FillMeta(article)
	epub.AddContent(article)
	epub.AddComments(article)
	epub.Close()
}

// Returns 200 OK if the server is running (for monitoring)
func Status(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Parse the command-line
	var addr string
	var logs string
	flag.StringVar(&addr, "a", "127.0.0.1:8000", "Bind to this address:port")
	flag.StringVar(&logs, "l", "-", "Use this file for logs")
	flag.StringVar(&Host, "h", "linuxfr.org", "Use this host to fetch pages")
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
	m.Get("/news/:slug.epub", http.HandlerFunc(Content))
	m.Get("/users/:user/journaux/:slug.epub", http.HandlerFunc(Content))
	m.Get("/forums/:forum/posts/:slug.epub", http.HandlerFunc(Content))
	m.Get("/sondages/:slug.epub", http.HandlerFunc(Content))
	m.Get("/suivi/:slug.epub", http.HandlerFunc(Content))
	m.Get("/wiki/:slug.epub", http.HandlerFunc(Content))
	http.Handle("/", m)

	// Start the HTTP server
	log.Printf("Listening on http://%s/\n", addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
