package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/joho/godotenv"
	snowballeng "github.com/kljensen/snowball/english"
	snowballrus "github.com/kljensen/snowball/russian"
)

const ARG_SEARCH_CONTENT string = "SEARCH_CONTENT"
const ARG_STOP_WORDS string = "STOP_WORDS"
const ARG_DICTS_DIR string = "DICTS_DIR"
const ARG_APP_NAME string = "APP_NAME"
const ARG_APP_HOST string = "APP_HOST"
const ARG_APP_PORT string = "APP_PORT"
const ARG_APP_LOG_LIMIT string = "APP_LOG_LIMIT"

const ARG_WORDS_MARKER_TAG string = "WORDS_MARKER_TAG"
const ARG_WORDS_DISTANCE_BETWEEN string = "WORDS_DISTANCE_BETWEEN"
const ARG_WORDS_TRIMMER_PLACEHOLDER string = "WORDS_TRIMMER_PLACEHOLDER"
const ARG_WORDS_OCCURRENCES string = "WORDS_OCCURRENCES"
const ARG_WORDS_AROUND_RANGE string = "WORDS_AROUND_RANGE"
const ARG_WORDS_DISTANCE_LIMIT string = "WORDS_DISTANCE_LIMIT"
const ARG_WORDS_FREQUENCY_LIMIT string = "WORDS_FREQUENCY_LIMIT"
const ARG_WORDS_TITLE_WEIGHT string = "WORDS_TITLE_WEIGHT"
const ARG_WORDS_KEYWORDS_WEIGHT string = "WORDS_KEYWORDS_WEIGHT"

// Значения по умолчанию
const APP_NAME string = "SEARCH-DB-LESS"
const APP_HOST string = ""
const APP_PORT string = "8080"
const APP_LOG_LIMIT int = 100
const WORDS_MARKER_TAG string = "mark"
const WORDS_DISTANCE_BETWEEN int = 20
const WORDS_TRIMMER_PLACEHOLDER string = "..."
const WORDS_OCCURRENCES int = -1
const WORDS_AROUND_RANGE int = 42
const WORDS_DISTANCE_LIMIT int = 3
const WORDS_FREQUENCY_LIMIT float64 = 0.5
const WORDS_TITLE_WEIGHT float64 = 10.0
const WORDS_KEYWORDS_WEIGHT float64 = 1.0

type SearchError struct {
	When time.Time
	What string
}

func (e SearchError) Error() string {
	return fmt.Sprintf("%v: %v", e.When, e.What)
}

type Document struct {
	ObjectId string   `json:"objectID"`
	Title    string   `json:"title"`
	Keywords []string `json:"keywords,omitempty"`
	Tags     []string `json:"tags"`
	Category string   `json:"category"`
	Content  []string `json:"content"`
}

type DocStat struct {
	DocIndex     int
	DocFrequency float64
	DocTags      []string
	DocCategory  string
}

type ByFrequency []DocStat

func (a ByFrequency) Len() int           { return len(a) }
func (a ByFrequency) Less(i, j int) bool { return a[i].DocFrequency > a[j].DocFrequency }
func (a ByFrequency) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type StemStat map[string][]DocStat

type Dictionary map[string][]string

type Hit struct {
	Title     string   `json:"title"`
	Link      string   `json:"link"`
	Fragments []string `json:"fragments"`
	Tags      []string `json:"tags"`
	Category  string   `json:"category"`
}

type LogRecord struct {
	RequestTime    string
	RequestHost    string
	SearchRequest  string
	SearchCategory []string
	SearchTags     []string
	SearchTime     string
}

var searchLog []LogRecord = nil

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

func removeDuplicates(list []int) []int {
	result := []int{}
	for _, l := range list {
		isDuplicated := false
		for _, r := range result {
			if r == l {
				isDuplicated = true
				break
			}
		}
		if !isDuplicated {
			result = append(result, l)
		}
	}
	return result
}

