package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/joho/godotenv"
	snowballeng "github.com/kljensen/snowball/english"
	snowballrus "github.com/kljensen/snowball/russian"
)

const MARKER string = "mark"
const DISTANCE_BETWEEN_WORDS int = 20
const WORDS_TRIMMER_PLACEHOLDER string = "..."
const WORDS_OCCURRENCES int = -1   // Все обнаруженные подстроки
const WORDS_AROUND_RANGE int = 42  // Количество символов до и после для понимания контекста
const WORDS_DISTANCE_LIMIT int = 3 // Редакционное расстояние для основ слов в запросе и в поисковом индексе

type Document struct {
	ObjectId string   `json:"objectID"`
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

type ByFrequency []DocStat

func (a ByFrequency) Len() int           { return len(a) }
func (a ByFrequency) Less(i, j int) bool { return a[i].DocFrequency > a[j].DocFrequency }
func (a ByFrequency) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type StemStat map[string][]DocStat

type Dictionary map[string][]string

type Hit struct {
	Title     string
	Link      string
	Fragments []string
}

type LogRecord struct {
	SearchRequest string
	SearchTime    string
}

var searchLog []LogRecord = nil

// --------------------------- Служебные функции ---------------------------------

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func Max(x int, y int) int {
	if x >= y {
		return x
	} else {
		return y
	}
}

func Min(x int, y int) int {
	if x <= y {
		return x
	} else {
		return y
	}
}

func Min3(a int, b int, c int) int {
	if a < b {
		if a < c {
			return a
		}
	} else {
		if b < c {
			return b
		}
	}
	return c
}

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

func loadStopWords(path string) (map[string]struct{}, error) {
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("Не могу получить доступ к файлу '%s': %v", path, err.Error())
		return nil, err
	}
	defer f.Close()
	jsonParser := json.NewDecoder(f)

	var dump map[string]struct{}
	jsonParser.Decode(&dump)
	return dump, err
}

func loadDictionary(path string) (Dictionary, error) {
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("Не могу получить доступ к файлу '%s': %v", path, err.Error())
		return nil, err
	}
	defer f.Close()
	jsonParser := json.NewDecoder(f)

	var dump Dictionary
	jsonParser.Decode(&dump)
	return dump, err
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("Поиск '%s' за %s", name, elapsed)
	searchLog = append(searchLog, LogRecord{
		SearchRequest: name,
		SearchTime:    elapsed.String(),
	})
}

// ----------------------- Построение поискового индекса --------------------------

func tokenize(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func transformLettersFilter(tokens []string) []string {
	r := make([]string, 0, len(tokens))
	for _, token := range tokens {
		r = append(r, strings.ToLower(strings.ReplaceAll(token, "ё", "е")))
	}
	return r
}

func stopWordFilter(tokens []string, stopWords map[string]struct{}) []string {
	r := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if _, ok := stopWords[token]; !ok {
			r = append(r, token)
		}
	}
	return r
}

func stemmerFilter(tokens []string) []string {
	r := make([]string, len(tokens))
	for i, token := range tokens {
		matched, err := regexp.Match(`[А-Яа-я]+`, []byte(token))
		if err != nil {
			log.Fatalf("Не правильное регулярное выражение: %v", err.Error())
		}
		if matched {
			r[i] = snowballrus.Stem(token, false)
		} else {
			r[i] = snowballeng.Stem(token, false)
		}
	}
	return r
}

func extractTokens(text string, stopWords map[string]struct{}) []string {
	tokens := tokenize(text)
	tokens = transformLettersFilter(tokens)
	tokens = stopWordFilter(tokens, stopWords)
	tokens = stemmerFilter(tokens)
	return tokens
}

func appendStemStats(old []DocStat, new []DocStat) []DocStat {
	var result []DocStat = nil
	var excludeIndices []int = nil
	for _, oldStat := range old {
		for index, newStat := range new {
			if oldStat.DocIndex == newStat.DocIndex {
				result = append(result, DocStat{DocIndex: oldStat.DocIndex, DocFrequency: oldStat.DocFrequency + newStat.DocFrequency})
			} else {
				isInList := false
				for _, i := range excludeIndices {
					if i == index {
						isInList = true
						break
					}
				}
				if !isInList {
					excludeIndices = append(excludeIndices, index)
				}
			}
		}
	}
	for _, index := range excludeIndices {
		result = append(result, new[index])
	}
	sort.Sort(ByFrequency(result))
	return result
}

func (stemStat StemStat) keys() []string {
	result := []string{}
	for k := range stemStat {
		result = append(result, k)
	}
	return result
}

