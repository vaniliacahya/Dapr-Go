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

type Customer struct {
	ID    string `json:"customer_id"`
	Name  string `json:"name"`
	Email string `json:"email"`
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
		os.Getenv("DB_CUSTOMER"))

	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
}

// GET /customer/{id}
func getCustomer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	var customer Customer
	err := db.
		QueryRow(""+
			"SELECT customer_id, name, email "+
			"FROM customers "+
			"WHERE customer_id=$1", id).
		Scan(&customer.ID, &customer.Name, &customer.Email)
	if err != nil {
		http.Error(w, "Customer not found"+err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(customer)
}

func createCustomer(w http.ResponseWriter, r *http.Request) {
	var customer Customer

	// Decode JSON Request
	if err := json.NewDecoder(r.Body).Decode(&customer); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Insert Data ke PostgreSQL
	err := db.
		QueryRow(
			""+
				"INSERT INTO customers (name, email) "+
				"VALUES ($1, $2) RETURNING customer_id", customer.Name, customer.Email).
		Scan(&customer.ID)

	if err != nil {
		http.Error(w, "Failed to create customer: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Kirim Response
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(customer)
}

func main() {
	initDB()

	// Menggunakan gorilla/mux untuk routing
	r := mux.NewRouter()
	r.HandleFunc("/customer/{id}", getCustomer).Methods("GET")
	r.HandleFunc("/customer", createCustomer).Methods("POST")
	r.Handle("/metrics", promhttp.Handler()) // Endpoint untuk Prometheus

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	fmt.Println("Customer Service running on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
