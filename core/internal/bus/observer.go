package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// DefaultObserverPort is the default port for the WebSocket observer.
	DefaultObserverPort = 8765

	// WebSocketEndpoint is the path for WebSocket connections.
	WebSocketEndpoint = "/brain-events"

	// HealthEndpoint is the path for health checks.
	HealthEndpoint = "/health"

	// WriteWait is the timeout for writing to a WebSocket.
	WriteWait = 10 * time.Second

	// PongWait is the timeout for pong responses.
	PongWait = 60 * time.Second

	// PingPeriod is how often to send ping frames.
	PingPeriod = (PongWait * 9) / 10

	// MaxMessageSize is the maximum message size allowed.
	MaxMessageSize = 512
)

// Observer is a WebSocket server that exposes CortexBrain events to external clients.
// It subscribes to all bus events and forwards them to connected WebSocket clients.
type Observer struct {
	bus      *Bus
	port     int
	upgrader websocket.Upgrader
	server   *http.Server
	
	// Client management
	clients    map[*Client]bool
	clientsMu  sync.RWMutex
	register   chan *Client
	unregister chan *Client
	
	// Control
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	running    bool
	runningMu  sync.RWMutex
}

// Client represents a single WebSocket connection.
type Client struct {
	observer *Observer
	conn     *websocket.Conn
	send     chan []byte
	
	// Configuration
	replayHistory bool
	historyCount  int
}

// ObserverConfig configures the WebSocket observer.
type ObserverConfig struct {
	Port          int
	ReplayHistory bool
	HistoryCount  int
}

// DefaultObserverConfig returns the default observer configuration.
func DefaultObserverConfig() ObserverConfig {
	return ObserverConfig{
		Port:          DefaultObserverPort,
		ReplayHistory: true,
		HistoryCount:  100,
	}
}

// NewObserver creates a new WebSocket observer attached to the given bus.
func NewObserver(bus *Bus, config ObserverConfig) *Observer {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Observer{
		bus:    bus,
		port:   config.Port,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Allow connections from any origin for development
				// In production, restrict this to specific origins
				return true
			},
		},
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins the WebSocket observer.
func (o *Observer) Start() error {
	o.runningMu.Lock()
	if o.running {
		o.runningMu.Unlock()
		return fmt.Errorf("observer already running")
	}
	o.running = true
	o.runningMu.Unlock()

	// Subscribe to all bus events
	o.bus.Subscribe(EventType(""), o.handleBusEvent)

	// Start the client manager
	o.wg.Add(1)
	go o.runClientManager()

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc(WebSocketEndpoint, o.handleWebSocket)
	mux.HandleFunc(HealthEndpoint, o.handleHealth)
	mux.HandleFunc("/", o.handleIndex)

	// CORS wrapper for cross-origin monitor access
	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		mux.ServeHTTP(w, r)
	})

	o.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", o.port),
		Handler: corsHandler,
	}

	// Start the HTTP server
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		log.Printf("[Observer] Starting WebSocket server on :%d", o.port)
		if err := o.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Observer] Server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the observer.
func (o *Observer) Stop() error {
	o.runningMu.Lock()
	if !o.running {
		o.runningMu.Unlock()
		return nil
	}
	o.running = false
	o.runningMu.Unlock()

	// Cancel context
	o.cancel()

	// Close all client connections
	o.clientsMu.Lock()
	for client := range o.clients {
		close(client.send)
		delete(o.clients, client)
	}
	o.clientsMu.Unlock()

	// Shutdown the HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := o.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	// Wait for all goroutines
	o.wg.Wait()
	
	log.Println("[Observer] Server stopped")
	return nil
}

// IsRunning returns whether the observer is currently running.
func (o *Observer) IsRunning() bool {
	o.runningMu.RLock()
	defer o.runningMu.RUnlock()
	return o.running
}

// ClientCount returns the number of connected WebSocket clients.
func (o *Observer) ClientCount() int {
	o.clientsMu.RLock()
	defer o.clientsMu.RUnlock()
	return len(o.clients)
}

