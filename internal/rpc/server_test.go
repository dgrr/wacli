package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/steipete/wacli/internal/store"
	"go.mau.fi/whatsmeow/types"
)

func setupTestDB(t *testing.T) (*store.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db, func() {
		_ = db.Close()
		_ = os.RemoveAll(dir)
	}
}

func TestServer_Ping(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	srv, err := New(Options{Addr: "localhost:0", DB: db})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", srv.handlePing)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["ok"] != true {
		t.Errorf("expected ok=true, got %v", resp["ok"])
	}
	if resp["pong"] != true {
		t.Errorf("expected pong=true, got %v", resp["pong"])
	}
}

func TestServer_Status(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	srv, err := New(Options{Addr: "localhost:0", DB: db})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	srv.SetSyncRunning(true)

	mux := http.NewServeMux()
	mux.HandleFunc("/status", srv.handleStatus)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp statusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !resp.OK {
		t.Errorf("expected ok=true")
	}
	if !resp.SyncRunning {
		t.Errorf("expected sync_running=true")
	}
}

func TestServer_Chats(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert test chats
	_ = db.UpsertChat("123@s.whatsapp.net", "dm", "Alice", time.Now())
	_ = db.UpsertChat("456@g.us", "group", "Test Group", time.Now())

	srv, err := New(Options{Addr: "localhost:0", DB: db})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/chats", srv.handleChats)

	req := httptest.NewRequest(http.MethodGet, "/chats?limit=10", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp chatsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !resp.OK {
		t.Errorf("expected ok=true")
	}
	if len(resp.Chats) != 2 {
		t.Errorf("expected 2 chats, got %d", len(resp.Chats))
	}
}

func TestServer_Messages(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	chatJID := "123@s.whatsapp.net"
	_ = db.UpsertChat(chatJID, "dm", "Alice", time.Now())
	_ = db.UpsertMessage(store.UpsertMessageParams{
		ChatJID:   chatJID,
		ChatName:  "Alice",
		MsgID:     "msg1",
		SenderJID: chatJID,
		Timestamp: time.Now(),
		FromMe:    false,
		Text:      "Hello!",
	})

	srv, err := New(Options{Addr: "localhost:0", DB: db})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/messages", srv.handleMessages)

	// Test missing chat_jid
	req := httptest.NewRequest(http.MethodGet, "/messages", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 without chat_jid, got %d", w.Code)
	}

	// Test with chat_jid
	req = httptest.NewRequest(http.MethodGet, "/messages?chat_jid="+chatJID, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp messagesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !resp.OK {
		t.Errorf("expected ok=true")
	}
	if len(resp.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(resp.Messages))
	}
	if resp.Messages[0].Text != "Hello!" {
		t.Errorf("expected 'Hello!', got %q", resp.Messages[0].Text)
	}
}

func TestServer_Search(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	chatJID := "123@s.whatsapp.net"
	_ = db.UpsertChat(chatJID, "dm", "Alice", time.Now())
	_ = db.UpsertMessage(store.UpsertMessageParams{
		ChatJID:   chatJID,
		ChatName:  "Alice",
		MsgID:     "msg1",
		SenderJID: chatJID,
		Timestamp: time.Now(),
		FromMe:    false,
		Text:      "Hello world!",
	})
	_ = db.UpsertMessage(store.UpsertMessageParams{
		ChatJID:   chatJID,
		ChatName:  "Alice",
		MsgID:     "msg2",
		SenderJID: chatJID,
		Timestamp: time.Now(),
		FromMe:    false,
		Text:      "Goodbye!",
	})

	srv, err := New(Options{Addr: "localhost:0", DB: db})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/search", srv.handleSearch)

	// Test POST search
	body := `{"query": "world"}`
	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp searchResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !resp.OK {
		t.Errorf("expected ok=true")
	}
	if len(resp.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(resp.Results))
	}

	// Test GET search
	req = httptest.NewRequest(http.MethodGet, "/search?query=goodbye", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for GET search, got %d", w.Code)
	}
}

// mockWA is a mock WhatsApp client for testing.
type mockWA struct {
	connected bool
	sentMsgs  []string
}

func (m *mockWA) IsConnected() bool { return m.connected }
func (m *mockWA) SendText(ctx context.Context, to types.JID, text string) (types.MessageID, error) {
	m.sentMsgs = append(m.sentMsgs, text)
	return "test_msg_id", nil
}
func (m *mockWA) ResolveChatName(ctx context.Context, chat types.JID, pushName string) string {
	return "Test Chat"
}

func TestServer_Send(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	mock := &mockWA{connected: true}

	srv, err := New(Options{Addr: "localhost:0", DB: db, WA: mock})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/send", srv.handleSend)

	// Test send
	body := `{"to": "123456789", "message": "Hello from RPC!"}`
	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp sendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !resp.OK {
		t.Errorf("expected ok=true, got error: %s", resp.Error)
	}
	if resp.MessageID != "test_msg_id" {
		t.Errorf("expected message_id='test_msg_id', got %q", resp.MessageID)
	}
	if len(mock.sentMsgs) != 1 || mock.sentMsgs[0] != "Hello from RPC!" {
		t.Errorf("expected message to be sent, got %v", mock.sentMsgs)
	}
}

func TestServer_Send_NoWA(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	srv, err := New(Options{Addr: "localhost:0", DB: db})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/send", srv.handleSend)

	body := `{"to": "123456789", "message": "Hello!"}`
	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when WA not connected, got %d", w.Code)
	}
}

func TestServer_MethodNotAllowed(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	srv, err := New(Options{Addr: "localhost:0", DB: db})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", srv.handleStatus)
	mux.HandleFunc("/send", srv.handleSend)

	// POST to GET endpoint
	req := httptest.NewRequest(http.MethodPost, "/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST to /status, got %d", w.Code)
	}

	// GET to POST endpoint
	req = httptest.NewRequest(http.MethodGet, "/send", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET to /send, got %d", w.Code)
	}
}
