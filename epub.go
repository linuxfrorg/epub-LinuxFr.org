package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/bmizerany/pat"
	"github.com/jbowtie/gokogiri"
	"github.com/jbowtie/gokogiri/css"
	"github.com/jbowtie/gokogiri/xml"
)

type Item struct {
	Id    string
	Href  string
	Type  string
	Spine bool
}

type Image struct {
	Filename string
	Mimetype string
	Content  []byte
}

type Epub struct {
	Zip          *zip.Writer
	Identifier   string
	Title        string
	Subject      string
	Date         string
	Cover        string
	Creator      string
	Contributors []string
	Items        []Item
	Images       []string
	ChanImages   chan *Image
}

const (
	// The maximal size for an image is 5MiB
	maxSize = 5 * (1 << 20)

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
          <li><a href="content.xhtml">Aller au contenu</a></li>
        </ol>
      </nav>
    </section>
  </body>
</html>`

	Stylesheet = `
body { font-family: sans-serif; }
img { display: block; margin: 0 auto; max-width: 100%; border: none; }
blockquote { border-left: 3px solid #4C575F; padding-left: 5px; margin: 10px 0 10px 10px; }
code { white-space: pre-wrap; border: 1px solid #E9E6E4; border-radius: 4px; padding: 1px 4px; }
pre code { display: block; border-width: 0 0 0 3px; border-color: #4C575F; }
article, ul.threads > li.comment {
	display: block;
	padding: 10px;
	border-radius: 6px;
	border: 1px solid #93877B;
	min-height: 70px;
	line-height: 1.4em;
	text-align: justify; }
header .topic:after { content: " :"; }
article .image { float: left; margin: 10px; }
article h1 {
	border-left: solid 6px #4C575F;
	padding-left: 13px;
	font-size: 1.5em;
	margin-top: 7px;
	margin-bottom: 8px; }
article h1 a { color: inherit; text-decoration: none; }
.meta { color: #93877B; }
.meta a { color: inherit; font-weight: bold; text-decoration: none; }
.tags ul { display: inline; }
.tags ul li { display: inline; padding: 0; list-style: none; }
.tags ul li:after { content: ", ";  }
.tags ul li:last-child:after { content: "";  }
ul.poll .result { background: #F1ABC5; font-size: x-small; border-top: 1px solid #4C575F; border-bottom: 1px solid #4C575F; }
ul.threads li { list-style: none; }
li.comment > h2 { background: #E9E6E4; clear: right; }
li.comment > h2 a { color: inherit; text-decoration: none; margin-bottom: 0; }
li.comment .meta { margin-top: 5px; }
li.comment .avatar { float: right; margin: 0 5px 5px 10px; }
li.comment .content { border-left: 1px solid #93877B; padding-left: 5px; }
.deleted { border-left: 3px solid red; font-style: italic; }
.signature { color: #999; font-size: 11px; }
.signature:before { white-space: pre; content: "-- \a"; }
`

	HeaderHtml = XmlDeclaration + `
<html xmlns="http://www.w3.org/1999/xhtml" lang="fr" xml:lang="fr">
  <head>
    <title>LinuxFr.org</title>
    <meta charset="utf-8" />
    <link rel="stylesheet" type="text/css" href="RonRonnement.css" />
  </head>
  <body>`

	FooterHtml = "</body></html>"
)

var PackageTemplate = template.Must(template.New("package").Parse(`
<package xmlns="http://www.idpf.org/2007/opf" unique-identifier="pub-identifier" xml:lang="fr" version="3.0">
	<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
		<dc:language id="pub-language">fr</dc:language>
		<dc:identifier id="pub-identifier">{{.Identifier}}</dc:identifier>
		<dc:date>{{.Date}}</dc:date>
		<meta property="dcterms:modified">{{.Date}}</meta>
		{{if .Title}}<dc:title id="pub-title">{{.Title}}</dc:title>{{end}}
		{{if .Creator}}<dc:creator id="pub-creator">{{.Creator}}</dc:creator>{{end}}
		{{range .Contributors}}<dc:contributor>{{.}}</dc:contributor>
		{{end}}
		<meta name="cover" content="cover"/>
	</metadata>
	<manifest>
		<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
		<item id="css" href="RonRonnement.css" media-type="text/css"/>
		<item id="cover" href="{{.Cover}}" media-type="image/png"/>
		{{range .Items}}<item id="{{.Id}}" href="{{.Href}}" media-type="{{.Type}}"/>
		{{end}}
	</manifest>
	<spine>
		{{range .Items}}{{if .Spine}}<itemref idref="{{.Id}}"/>{{end}}
		{{end}}
	</spine>
</package>`))

var Host string

var selector2xpath map[string]string

func Css2xpath(selector string) string {
	xpath, ok := selector2xpath[selector]
	if !ok {
		xpath = css.Convert(selector, css.LOCAL)
		selector2xpath[selector] = xpath
	}
	return xpath
}

func NewEpub(w io.Writer, id string) (epub *Epub) {
	epub = &Epub{
		Zip:        zip.NewWriter(w),
		Identifier: id,
		ChanImages: make(chan *Image),
		Items:      []Item{},
		Images:     []string{},
	}
	err := epub.AddMimetype()
	if err != nil {
		log.Print("Error: ", err)
	}
	err = epub.AddDir("META-INF")
	if err != nil {
		log.Print("Error: ", err)
	}
	err = epub.AddFile("META-INF/container.xml", []byte(Container))
	if err != nil {
		log.Print("Error: ", err)
	}
	err = epub.AddDir("EPUB")
	if err != nil {
		log.Print("Error: ", err)
	}
	err = epub.AddFile("EPUB/nav.xhtml", []byte(Nav))
	if err != nil {
		log.Print("Error: ", err)
	}
	err = epub.AddFile("EPUB/RonRonnement.css", []byte(Stylesheet))
	if err != nil {
		log.Print("Error: ", err)
	}
	return
}

func (epub *Epub) importImage(uri *url.URL, filename string) {
	if uri.Scheme == "data" {
		epub.ChanImages <- nil
		return
	}
	if uri.Host == "" {
		uri.Host = Host
	}

	if uri.Scheme == "" {
		uri.Scheme = "https"
	}

	resp, err := http.Get(uri.String())
	if err != nil {
		log.Print("Error: ", err)
		epub.ChanImages <- nil
		return
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()

	if resp.StatusCode != 200 {
		log.Printf("Status code of %s is: %d\n", uri, resp.StatusCode)
		epub.ChanImages <- nil
		return
	}
	if resp.ContentLength > maxSize {
		log.Printf("Exceeded max size for %s: %d\n", uri, resp.ContentLength)
		epub.ChanImages <- nil
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print("Error: ", err)
		epub.ChanImages <- nil
		return
	}

	mimetype := resp.Header.Get("Content-Type")
	img := &Image{filename, mimetype, body}
	select {
	case epub.ChanImages <- img:
		// OK
	case <-time.After(30 * time.Second):
		log.Printf("Timeout for %s", filename)
	}
}

func (epub *Epub) toHtml(node xml.Node) string {
	// Remove some actions buttons/links
	xpath := Css2xpath(".actions, a.close, a.anchor, a.parent, .datePourCss, figure.score, meta")
	actions, err := node.Search(xpath)
	if err == nil {
		for _, action := range actions {
			action.Remove()
		}
	}

	// Remove itemprop attributes
	itemtype := node.Attribute("itemtype")
	if itemtype != nil {
		itemtype.Remove()
	}
	itemscope := node.Attribute("itemscope")
	if itemscope != nil {
		itemscope.Remove()
	}
	xpath = Css2xpath("[itemprop]")
	props, err := node.Search(xpath)
	if err == nil {
		for _, prop := range props {
			attr := prop.Attribute("itemprop")
			if attr != nil {
				attr.Remove()
			}
		}
	}

	// Fix relative links
	xpath = Css2xpath("a")
	links, err := node.Search(xpath)
	if err == nil {
		for _, link := range links {
			href := link.Attr("href")
			if len(href) > 2 && href[0] == '/' {
				if href[1] == '/' {
					link.SetAttr("href", "https:"+href)
				} else {
					link.SetAttr("href", "https://"+Host+href)
				}
			}
		}
	}

	// Import images
	xpath = Css2xpath("img")
	imgs, err := node.Search(xpath)
	if err == nil {
		for _, img := range imgs {
			uri, err := url.Parse(img.Attr("src"))
			if uri.Scheme == "data" {
				continue
			}
			if err == nil {
				found := false
				for _, s := range epub.Images {
					if s == uri.Path {
						found = true
					}
				}
				filename := strings.ReplaceAll(uri.Path, "/", "")
				if len(filename) > 64 {
					hashpart := sha256.Sum224([]byte(uri.String()))
					filename = hex.EncodeToString(hashpart[:]) + path.Ext(filename)
				}
				if !found {
					go epub.importImage(uri, filename)
					epub.Images = append(epub.Images, uri.Path)
				}
				img.SetAttr("src", filename)
			}
		}
	}

	flags := xml.XML_SAVE_AS_HTML | xml.XML_SAVE_FORMAT | xml.XML_SAVE_XHTML
	b, size := node.SerializeWithFormat(flags, nil, nil)
	if b == nil {
		return ""
	}
	return string(b[:size])
}

func (epub *Epub) AddContent(article xml.Node) {
	html := HeaderHtml +
		epub.toHtml(article) +
		FooterHtml
	filename := "content.xhtml"
	epub.Items = append(epub.Items, Item{"item-content", filename, "application/xhtml+xml", true})
	err := epub.AddFile("EPUB/"+filename, []byte(html))
	if err != nil {
		log.Print("Error: ", err)
	}
}

func (epub *Epub) AddComments(article xml.Node) {
	list := article.NextSibling()
	xpath := Css2xpath(".threads>li")
	threads, err := list.Search(xpath)
	if err != nil {
		return
	}
	for _, thread := range threads {
		html := HeaderHtml +
			`<ul class="threads">` +
			epub.toHtml(thread) +
			`</ul>` +
			FooterHtml
		id := thread.Attr("id")
		filename := id + ".html"
		epub.Items = append(epub.Items, Item{id, filename, "application/xhtml+xml", true})
		err := epub.AddFile("EPUB/"+filename, []byte(html))
		if err != nil {
			log.Print("Error: ", err)
		}
	}
}

func (epub *Epub) FindMeta(article xml.Node, selector string) string {
	xpath := Css2xpath(selector)
	nodes, err := article.Search(xpath)
	if err != nil || len(nodes) == 0 {
		return ""
	}
	return nodes[0].Content()
}

func (epub *Epub) FindMetas(article xml.Node, selector string) []string {
	xpath := Css2xpath(selector)
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

func (epub *Epub) FindCover(article xml.Node) string {
	root := article.MyDocument().Root()
	xpath := "//style"
	nodes, err := root.Search(xpath)
	if err != nil || len(nodes) == 0 {
		return ""
	}
	var re = regexp.MustCompile(`^.*url\((.*)\).*$`)
	imgpath := re.ReplaceAllString(nodes[0].Content(), `$1`)
	filename := strings.ReplaceAll(imgpath, "/", "")
	go epub.importImage(&url.URL{Host: Host, Path: imgpath}, filename)
	epub.Images = append(epub.Images, imgpath)
	return filename
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
	epub.Date = date.Format("2006-01-02T15:04:05Z")
	epub.Cover = epub.FindCover(article)
	epub.Creator = epub.FindMeta(article, "header .meta a[rel=\"author\"]")
	epub.Contributors = epub.FindMetas(article, "header .meta .edited_by a")
}

func (epub *Epub) AddMimetype() (err error) {
	header := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	f, err := epub.Zip.CreateHeader(header)
	if err != nil {
		log.Print("Zip error: ", err)
		return
	}

	_, err = f.Write([]byte(ContentType))
	if err != nil {
		log.Print("Zip error: ", err)
		return
	}

	return
}

func (epub *Epub) AddDir(dirname string) error {
	_, err := epub.Zip.Create(dirname + "/")
	if err != nil {
		log.Print("Zip error: ", err)
	}
	return err
}

func (epub *Epub) AddFile(filename string, content []byte) (err error) {
	f, err := epub.Zip.Create(filename)
	if err != nil {
		log.Print("Zip error: ", err)
		return
	}

	_, err = f.Write(content)
	if err != nil {
		log.Print("Zip error: ", err)
		return
	}

	return
}

func (epub *Epub) Close() {
	for i := 0; i < len(epub.Images); i++ {
		image := <-epub.ChanImages
		if image != nil {
			err := epub.AddFile("EPUB/"+image.Filename, image.Content)
			if err != nil {
				log.Print("Error: ", err)
			} else if image.Filename != epub.Cover {
				id := fmt.Sprintf("img-%d", i)
				item := Item{id, image.Filename, image.Mimetype, false}
				epub.Items = append(epub.Items, item)
			}
		}
	}

	var opf bytes.Buffer
	err := PackageTemplate.Execute(&opf, epub)
	if err != nil {
		log.Print("Template error: ", err)
		return
	}

	err = epub.AddFile("EPUB/package.opf", []byte(XmlDeclaration+opf.String()))
	if err != nil {
		log.Print("Error: ", err)
	}
	err = epub.Zip.Close()
	if err != nil {
		log.Print("Error on closing zip: ", err)
	}
	epub.Zip = nil
}

func FetchArticle(uri string) (article xml.Node, err error) {
	log.Printf("Fetch %s", uri)
	resp, err := http.Get(uri)
	if err != nil {
		return
	}

	body, err := io.ReadAll(resp.Body)
	err_close := resp.Body.Close()
	if err_close != nil {
		log.Fatal(err_close)
	}
	if err != nil {
		log.Printf("Error on io.ReadAll for %s: %s\n", uri, err)
		return
	}

	doc, err := gokogiri.ParseHtml(body)
	if err != nil {
		log.Printf("Gokogiri error: %s\n", err)
		return
	}
	if doc.Root() == nil {
		log.Printf("No root found in the page\n")
		return
	}

	xpath := Css2xpath("#contents article")
	articles, err := doc.Root().Search(xpath)
	if err != nil {
		log.Printf("Gokogiri error: %s\n", err)
		return
	}

	if len(articles) == 0 {
		err = errors.New("no article found in the page")
		return
	}

	article = articles[0]
	return
}

func Content(w http.ResponseWriter, r *http.Request) {
	uri := "https://" + Host + strings.Replace(r.URL.Path, ".epub", "", 1)
	if strings.HasPrefix(r.URL.Path, "/sondages") {
		uri += "?results=1"
	}
	article, err := FetchArticle(uri)
	if err != nil || article == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Add("Content-Type", ContentType)
	w.Header().Add("Link", fmt.Sprintf("<%s>; rel=\"canonical\"", uri))

	epub := NewEpub(w, r.URL.Path)
	epub.FillMeta(article)
	epub.AddContent(article)
	epub.AddComments(article)
	epub.Close()
	article.MyDocument().Free()
}

// Returns 200 OK if the server is running (for monitoring)
func Status(w http.ResponseWriter, r *http.Request) {
	_, err := fmt.Fprintf(w, "OK")
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.MemProfileRate = 1
	selector2xpath = make(map[string]string)

	// Parse the command-line
	var addr string
	var logs string
	flag.StringVar(&addr, "a", "127.0.0.1:9000", "Bind to this address:port")
	flag.StringVar(&logs, "l", "-", "Use this file for logs")
	flag.StringVar(&Host, "H", "linuxfr.org", "Use this host to fetch pages")
	flag.Parse()

	// Logging
	if logs != "-" {
		f, err := os.OpenFile(logs, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			log.Fatal("OpenFile: ", err)
		}
		err = syscall.Dup2(int(f.Fd()), int(os.Stdout.Fd()))
		if err != nil {
			log.Fatal("Dup2 (stdout): ", err)
		}
		err = syscall.Dup2(int(f.Fd()), int(os.Stderr.Fd()))
		if err != nil {
			log.Fatal("Dup2 (stderr): ", err)
		}
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
	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
