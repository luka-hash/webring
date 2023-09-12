// Copyright (c) 2023 Luka Ivanović
// This code is licensed under MIT licence (see LICENCE for details)

package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"

	"github.com/gorilla/mux"
)

type Member struct {
	Name string
	URL  string
}

type Webring struct {
	membersFile string
	Members     []Member
	Index       *template.Template
	Static      string
}

func isAlive(url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		return false
	}
	if resp.StatusCode == http.StatusOK {
		return true
	}
	return false
}

func readMembers(filename string) ([]Member, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	r := csv.NewReader(file)
	r.TrimLeadingSpace = true
	_, err = r.Read() // skip header
	if err == io.EOF {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]Member, 0)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, err
		}
		result = append(result, Member{
			Name: record[0],
			URL:  record[1],
		})
	}
	return result, nil
}

func modulo(a, b int) int {
	return (((a % b) + b) % b)
}

func index(w http.ResponseWriter, r *http.Request, webring *Webring) {
	webring.Index.Execute(w, webring)
}

func next(w http.ResponseWriter, r *http.Request, webring *Webring) {
	referrer, err := getReferrer(r)
	if err != nil {
		random(w, r, webring)
		return
	}
	for i, member := range webring.Members {
		if member.URL == referrer {
			http.Redirect(w, r, webring.Members[modulo(i+1, len(webring.Members))].URL, http.StatusFound)
			return
		}
	}
	// if the referrer is not in the members list, just redirect to random member
	random(w, r, webring)
}

func previous(w http.ResponseWriter, r *http.Request, webring *Webring) {
	referrer, err := getReferrer(r)
	if err != nil {
		random(w, r, webring)
		return
	}
	for i, member := range webring.Members {
		if member.URL == referrer {
			http.Redirect(w, r, webring.Members[modulo(i-1, len(webring.Members))].URL, http.StatusFound)
			return
		}
	}
	// if the referrer is not in the members list, just redirect to random member
	random(w, r, webring)
}

func random(w http.ResponseWriter, r *http.Request, webring *Webring) {
	http.Redirect(w, r, webring.Members[rand.Intn(len(webring.Members))].URL, http.StatusFound)
}

func getReferrer(r *http.Request) (string, error) {
	referrerRaw := r.Referer()
	referrerURL, err := url.Parse(referrerRaw)
	if err != nil {
		return "", err
	}
	return (referrerURL.Scheme + "://" + referrerURL.Host), nil
}

func main() {
	membersFile := flag.String("members", "members.csv", "csv file containing members of the webring")
	staticDir := flag.String("static", "static/", "directory containing favicon, badges and other static resources")
	indexFile := flag.String("index", "index.html", "template file for webring home")
	flag.Parse()

	members, err := readMembers(*membersFile)
	if err != nil {
		panic(err)
	}
	fmt.Println("Members:", len(members))
	fmt.Println("Checking for problems with members...")
	problematic := 0
	for _, member := range members {
		if !isAlive(member.URL) {
			fmt.Println("There is a possible problem with:", member.Name, member.URL)
			problematic += 1
		}
	}
	fmt.Println("Finished.")
	if problematic > 0 {
		fmt.Println("There is a possible problem with", problematic, "members.")
		if problematic == len(members) {
			fmt.Println("Aborting due the insufficient number of healthy members in the webring.")
		}
	}
	tmpl, err := template.ParseFiles(*indexFile)
	if err != nil {
		panic(err)
	}
	webring := &Webring{
		Members: members,
		Index:   tmpl,
	}

	router := mux.NewRouter()
	static := http.FileServer(http.Dir(*staticDir))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", static))
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		index(w, r, webring)
	}).Methods("GET")
	router.HandleFunc("/next", func(w http.ResponseWriter, r *http.Request) {
		next(w, r, webring)
	}).Methods("GET")
	router.HandleFunc("/previous", func(w http.ResponseWriter, r *http.Request) {
		previous(w, r, webring)
	}).Methods("GET")
	router.HandleFunc("/im-feeling-lucky", func(w http.ResponseWriter, r *http.Request) {
		random(w, r, webring)
	}).Methods("GET")

	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalln(err)
	}
}