// runClientManager handles client registration/unregistration.
func (o *Observer) runClientManager() {
	defer o.wg.Done()
	
	for {
		select {
		case client := <-o.register:
			o.clientsMu.Lock()
			o.clients[client] = true
			o.clientsMu.Unlock()
			log.Printf("[Observer] Client connected (%d total)", len(o.clients))
			
			// Replay history if requested
			if client.replayHistory {
				o.replayHistoryToClient(client, client.historyCount)
			}

		case client := <-o.unregister:
			o.clientsMu.Lock()
			if _, ok := o.clients[client]; ok {
				delete(o.clients, client)
				close(client.send)
				client.conn.Close()
			}
			o.clientsMu.Unlock()
			log.Printf("[Observer] Client disconnected (%d remaining)", len(o.clients))

		case <-o.ctx.Done():
			return
		}
	}
}

// replayHistoryToClient sends recent events to a newly connected client.
func (o *Observer) replayHistoryToClient(client *Client, count int) {
	history := o.bus.GetHistorySlice(count)
	for _, event := range history {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}
		
		select {
		case client.send <- data:
		default:
			// Client channel full, skip
			return
		}
	}
}

// handleWebSocket upgrades HTTP connections to WebSocket.
func (o *Observer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check for replay parameter
	replay := r.URL.Query().Get("replay") != "false"
	count := 100 // default
	if n := r.URL.Query().Get("count"); n != "" {
		fmt.Sscanf(n, "%d", &count)
	}

	conn, err := o.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Observer] WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		observer:      o,
		conn:          conn,
		send:          make(chan []byte, 256),
		replayHistory: replay,
		historyCount:  count,
	}

	o.register <- client

	// Start client goroutines
	o.wg.Add(2)
	go o.writePump(client)
	go o.readPump(client)
}

// writePump handles sending messages to the WebSocket client.
func (o *Observer) writePump(client *Client) {
	defer o.wg.Done()
	
	ticker := time.NewTicker(PingPeriod)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if !ok {
				// Channel closed
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := client.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages
			n := len(client.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-client.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-o.ctx.Done():
			return
		}
	}
}

// readPump handles reading messages from the WebSocket client.
func (o *Observer) readPump(client *Client) {
	defer o.wg.Done()
	defer func() {
		o.unregister <- client
	}()

	client.conn.SetReadLimit(MaxMessageSize)
	client.conn.SetReadDeadline(time.Now().Add(PongWait))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(PongWait))
		return nil
	})

	for {
		_, _, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[Observer] WebSocket error: %v", err)
			}
			break
		}
		// Currently we don't handle incoming messages from clients
		// This could be extended for bidirectional communication
	}
}

// handleBusEvent is called for every event published to the bus.
func (o *Observer) handleBusEvent(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[Observer] Failed to marshal event: %v", err)
		return
	}

	o.clientsMu.RLock()
	clients := make([]*Client, 0, len(o.clients))
	for client := range o.clients {
		clients = append(clients, client)
	}
	o.clientsMu.RUnlock()

	// Send to all clients
	for _, client := range clients {
		select {
		case client.send <- data:
		default:
			// Client channel full, close it
			o.unregister <- client
		}
	}
}

// handleHealth responds to health check requests.
func (o *Observer) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := struct {
		Status      string `json:"status"`
		Service     string `json:"service"`
		Name        string `json:"name"`
		Version     string `json:"version"`
		Port        int    `json:"port"`
		Clients     int    `json:"clients"`
		BusSubs     int    `json:"bus_subscriptions"`
		HistorySize int    `json:"history_size"`
	}{
		Status:      "healthy",
		Service:     "cortex-brain-observer",
		Name:        "CortexBrain",
		Version:     "1.0.0",
		Port:        o.port,
		Clients:     o.ClientCount(),
		BusSubs:     o.bus.SubscriptionsCount(),
		HistorySize: len(o.bus.GetHistory()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleIndex provides basic info at the root endpoint.
func (o *Observer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	info := struct {
		Name        string   `json:"name"`
		Version     string   `json:"version"`
		WebSocket   string   `json:"websocket_endpoint"`
		Health      string   `json:"health_endpoint"`
		EventTypes  []string `json:"event_types"`
	}{
		Name:       "CortexBrain Neural Bus Observer",
		Version:    "1.0.0",
		WebSocket:  WebSocketEndpoint,
		Health:     HealthEndpoint,
		EventTypes: []string{
			string(EventLobeStart),
			string(EventLobeComplete),
			string(EventLobeError),
			string(EventPhaseStart),
			string(EventPhaseComplete),
			string(EventPathway),
			string(EventBlackboard),
			string(EventMessageIn),
			string(EventMessageOut),
			string(EventHeartbeat),
			string(EventLLMRequest),
			string(EventLLMResponse),
			string(EventLLMError),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
