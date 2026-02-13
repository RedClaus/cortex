package webchat

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/cortexhub/cortex-gateway/internal/channel"
	"github.com/gorilla/websocket"
)

type WebChatAdapter struct {
	port     int
	incoming chan *channel.Message
	upgrader websocket.Upgrader
	conns    map[string]*websocket.Conn
	connMux  sync.RWMutex
	stopCh   chan struct{}
}

type WSMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	UserID  string `json:"user_id,omitempty"`
}

func NewWebChatAdapter(port int) *WebChatAdapter {
	return &WebChatAdapter{
		port:     port,
		incoming: make(chan *channel.Message, 100),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true }, // Allow all origins for now
		},
		conns:   make(map[string]*websocket.Conn),
		stopCh:  make(chan struct{}),
	}
}

func (w *WebChatAdapter) Name() string {
	return "webchat"
}

func (w *WebChatAdapter) IsEnabled() bool {
	return w.port > 0
}

func (w *WebChatAdapter) Start(ctx context.Context) error {
	http.HandleFunc("/ws", w.wsHandler)
	server := &http.Server{Addr: ":" + strconv.Itoa(w.port)}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("WebChat server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
		close(w.stopCh)
	}()

	return nil
}

func (w *WebChatAdapter) Stop() error {
	close(w.incoming)
	return nil
}

func (w *WebChatAdapter) SendMessage(userID string, resp *channel.Response) error {
	w.connMux.RLock()
	conn, exists := w.conns[userID]
	w.connMux.RUnlock()

	if !exists {
		return nil // Connection not found
	}

	msg := WSMessage{
		Type:    "message",
		Content: resp.Content,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, data)
}

func (w *WebChatAdapter) Incoming() <-chan *channel.Message {
	return w.incoming
}

func (w *WebChatAdapter) wsHandler(rw http.ResponseWriter, r *http.Request) {
	conn, err := w.upgrader.Upgrade(rw, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "anonymous_" + strconv.FormatInt(time.Now().Unix(), 10)
	}

	w.connMux.Lock()
	w.conns[userID] = conn
	w.connMux.Unlock()

	defer func() {
		w.connMux.Lock()
		delete(w.conns, userID)
		w.connMux.Unlock()
		conn.Close()
	}()

	for {
		select {
		case <-w.stopCh:
			return
		default:
			var msg WSMessage
			err := conn.ReadJSON(&msg)
			if err != nil {
				log.Printf("WebSocket read error: %v", err)
				return
			}

			if msg.Type == "message" {
				message := &channel.Message{
					ID:       strconv.FormatInt(time.Now().UnixNano(), 10),
					Channel:  "webchat",
					UserID:   userID,
					Content:  msg.Content,
					Metadata: map[string]string{"connection_id": userID},
					Timestamp: time.Now().Unix(),
				}
				w.incoming <- message
			}
		}
	}
}