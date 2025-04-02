package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/redis/v3"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var (
	db           *sql.DB
	sessionStore *session.Store
)

func main() {

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		panic(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			login VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL
		)`)
	if err != nil {
		panic(err)
	}

	storage := redis.New(redis.Config{
		URL:   "redis://redis:6379",
		Reset: false,
	})

	sessionStore = session.New(session.Config{
		Storage:        storage,
		Expiration:     24 * time.Hour,
		KeyLookup:      "cookie:session_id",
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
	})

	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost",
		AllowHeaders:     "Origin, Content-Type, Accept",
		AllowCredentials: true,
	}))

	app.Post("/api/register", register)
	app.Post("/api/login", login)
	app.Get("/api/profile", profile)
	app.Post("/api/logout", logout)
	app.Get("/api/data", getData)

	app.Listen(":8080")
}

func register(c *fiber.Ctx) error {
	type Request struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	var exists bool
	err := db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM users WHERE login = $1)",
		req.Login,
	).Scan(&exists)

	if err != nil || exists {
		return c.Status(fiber.StatusConflict).SendString("User already exists")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	_, err = db.Exec(
		"INSERT INTO users (login, password_hash) VALUES ($1, $2)",
		req.Login,
		string(hashed),
	)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Registration failed")
	}

	return c.SendStatus(fiber.StatusCreated)
}

func login(c *fiber.Ctx) error {
	type Request struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	var (
		userID       int
		passwordHash string
	)

	err := db.QueryRow(
		"SELECT id, password_hash FROM users WHERE login = $1",
		req.Login,
	).Scan(&userID, &passwordHash)

	if err != nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	sess, err := sessionStore.Get(c)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	sess.Set("userID", userID)
	if err := sess.Save(); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.SendStatus(fiber.StatusOK)
}

func profile(c *fiber.Ctx) error {
	sess, err := sessionStore.Get(c)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	userID := sess.Get("userID")
	if userID == nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	var login string
	err = db.QueryRow(
		"SELECT login FROM users WHERE id = $1",
		userID,
	).Scan(&login)

	if err != nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	return c.JSON(fiber.Map{"login": login})
}

func logout(c *fiber.Ctx) error {
	sess, err := sessionStore.Get(c)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	if err := sess.Destroy(); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.SendStatus(fiber.StatusOK)
}

func getData(c *fiber.Ctx) error {
	const cacheFile = "/tmp/data_cache.txt"
	const cacheDuration = time.Minute

	if info, err := os.Stat(cacheFile); err == nil {
		if time.Since(info.ModTime()) < cacheDuration {
			data, _ := os.ReadFile(cacheFile)
			return c.Send(data)
		}
	}

	data := []byte(fmt.Sprintf("Data generated at: %s", time.Now().UTC()))
	os.WriteFile(cacheFile, data, 0644)
	return c.Send(data)
}
