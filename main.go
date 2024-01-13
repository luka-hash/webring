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
	"os"
	"strings"

	"github.com/gorilla/mux"
)

type Member struct {
	Name string
	URL  string
}

type Webring struct {
	Members    []Member
	MembersMap map[string]int
	Index      *template.Template
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
	r.Comment = '#'
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
			return nil, err
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
	member := r.URL.Query().Get("member")
	if member == "" {
		random(w, r, webring)
		return
	}
	if !strings.HasPrefix(member, "https://") {
		member = "https://"+member
	}
	memberIndex, ok := webring.MembersMap[member]
	if !ok {
		random(w, r, webring)
		return
	}
	// TODO: Maybe check if the next member is OK before redirecting
	http.Redirect(w, r, webring.Members[modulo(memberIndex+1, len(webring.Members))].URL, http.StatusFound)
}

func previous(w http.ResponseWriter, r *http.Request, webring *Webring) {
	member := r.URL.Query().Get("member")
	if member == "" {
		random(w, r, webring)
		return
	}
	if !strings.HasPrefix(member, "https://") {
		member = "https://"+member
	}
	memberIndex, ok := webring.MembersMap[member]
	if !ok {
		random(w, r, webring)
		return
	}
	http.Redirect(w, r, webring.Members[modulo(memberIndex-1, len(webring.Members))].URL, http.StatusFound)
}

func random(w http.ResponseWriter, r *http.Request, webring *Webring) {
	http.Redirect(w, r, webring.Members[rand.Intn(len(webring.Members))].URL, http.StatusFound)
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
	if len(members) < 2 {
		panic("Cannot create a ring with less than 2 members")
	}
	fmt.Println("Checking for problems with members...")
	problematic := 0
	for _, member := range members {
		if !isAlive(member.URL) {
			fmt.Println("There is a possible problem with:", member.URL)
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
	membersMap := make(map[string]int)
	for i, member := range members {
		membersMap[member.URL] = i
	}
	tmpl, err := template.ParseFiles(*indexFile)
	if err != nil {
		panic(err)
	}
	webring := &Webring{
		Members:    members,
		MembersMap: membersMap,
		Index:      tmpl,
	}

	router := mux.NewRouter()
	static := http.FileServer(http.Dir(*staticDir))
	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index(w, r, webring)
	})
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

	if err := http.ListenAndServe(":5852", router); err != nil {
		log.Fatalln(err)
	}
}
