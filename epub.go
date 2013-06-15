package main

import (
	"flag"
	"fmt"
	"github.com/bmizerany/pat"
	"github.com/moovweb/gokogiri"
	"github.com/moovweb/gokogiri/xml"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"syscall"
)

const Host = "linuxfr.org"

// Create an epub for a news
func News(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get(":slug")
	uri := fmt.Sprintf("http://%s/news/%s", Host, slug)
	fmt.Println(uri)
	resp, err := http.Get(uri)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error on ioutil.ReadAll for %s: %s\n", uri, err)
		http.NotFound(w, r)
		return
	}

	doc, err := gokogiri.ParseHtml(body)
	if err != nil {
		log.Printf("Gokogiri error: %s\n", err)
		http.Error(w, err.Error(), 500)
		return
	}

	buffer := make([]byte, 4096)
	buffer, _ = doc.Root().ToHtml([]byte(xml.DefaultEncoding), buffer)
	w.Write(buffer)
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
