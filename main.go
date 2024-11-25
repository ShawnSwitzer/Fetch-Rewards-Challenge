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

	"github.com/gin-gonic/gin"

	"github.com/google/uuid"
)

type receipt struct {
	Retailer     string `json:"retailer"`
	PurchaseDate string `json:"purchaseDate"`
	PurchaseTime string `json:"purchaseTime"`
	Items        []struct {
		ShortDescription string `json:"shortDescription"`
		Price            string `json:"price"`
	} `json:"items"`
	Total string `json:"total"`
}

var storeReceipts = struct {
	sync.RWMutex
	receipts map[string]receipt
}{receipts: make(map[string]receipt)}

func generateID() string {
	id := uuid.New()
	return id.String()
}

func processReceipt(c *gin.Context) {
	var rec receipt
	jsonInput := c.PostForm("jsonInput")

	err := json.Unmarshal([]byte(jsonInput), &rec)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decode JSON"})
		return
	}

	id := generateID()

	storeReceipts.Lock()
	storeReceipts.receipts[id] = rec
	storeReceipts.Unlock()

	redirectURL := "/receipt-id.html?id=" + id
	c.Redirect(http.StatusFound, redirectURL)
}

func calculatePoints(rec receipt) int {
	points := 0

	// One point for every alphanumeric character in the retailer name.
	alphaNumRegex := regexp.MustCompile(`[a-zA-z0-9]`)
	alphaNumChars := alphaNumRegex.FindAllString(rec.Retailer, -1)
	points += len(alphaNumChars)

	// 50 points if the total is a round dollar amount with no cents.
	if strings.HasSuffix(rec.Total, ".00") {
		points += 50
	}

	// 25 points if the total is a multiple of 0.25.
	total, err := strconv.ParseFloat(rec.Total, 64)
	if err == nil && math.Mod(total, .25) == 0 {
		points += 25
	}

	// 5 points for every two items on the receipt.
	points += (len(rec.Items) / 2) * 5

	// If the trimmed length of the item description is a multiple of 3, multiply the price by 0.2 and round up to the nearest integer. The result is the number of points earned.
	for _, item := range rec.Items {
		trimItemDesc := strings.TrimSpace(item.ShortDescription)
		if len(trimItemDesc)%3 == 0 {
			price, err := strconv.ParseFloat(item.Price, 64)
			if err == nil {
				points += int(math.Ceil(price * 0.2))
			}
		}
	}

	// 6 points if the day in the purchase date is odd.
	date, err := time.Parse("2006-01-02", rec.PurchaseDate)
	if err == nil && date.Day()%2 != 0 {
		points += 6
	}

	// 10 points if the time of purchase is after 2:00pm and before 4:00pm.
	purchaseTime, err := time.Parse("15:04", rec.PurchaseTime)
	if err == nil {
		if purchaseTime.Hour() >= 14 && purchaseTime.Hour() < 16 {
			points += 10
		}
	}

	return points

}

func getReceiptPoints(c *gin.Context) {
	id := c.Param("id")

	storeReceipts.RLock()
	rec, exists := storeReceipts.receipts[id]
	storeReceipts.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Receipt not found"})
		return
	}

	points := calculatePoints(rec)
	c.JSON(http.StatusOK, gin.H{"points": points})
}

func main() {
	router := gin.Default()
	log.Println("Serving static files from current directory")
	router.StaticFile("/", "./index.html")
	router.StaticFile("/receipt-id.html", "./receipt-id.html")
	router.POST("/receipts/process", processReceipt)
	router.GET("/receipts/:id/points", getReceiptPoints)

	err := router.Run("localhost:8080")
	if err != nil {
		log.Fatal(err)
	}
}