func (stemStat StemStat) addToIndex(docs []Document, stopWords map[string]struct{}) {
	for docIndex, doc := range docs {
		docTokenStat := make(map[string]float64)
		docTokenCounter := 0
		for _, content := range doc.Content {
			tokensInContent := extractTokens(content, stopWords)
			docTokenCounter += len(tokensInContent)
			for _, token := range tokensInContent {
				docTokenStat[token] += 1.0
			}
		}
		for token, amount := range docTokenStat {
			stemStat[token] = append(stemStat[token], DocStat{DocIndex: docIndex, DocFrequency: amount / float64(docTokenCounter)})
		}
	}
	for _, docStat := range stemStat {
		sort.Sort(ByFrequency(docStat))
	}
}

func (stemStat StemStat) findAndInsertVariations(term string, termVariations []string, stopWords map[string]struct{}) {
	tokenizedTerms := extractTokens(term, stopWords)
	tokenizedVariation := make(map[string][]string)
	for _, v := range termVariations {
		tokenizedVariation[v] = extractTokens(v, stopWords)
	}
	for _, t := range tokenizedTerms {
		if docStat, ok := stemStat[t]; ok {
			for _, tv := range tokenizedVariation {
				for _, v := range tv {
					if _, ok := stemStat[v]; ok {
						stemStat[t] = appendStemStats(docStat, stemStat[v])
					} else {
						stemStat[v] = docStat
					}
				}
			}
		}
	}
}

func (stemStat StemStat) applyDictionaries(dir string, stopWords map[string]struct{}) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		dic, err := loadDictionary(fmt.Sprintf("%s/%s", dir, file.Name()))
		if err != nil {
			log.Fatal(err)
		}
		for dTerm, dVars := range dic {
			stemStat.findAndInsertVariations(dTerm, dVars, stopWords)
		}
	}
}

// ----------------------------- Реализация поиска ---------------------------------

func levenshtein(str1 string, str2 string) int {
	s1len := len([]rune(str1))
	s2len := len([]rune(str2))
	column := make([]int, len(str1)+1)

	for y := 1; y <= s1len; y++ {
		column[y] = y
	}
	for x := 1; x <= s2len; x++ {
		column[0] = x
		lastkey := x - 1
		for y := 1; y <= s1len; y++ {
			oldkey := column[y]
			var incr int
			if str1[y-1] != str2[x-1] {
				incr = 1
			}

			column[y] = Min3(column[y]+1, column[y-1]+1, lastkey+incr)
			lastkey = oldkey
		}
	}
	return column[s1len]
}

func changeKeyboardLayout(s string) string {
	layoutMap := map[string]string{
		// Latin
		"q": "й", "w": "ц", "e": "у", "r": "к", "t": "е", "y": "н", "u": "г", "i": "ш", "o": "щ", "p": "з", "[": "х", "]": "ъ",
		"a": "ф", "s": "ы", "d": "в", "f": "а", "g": "п", "h": "р", "j": "о", "k": "л", "l": "д", ";": "ж", "'": "э", "\\": "ё", "`": "ё",
		"z": "я", "x": "ч", "c": "с", "v": "м", "b": "и", "n": "т", "m": "ь", ",": "б", ".": "ю",
		// Cyrillic
		"й": "q", "ц": "w", "у": "e", "к": "r", "е": "t", "н": "y", "г": "u", "ш": "i", "щ": "o", "з": "p",
		"ф": "a", "ы": "s", "в": "d", "а": "f", "п": "g", "р": "h", "о": "j", "л": "k", "д": "l",
		"я": "z", "ч": "x", "с": "c", "м": "v", "и": "b", "т": "n", "ь": "m",
	}
	result := ""
	for _, rune := range strings.Split(s, "") {
		result += layoutMap[rune]
	}
	return result
}

func preproccessRequestTokens(tokens []string, stemKeys []string) []string {
	results := []string{}
	for index, t := range tokens {
		variants := make(map[string]int)
		transformedLayoutT := changeKeyboardLayout(t)
		for _, s := range stemKeys {
			if t == s {
				results = append(results, t)
				break
			} else if transformedLayoutT == s {
				results = append(results, transformedLayoutT)
				break
			} else if l := levenshtein(t, s); l <= WORDS_DISTANCE_LIMIT {
				variants[s] = l
			}
		}
		if len(results) < index+1 {
			clotherWord := ""
			minV := int(^uint(0) >> 1)
			for key, v := range variants {
				if v < minV {
					minV = v
					clotherWord = key
				}
			}
			results = append(results, clotherWord)
		}
	}
	return results
}

func mergeDocStat(docStats [][]DocStat) []int {
	var result []int = nil
	var stats []DocStat = nil
	for _, docStatForWord := range docStats {
		stats = append(stats, docStatForWord...)
	}
	sort.Sort(ByFrequency(stats))
	for _, s := range stats {
		result = append(result, s.DocIndex)
	}
	return result
}

