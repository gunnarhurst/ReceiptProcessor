package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// structure of a receipt
type Receipt struct {
	Retailer     string `json:"retailer"`
	PurchaseDate string `json:"purchaseDate"`
	PurchaseTime string `json:"purchaseTime"`
	Items        []Item `json:"items"`
	Total        string `json:"total"`
}

type Item struct {
	ShortDescr string `json:"shortDescription"`
	Price      string `json:"price"`
}

type ID struct {
	ID string `json:"id"`
}

type Points struct {
	Points int `json:"points"`
}

type Storage struct {
	receipts map[string]int
	io       sync.RWMutex
}

var store = &Storage{
	receipts: make(map[string]int),
}

func main() {
	router := mux.NewRouter()

	router.HandleFunc("/receipts/process", processReceipt).Methods("POST")
	router.HandleFunc("/receipts/{id}/points", getPoints).Methods("GET")

	log.Printf("Server starting on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", router))
}

// process each receipt and calculate points
func processReceipt(writer http.ResponseWriter, router *http.Request) {
	var receipt Receipt
	if err := json.NewDecoder(router.Body).Decode(&receipt); err != nil {
		http.Error(writer, "Invalid receipt format", http.StatusBadRequest)
		return
	}

	points := calculatePoints(receipt)
	id := uuid.New().String()

	store.io.Lock()
	store.receipts[id] = points
	store.io.Unlock()

	response := ID{ID: id}
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(response)
}

// get points for each receipt
func getPoints(writer http.ResponseWriter, router *http.Request) {
	vars := mux.Vars(router)
	id := vars["id"]

	store.io.RLock()
	points, exists := store.receipts[id]
	store.io.RUnlock()

	if !exists {
		http.Error(writer, "Receipt not found", http.StatusNotFound)
		return
	}

	response := Points{Points: points}
	writer.Header().Set("Content-Type", "application/json")
	json.NewEncoder(writer).Encode(response)
}

// calculate point values according to spec. Returns point amt.
func calculatePoints(receipt Receipt) int {
	points := 0

	// 1: One point for every alphanumeric character in the retailer name
	alphanumeric := regexp.MustCompile(`[a-zA-Z0-9]`)
	points += len(alphanumeric.FindAllString(receipt.Retailer, -1))

	// 2: 50 points if the total is a round dollar amount
	total, _ := strconv.ParseFloat(receipt.Total, 64)
	if total == math.Floor(total) {
		points += 50
	}

	// 3: 25 points if the total is a multiple of 0.25
	if math.Mod(total*100, 25) == 0 {
		points += 25
	}

	// 4: 5 points for every two items
	points += (len(receipt.Items) / 2) * 5

	// 5: Points for items whose description length is a multiple of 3
	for _, item := range receipt.Items {

		trimmedLen := len(strings.TrimSpace(item.ShortDescr))
		if trimmedLen%3 == 0 {
			price, _ := strconv.ParseFloat(item.Price, 64)
			points += int(math.Ceil(price * 0.2))
		}
	}

	// 6: 6 points if the day in the purchase date is odd
	purchaseDate, _ := time.Parse("2006-01-02", receipt.PurchaseDate)
	if purchaseDate.Day()%2 == 1 {
		points += 6
	}

	// 7: 10 points if time is between 2:00pm and 4:00pm
	purchaseTime, _ := time.Parse("15:04", receipt.PurchaseTime)
	targetStart, _ := time.Parse("15:04", "14:00")
	targetEnd, _ := time.Parse("15:04", "16:00")

	if purchaseTime.After(targetStart) && purchaseTime.Before(targetEnd) {
		points += 10
	}

	return points
}
