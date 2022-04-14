package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/joho/godotenv"
)

const MARKER string = "b"
const DISTANCE int = 2

type Document struct {
	Objectid string   `json:"objectID"`
	Title    string   `json:"title"`
	Keywords []string `json:"keywords"`
	Tags     []string `json:"tags"`
	Category string   `json:"category"`
	Content  []string `json:"content"`
}

type DocStat struct {
	DocIndex     int     `json:"d"`
	DocFrequency float64 `json:"f"`
}

type DocStatArray []DocStat

type WordStat map[string]DocStatArray

type StemStat map[string]WordStat

type Hit struct {
	Title     string
	Fragments []string
}

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func removeDuplicates(intSlice []int) []int {
	keys := make(map[int]bool)
	list := []int{}
	for _, entry := range intSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

var docs []Document
var stemTree StemStat
var stemKeys []string

func loadEnv() {
	var err = godotenv.Load()
	if err != nil {
		log.Fatalf("Не могу получить доступ к файлу '.env': %v", err.Error())
	} else {
		fmt.Println("Значения из файла '.env' получены.")
	}
}

func loadDocuments(path string) ([]Document, error) {
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("Не могу получить доступ к файлу '%s': %v", path, err.Error())
		return nil, err
	}
	defer f.Close()
	jsonParser := json.NewDecoder(f)

	var dump []Document
	jsonParser.Decode(&dump)
	return dump, err
}

func loadTree(path string) (StemStat, error) {
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("Не могу получить доступ к файлу '%s': %v", path, err.Error())
		return nil, err
	}
	defer f.Close()
	jsonParser := json.NewDecoder(f)

	var dump StemStat
	jsonParser.Decode(&dump)
	return dump, err
}

func getDocIndices(words []string) []int {
	var stats []DocStat
	for _, stem := range stemKeys {
		for _, word := range words {
			if strings.Contains(strings.ToLower(word), stem) {
				for w, wStat := range stemTree[stem] {
					if strings.Contains(w, word) {
						for _, docStat := range wStat {
							stats = append(stats, docStat)
						}
					}
				}
			}
		}
	}
	var docIndices []int
	sort.SliceStable(stats, func(i, j int) bool {
		return stats[i].DocFrequency > stats[j].DocFrequency
	})
	for _, s := range stats {
		docIndices = append(docIndices, s.DocIndex)
	}
	return removeDuplicates(docIndices)
}

func markWord(ws []string, s string) (bool, string) {
	matched, err := regexp.Match(strings.ToLower(strings.Join(ws, ".*")), []byte(strings.ToLower(s)))
	if err != nil {
		log.Fatalf("Не правильное регулярное выражение: %v", err.Error())
		return false, s
	}
	return matched, s
}

func prepareFragments(words []string, docNumber int) []string {
	var fragments []string
	for _, p := range docs[docNumber].Content {
		contains, marked := markWord(words, p)
		if contains {
			fragments = append(fragments, marked)
		}
	}
	return fragments
}

func getHits(words []string) []Hit {
	var result []Hit
	for _, index := range getDocIndices(words) {
		_, title := markWord(words, docs[index].Title)
		result = append(result, Hit{Title: title, Fragments: prepareFragments(words, index)})
	}
	return result
}

func handler(w http.ResponseWriter, r *http.Request) {
	searchRequest := r.URL.Query()["search"][0]
	fmt.Fprintf(w, "<h1>Поиск</h1>")
	fmt.Fprintf(w, "<h2>Искали: '%s'</h2>", searchRequest)
	words := strings.Fields(searchRequest)
	fmt.Fprintf(w, "<h2>Нашли:</h2>")
	for i, hit := range getHits(words) {
		fmt.Fprintf(w, "<h3>Hit #%d '%s'</h3>", i, hit.Title)
		for _, fragment := range hit.Fragments {
			fmt.Fprintf(w, "<p>%s</p>", fragment)
		}
	}
}

func main() {
	loadEnv()
	docs, _ = loadDocuments(os.Getenv("SEARCH_CONTENT"))
	stemTree, _ = loadTree(os.Getenv("SEARCH_TREE"))
	for key, _ := range stemTree {
		stemKeys = append(stemKeys, key)
	}

	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(os.Getenv("APP_HOST")+":"+os.Getenv("APP_PORT"), nil))
}
