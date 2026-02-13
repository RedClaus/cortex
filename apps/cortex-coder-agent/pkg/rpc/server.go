// Package rpc provides the JSON-RPC server for the Cortex Coder Agent
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/RedClaus/cortex-coder-agent/pkg/sdk"
	"github.com/gorilla/websocket"
)

// Server is the JSON-RPC server
type Server struct {
	addr      string
	upgrader  websocket.Upgrader
	sdk       *sdk.CortexCoder
	methods   map[string]Method
	mu        sync.RWMutex
}

// Method represents an RPC method
type Method func(ctx context.Context, params json.RawMessage) (interface{}, error)

// Request represents a JSON-RPC request
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

// Response represents a JSON-RPC response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
	ID      interface{} `json:"id,omitempty"`
}

// Error represents a JSON-RPC error
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewServer creates a new RPC server
func NewServer(addr string, c *sdk.CortexCoder) *Server {
	s := &Server{
		addr:     addr,
		sdk:      c,
		methods:  make(map[string]Method),
		upgrader: websocket.Upgrader{},
	}
	
	// Register default methods
	s.registerDefaultMethods()
	
	return s
}

// registerDefaultMethods registers the default RPC methods
func (s *Server) registerDefaultMethods() {
	s.Register("agent.execute", s.executeAgent)
	s.Register("agent.health", s.health)
	s.Register("skills.list", s.listSkills)
	s.Register("extensions.list", s.listExtensions)
	s.Register("tools.list", s.listTools)
}

// Register registers an RPC method
func (s *Server) Register(name string, method Method) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.methods[name] = method
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, nil, -32700, "parse error")
		return
	}
	
	resp := s.handle(r.Context(), req)
	json.NewEncoder(w).Encode(resp)
}

// HandleWS handles WebSocket connections
func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		
		var req Request
		if err := json.Unmarshal(message, &req); err != nil {
			s.writeWSError(conn, nil, -32700, "parse error")
			continue
		}
		
		resp := s.handle(r.Context(), req)
		responseJSON, _ := json.Marshal(resp)
		conn.WriteMessage(websocket.TextMessage, responseJSON)
	}
}

// handle processes a request
func (s *Server) handle(ctx context.Context, req Request) Response {
	s.mu.RLock()
	method, ok := s.methods[req.Method]
	s.mu.RUnlock()
	
	if !ok {
		return Response{
			JSONRPC: "2.0",
			Error:   &Error{Code: -32601, Message: "method not found"},
			ID:      req.ID,
		}
	}
	
	result, err := method(ctx, req.Params)
	if err != nil {
		return Response{
			JSONRPC: "2.0",
			Error:   &Error{Code: -32000, Message: err.Error()},
			ID:      req.ID,
		}
	}
	
	return Response{
		JSONRPC: "2.0",
		Result:  result,
		ID:      req.ID,
	}
}

// Start starts the RPC server
func (s *Server) Start() error {
	return http.ListenAndServe(s.addr, s)
}

// StartWS starts the WebSocket RPC server
func (s *Server) StartWS() error {
	http.HandleFunc("/ws", s.HandleWS)
	return s.Start()
}

func (s *Server) writeError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := Response{
		JSONRPC: "2.0",
		Error:   &Error{Code: code, Message: message},
		ID:      id,
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) writeWSError(conn *websocket.Conn, id interface{}, code int, message string) {
	resp := Response{
		JSONRPC: "2.0",
		Error:   &Error{Code: code, Message: message},
		ID:      id,
	}
	responseJSON, _ := json.Marshal(resp)
	conn.WriteMessage(websocket.TextMessage, responseJSON)
}

// RPC method implementations
func (s *Server) executeAgent(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var input string
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	return s.sdk.Execute(ctx, input)
}

func (s *Server) health(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return s.sdk.Health(ctx), nil
}

func (s *Server) listSkills(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return s.sdk.Agent().ListSkills(), nil
}

func (s *Server) listExtensions(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return s.sdk.Extensions().List(), nil
}

func (s *Server) listTools(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return s.sdk.Tools().List(), nil
}
