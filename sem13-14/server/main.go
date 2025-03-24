package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB
var cfg *Config

func main() {
	cfg = LoadConfig()

	// DB Connection
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
	)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create tables
	if _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id SERIAL PRIMARY KEY,
            email TEXT UNIQUE NOT NULL,
            password TEXT NOT NULL,
            created_at TIMESTAMP DEFAULT NOW()
        )`); err != nil {
		log.Fatal(err)
	}

	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	// Routes
	app.Post("/api/register", register)
	app.Post("/api/login", login)
	app.Get("/api/me", authMiddleware, getProfile)
	app.Post("/api/refresh", refreshToken)
	app.Get("/api/protected", authMiddleware, protected)

	log.Fatal(app.Listen(":8080"))
}

func authMiddleware(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Authorization header is required"})
	}

	// Разбиваем заголовок по пробелу
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid authorization header format"})
	}

	// Проверяем схему (Bearer)
	if parts[0] != "Bearer" {
		return c.Status(401).JSON(fiber.Map{"error": "Authorization scheme not supported"})
	}

	tokenString := parts[1]
	if tokenString == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Token is empty"})
	}

	// Парсим токен
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid token", "details": err.Error()})
	}

	if !token.Valid {
		return c.Status(401).JSON(fiber.Map{"error": "Token is invalid"})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid token claims"})
	}

	c.Locals("userID", claims["sub"])
	return c.Next()
}

// Handlers
func register(c *fiber.Ctx) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Bad request"})
	}

	// Check existing user
	var exists bool
	db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", req.Email).Scan(&exists)
	if exists {
		return c.Status(400).JSON(fiber.Map{"error": "User exists"})
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}

	// Create user
	_, err = db.Exec("INSERT INTO users (email, password) VALUES ($1, $2)", req.Email, string(hash))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create user"})
	}

	return c.JSON(fiber.Map{"message": "User created"})
}

func login(c *fiber.Ctx) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Bad request"})
	}

	// Get user
	var user struct {
		ID       int
		Email    string
		Password string
	}

	err := db.QueryRow(
		"SELECT id, email, password FROM users WHERE email = $1",
		req.Email,
	).Scan(&user.ID, &user.Email, &user.Password)

	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	// Generate tokens
	accessToken, refreshToken, err := generateTokens(user.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Token error"})
	}

	return c.JSON(fiber.Map{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func generateTokens(userID int) (string, string, error) {
	// Access token (15m)
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(15 * time.Minute).Unix(),
	})

	// Refresh token (7d)
	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(168 * time.Hour).Unix(),
	})

	accessSigned, _ := access.SignedString([]byte(cfg.JWTSecret))
	refreshSigned, _ := refresh.SignedString([]byte(cfg.JWTSecret))

	return accessSigned, refreshSigned, nil
}

func refreshToken(c *fiber.Ctx) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Bad request"})
	}

	token, err := jwt.Parse(req.RefreshToken, func(t *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
	}

	claims := token.Claims.(jwt.MapClaims)
	userID := int(claims["sub"].(float64))

	access, refresh, err := generateTokens(userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Token error"})
	}

	return c.JSON(fiber.Map{
		"access_token":  access,
		"refresh_token": refresh,
	})
}

func getProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(float64)

	var user struct {
		Email     string    `json:"email"`
		CreatedAt time.Time `json:"created_at"`
	}

	err := db.QueryRow(
		"SELECT email, created_at FROM users WHERE id = $1",
		int(userID),
	).Scan(&user.Email, &user.CreatedAt)

	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(user)
}

func protected(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "Secret data",
		"user_id": c.Locals("userID"),
	})
}