func getWordStem(word string) string {
	if strings.ContainsAny(word, "абвгдеёжхзиклмнопрстуфхцчшщьыъэюяАБВГДЕЁЖХЗИКЛМНОПРСТУФХЦЧШЩЬЫЪЭЮЯ") {
		return snowballrus.Stem(word, false)
	} else {
		return snowballeng.Stem(word, false)
	}
}

func loadSettings() map[string]string {
	defer timeTrackLoading(time.Now(), "настроек из файла")
	var err = godotenv.Load()
	if err != nil {
		args := os.Args[1:]
		result := make(map[string]string)
		result[ARG_APP_NAME] = APP_NAME
		result[ARG_APP_HOST] = APP_HOST
		result[ARG_APP_PORT] = APP_PORT
		result[ARG_APP_LOG_LIMIT] = fmt.Sprintf("%d", APP_LOG_LIMIT)
		result[ARG_WORDS_MARKER_TAG] = WORDS_MARKER_TAG
		result[ARG_WORDS_DISTANCE_BETWEEN] = fmt.Sprintf("%d", WORDS_DISTANCE_BETWEEN)
		result[ARG_WORDS_TRIMMER_PLACEHOLDER] = WORDS_TRIMMER_PLACEHOLDER
		result[ARG_WORDS_OCCURRENCES] = fmt.Sprintf("%d", WORDS_OCCURRENCES)
		result[ARG_WORDS_AROUND_RANGE] = fmt.Sprintf("%d", WORDS_AROUND_RANGE)
		result[ARG_WORDS_DISTANCE_LIMIT] = fmt.Sprintf("%d", WORDS_DISTANCE_LIMIT)
		result[ARG_WORDS_FREQUENCY_LIMIT] = fmt.Sprintf("%f", WORDS_FREQUENCY_LIMIT)
		result[ARG_WORDS_TITLE_WEIGHT] = fmt.Sprintf("%f", WORDS_TITLE_WEIGHT)
		result[ARG_WORDS_KEYWORDS_WEIGHT] = fmt.Sprintf("%f", WORDS_KEYWORDS_WEIGHT)
		for i, a := range args {
			switch a {
			case "-c", "--search-content":
				result[ARG_SEARCH_CONTENT] = args[i+1]
			case "-w", "--stop-words":
				result[ARG_STOP_WORDS] = args[i+1]
			case "-d", "--dicts-dir":
				result[ARG_DICTS_DIR] = args[i+1]
			case "-n", "--app-name":
				result[ARG_APP_NAME] = args[i+1]
			case "-h", "--app-host":
				result[ARG_APP_HOST] = args[i+1]
			case "-p", "--app-port":
				result[ARG_APP_PORT] = args[i+1]
			case "-l", "--app-log":
				result[ARG_APP_LOG_LIMIT] = args[i+1]
			case "--words-marker-tag":
				result[ARG_WORDS_MARKER_TAG] = args[i+1]
			case "--words-distance-between":
				result[ARG_WORDS_DISTANCE_BETWEEN] = args[i+1]
			case "--words-trimmer-placeholder":
				result[ARG_WORDS_TRIMMER_PLACEHOLDER] = args[i+1]
			case "--words-occurrences`":
				result[ARG_WORDS_OCCURRENCES] = args[i+1]
			case "--words-around-range":
				result[ARG_WORDS_AROUND_RANGE] = args[i+1]
			case "--words-distance-limit":
				result[ARG_WORDS_DISTANCE_LIMIT] = args[i+1]
			case "--words-frequency-limit":
				result[ARG_WORDS_FREQUENCY_LIMIT] = args[i+1]
			case "--words-title-weight":
				result[ARG_WORDS_TITLE_WEIGHT] = args[i+1]
			case "--words-keywords_weight":
				result[ARG_WORDS_KEYWORDS_WEIGHT] = args[i+1]
			}
		}
		return result
	} else {
		log.Printf("Значения из файла '.env' получены.")
		result := make(map[string]string)
		result[ARG_SEARCH_CONTENT] = os.Getenv(ARG_SEARCH_CONTENT)
		result[ARG_STOP_WORDS] = os.Getenv(ARG_STOP_WORDS)
		result[ARG_DICTS_DIR] = os.Getenv(ARG_DICTS_DIR)
		if os.Getenv(ARG_APP_NAME) != "" {
			result[ARG_APP_NAME] = os.Getenv(ARG_APP_NAME)
		} else {
			result[ARG_APP_NAME] = APP_NAME
		}
		if os.Getenv(ARG_APP_HOST) != "" {
			result[ARG_APP_HOST] = os.Getenv(ARG_APP_HOST)
		} else {
			result[ARG_APP_HOST] = APP_HOST
		}
		if os.Getenv(ARG_APP_PORT) != "" {
			result[ARG_APP_PORT] = os.Getenv(ARG_APP_PORT)
		} else {
			result[ARG_APP_PORT] = APP_PORT
		}
		if os.Getenv(ARG_APP_LOG_LIMIT) != "" {
			result[ARG_APP_LOG_LIMIT] = os.Getenv(ARG_APP_LOG_LIMIT)
		} else {
			result[ARG_APP_LOG_LIMIT] = fmt.Sprintf("%d", APP_LOG_LIMIT)
		}
		if os.Getenv(ARG_WORDS_MARKER_TAG) != "" {
			result[ARG_WORDS_MARKER_TAG] = os.Getenv(ARG_WORDS_MARKER_TAG)
		} else {
			result[ARG_WORDS_MARKER_TAG] = WORDS_MARKER_TAG
		}
		if os.Getenv(ARG_WORDS_DISTANCE_BETWEEN) != "" {
			result[ARG_WORDS_DISTANCE_BETWEEN] = os.Getenv(ARG_WORDS_DISTANCE_BETWEEN)
		} else {
			result[ARG_WORDS_DISTANCE_BETWEEN] = fmt.Sprintf("%d", WORDS_DISTANCE_BETWEEN)
		}
		if os.Getenv(ARG_WORDS_TRIMMER_PLACEHOLDER) != "" {
			result[ARG_WORDS_TRIMMER_PLACEHOLDER] = os.Getenv(ARG_WORDS_TRIMMER_PLACEHOLDER)
		} else {
			result[ARG_WORDS_TRIMMER_PLACEHOLDER] = WORDS_TRIMMER_PLACEHOLDER
		}
		if os.Getenv(ARG_WORDS_OCCURRENCES) != "" {
			result[ARG_WORDS_OCCURRENCES] = os.Getenv(ARG_WORDS_OCCURRENCES)
		} else {
			result[ARG_WORDS_OCCURRENCES] = fmt.Sprintf("%d", WORDS_OCCURRENCES)
		}
		if os.Getenv(ARG_WORDS_AROUND_RANGE) != "" {
			result[ARG_WORDS_AROUND_RANGE] = os.Getenv(ARG_WORDS_AROUND_RANGE)
		} else {
			result[ARG_WORDS_AROUND_RANGE] = fmt.Sprintf("%d", WORDS_AROUND_RANGE)
		}
		if os.Getenv(ARG_WORDS_DISTANCE_LIMIT) != "" {
			result[ARG_WORDS_DISTANCE_LIMIT] = os.Getenv(ARG_WORDS_DISTANCE_LIMIT)
		} else {
			result[ARG_WORDS_DISTANCE_LIMIT] = fmt.Sprintf("%d", WORDS_DISTANCE_LIMIT)
		}
		if os.Getenv(ARG_WORDS_FREQUENCY_LIMIT) != "" {
			result[ARG_WORDS_FREQUENCY_LIMIT] = os.Getenv(ARG_WORDS_FREQUENCY_LIMIT)
		} else {
			result[ARG_WORDS_FREQUENCY_LIMIT] = fmt.Sprintf("%f", WORDS_FREQUENCY_LIMIT)
		}
		if os.Getenv(ARG_WORDS_TITLE_WEIGHT) != "" {
			result[ARG_WORDS_TITLE_WEIGHT] = os.Getenv(ARG_WORDS_TITLE_WEIGHT)
		} else {
			result[ARG_WORDS_TITLE_WEIGHT] = fmt.Sprintf("%f", WORDS_TITLE_WEIGHT)
		}
		if os.Getenv(ARG_WORDS_KEYWORDS_WEIGHT) != "" {
			result[ARG_WORDS_KEYWORDS_WEIGHT] = os.Getenv(ARG_WORDS_KEYWORDS_WEIGHT)
		} else {
			result[ARG_WORDS_KEYWORDS_WEIGHT] = fmt.Sprintf("%f", WORDS_KEYWORDS_WEIGHT)
		}
		return result
	}
}

