package main

import (
	"database/sql"
	"github.com/gofiber/fiber/v2"
	_ "github.com/lib/pq"
	"log"
)

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("postgres", "user=postgres password=secret dbname=shop sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
}

type Product struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
	Categories  []int   `json:"categories"`
}

func getCard(c *fiber.Ctx) error {
	rows, err := db.Query("SELECT id, name, price, description FROM products")
	if err != nil {
		log.Fatal(err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var product Product
		rows.Scan(&product.ID, &product.Name, &product.Price, &product.Description)
		products = append(products, product)

	}

	return c.Status(fiber.StatusOK).JSON(products)
}

func healthCheck(c *fiber.Ctx) error {
	return c.SendString("hello")
}

func main() {
	initDB()
	defer db.Close()

	app := fiber.New()
	app.Get("/products", getCard)
	app.Get("/health", healthCheck)

	log.Println("Server running on port 8080")
	log.Fatal(app.Listen(":8080"))
}
