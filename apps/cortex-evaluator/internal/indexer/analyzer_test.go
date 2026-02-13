package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAnalyzer(t *testing.T) {
	analyzer := NewAnalyzer()

	assert.NotNil(t, analyzer)
	assert.NotEmpty(t, analyzer.languagePatterns)
	assert.NotEmpty(t, analyzer.purposePatterns)
}

func TestAnalyzeFile_GoFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "handler.go")

	content := `// Package handler provides HTTP request handling.
package handler

import (
	"encoding/json"
	"net/http"
)

// Response represents an API response.
type Response struct {
	Data    interface{} ` + "`json:\"data\"`" + `
	Message string      ` + "`json:\"message\"`" + `
}

// Handler handles HTTP requests.
type Handler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	HandleCreate(w http.ResponseWriter, r *http.Request)
}

// UserHandler handles user-related requests.
type UserHandler struct {
	db Database
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(db Database) *UserHandler {
	return &UserHandler{db: db}
}

// GetUser retrieves a user by ID.
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(Response{Data: "user"})
}

// Database is the database interface.
type Database interface {
	Query(sql string) ([]Row, error)
	Execute(sql string) error
}

var DefaultTimeout = 30
const MaxRetries = 3
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	assert.Equal(t, "handler.go", analysis.Path)
	assert.Equal(t, "go", analysis.Language)
	assert.Contains(t, analysis.Purpose, "handler")
	assert.False(t, analysis.HasTests)
	assert.False(t, analysis.IsEntryPoint)
	assert.Greater(t, analysis.LineCount, 0)
	assert.Greater(t, analysis.CodeLineCount, 0)

	// Check imports
	assert.Contains(t, analysis.Imports, "encoding/json")
	assert.Contains(t, analysis.Imports, "net/http")

	// Check exports (capitalized types, funcs, etc.)
	// Note: Interface types are detected in analysis.Interfaces, not Exports
	exportNames := make([]string, 0, len(analysis.Exports))
	for _, e := range analysis.Exports {
		exportNames = append(exportNames, e.Name)
	}
	assert.Contains(t, exportNames, "Response")
	assert.Contains(t, exportNames, "UserHandler")
	assert.Contains(t, exportNames, "NewUserHandler")
	assert.Contains(t, exportNames, "DefaultTimeout")
	assert.Contains(t, exportNames, "MaxRetries")

	// Check interfaces
	interfaceNames := make([]string, 0, len(analysis.Interfaces))
	for _, iface := range analysis.Interfaces {
		interfaceNames = append(interfaceNames, iface.Name)
	}
	assert.Contains(t, interfaceNames, "Handler")
	assert.Contains(t, interfaceNames, "Database")

	// Check interface methods
	for _, iface := range analysis.Interfaces {
		if iface.Name == "Handler" {
			assert.Contains(t, iface.Methods, "ServeHTTP")
			assert.Contains(t, iface.Methods, "HandleCreate")
		}
		if iface.Name == "Database" {
			assert.Contains(t, iface.Methods, "Query")
			assert.Contains(t, iface.Methods, "Execute")
		}
	}
}

func TestAnalyzeFile_GoTestFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "handler_test.go")

	content := `package handler

import (
	"testing"
)

func TestNewUserHandler(t *testing.T) {
	handler := NewUserHandler(nil)
	if handler == nil {
		t.Error("expected handler")
	}
}

func TestGetUser(t *testing.T) {
	// test implementation
}
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	assert.True(t, analysis.HasTests)
	assert.Contains(t, analysis.Purpose, "test")
}

func TestAnalyzeFile_GoMainFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "main.go")

	content := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	assert.True(t, analysis.IsEntryPoint)
	assert.Contains(t, analysis.Purpose, "entry point")
}

func TestAnalyzeFile_GoHTTPHandlers(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "routes.go")

	content := `package api

import "net/http"

func SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/users", handleUsers)
	mux.Handle("/api/health", healthHandler)
}

type Router struct {
	mux *http.ServeMux
}

func (r *Router) Setup() {
	r.mux.Get("/users/{id}", r.getUser)
	r.mux.Post("/users", r.createUser)
	r.mux.Delete("/users/{id}", r.deleteUser)
}
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	// Check detected HTTP protocols
	assert.NotEmpty(t, analysis.Protocols)

	endpoints := make([]string, 0, len(analysis.Protocols))
	methods := make([]string, 0, len(analysis.Protocols))
	for _, p := range analysis.Protocols {
		endpoints = append(endpoints, p.Endpoint)
		methods = append(methods, p.Method)
	}

	assert.Contains(t, endpoints, "/api/users")
	assert.Contains(t, endpoints, "/users/{id}")
	assert.Contains(t, endpoints, "/users")
}

func TestAnalyzeFile_TypeScriptFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "service.ts")

	content := `import { Injectable } from '@nestjs/common';
