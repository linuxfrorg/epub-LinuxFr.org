package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"github.com/bmizerany/pat"
	"github.com/moovweb/gokogiri"
	"github.com/moovweb/gokogiri/html"
	"github.com/moovweb/gokogiri/xml"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"syscall"
)

type Item struct {
	Id string
	Href string
	Type string
}

type Epub struct {
	Zip *zip.Writer
	// TODO identifier / language / publisher / rights
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

func NewEpub(w io.Writer) (epub *Epub) {
	z := zip.NewWriter(w)
	epub = &Epub{Zip: z}
	epub.AddFile("mimetype", ContentType)
	epub.AddFile("META-INF/container.xml", Container)
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
	epub.AddFile("EPUB/package.opf", "XXX") // FIXME
	err := epub.Zip.Close()
	if err != nil {
		log.Println(err)
	}
}

func fetchContent(uri string) (doc *html.HtmlDocument, err error) {
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

	doc, err = gokogiri.ParseHtml(body)
	if err != nil {
		log.Printf("Gokogiri error: %s\n", err)
		return
	}

	return
}

// Create an epub for a news
func News(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", ContentType)

	slug := r.URL.Query().Get(":slug")
	uri := fmt.Sprintf("http://%s/news/%s", Host, slug)
	doc, err := fetchContent(uri)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	buffer := make([]byte, 4096)
	buffer, _ = doc.Root().ToHtml([]byte(xml.DefaultEncoding), buffer)

	epub := NewEpub(w)
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