func loadDocuments(path string) ([]Document, error) {
	defer timeTrackLoading(time.Now(), fmt.Sprintf("документов из файла '%s'", path))
	if path == "" {
		return nil, SearchError{
			time.Now(),
			"Путь к файлу не может быть пустым",
		}
	}

	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("Не могу получить доступ к файлу '%s'", path)
		return nil, err
	}
	defer f.Close()
	jsonParser := json.NewDecoder(f)

	var dump []Document
	err = jsonParser.Decode(&dump)
	return dump, err
}

func loadStopWords(path string) (map[string]struct{}, error) {
	defer timeTrackLoading(time.Now(), fmt.Sprintf("словаря стоп-слов из файла '%s'", path))
	if path == "" {
		return make(map[string]struct{}), SearchError{
			time.Now(),
			"Путь к файлу не может быть пустым",
		}
	}

	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("Не могу получить доступ к файлу '%s'", path)
		return nil, err
	}
	defer f.Close()
	jsonParser := json.NewDecoder(f)

	var dump map[string]struct{}
	err = jsonParser.Decode(&dump)
	return dump, err
}

func loadDictionary(path string) (Dictionary, error) {
	defer timeTrackLoading(time.Now(), fmt.Sprintf("словаря из файла '%s'", path))
	if path == "" {
		return make(Dictionary), SearchError{
			time.Now(),
			"Путь к файлу не может быть пустым",
		}
	}

	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("Не могу получить доступ к файлу '%s'", path)
		return nil, err
	}
	defer f.Close()
	jsonParser := json.NewDecoder(f)

	var dump Dictionary
	err = jsonParser.Decode(&dump)
	return dump, err
}

