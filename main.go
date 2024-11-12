package main

import (
    "bytes"
    "context"
    "encoding/json"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/joho/godotenv"
    "github.com/rs/cors"
)

type ChatRequest struct {
    Model    string    `json:"model"`
    Messages []Message `json:"messages"`
    Stream   bool      `json:"stream"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type ChatResponse struct {
    Model     string  `json:"model"`
    CreatedAt string  `json:"created_at"`
    Done      bool    `json:"done"`
    Message   Message `json:"message"`
}

func main() {
    if err := godotenv.Load(); err != nil {
        log.Fatalf("Error loading .env file")
    }

    c := cors.New(cors.Options{
        AllowedOrigins: []string{"*"},
        AllowedMethods: []string{"GET", "POST", "OPTIONS"},
        AllowedHeaders: []string{"Content-Type", "Authorization"},
    })

    http.HandleFunc("/api/chat", handleChat)

    handler := c.Handler(http.DefaultServeMux)

    port := ":8080"
    log.Printf("FlowNLP server running on port %s", port)
    log.Fatal(http.ListenAndServe(port, handler))
}

func handleChat(w http.ResponseWriter, r *http.Request) {
    clientIP := r.RemoteAddr
    log.Printf("Received request from %s on /api/chat", clientIP)

    apiKey := r.Header.Get("Authorization")
    if apiKey != "Bearer "+os.Getenv("API_KEY") {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        log.Println("Unauthorized request from", clientIP)
        return
    }

    var chatReq ChatRequest
    if err := json.NewDecoder(r.Body).Decode(&chatReq); err != nil {
        http.Error(w, "Invalid request payload", http.StatusBadRequest)
        log.Println("Invalid request payload from", clientIP)
        return
    }

    if chatReq.Model == "" || len(chatReq.Messages) == 0 {
        http.Error(w, "Missing required fields", http.StatusBadRequest)
        log.Println("Missing required fields in request from", clientIP)
        return
    }

    log.Printf("Request sent: Model: %s, Messages: %v", chatReq.Model, chatReq.Messages)

    ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
    defer cancel()

    ollamaURL := "http://localhost:11434/api/chat"
    reqBody, err := json.Marshal(chatReq)
    if err != nil {
        http.Error(w, "Error creating request", http.StatusInternalServerError)
        log.Println("Error creating request body from", clientIP)
        return
    }

    req, err := http.NewRequestWithContext(ctx, "POST", ollamaURL, ioutil.NopCloser(bytes.NewReader(reqBody)))
    req.Header.Set("Content-Type", "application/json")
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            http.Error(w, "Request timed out", http.StatusGatewayTimeout)
            log.Println("Request timed out from", clientIP)
        } else {
            http.Error(w, "Error contacting Ollama API", http.StatusInternalServerError)
            log.Println("Error contacting Ollama API from", clientIP)
        }
        return
    }
    defer resp.Body.Close()
    var chatResp ChatResponse
    if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
        http.Error(w, "Error reading Ollama response", http.StatusInternalServerError)
        log.Println("Error reading Ollama response from", clientIP)
        return
    }

    log.Printf("Response received from %s: Model: %s, Message: %v, Done: %v", clientIP, chatResp.Model, chatResp.Message, chatResp.Done)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(chatResp)
}
