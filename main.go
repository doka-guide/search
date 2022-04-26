package main

import (
	"bufio"
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

const ARG_MARKER string = "MARKER"
const ARG_DISTANCE_BETWEEN_WORDS string = "DISTANCE_BETWEEN_WORDS"
const ARG_WORDS_TRIMMER_PLACEHOLDER string = "WORDS_TRIMMER_PLACEHOLDER"
const ARG_WORDS_OCCURRENCES string = "WORDS_OCCURRENCES"
const ARG_WORDS_AROUND_RANGE string = "WORDS_AROUND_RANGE"
const ARG_WORDS_DISTANCE_LIMIT string = "WORDS_DISTANCE_LIMIT"

// Значения по умолчанию
const APP_NAME string = "SEARCH-DB-LESS"
const APP_HOST string = ""
const APP_PORT string = "8080"
const APP_LOG_LIMIT int = 10
const MARKER string = "mark"
const DISTANCE_BETWEEN_WORDS int = 20
const WORDS_TRIMMER_PLACEHOLDER string = "..."
const WORDS_OCCURRENCES int = -1
const WORDS_AROUND_RANGE int = 42
const WORDS_DISTANCE_LIMIT int = 3

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
	Title     string
	Link      string
	Fragments []string
	Tags      []string
	Category  string
}