import { HttpService } from '@nestjs/axios';
import type { User } from './types';

export interface UserService {
  getUser(id: string): Promise<User>;
  createUser(data: CreateUserDto): Promise<User>;
}

export interface CreateUserDto {
  name: string;
  email: string;
}

export class UserServiceImpl implements UserService {
  constructor(private readonly http: HttpService) {}

  async getUser(id: string): Promise<User> {
    return this.http.get('/users/' + id);
  }

  async createUser(data: CreateUserDto): Promise<User> {
    return this.http.post('/users', data);
  }
}

export const DEFAULT_TIMEOUT = 5000;
export type UserRole = 'admin' | 'user';

export default class MainService {}
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	assert.Equal(t, "typescript", analysis.Language)
	assert.Contains(t, analysis.Purpose, "service")

	// Check imports
	assert.Contains(t, analysis.Imports, "@nestjs/common")
	assert.Contains(t, analysis.Imports, "@nestjs/axios")
	assert.Contains(t, analysis.Imports, "./types")

	// Check exports
	exportNames := make([]string, 0, len(analysis.Exports))
	for _, e := range analysis.Exports {
		exportNames = append(exportNames, e.Name)
	}
	assert.Contains(t, exportNames, "UserServiceImpl")
	assert.Contains(t, exportNames, "DEFAULT_TIMEOUT")
	assert.Contains(t, exportNames, "UserRole")
	assert.Contains(t, exportNames, "MainService")

	// Check interfaces
	interfaceNames := make([]string, 0, len(analysis.Interfaces))
	for _, iface := range analysis.Interfaces {
		interfaceNames = append(interfaceNames, iface.Name)
	}
	assert.Contains(t, interfaceNames, "UserService")
	assert.Contains(t, interfaceNames, "CreateUserDto")
}

func TestAnalyzeFile_TypeScriptTestFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "service.test.ts")

	content := `import { describe, it, expect } from 'vitest';
import { UserService } from './service';

describe('UserService', () => {
  it('should get user by id', async () => {
    const service = new UserService();
    const user = await service.getUser('123');
    expect(user).toBeDefined();
  });

  test('should create user', async () => {
    // test implementation
  });
});
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	assert.True(t, analysis.HasTests)
	assert.Contains(t, analysis.Purpose, "test")
}

func TestAnalyzeFile_PythonFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "service.py")

	content := `from typing import Protocol, List, Optional
from dataclasses import dataclass
import requests

@dataclass
class User:
    id: str
    name: str
    email: str

class UserRepository(Protocol):
    def get_user(self, id: str) -> Optional[User]:
        ...

    def list_users(self) -> List[User]:
        ...

class UserService:
    def __init__(self, repository: UserRepository):
        self._repository = repository

    def get_user(self, id: str) -> Optional[User]:
        return self._repository.get_user(id)

    def create_user(self, name: str, email: str) -> User:
        return User(id="123", name=name, email=email)

def _private_helper():
    pass

def public_utility():
    return "utility"
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	assert.Equal(t, "python", analysis.Language)
	assert.Contains(t, analysis.Purpose, "service")

	// Check imports
	assert.Contains(t, analysis.Imports, "typing")
	assert.Contains(t, analysis.Imports, "dataclasses")
	assert.Contains(t, analysis.Imports, "requests")

	// Check exports (non-underscore functions and classes)
	exportNames := make([]string, 0, len(analysis.Exports))
	for _, e := range analysis.Exports {
		exportNames = append(exportNames, e.Name)
	}
	assert.Contains(t, exportNames, "User")
	assert.Contains(t, exportNames, "UserRepository")
	assert.Contains(t, exportNames, "UserService")
	assert.Contains(t, exportNames, "public_utility")
	assert.NotContains(t, exportNames, "_private_helper")

	// Check Protocol interfaces
	interfaceNames := make([]string, 0, len(analysis.Interfaces))
	for _, iface := range analysis.Interfaces {
		interfaceNames = append(interfaceNames, iface.Name)
	}
	assert.Contains(t, interfaceNames, "UserRepository")
}

