package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
)

var (
	db *sql.DB
)

type Product struct {
	ID    string  `json:"product_id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
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
		os.Getenv("DB_PRODUCT"))

	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
}

func getProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	var product Product
	err := db.
		QueryRow(""+
			"SELECT product_id, name, price, stock "+
			"FROM products "+
			"WHERE product_id = $1", id).
		Scan(&product.ID, &product.Name, &product.Price, &product.Stock)
	if err != nil {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(product)
}

func createProduct(w http.ResponseWriter, r *http.Request) {
	var product Product

	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := db.
		QueryRow(""+
			"INSERT INTO products (name, price, stock) "+
			"VALUES ($1, $2, $3) "+
			"RETURNING product_id",
			product.Name, product.Price, product.Stock,
		).Scan(&product.ID)

	if err != nil {
		http.Error(w, "Failed to create product", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(product)
}

func main() {
	initDB()

	// Menggunakan gorilla/mux untuk routing
	r := mux.NewRouter()
	r.HandleFunc("/product/{id}", getProduct).Methods("GET")
	r.HandleFunc("/product", createProduct).Methods("POST")
	r.Handle("/metrics", promhttp.Handler()) // Endpoint untuk Prometheus

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	fmt.Println("Product Service running on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
