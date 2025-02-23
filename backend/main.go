package main

import (
    "database/sql"
    "log"
    "strings"
    "time"

    "bytes"
    "encoding/json"
    "io/ioutil"
    "net/http"
    "os"

    "github.com/dgrijalva/jwt-go"
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/cors"
    "github.com/gofiber/websocket/v2"
    "github.com/joho/godotenv"
    _ "github.com/lib/pq"
)

var db *sql.DB
var jwtSecret = []byte("secret-key")

type User struct {
    ID       int    `json:"id"`
    Username string `json:"username"`
    Password string `json:"password"`
}

type Task struct {
    ID          int       `json:"id"`
    Title       string    `json:"title"`
    Description string    `json:"description"`
    AssignedTo  int       `json:"assigned_to"`
    CreatedAt   time.Time `json:"created_at"`
}

type Claims struct {
    UserID int `json:"user_id"`
    jwt.StandardClaims
}

var clients = make(map[*websocket.Conn]bool)

func main() {
    err := godotenv.Load()
    if err != nil {
        log.Println("Warning: Could not load .env file")
    }

    // Database connection
    dbURL := os.Getenv("DATABASE_URL")
    db, err = sql.Open("postgres", dbURL)

    // db, err = sql.Open("postgres", "user=postgres password=pass123 dbname=task_manager sslmode=disable")
    // if err != nil {
    //     log.Fatal("Database connection error: ", err)
    // }
    // defer db.Close()

    // // Test database connection
    // if err := db.Ping(); err != nil {
    //     log.Fatal("Database ping failed: ", err)
    // }
    // log.Println("Successfully connected to database")

    app := fiber.New()

    app.Use(cors.New(cors.Config{
        AllowOrigins:     "localhost:3000",
        AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
        AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
        AllowCredentials: true,
    }))

    app.Use("/ws", func(c *fiber.Ctx) error {
        if websocket.IsWebSocketUpgrade(c) {
            return c.Next()
        }
        return fiber.ErrUpgradeRequired
    })

    app.Get("/ws/updates", websocket.New(handleWebSocket))

    app.Post("/register", register)
    app.Post("/login", login)
    app.Post("/tasks", authMiddleware, createTask)
    app.Get("/tasks", authMiddleware, getTasks)
    app.Get("/suggest-tasks", authMiddleware, suggestTasks)

    log.Println("Server starting on :3001")
    log.Fatal(app.Listen(":3001"))
}

func register(c *fiber.Ctx) error {
    var user User
    if err := c.BodyParser(&user); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
    }

    _, err := db.Exec("INSERT INTO users (username, password) VALUES ($1, $2)", user.Username, user.Password)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": "User registration failed"})
    }
    return c.JSON(fiber.Map{"message": "User registered successfully"})
}

func login(c *fiber.Ctx) error {
    var user User
    if err := c.BodyParser(&user); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
    }

    var dbUser User
    err := db.QueryRow("SELECT id, username, password FROM users WHERE username=$1", user.Username).Scan(&dbUser.ID, &dbUser.Username, &dbUser.Password)
    if err != nil || dbUser.Password != user.Password {
        return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{UserID: dbUser.ID})
    tokenString, err := token.SignedString(jwtSecret)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": "Token generation failed"})
    }

    return c.JSON(fiber.Map{"token": tokenString})
}

func authMiddleware(c *fiber.Ctx) error {
    authHeader := c.Get("Authorization")
    if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
        return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
    }

    tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
    token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return jwtSecret, nil
    })
    if err != nil || !token.Valid {
        return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
    }

    claims := token.Claims.(*Claims)
    c.Locals("userID", claims.UserID)
    return c.Next()
}

func createTask(c *fiber.Ctx) error {
    var task Task
    if err := c.BodyParser(&task); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
    }

    task.CreatedAt = time.Now()
    task.AssignedTo = c.Locals("userID").(int)

    var taskID int
    err := db.QueryRow(
        "INSERT INTO tasks (title, description, assigned_to, created_at) VALUES ($1, $2, $3, $4) RETURNING id",
        task.Title, task.Description, task.AssignedTo, task.CreatedAt,
    ).Scan(&taskID)

    if err != nil {
        log.Printf("Task creation error: %v", err)
        return c.Status(500).JSON(fiber.Map{"error": "Task creation failed"})
    }

    task.ID = taskID
    broadcastTask(task)
    return c.JSON(task)
}