func saveSearchLog(constants map[string]string) {
	fileName := fmt.Sprintf("%v-%s.log", time.Unix(time.Now().Unix(), 0).UTC(), constants[ARG_APP_NAME])
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Не могу создать файл '%s'", err)
	}
	datawriter := bufio.NewWriter(file)
	for _, log := range searchLog {
		string := fmt.Sprintf(
			"%s - %s - %s - %s - %s - %s\n",
			log.RequestTime,
			log.RequestHost,
			log.SearchCategory,
			log.SearchTags,
			log.SearchRequest,
			log.SearchTime,
		)
		_, _ = datawriter.WriteString(string)
	}
	datawriter.Flush()
	file.Close()
	searchLog = nil
}

func timeTrackLoading(start time.Time, funcName string) {
	elapsed := time.Since(start)
	log.Printf("Загрузка %s прошла за %s", funcName, elapsed.String())
}

func timeTrackSearch(start time.Time, searchRequest string, host string, category []string, tags []string, constants map[string]string) {
	elapsed := time.Since(start)
	searchLog = append(searchLog, LogRecord{
		RequestTime:    start.String(),
		RequestHost:    host,
		SearchRequest:  searchRequest,
		SearchCategory: category,
		SearchTags:     tags,
		SearchTime:     elapsed.String(),
	})
	logLength := len(searchLog)
	log.Printf(
		"%s - %s - %s - %s - %s\n",
		searchLog[logLength-1].RequestHost,
		searchLog[logLength-1].SearchCategory,
		searchLog[logLength-1].SearchTags,
		searchLog[logLength-1].SearchRequest,
		searchLog[logLength-1].SearchTime,
	)
	limit, _ := strconv.Atoi(constants[ARG_APP_LOG_LIMIT])
	if logLength >= limit {
		saveSearchLog(constants)
	}
}

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
		r[i] = getWordStem(token)
	}
	return r
}