func TestAnalyzeFile_PythonTestFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_service.py")

	content := `import pytest
from service import UserService

def test_get_user():
    service = UserService(None)
    user = service.get_user("123")
    assert user is not None

@pytest.fixture
def mock_repository():
    return MockRepository()

class TestUserService:
    def test_create_user(self):
        pass
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	assert.True(t, analysis.HasTests)
	assert.Contains(t, analysis.Purpose, "test")
}

func TestAnalyzeFile_PythonFastAPIRoutes(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "routes.py")

	content := `from fastapi import FastAPI, APIRouter

app = FastAPI()
router = APIRouter()

@app.get("/health")
def health_check():
    return {"status": "ok"}

@router.post("/users")
async def create_user(user: UserCreate):
    return user

@router.delete("/users/{id}")
async def delete_user(id: str):
    return {"deleted": id}
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	// Check detected HTTP protocols
	assert.NotEmpty(t, analysis.Protocols)

	endpoints := make([]string, 0, len(analysis.Protocols))
	methods := make([]string, 0, len(analysis.Protocols))
	for _, p := range analysis.Protocols {
		endpoints = append(endpoints, p.Endpoint)
		methods = append(methods, p.Method)
	}

	assert.Contains(t, endpoints, "/health")
	assert.Contains(t, endpoints, "/users")
	assert.Contains(t, endpoints, "/users/{id}")
	assert.Contains(t, methods, "GET")
	assert.Contains(t, methods, "POST")
	assert.Contains(t, methods, "DELETE")
}

func TestAnalyzeFile_NonExistentFile(t *testing.T) {
	analyzer := NewAnalyzer()
	_, err := analyzer.AnalyzeFile("/nonexistent/file.go", "/nonexistent")
	assert.Error(t, err)
}

func TestAnalyzeFile_UnknownLanguage(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.xyz")

	content := `some random content
with multiple lines
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	assert.Equal(t, "unknown", analysis.Language)
	assert.Equal(t, 2, analysis.LineCount)
	assert.Equal(t, 2, analysis.CodeLineCount)
}

func TestAnalyzeFile_JSONFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.json")

	content := `{
  "name": "test-app",
  "version": "1.0.0"
}`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	assert.Equal(t, "json", analysis.Language)
	assert.Contains(t, analysis.Purpose, "config")
}

func TestAnalyzeFile_Dockerfile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "Dockerfile")

	content := `FROM golang:1.21
WORKDIR /app
COPY . .
RUN go build -o main .
CMD ["./main"]
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	assert.Contains(t, analysis.Purpose, "container")
}

func TestAnalyzeFile_MiddlewareFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "middleware.go")

	content := `package middleware

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	assert.Contains(t, analysis.Purpose, "middleware")
}

func TestAnalyzeFile_RepositoryFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "user_repository.go")

	content := `package repository

type UserRepository struct {
	db *sql.DB
}

func (r *UserRepository) FindByID(id string) (*User, error) {
	return nil, nil
}
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	assert.Contains(t, analysis.Purpose, "data access")
}

func TestDetectLanguage(t *testing.T) {
	analyzer := NewAnalyzer()

	tests := []struct {
		ext      string
		expected string
	}{
		{".go", "go"},
		{".ts", "typescript"},
		{".tsx", "typescript"},
		{".js", "javascript"},
		{".jsx", "javascript"},
		{".py", "python"},
		{".rs", "rust"},
		{".java", "java"},
		{".proto", "protobuf"},
		{".unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			lang := analyzer.detectLanguage(tt.ext)
			assert.Equal(t, tt.expected, lang)
		})
	}
}

func TestDetectPurpose_ByFilename(t *testing.T) {
	analyzer := NewAnalyzer()

	tests := []struct {
		baseName string
		relPath  string
		contains string
	}{
		{"main.go", "cmd/main.go", "entry point"},
		{"index.ts", "src/index.ts", "entry point"},
		{"handler.go", "internal/handler.go", "handler"},
		{"controller.ts", "src/controller.ts", "controller"},
		{"service.py", "app/service.py", "service"},
		{"repository.go", "internal/repository.go", "data access"},
		{"middleware.ts", "src/middleware.ts", "middleware"},
		{"router.go", "internal/router.go", "routing"},
		{"config.yaml", "config.yaml", "config"},
	}

	for _, tt := range tests {
		t.Run(tt.baseName, func(t *testing.T) {
			purpose := analyzer.detectPurpose(tt.baseName, tt.relPath, nil)
			assert.Contains(t, purpose, tt.contains)
		})
	}
}

