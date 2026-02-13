package cortexbrain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:18892", "ws://localhost:18892/bus", "test-token")
	
	assert.Equal(t, "http://localhost:18892", client.baseURL)
	assert.Equal(t, "ws://localhost:18892/bus", client.wsURL)
	assert.Equal(t, "test-token", client.authToken)
	assert.Equal(t, 3, client.maxRetries)
	assert.Equal(t, 1*time.Second, client.retryDelay)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.eventHandlers)
}

func TestSetRetryConfig(t *testing.T) {
	client := NewClient("http://localhost:18892", "ws://localhost:18892/bus", "")
	client.SetRetryConfig(5, 2*time.Second)
	
	assert.Equal(t, 5, client.maxRetries)
	assert.Equal(t, 2*time.Second, client.retryDelay)
}

func TestHealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/health", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthResponse{
			Status:  "healthy",
			Version: "1.0.0",
			Uptime:  "1h30m",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "")
	ctx := context.Background()
	
	health, err := client.HealthCheck(ctx)
	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, "1.0.0", health.Version)
}

func TestPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/ping", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "")
	ctx := context.Background()
	
	err := client.Ping(ctx)
	require.NoError(t, err)
}

func TestSendPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/prompt", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		
		var req PromptRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "test-session", req.SessionID)
		assert.Equal(t, "Hello, CortexBrain!", req.Prompt)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PromptResponse{
			ID:      "resp-123",
			Content: "Hello! How can I help?",
			Type:    "text",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "")
	ctx := context.Background()
	
	resp, err := client.SendPrompt(ctx, PromptRequest{
		SessionID: "test-session",
		Prompt:    "Hello, CortexBrain!",
	})
	require.NoError(t, err)
	assert.Equal(t, "resp-123", resp.ID)
	assert.Equal(t, "Hello! How can I help?", resp.Content)
}

func TestSearchKnowledge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/knowledge/search", r.URL.Path)
		
		var req SearchRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "go error handling", req.Query)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SearchResponse{
			Query: "go error handling",
			Results: []KnowledgeEntry{
				{ID: "1", Content: "Use errors.New() for simple errors"},
				{ID: "2", Content: "Use fmt.Errorf() with wrapping"},
			},
			Total: 2,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "")
	ctx := context.Background()
	
	resp, err := client.SearchKnowledge(ctx, SearchRequest{
		Query: "go error handling",
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Total)
	assert.Len(t, resp.Results, 2)
}

func TestStoreSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/memory/session", r.URL.Path)
		
		var session Session
		json.NewDecoder(r.Body).Decode(&session)
		assert.Equal(t, "session-123", session.ID)
		assert.Equal(t, "test-project", session.Name)
		
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "")
	ctx := context.Background()
	
	err := client.StoreSession(ctx, Session{
		ID:          "session-123",
		Name:        "test-project",
		ProjectPath: "/path/to/project",
	})
	require.NoError(t, err)
}

func TestGetSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/memory/session/session-123", r.URL.Path)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Session{
			ID:          "session-123",
			Name:        "test-project",
			ProjectPath: "/path/to/project",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "")
	ctx := context.Background()
	
	session, err := client.GetSession(ctx, "session-123")
	require.NoError(t, err)
	assert.Equal(t, "session-123", session.ID)
	assert.Equal(t, "test-project", session.Name)
}

func TestHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "")
	ctx := context.Background()
	
	_, err := client.HealthCheck(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestSubscribeAndPublish(t *testing.T) {
	// Create test WebSocket server
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		
		// Echo back any messages
		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				return
			}
			conn.WriteMessage(mt, message)
		}
	}))
	defer wsServer.Close()
	
	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(wsServer.URL, "http")
	
	client := NewClient("http://localhost:18892", wsURL, "")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()
	
	// Subscribe to test event
	received := make(chan Event, 1)
	client.Subscribe("test-event", func(e Event) error {
		received <- e
		return nil
	})
	
	// Publish event
	testEvent := Event{
		Type:    "test-event",
		Payload: map[string]interface{}{"message": "hello"},
	}
	
	err = client.Publish(testEvent)
	require.NoError(t, err)
	
	// Wait for event to be received
	select {
	case event := <-received:
		assert.Equal(t, "test-event", event.Type)
		assert.Equal(t, "hello", event.Payload["message"])
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestIsConnected(t *testing.T) {
	client := NewClient("http://localhost:18892", "ws://localhost:18892/bus", "")
	assert.False(t, client.IsConnected())
}

func TestUnsubscribe(t *testing.T) {
	client := NewClient("http://localhost:18892", "ws://localhost:18892/bus", "")
	
	// Add handler
	client.Subscribe("test", func(e Event) error { return nil })
	
	// Remove handler
	client.Unsubscribe("test")
	
	// Verify it's removed
	client.handlerMutex.RLock()
	_, exists := client.eventHandlers["test"]
	client.handlerMutex.RUnlock()
	
	assert.False(t, exists)
}