func extractStems(text string, stopWords map[string]struct{}) []string {
	tokens := tokenize(text)
	tokens = transformLettersFilter(tokens)
	tokens = stopWordFilter(tokens, stopWords)
	tokens = stemmerFilter(tokens)
	return tokens
}

func (stemStat StemStat) keys() []string {
	result := []string{}
	for k := range stemStat {
		result = append(result, k)
	}
	return result
}

func (stemStat StemStat) addToIndex(docs []Document, stopWords map[string]struct{}, constants map[string]string) {
	titleWeight, _ := strconv.ParseFloat(constants[ARG_WORDS_TITLE_WEIGHT], 64)
	keywordWeight, _ := strconv.ParseFloat(constants[ARG_WORDS_KEYWORDS_WEIGHT], 64)
	for docIndex, doc := range docs {
		docTokenStat := make(map[string]float64)
		docTokenCounter := 0
		for _, content := range doc.Content {
			tokensInContent := extractStems(content, stopWords)
			docTokenCounter += len(tokensInContent)
			for _, token := range tokensInContent {
				docTokenStat[token] += 1.0
			}
		}
		for token, amount := range docTokenStat {
			stemStat[token] = append(stemStat[token], DocStat{
				DocIndex:     docIndex,
				DocFrequency: amount / float64(docTokenCounter),
				DocTags:      doc.Tags,
				DocCategory:  doc.Category,
			})
		}
		if doc.Title != "" {
			tokens := extractStems(doc.Title, stopWords)
			for _, token := range tokens {
				stemStat[token] = append(stemStat[token], DocStat{
					DocIndex:     docIndex,
					DocFrequency: titleWeight * float64(len(stemStat[token])) / float64(len(doc.Title)),
					DocTags:      doc.Tags,
					DocCategory:  doc.Category,
				})
			}
		}
		if doc.Keywords != nil {
			l := len(doc.Keywords)
			for _, keywordPhrase := range doc.Keywords {
				for index, token := range extractStems(keywordPhrase, stopWords) {
					stemStat[token] = append(stemStat[token], DocStat{
						DocIndex:     docIndex,
						DocFrequency: keywordWeight * (1.0 + float64(index)) / float64(l),
						DocTags:      doc.Tags,
						DocCategory:  doc.Category,
					})
				}
			}
		}
	}
	for _, docStat := range stemStat {
		sort.Sort(ByFrequency(docStat))
	}
}