func TestDetectTests(t *testing.T) {
	analyzer := NewAnalyzer()

	tests := []struct {
		baseName string
		lang     string
		content  []string
		expected bool
	}{
		{"handler_test.go", "go", nil, true},
		{"service.test.ts", "typescript", nil, true},
		{"service.spec.ts", "typescript", nil, true},
		{"test_service.py", "python", nil, true},
		{"service_test.py", "python", nil, true},
		{"handler.go", "go", []string{"func TestSomething(t *testing.T) {"}, true},
		{"app.ts", "typescript", []string{"describe('test', () => {"}, true},
		{"app.py", "python", []string{"def test_something():"}, true},
		{"handler.go", "go", []string{"func Handle() {}"}, false},
		{"service.ts", "typescript", []string{"export class Service {}"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.baseName, func(t *testing.T) {
			result := analyzer.detectTests(tt.baseName, tt.content, tt.lang)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectEntryPoint(t *testing.T) {
	analyzer := NewAnalyzer()

	tests := []struct {
		baseName string
		lang     string
		content  []string
		expected bool
	}{
		{"main.go", "go", nil, true},
		{"main.py", "python", nil, true},
		{"index.ts", "typescript", nil, true},
		{"index.js", "javascript", nil, true},
		{"app.ts", "typescript", nil, true},
		{"handler.go", "go", []string{"func main() {"}, true},
		{"script.py", "python", []string{"if __name__ == '__main__':"}, true},
		{"handler.go", "go", []string{"func Handle() {}"}, false},
		{"util.py", "python", []string{"def helper():"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.baseName, func(t *testing.T) {
			result := analyzer.detectEntryPoint(tt.baseName, tt.content, tt.lang)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExportKinds(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "types.go")

	content := `package types

type User struct {}
type Config struct {}
func Process() {}
const MaxSize = 100
var GlobalVar = "test"
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	kinds := make(map[string]string)
	for _, e := range analysis.Exports {
		kinds[e.Name] = e.Kind
	}

	assert.Equal(t, "type", kinds["User"])
	assert.Equal(t, "type", kinds["Config"])
	assert.Equal(t, "func", kinds["Process"])
	assert.Equal(t, "const", kinds["MaxSize"])
	assert.Equal(t, "var", kinds["GlobalVar"])
}

func TestLineNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "example.go")

	content := `package example

// Line 3
type First struct{}

// Line 6
type Second struct{}

// Line 9
func Third() {}
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	lines := make(map[string]int)
	for _, e := range analysis.Exports {
		lines[e.Name] = e.Line
	}

	assert.Equal(t, 4, lines["First"])
	assert.Equal(t, 7, lines["Second"])
	assert.Equal(t, 10, lines["Third"])
}

func TestEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.go")

	require.NoError(t, os.WriteFile(filePath, []byte(""), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 0, analysis.LineCount)
	assert.Equal(t, 0, analysis.CodeLineCount)
	assert.Empty(t, analysis.Exports)
	assert.Empty(t, analysis.Imports)
	assert.Empty(t, analysis.Interfaces)
	assert.Empty(t, analysis.Protocols)
}

func TestGRPCDetection(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "server.go")

	content := `package main

import pb "example/proto"

func main() {
	server := grpc.NewServer()
	pb.RegisterUserServiceServer(server, &userServer{})
	server.Serve(lis)
}
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	hasGRPC := false
	for _, p := range analysis.Protocols {
		if p.Type == "grpc" {
			hasGRPC = true
			break
		}
	}
	assert.True(t, hasGRPC, "should detect gRPC protocol")
}

func TestWebSocketDetection(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "client.ts")

	content := `import { io } from 'socket.io-client';

const socket = new WebSocket('ws://localhost:8080');

socket.on('message', (data) => {
  console.log(data);
});
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	analyzer := NewAnalyzer()
	analysis, err := analyzer.AnalyzeFile(filePath, tmpDir)
	require.NoError(t, err)

	hasWebSocket := false
	for _, p := range analysis.Protocols {
		if p.Type == "websocket" {
			hasWebSocket = true
			break
		}
	}
	assert.True(t, hasWebSocket, "should detect WebSocket protocol")
}