func intersectDocStat(first []DocStat, second []DocStat) []DocStat {
	result := []DocStat{}
	for _, f := range first {
		for _, s := range second {
			if f.DocIndex == s.DocIndex {
				result = append(result, DocStat{DocIndex: f.DocIndex, DocFrequency: f.DocFrequency + s.DocFrequency})
			}
		}
	}
	sort.Sort(ByFrequency(result))
	return result
}

func subtractDocStat(first []DocStat, second []DocStat) []DocStat {
	var indicesForSubtraction []int
	for i, f := range first {
		for _, s := range second {
			if f.DocIndex == s.DocIndex {
				indicesForSubtraction = append(indicesForSubtraction, i)
			}
		}
	}
	result := []DocStat{}
	for number, f := range first {
		isNotSelected := true
		for _, index := range indicesForSubtraction {
			if number == index {
				isNotSelected = false
				break
			}
		}
		if isNotSelected {
			result = append(result, f)
		}
	}
	return result
}

func getDocIndices(words []string, stemStat StemStat, stemKeys []string, stopWords map[string]struct{}) []int {
	var r [][]DocStat
	for wordIndex, word := range words {
		tokens := extractTokens(word, stopWords)
		tokens = preproccessRequestTokens(tokens, stemKeys)
		r = append(r, []DocStat{})
		if strings.Contains(word, "+") {
			m := []DocStat{}
			for i, token := range tokens {
				if i == 0 {
					m = append(m, stemStat[token]...)
				} else {
					m = intersectDocStat(m, stemStat[token])
				}
			}
			r[wordIndex] = append(r[wordIndex], m...)
		} else if strings.Contains(word, "-") {
			m := []DocStat{}
			for i, token := range tokens {
				if i == 0 {
					m = append(m, stemStat[token]...)
				} else {
					m = subtractDocStat(m, stemStat[token])
				}
			}
			r[wordIndex] = append(r[wordIndex], m...)
		} else {
			for _, token := range tokens {
				r[wordIndex] = append(r[wordIndex], stemStat[token]...)
			}
		}
	}
	result := mergeDocStat(r)
	return result
}

func prepareWords(words []string, stemKeys []string, stopWords map[string]struct{}) []string {
	processedWords := []string{}
	for _, word := range words {
		tokens := extractTokens(word, stopWords)
		preprocessed := preproccessRequestTokens(tokens, stemKeys)
		processedWords = append(processedWords, strings.ToLower(word))
		for i, t := range tokens {
			processedWords[len(processedWords)-1] = strings.ReplaceAll(processedWords[len(processedWords)-1], t, preprocessed[i])
		}
	}
	return processedWords
}

func getHits(words []string, documents []Document, stemStat StemStat, stemKeys []string, stopWords map[string]struct{}) []Hit {
	defer timeTrack(time.Now(), fmt.Sprintf("Поиск '%s' выполнен", strings.Join(words, " ")))
	var result []Hit
	for _, index := range getDocIndices(words, stemStat, stemKeys, stopWords) {
		_, title := markWord(words, stopWords, documents[index].Title)
		result = append(result, Hit{
			Title:     title,
			Link:      fmt.Sprintf("/%s", documents[index].ObjectId),
			Fragments: prepareFragments(words, stopWords, documents, index),
		})
	}
	return result
}

// ----------------------------- Подготовка поисковоого ответа ---------------------------------

func markWord(words []string, stopWords map[string]struct{}, s string) (bool, string) {
	lowerCase := strings.ToLower(strings.ReplaceAll(s, "ё", "е"))
	var searchWords []string
	for _, w := range words {
		w = strings.ReplaceAll(w, "ё", "е")
		if strings.Contains(w, "+") {
			searchWords = append(searchWords, strings.ReplaceAll(w, "+", fmt.Sprintf(".{0,%d}", DISTANCE_BETWEEN_WORDS)))
		} else if strings.Contains(w, "-") {
			searchWords = append(searchWords, strings.Split(w, "-")...)
		} else {
			searchWords = append(searchWords, w)
		}
		searchWords = append(searchWords, extractTokens(w, stopWords)...)
	}
	re := regexp.MustCompile("(" + strings.ToLower(strings.Join(searchWords, "|")) + ")")
	occurrences := re.FindAllIndex([]byte(lowerCase), WORDS_OCCURRENCES)
	oLength := len(occurrences)
	if oLength > 0 {
		stack := [][]int{}
		j := 0
		for i, o := range occurrences {
			if i > 0 && o[0] <= occurrences[i-1][1]+2*(WORDS_AROUND_RANGE+(o[1]-o[0])) {
				stack[j] = append(stack[j], o...)
			} else if i == 0 {
				stack = append(stack, o)
			} else {
				j++
				stack = append(stack, o)
			}
		}
		r := []string{}
		for _, indices := range stack {
			indicesLength := len(indices)
			startIndex := Max(indices[0]-WORDS_AROUND_RANGE, 0)
			stopIndex := Min(indices[indicesLength-1]+WORDS_AROUND_RANGE, len(s))
			sPart := s
			sCounter := 0
			bracketsLength := len("<></>") + 2*len(MARKER)
			for i := indicesLength - 1; i > 0; i -= 2 {
				startWord := indices[i-1]
				stopWord := indices[i]
				sPart = sPart[:startWord] +
					"<" + MARKER + ">" + sPart[startWord:stopWord] + "</" + MARKER + ">" +
					sPart[stopWord:]
				sCounter += bracketsLength
			}
			if stopIndex-startIndex < len(sPart)+sCounter {
				sPart := sPart[startIndex : stopIndex+sCounter]
				r = append(r, trimAndWrap(sPart))
			} else {
				r = append(r, sPart)
			}
		}
		return true, strings.Join(r, " ")
	}
	return false, s
}