func getTasks(c *fiber.Ctx) error {
    rows, err := db.Query(
        "SELECT id, title, description, assigned_to, created_at FROM tasks WHERE assigned_to=$1 ORDER BY created_at DESC",
        c.Locals("userID"),
    )
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch tasks"})
    }
    defer rows.Close()

    var tasks []Task
    for rows.Next() {
        var t Task
        if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.AssignedTo, &t.CreatedAt); err != nil {
            return c.Status(500).JSON(fiber.Map{"error": "Error scanning tasks"})
        }
        tasks = append(tasks, t)
    }
    return c.JSON(tasks)
}

func suggestTasks(c *fiber.Ctx) error {
    apiKey := os.Getenv("GEMINI_API_KEY")
    if apiKey == "" {
        log.Println("Gemini API key not found")
        return c.Status(500).JSON(fiber.Map{"error": "API key not configured"})
    }

    url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key=" + apiKey

    requestBody := map[string]interface{}{
        "contents": []map[string]interface{}{
            {
                "parts": []map[string]interface{}{
                    {
                        "text": "Generate 5 specific software development tasks. Format them as a simple list without numbers or bullets. Make them practical and actionable.",
                    },
                },
            },
        },
    }

    jsonBody, err := json.Marshal(requestBody)
    if err != nil {
        log.Printf("Error marshaling request: %v", err)
        return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
    }

    // Debug: Print request body
    log.Printf("Request Body: %s", string(jsonBody))

    req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
    if err != nil {
        log.Printf("Error creating request: %v", err)
        return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
    }

    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("Error making request to Gemini: %v", err)
        return c.Status(500).JSON(fiber.Map{"error": "Failed to get suggestions"})
    }
    defer resp.Body.Close()

    // Debug: Print response status
    log.Printf("Response Status: %s", resp.Status)

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Printf("Error reading response: %v", err)
        return c.Status(500).JSON(fiber.Map{"error": "Failed to read suggestions"})
    }

    // Debug: Print raw response
    log.Printf("Raw Response: %s", string(body))

    var response map[string]interface{}
    if err := json.Unmarshal(body, &response); err != nil {
        log.Printf("Error parsing response: %v", err)
        return c.Status(500).JSON(fiber.Map{"error": "Failed to parse suggestions"})
    }

    var suggestions []string

    // Updated response parsing based on Gemini's response structure
    if candidates, ok := response["candidates"].([]interface{}); ok && len(candidates) > 0 {
        if candidate, ok := candidates[0].(map[string]interface{}); ok {
            if content, ok := candidate["content"].(map[string]interface{}); ok {
                if parts, ok := content["parts"].([]interface{}); ok && len(parts) > 0 {
                    if part, ok := parts[0].(map[string]interface{}); ok {
                        if text, ok := part["text"].(string); ok {
                            // Split the text into lines
                            lines := strings.Split(text, "\n")
                            for _, line := range lines {
                                line = strings.TrimSpace(line)
                                if line != "" {
                                    // Remove any leading numbers, bullets, or special characters
                                    line = strings.TrimLeft(line, "0123456789.- *•")
                                    line = strings.TrimSpace(line)
                                    if line != "" {
                                        suggestions = append(suggestions, line)
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    // Debug: Print processed suggestions
    log.Printf("Processed Suggestions: %v", suggestions)

    if len(suggestions) == 0 {
        log.Println("No suggestions generated from Gemini response")
        return c.Status(500).JSON(fiber.Map{"error": "No suggestions generated"})
    }

    response = fiber.Map{
        "suggestions": suggestions,
        "status":     "success",
    }

    // Debug: Print final response
    log.Printf("Final Response: %+v", response)

    return c.JSON(response)
}

func handleWebSocket(c *websocket.Conn) {
    clients[c] = true
    defer func() {
        delete(clients, c)
        c.Close()
    }()

    for {
        if _, _, err := c.ReadMessage(); err != nil {
            break
        }
    }
}

func broadcastTask(task Task) {
    for client := range clients {
        err := client.WriteJSON(task)
        if err != nil {
            client.Close()
            delete(clients, client)
        }
    }
}