type LogRecord struct {
	RequestTime    string
	RequestHost    string
	SearchRequest  string
	SearchCategory string
	SearchTags     string
	SearchTime     string
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

func loadSettings() map[string]string {
	defer timeTrackLoading(time.Now(), "настроек из файла")
	var err = godotenv.Load()
	if err != nil {
		log.Fatalf("Не могу получить доступ к файлу '.env': %v", err.Error())
		args := os.Args[1:]
		result := make(map[string]string)
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
				result[ARG_MARKER] = args[i+1]
			case "--words-distance-between":
				result[ARG_DISTANCE_BETWEEN_WORDS] = args[i+1]
			case "--words-trimmer-placeholder":
				result[ARG_WORDS_TRIMMER_PLACEHOLDER] = args[i+1]
			case "--words-occurrences`":
				result[ARG_WORDS_OCCURRENCES] = args[i+1]
			case "--words-around-range":
				result[ARG_WORDS_AROUND_RANGE] = args[i+1]
			case "--words-distance-limit":
				result[ARG_WORDS_DISTANCE_LIMIT] = args[i+1]
			}
		}
		return result
	} else {
		fmt.Println("Значения из файла '.env' получены.")
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
		if os.Getenv(ARG_MARKER) != "" {
			result[ARG_MARKER] = os.Getenv(ARG_MARKER)
		} else {
			result[ARG_MARKER] = MARKER
		}
		if os.Getenv(ARG_DISTANCE_BETWEEN_WORDS) != "" {
			result[ARG_DISTANCE_BETWEEN_WORDS] = os.Getenv(ARG_DISTANCE_BETWEEN_WORDS)
		} else {
			result[ARG_DISTANCE_BETWEEN_WORDS] = fmt.Sprintf("%d", DISTANCE_BETWEEN_WORDS)
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

func timeTrackLoading(start time.Time, funcName string) {
	elapsed := time.Since(start)
	log.Printf("Загрузка %s прошла за %s", funcName, elapsed.String())
}

func timeTrackSearch(start time.Time, searchRequest string, host string, category string, tags string, constants map[string]string) {
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
		"%s - %s - %s - %s - %s - %s\n",
		searchLog[logLength-1].RequestTime,
		searchLog[logLength-1].RequestHost,
		searchLog[logLength-1].SearchCategory,
		searchLog[logLength-1].SearchTags,
		searchLog[logLength-1].SearchRequest,
		searchLog[logLength-1].SearchTime,
	)
	limit, _ := strconv.Atoi(constants[ARG_APP_LOG_LIMIT])
	fileName := fmt.Sprintf("%v-%s.log", time.Unix(time.Now().Unix(), 0).UTC(), constants[ARG_APP_NAME])
	if logLength >= limit {
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
				result = append(result, DocStat{
					DocIndex:     oldStat.DocIndex,
					DocFrequency: oldStat.DocFrequency + newStat.DocFrequency,
					DocTags:      oldStat.DocTags,
					DocCategory:  oldStat.DocCategory,
				})
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
			stemStat[token] = append(stemStat[token], DocStat{
				DocIndex:     docIndex,
				DocFrequency: amount / float64(docTokenCounter),
				DocTags:      doc.Tags,
				DocCategory:  doc.Category,
			})
		}
		if doc.Keywords != nil {
			l := len(doc.Keywords)
			for _, keywordPhrase := range doc.Keywords {
				for keywordIndex, keyword := range extractTokens(keywordPhrase, stopWords) {
					stemStat[keyword] = append(stemStat[keyword], DocStat{
						DocIndex:     docIndex,
						DocFrequency: (1.0 + float64(keywordIndex)) / float64(l),
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

func mergeDocStat(docStats [][]DocStat, category string, tags []string) []int {
	var result []int = nil
	var stats []DocStat = nil
	for _, docStatForWord := range docStats {
		stats = append(stats, docStatForWord...)
	}
	sort.Sort(ByFrequency(stats))
	for _, s := range stats {
		if category != "" {
			if s.DocCategory == category {
				result = append(result, s.DocIndex)
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
	category string,
	tags []string,
) []int {
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
	result := mergeDocStat(r, category, tags)
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

func getHits(
	host string,
	words []string,
	documents []Document,
	stemStat StemStat,
	stemKeys []string,
	stopWords map[string]struct{},
	constants map[string]string,
	category string,
	tags []string,
) []Hit {
	defer timeTrackSearch(time.Now(), strings.Join(words, " "), host, category, strings.Join(tags, ", "), constants)
	var result []Hit
	for _, index := range getDocIndices(words, stemStat, stemKeys, stopWords, category, tags) {
		_, title := markWord(words, stopWords, documents[index].Title, constants)
		result = append(result, Hit{
			Title:     title,
			Link:      fmt.Sprintf("/%s", documents[index].ObjectId),
			Fragments: prepareFragments(words, stopWords, documents, index, constants),
			Tags:      documents[index].Tags,
			Category:  documents[index].Category,
		})
	}
	return result
}

// ----------------------------- Подготовка поисковоого ответа ---------------------------------

func markWord(words []string, stopWords map[string]struct{}, s string, constants map[string]string) (bool, string) {
	distance := constants[ARG_DISTANCE_BETWEEN_WORDS]
	marker := constants[ARG_MARKER]
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
		searchWords = append(searchWords, extractTokens(w, stopWords)...)
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

func prepareFragments(words []string, stopWords map[string]struct{}, documents []Document, docNumber int, constants map[string]string) []string {
	var fragments []string
	for _, p := range documents[docNumber].Content {
		contains, marked := markWord(words, stopWords, p, constants)
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

func callbackHandler(documents []Document, stemStat StemStat, stemKeys []string, stopWords map[string]struct{}, constants map[string]string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		searchTags := []string{}
		searchCategory := ""
		searchRequest := prepareSearchRequest(r.URL.Query()["search"][0])
		if r.URL.Query()["tags"] != nil {
			searchTags = r.URL.Query()["tags"]
		}
		if r.URL.Query()["category"] != nil {
			searchCategory = r.URL.Query()["category"][0]
		}
		selectControl := "<select name=\"category\"><option value=\"\">-- Выберете категорию для фильтрации--</option>"
		categoriesKeys := []string{"html", "css", "js", "tools", "recipes"}
		categoriesNames := []string{"HTML", "CSS", "JavaScript", "Инструменты", "Рецепты"}
		for i, c := range categoriesKeys {
			if searchCategory == c {
				selectControl += "<option value=\"" + c + "\" selected>" + categoriesNames[i] + "</option>"
			} else {
				selectControl += "<option value=\"" + c + "\">" + categoriesNames[i] + "</option>"
			}
		}
		selectControl += "</select>"
		fmt.Fprintf(w, "<!DOCTYPE html><html><body><h1>Поиск</h1><form action=\"/\" method=\"get\"><input type=\"text\" name=\"search\" value=\"%s\">%s<input type=\"submit\" value=\"Искать\">", searchRequest, selectControl)
		fmt.Fprintf(w, "<h2>Искали: '%s'</h2>", searchRequest)
		words := prepareWords(strings.Split(searchRequest, " "), stemKeys, stopWords)
		hits := getHits(r.Host, words, documents, stemStat, stemKeys, stopWords, constants, searchCategory, searchTags)
		if len(hits) == 0 {
			searchRequest = changeKeyboardLayout(searchRequest)
			words = prepareWords(strings.Split(searchRequest, " "), stemKeys, stopWords)
			hits = getHits(r.Host, words, documents, stemStat, stemKeys, stopWords, constants, searchCategory, searchTags)
		}
		fmt.Fprintf(w, "<h2>Нашли (%d рез. для '%s' за %s):</h2>", len(hits), strings.Join(words, " "), searchLog[len(searchLog)-1].SearchTime)
		for i, hit := range hits {
			fmt.Fprintf(w, "<a href=\"https://doka.guide%s\"><h3>Hit #%d '%s'</h3></a>", hit.Link, i+1, hit.Title)
			fmt.Fprintf(w, "<h4>Категория: </h4><p>%s<p>", hit.Category)
			fmt.Fprint(w, "<h4>Теги: </h4>")
			fmt.Fprint(w, "<ul>")
			for _, tag := range hit.Tags {
				fmt.Fprintf(w, "<li><a href=\"/?search=#%s\">#%s</a></li>", tag, tag)
			}
			fmt.Fprint(w, "</ul>")
			fmt.Fprint(w, "<h4>Фрагменты текста: </h4>")
			for _, fragment := range hit.Fragments {
				fmt.Fprintf(w, "<p>%s</p>", fragment)
			}
		}
		fmt.Fprintf(w, "</body></html>")
	}
}

func main() {
	stems := make(StemStat)

	args := loadSettings()
	docs, _ := loadDocuments(args[ARG_SEARCH_CONTENT])
	stopWords, _ := loadStopWords(args[ARG_STOP_WORDS])
	stems.addToIndex(docs, stopWords)
	stems.applyDictionaries(args[ARG_DICTS_DIR], stopWords)

	http.HandleFunc("/", callbackHandler(docs, stems, stems.keys(), stopWords, args))
	log.Fatal(http.ListenAndServe(args[ARG_APP_HOST]+":"+args[ARG_APP_PORT], nil))
}
