package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[1:])
}

func main() {
	var err = godotenv.Load()
	if err != nil {
		log.Fatalf("Не могу получить доступ к файлу '.env': %v", err)
	} else {
		fmt.Println("Значения из файла '.env' получены.")
	}

	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(os.Getenv("APP_HOST")+":"+os.Getenv("APP_PORT"), nil))
}