func (stemStat StemStat) findAndInsertVariations(stem string, termVariations []string, stopWords map[string]struct{}) {
	for _, tv := range termVariations {
		if !strings.ContainsAny(tv, " ,!?") {
			newStem := getWordStem(tv)
			if _, ok := stemStat[newStem]; ok {
				stemStat[newStem] = append(stemStat[newStem], stemStat[stem]...)
			} else {
				stemStat[newStem] = stemStat[stem]
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
		counter := 0
		for dTerm, dVars := range dic {
			for _, stem := range stemStat.keys() {
				if !strings.ContainsAny(dTerm, " ,!?") && strings.Contains(dTerm, stem) {
					stemStat.findAndInsertVariations(stem, dVars, stopWords)
					counter++
					break
				}
			}
		}
		log.Printf("%d терминов добавлено из словаря '%s'", counter, file.Name())
	}
}

func editorDistance(token string, stem string) int {
	s1len := len([]rune(token))
	s2len := len([]rune(stem))
	if strings.HasPrefix(stem, token) && float64((s2len-s1len)/s2len) < 0.5 {
		return 0
	}
	column := make([]int, len(token)+1)

	for y := 1; y <= s1len; y++ {
		column[y] = y
	}
	for x := 1; x <= s2len; x++ {
		column[0] = x
		lastkey := x - 1
		for y := 1; y <= s1len; y++ {
			oldkey := column[y]
			var incr int
			if token[y-1] != stem[x-1] {
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

func preproccessRequestTokens(tokens []string, stemKeys []string, constants map[string]string) map[int][]string {
	results := make(map[int][]string)
	limit, _ := strconv.Atoi(constants[ARG_WORDS_DISTANCE_LIMIT])
	for i, t := range tokens {
		closeStems := make(map[int][]string)
		for _, s := range stemKeys {
			if l := editorDistance(t, s); l <= limit {
				closeStems[l] = append(closeStems[l], s)
			} else if l := editorDistance(changeKeyboardLayout(t), s); l <= limit {
				closeStems[l] = append(closeStems[l], s)
			}
		}
		if len(closeStems[0]) > 0 {
			results[i] = append(results[i], closeStems[0]...)
		} else {
			min := len(t)
			for j, _ := range closeStems {
				if j < min {
					min = j
				}
			}
			results[i] = append(results[i], closeStems[min]...)
		}
	}
	return results
}

func mergeDocStat(docStats [][]DocStat, category []string, tags []string, constants map[string]string) []int {
	var result []int = nil
	var stats []DocStat = nil
	for _, docStatForWord := range docStats {
		stats = append(stats, docStatForWord...)
	}
	sort.Sort(ByFrequency(stats))
	limit, _ := strconv.ParseFloat(constants[ARG_WORDS_FREQUENCY_LIMIT], 64)
	minFreqLimit := 0.0
	if len(stats) > 0 {
		minFreqLimit = stats[0].DocFrequency * limit
	}
	for _, s := range stats {
		if s.DocFrequency < minFreqLimit {
			continue
		}
		if len(category) > 0 && category[0] != "" {
			for _, category := range category {
				if category == s.DocCategory {
					if len(tags) > 0 && tags[0] != "" {
						for _, tag := range tags {
							for _, dTag := range s.DocTags {
								if tag == dTag {
									result = append(result, s.DocIndex)
								}
							}
						}
					} else {
						result = append(result, s.DocIndex)
					}
				}
			}
		} else if len(tags) > 0 && tags[0] != "" {
			for _, tag := range tags {
				for _, dTag := range s.DocTags {
					if tag == dTag {
						result = append(result, s.DocIndex)
					}
				}
			}
		} else {
			result = append(result, s.DocIndex)
		}
	}
	return removeDuplicates(result)
}

func intersectDocStat(first []DocStat, second []DocStat) []DocStat {
	result := []DocStat{}
	for _, f := range first {
		for _, s := range second {
			if f.DocIndex == s.DocIndex {
				result = append(result, DocStat{
					DocIndex:     f.DocIndex,
					DocFrequency: f.DocFrequency + s.DocFrequency,
					DocTags:      f.DocTags,
					DocCategory:  f.DocCategory,
				})
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

func getDocIndices(
	words []string,
	stemStat StemStat,
	stemKeys []string,
	stopWords map[string]struct{},
	constants map[string]string,
	category []string,
	tags []string,
) []int {
	var r [][]DocStat
	for wordIndex, word := range words {
		r = append(r, []DocStat{})
		if strings.Contains(word, "+") {
			m := []DocStat{}
			tokens := strings.Split(word, "+")
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
			tokens := strings.Split(word, "-")
			for i, token := range tokens {
				if i == 0 {
					m = append(m, stemStat[token]...)
				} else {
					m = subtractDocStat(m, stemStat[token])
				}
			}
			r[wordIndex] = append(r[wordIndex], m...)
		} else {
			tokens := extractStems(word, stopWords)
			for _, token := range tokens {
				r[wordIndex] = append(r[wordIndex], stemStat[token]...)
			}
		}
	}
	result := mergeDocStat(r, category, tags, constants)
	return result
}

func prepareWords(
	words []string,
	stemKeys []string,
	stopWords map[string]struct{},
	constants map[string]string,
) []string {
	preprocessed := []string{}
	for _, word := range words {
		variants := preproccessRequestTokens(extractStems(word, stopWords), stemKeys, constants)
		if strings.Contains(word, "+") {
			counter := 0
			buffer := []string{}
			for l, vars := range variants {
				varLength := len(vars)
				bufferLength := len(buffer)
				if l > 0 {
					for j := counter - 1; j < bufferLength; j++ {
						for i := 0; i < varLength; i++ {
							buffer = append(buffer, buffer[j]+"+"+vars[i])
						}
					}
				} else {
					buffer = append(buffer, vars...)
				}
				if l < len(variants)-1 {
					counter += varLength
				}
			}
			preprocessed = append(preprocessed, buffer[counter:]...)
		} else if strings.Contains(word, "-") {
			counter := 0
			buffer := []string{}
			for l, vars := range variants {
				varLength := len(vars)
				bufferLength := len(buffer)
				if l > 0 {
					for i := 0; i < varLength; i++ {
						for j := counter - varLength + 1; j < bufferLength; j++ {
							buffer = append(buffer, buffer[j]+"-"+vars[i])
						}
					}
				} else {
					buffer = append(buffer, vars...)
				}
				counter += varLength
			}
			preprocessed = append(preprocessed, buffer[counter:]...)
		} else {
			for _, v := range variants {
				preprocessed = append(preprocessed, v...)
			}
		}
	}
	return preprocessed
}

func getHits(
	host string,
	words []string,
	documents []Document,
	stemStat StemStat,
	stemKeys []string,
	stopWords map[string]struct{},
	constants map[string]string,
	category []string,
	tags []string,
) []Hit {
	defer timeTrackSearch(time.Now(), strings.Join(words, " "), host, category, tags, constants)
	var resultWithFragments []Hit
	preparedWords := prepareWords(words, stemKeys, stopWords, constants)
	for _, index := range getDocIndices(preparedWords, stemStat, stemKeys, stopWords, constants, category, tags) {
		_, title := markWord(words, stopWords, documents[index].Title, constants, false)
		fragments := prepareFragments(words, stopWords, documents, index, constants)
		if len(fragments) > 0 {
			resultWithFragments = append(resultWithFragments, Hit{
				Title:     title,
				Link:      fmt.Sprintf("/%s", documents[index].ObjectId),
				Fragments: fragments,
				Tags:      documents[index].Tags,
				Category:  documents[index].Category,
			})
		}
	}
	return resultWithFragments
}

func markWord(
	words []string,
	stopWords map[string]struct{},
	s string,
	constants map[string]string,
	trim bool,
) (bool, string) {
	distance := constants[ARG_WORDS_DISTANCE_BETWEEN]
	marker := constants[ARG_WORDS_MARKER_TAG]
	occurencesStart, _ := strconv.Atoi(constants[ARG_WORDS_OCCURRENCES])
	aroundRange, _ := strconv.Atoi(constants[ARG_WORDS_AROUND_RANGE])
	lowerCase := strings.ToLower(strings.ReplaceAll(s, "ё", "е"))
	var searchWords []string
	for _, w := range words {
		w = strings.ReplaceAll(w, "ё", "е")
		if strings.Contains(w, "+") {
			searchWords = append(searchWords, strings.ReplaceAll(w, "+", fmt.Sprintf(".{0,%s}", distance)))
		} else if strings.Contains(w, "-") {
			searchWords = append(searchWords, strings.Split(w, "-")...)
		} else {
			searchWords = append(searchWords, w)
		}
		searchWords = append(searchWords, extractStems(w, stopWords)...)
	}
	re := regexp.MustCompile("(" + strings.ToLower(strings.Join(searchWords, "|")) + ")")
	occurrences := re.FindAllIndex([]byte(lowerCase), occurencesStart)
	oLength := len(occurrences)
	if oLength > 0 {
		stack := [][]int{}
		j := 0
		for i, o := range occurrences {
			if i > 0 && o[0] <= occurrences[i-1][1]+2*(aroundRange+(o[1]-o[0])) {
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
			startIndex := Max(indices[0]-aroundRange, 0)
			stopIndex := Min(indices[indicesLength-1]+aroundRange, len(s))
			sPart := s
			sCounter := 0
			bracketsLength := len("<></>") + 2*len(marker)
			for i := indicesLength - 1; i > 0; i -= 2 {
				startWord := indices[i-1]
				stopWord := indices[i]
				sPart = sPart[:startWord] +
					"<" + marker + ">" + sPart[startWord:stopWord] + "</" + marker + ">" +
					sPart[stopWord:]
				sCounter += bracketsLength
			}
			if trim {
				if stopIndex-startIndex < len(sPart)+sCounter {
					sPart := sPart[startIndex : stopIndex+sCounter]
					r = append(r, trimAndWrap(sPart))
				}
			} else {
				r = append(r, sPart)
			}
		}
		return true, strings.Join(r, " ")
	}
	return false, s
}

func prepareFragments(words []string, stopWords map[string]struct{}, documents []Document, docNumber int, constants map[string]string) []string {
	var fragments []string
	for _, p := range documents[docNumber].Content {
		contains, marked := markWord(words, stopWords, p, constants, true)
		if contains {
			fragments = append(fragments, marked)
		}
	}
	return fragments
}

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

func callbackHandler(
	documents []Document,
	stemStat StemStat,
	stemKeys []string,
	stopWords map[string]struct{},
	constants map[string]string,
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		searchTags := []string{}
		searchCategory := []string{}
		searchRequest := prepareSearchRequest(r.URL.Query()["search"][0])
		if r.URL.Query()["tags"] != nil {
			searchTags = r.URL.Query()["tags"]
		}
		if r.URL.Query()["category"] != nil {
			searchCategory = r.URL.Query()["category"]
		}
		hits := getHits(r.RemoteAddr, strings.Split(searchRequest, " "), documents, stemStat, stemKeys, stopWords, constants, searchCategory, searchTags)
		bf := bytes.NewBuffer([]byte{})
		jsonEncoder := json.NewEncoder(bf)
		jsonEncoder.SetEscapeHTML(false)
		jsonEncoder.Encode(hits)
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Access-Control-Allow-Headers, Accept-Encoding, Authorization, Content-Length, Content-Type, X-CSRF-Token, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Content-Type", "application/json")
		w.Write(bf.Bytes())
	}
}

func main() {
	stems := make(StemStat)
	args := loadSettings()
	docs, _ := loadDocuments(args[ARG_SEARCH_CONTENT])
	stopWords, _ := loadStopWords(args[ARG_STOP_WORDS])
	stems.addToIndex(docs, stopWords)
	stems.applyDictionaries(args[ARG_DICTS_DIR], stopWords)
	log.Printf("Формирование поискового индекса завершено. Жду запросов...")
	http.HandleFunc("/", callbackHandler(docs, stems, stems.keys(), stopWords, args))
	log.Fatal(http.ListenAndServe(args[ARG_APP_HOST]+":"+args[ARG_APP_PORT], nil))
}
