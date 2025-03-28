package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	dapr "github.com/dapr/go-sdk/client"
	_ "github.com/lib/pq"
)

var (
	db           *sql.DB
	daprClient   dapr.Client
	storename    = "statestore"
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP Requests",
		},
		[]string{"method", "endpoint"},
	)
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP Requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
)

type Transaction struct {
	ID         string    `json:"transaction_id"`
	CustomerID string    `json:"customer_id"`
	ProductID  string    `json:"product_id"`
	Qty        int       `json:"qty"`
	TotalPrice float64   `json:"total_price"`
	CreatedAt  time.Time `json:"created_at"`
}

type Customer struct {
	ID   string `json:"customer_id"`
	Name string `json:"name"`
}

type Product struct {
	ID    string  `json:"product_id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func init() {
	prometheus.MustRegister(requestCount)
	prometheus.MustRegister(requestDuration)
}

func initDB() {
	var err error

	// Load .env file
	err = godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_TRANSACTION"))

	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	daprClient, err = dapr.NewClient()
	if err != nil {
		log.Fatal(err)
	}
}

// Gunakan Dapr Service Invocation untuk memanggil Customer Service
func getCustomerByID(customerID string) (*Customer, error) {
	ctx := context.Background()
	resp, err := daprClient.InvokeMethod(ctx, "customer-service", fmt.Sprintf("customer/%s", customerID), "GET")
	if err != nil {
		return nil, err
	}

	var customer Customer
	err = json.Unmarshal(resp, &customer)
	if err != nil {
		return nil, err
	}

	return &customer, nil
}

// Gunakan Dapr Service Invocation untuk memanggil Product Service
func getProductByID(productID string) (*Product, error) {
	ctx := context.Background()
	resp, err := daprClient.InvokeMethod(ctx, "product-service", fmt.Sprintf("product/%s", productID), "GET")
	if err != nil {
		return nil, err
	}

	var product Product
	err = json.Unmarshal(resp, &product)
	if err != nil {
		return nil, err
	}

	return &product, nil
}

// Gunakan Dapr State Store (Redis) untuk menyimpan transaksi sementara
func saveTransactionToCache(transaction Transaction) error {
	ctx := context.Background()
	data, _ := json.Marshal(transaction)

	err := daprClient.SaveState(ctx, "statestore", fmt.Sprintf("transaction-%d", transaction.ID), data, nil)
	return err
}

func createTransaction(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestCount.WithLabelValues("POST", "/transaction").Inc()
	defer func() {
		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues("POST", "/transaction").Observe(duration)
	}()

	var txn Transaction
	body, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(body, &txn)

	// Validasi Customer
	_, err := getCustomerByID(txn.CustomerID)
	if err != nil {
		http.Error(w, "Customer not found"+err.Error(), http.StatusBadRequest)
		return
	}

	// Validasi Product
	product, err := getProductByID(txn.ProductID)
	if err != nil {
		http.Error(w, "Product not found"+err.Error(), http.StatusBadRequest)
		return
	}

	// Hitung total harga
	txn.TotalPrice = float64(txn.Qty) * product.Price

	// Simpan transaksi ke database
	err = db.QueryRow(""+
		"INSERT INTO transactions (customer_id, product_id, qty, total_price, created_at) "+
		"VALUES ($1, $2, $3, $4, NOW()) "+
		"RETURNING transaction_id, created_at",
		txn.CustomerID, txn.ProductID, txn.Qty, txn.TotalPrice).
		Scan(&txn.ID, &txn.CreatedAt)

	if err != nil {
		http.Error(w, "Failed to create transaction"+err.Error(), http.StatusInternalServerError)
		return
	}

	// Simpan ke Redis setelah masuk ke DB
	err = saveTransactionToCache(txn)
	if err != nil {
		http.Error(w, "Failed to cache transaction", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(txn)
}

func getTransaction(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestCount.WithLabelValues("GET", "/transaction/{id}").Inc()
	defer func() {
		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues("GET", "/transaction/{id}").Observe(duration)
	}()

	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	var trx Transaction
	item, err := daprClient.GetState(context.Background(), storename, fmt.Sprintf("transaction-%d", id), nil)
	if err != nil {
		http.Error(w, "Transaction not found"+err.Error(), http.StatusNotFound)
		return
	}

	// Jika data ditemukan di Redis
	if item.Value != nil && len(item.Value) > 0 {
		err = json.Unmarshal(item.Value, &trx)
		json.NewEncoder(w).Encode(trx)
		return
	}

	err = db.
		QueryRow(""+
			"SELECT * "+
			"FROM transactions "+
			"WHERE transaction_id = $1", id).
		Scan(&trx.ID, &trx.CustomerID, &trx.ProductID, &trx.Qty, &trx.TotalPrice, &trx.CreatedAt)
	if err != nil {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	// Simpan ke Redis setelah masuk ke DB
	err = saveTransactionToCache(trx)
	if err != nil {
		http.Error(w, "Failed to cache transaction", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(trx)

}

func main() {
	initDB()
	r := mux.NewRouter()
	r.HandleFunc("/transaction", createTransaction).Methods("POST")
	r.HandleFunc("/transaction/{id}", getTransaction).Methods("GET")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}
	fmt.Println("Transaction Service running on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
