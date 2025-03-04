package main

import (
	"database/sql"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
	"github.com/gofiber/websocket/v2"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/lib/pq"
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
		categories TEXT[]
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

// ------------------------- REST API HANDLERS -------------------------

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

// @Summary Добавить один или несколько продуктов
// @Tags Products
// @Accept json
// @Produce json
// @Param products body []Product true "Данные продуктов"
// @Success 200 {array} Product "Продукты успешно добавлены"
// @Failure 400 {object} ErrorResponse "Некорректный запрос"
// @Failure 500 {object} ErrorResponse "Ошибка на сервере"
// @Router /api/products [post]
func addProducts(c *fiber.Ctx) error {
	var products []Product

	// Пытаемся распарсить массив продуктов
	if err := c.BodyParser(&products); err != nil {
		// Если не получилось, пробуем одиночный объект
		var singleProduct Product
		if err := c.BodyParser(&singleProduct); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "Invalid request"})
		}
		products = append(products, singleProduct)
	}

	query := "INSERT INTO products (name, price, description, categories) VALUES ($1, $2, $3, $4) RETURNING id"

	for i := range products {
		err := db.QueryRow(query, products[i].Name, products[i].Price, products[i].Description, pq.Array(products[i].Categories)).Scan(&products[i].ID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: err.Error()})
		}
	}

	return c.JSON(products)
}

// @Summary Обновить данные продукта
// @Tags Products
// @Accept json
// @Produce json
// @Param id path int true "ID продукта"
// @Param product body Product true "Данные продукта"
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

// ------------------------- GRAPHQL API -------------------------

// Определяем GraphQL-тип для товара
var productType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Product",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.Int},
			"name":        &graphql.Field{Type: graphql.String},
			"price":       &graphql.Field{Type: graphql.Float},
			"description": &graphql.Field{Type: graphql.String},
			"categories":  &graphql.Field{Type: graphql.NewList(graphql.String)},
		},
	},
)

func createSchema() graphql.Schema {
	// Корневой запрос: получение списка товаров
	rootQuery := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"products": &graphql.Field{
				Type: graphql.NewList(productType),
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					rows, err := db.Query("SELECT id, name, price, description, categories FROM products")
					if err != nil {
						return nil, err
					}
					defer rows.Close()

					var products []Product
					for rows.Next() {
						var product Product
						if err := rows.Scan(&product.ID, &product.Name, &product.Price, &product.Description, pq.Array(&product.Categories)); err != nil {
							return nil, err
						}
						products = append(products, product)
					}
					return products, nil
				},
			},
		},
	})

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: rootQuery,
	})
	if err != nil {
		log.Fatalf("failed to create new schema, error: %v", err)
	}
	return schema
}

// ------------------------- WEBSOCKET CHAT -------------------------

// Message описывает сообщение чата
type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

// clients хранит всех подключённых клиентов
var clients = make(map[*websocket.Conn]bool)

// broadcast канал для рассылки сообщений
var broadcast = make(chan Message)

func handleMessages() {
	for {
		// Принимаем сообщение из канала
		msg := <-broadcast
		// Отправляем сообщение всем клиентам
		for client := range clients {
			if err := client.WriteJSON(msg); err != nil {
				client.Close()
				delete(clients, client)
			}
		}
	}
}

// @title TEST API
// @version 1.0
// @BasePath /
func main() {
	initDB()
	defer db.Close()

	app := fiber.New()

	// REST API endpoints
	api := app.Group("/api")
	api.Get("/products", getProducts)
	api.Post("/products", addProducts)
	api.Put("/products/:id", updateProduct)
	api.Delete("/products/:id", deleteProduct)
	app.Get("/health", func(c *fiber.Ctx) error { return c.SendString("hello") })

	// GraphQL endpoint
	schema := createSchema()
	graphqlHandler := handler.New(&handler.Config{
		Schema: &schema,
		Pretty: true,
	})
	app.All("/graphql", adaptor.HTTPHandler(graphqlHandler))

	// WebSocket endpoint для чата
	go handleMessages() // запуск горутины для рассылки сообщений

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		// Регистрируем клиента
		clients[c] = true
		defer func() {
			delete(clients, c)
			c.Close()
		}()
		for {
			var msg Message
			if err := c.ReadJSON(&msg); err != nil {
				break
			}
			broadcast <- msg
		}
	}))

	// Swagger документация
	app.Get("/swagger/*", swagger.HandlerDefault)

	log.Println("Server running on port 8080")
	log.Fatal(app.Listen(":8080"))
}