func prepareFragments(words []string, stopWords map[string]struct{}, documents []Document, docNumber int) []string {
	var fragments []string
	for _, p := range documents[docNumber].Content {
		contains, marked := markWord(words, stopWords, p)
		if contains {
			fragments = append(fragments, marked)
		}
	}
	return fragments
}

// ----------------------------- Подготовка поисковоого запроса ---------------------------------

func trimAndWrap(s string) string {
	stringLength := len(s)
	trimmed := strings.ToValidUTF8(
		s,
		"",
	)
	if stringLength > 2*WORDS_AROUND_RANGE {
		regexCase := "[a-zа-я!?.:\"«»—]+"
		re := regexp.MustCompile("(^" + regexCase + " | " + regexCase + "$)")
		trimmed = re.ReplaceAllString(trimmed, "")
	}
	trimmedLength := len(trimmed)
	edge := ""
	if trimmedLength < stringLength {
		edge = WORDS_TRIMMER_PLACEHOLDER
	}
	return edge + trimmed + edge
}

func prepareSearchRequest(searchRequest string) string {
	re := regexp.MustCompile(" *[+] *")
	result := re.ReplaceAllString(searchRequest, "+")
	re = regexp.MustCompile(" *[-] *")
	result = re.ReplaceAllString(result, "-")
	re = regexp.MustCompile(" +")
	result = re.ReplaceAllString(result, " ")
	return result
}

func callbackHandler(documents []Document, stemStat StemStat, stemKeys []string, stopWords map[string]struct{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		searchRequest := prepareSearchRequest(r.URL.Query()["search"][0])
		fmt.Fprintf(w, "<!DOCTYPE html><html><body><h1>Поиск</h1><form action=\"/\" method=\"get\"><input type=\"text\" name=\"search\" value=\"%s\"><input type=\"submit\" value=\"Искать\">", searchRequest)
		fmt.Fprintf(w, "<h2>Искали: '%s'</h2>", searchRequest)
		words := prepareWords(strings.Split(searchRequest, " "), stemKeys, stopWords)
		hits := getHits(words, documents, stemStat, stemKeys, stopWords)
		if len(hits) == 0 {
			searchRequest = changeKeyboardLayout(searchRequest)
			words = prepareWords(strings.Split(searchRequest, " "), stemKeys, stopWords)
			hits = getHits(words, documents, stemStat, stemKeys, stopWords)
		}
		fmt.Fprintf(w, "<h2>Нашли (%d рез. для '%s' за %s):</h2>", len(hits), strings.Join(words, " "), searchLog[len(searchLog)-1].SearchTime)
		for i, hit := range hits {
			fmt.Fprintf(w, "<a href=\"https://doka.guide%s\"><h3>Hit #%d '%s'</h3></a>", hit.Link, i+1, hit.Title)
			for _, fragment := range hit.Fragments {
				fmt.Fprintf(w, "<p>%s</p>", fragment)
			}
		}
		fmt.Fprintf(w, "</body></html>")
	}
}

func main() {
	stems := make(StemStat)

	loadEnv()
	docs, _ := loadDocuments(os.Getenv("SEARCH_CONTENT"))
	stopWords, _ := loadStopWords(os.Getenv("STOP_WORDS"))
	stems.addToIndex(docs, stopWords)
	stems.applyDictionaries(os.Getenv("DICTS_DIR"), stopWords)

	http.HandleFunc("/", callbackHandler(docs, stems, stems.keys(), stopWords))
	log.Fatal(http.ListenAndServe(os.Getenv("APP_HOST")+":"+os.Getenv("APP_PORT"), nil))
}
