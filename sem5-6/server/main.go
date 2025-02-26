package main

import (
	"database/sql"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"log"
	_ "server/docs"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("postgres", "host=db port=5432 user=postgres password=12345678 dbname=db sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    description TEXT,
    categories TEXT[] -- Массив строк для хранения категорий
);
`)
	if err != nil {
		log.Fatal(err)
	}
}

type Product struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Price       float64  `json:"price"`
	Description string   `json:"description"`
	Categories  []string `json:"categories"`
}

type CreateProductRequest struct {
	Name        string   `json:"name"`
	Price       float64  `json:"price"`
	Description string   `json:"description"`
	Categories  []string `json:"categories"`
}

type UpdateProductRequest struct {
	Name        string   `json:"name"`
	Price       float64  `json:"price"`
	Description string   `json:"description"`
	Categories  []string `json:"categories"`
}

// @Summary Получение списка всех продуктов
// @Tags Products
// @Accept json
// @Produce json
// @Success 200 {array} Product "Успешный ответ"
// @Failure 500 {object} ErrorResponse "Ошибка на сервере"
// @Router /api/products [get]
func getProducts(c *fiber.Ctx) error {
	rows, err := db.Query("SELECT id, name, price, description, categories FROM products")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: err.Error()})
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var product Product
		if err := rows.Scan(&product.ID, &product.Name, &product.Price, &product.Description, pq.Array(&product.Categories)); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: err.Error()})
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: err.Error()})
	}

	return c.JSON(products)
}

// @Summary Добавить новый продукт
// @Tags Products
// @Accept json
// @Produce json
// @Param product body CreateProductRequest true "Данные продукта"
// @Success 200 {object} Product "Продукт успешно добавлен"
// @Failure 400 {object} ErrorResponse "Некорректный запрос"
// @Failure 500 {object} ErrorResponse "Ошибка на сервере"
// @Router /api/products [post]
func addProduct(c *fiber.Ctx) error {
	var product Product
	if err := c.BodyParser(&product); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "Invalid request"})
	}

	// Вставляем продукт с категориями
	query := "INSERT INTO products (name, price, description, categories) VALUES ($1, $2, $3, $4) RETURNING id"
	err := db.QueryRow(query, product.Name, product.Price, product.Description, pq.Array(product.Categories)).Scan(&product.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: err.Error()})
	}
	return c.JSON(product)
}

// @Summary Обновить данные продукта
// @Tags Products
// @Accept json
// @Produce json
// @Param id path int true "ID продукта"
// @Param product body UpdateProductRequest true "Данные продукта"
// @Success 200 {object} map[string]string "Продукт успешно обновлен"
// @Failure 400 {object} ErrorResponse "Некорректный запрос"
// @Failure 500 {object} ErrorResponse "Ошибка на сервере"
// @Router /api/products/{id} [put]
func updateProduct(c *fiber.Ctx) error {
	id := c.Params("id")
	var product Product
	if err := c.BodyParser(&product); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "Invalid request"})
	}

	// Обновляем продукт с новыми категориями
	query := "UPDATE products SET name=$1, price=$2, description=$3, categories=$4 WHERE id=$5"
	_, err := db.Exec(query, product.Name, product.Price, product.Description, pq.Array(product.Categories), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Product updated successfully"})
}

// @Summary Удалить продукт
// @Tags Products
// @Accept json
// @Produce json
// @Param id path int true "ID продукта"
// @Success 200 {object} map[string]string "Продукт успешно удален"
// @Failure 500 {object} ErrorResponse "Ошибка на сервере"
// @Router /api/products/{id} [delete]
func deleteProduct(c *fiber.Ctx) error {
	id := c.Params("id")
	query := "DELETE FROM products WHERE id=$1"
	_, err := db.Exec(query, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Product deleted successfully"})
}

func healthCheck(c *fiber.Ctx) error {
	return c.SendString("hello")
}

// @title TEST API
// @version 1.0
// @BasePath /
func main() {
	initDB()
	defer db.Close()

	app := fiber.New()

	// API endpoints
	app.Get("/products", getProducts)
	app.Post("/products", addProduct)
	app.Put("/products/:id", updateProduct)
	app.Delete("/products/:id", deleteProduct)
	app.Get("/health", healthCheck)

	app.Get("/swagger/*", swagger.HandlerDefault)

	log.Println("Server running on port 8080")
	log.Fatal(app.Listen(":8080"))
}
