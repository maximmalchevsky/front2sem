package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"log"
	"net/http"
)

func main() {
	app := fiber.New()

	app.Use(filesystem.New(filesystem.Config{
		Root:   http.Dir("."),
		Index:  "index.html",
		Browse: false,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("index.html")
	})

	app.Use(func(c *fiber.Ctx) error {
		return c.Status(404).SendFile("404.html")
	})

	log.Println("Сервер запущен на http://localhost:8080")
	err := app.Listen(":8080")
	if err != nil {
		log.Fatal(err)
	}
}
