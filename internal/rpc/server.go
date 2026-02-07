// Package rpc provides an HTTP RPC server for wacli that allows concurrent
// operations while sync runs continuously.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/steipete/wacli/internal/logging"
	"github.com/steipete/wacli/internal/store"
	"github.com/steipete/wacli/internal/wa"
	"go.mau.fi/whatsmeow/types"
)

// WAClient defines the interface for WhatsApp operations.
type WAClient interface {
	IsConnected() bool
	SendText(ctx context.Context, to types.JID, text string) (types.MessageID, error)
	ResolveChatName(ctx context.Context, chat types.JID, pushName string) string
}

// Server is the HTTP RPC server.
type Server struct {
	addr string
	db   *store.DB
	wa   WAClient

	server *http.Server
	mu     sync.RWMutex

	syncRunning atomic.Bool
	startTime   time.Time
	log         zerolog.Logger
}

// Options configures the RPC server.
type Options struct {
	Addr string // e.g., "localhost:5555"
	DB   *store.DB
	WA   WAClient
}

// New creates a new RPC server.
func New(opts Options) (*Server, error) {
	if opts.Addr == "" {
		opts.Addr = "localhost:5555"
	}
	if opts.DB == nil {
		return nil, fmt.Errorf("db is required")
	}

	s := &Server{
		addr:      opts.Addr,
		db:        opts.DB,
		wa:        opts.WA,
		startTime: time.Now(),
		log:       logging.WithComponent("rpc"),
	}
	return s, nil
}

// SetWA sets the WhatsApp client (for deferred initialization).
func (s *Server) SetWA(wa WAClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wa = wa
}

// SetSyncRunning updates the sync running status.
func (s *Server) SetSyncRunning(running bool) {
	s.syncRunning.Store(running)
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/chats", s.handleChats)
	mux.HandleFunc("/messages", s.handleMessages)
	mux.HandleFunc("/search", s.handleSearch)
	mux.HandleFunc("/send", s.handleSend)
	mux.HandleFunc("/ping", s.handlePing)

	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.addr, err)
	}

	s.log.Info().Str("addr", s.addr).Msg("RPC server starting")
	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.log.Error().Err(err).Msg("RPC server error")
		}
	}()

	return nil
}

// Stop gracefully stops the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	s.log.Info().Msg("RPC server stopping")
	return s.server.Shutdown(ctx)
}

// Addr returns the listen address.
func (s *Server) Addr() string {
	return s.addr
}

// --- Response helpers ---

type jsonResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, jsonResponse{OK: false, Error: msg})
}

func writeOK(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, data)
}

// --- Handlers ---

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]interface{}{
		"ok":   true,
		"pong": true,
	})
}

type statusResponse struct {
	OK            bool   `json:"ok"`
	SyncRunning   bool   `json:"sync_running"`
	WAConnected   bool   `json:"wa_connected"`
	ChatsCount    int64  `json:"chats_count"`
	MessagesCount int64  `json:"messages_count"`
	Uptime        string `json:"uptime"`
	FTSEnabled    bool   `json:"fts_enabled"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.mu.RLock()
	wa := s.wa
	s.mu.RUnlock()

	waConnected := false
	if wa != nil {
		waConnected = wa.IsConnected()
	}

	// Count chats (fast query)
	var chatsCount int64
	chats, err := s.db.ListChats("", 100000)
	if err == nil {
		chatsCount = int64(len(chats))
	}

	// Count messages
	msgsCount, _ := s.db.CountMessages()

	resp := statusResponse{
		OK:            true,
		SyncRunning:   s.syncRunning.Load(),
		WAConnected:   waConnected,
		ChatsCount:    chatsCount,
		MessagesCount: msgsCount,
		Uptime:        time.Since(s.startTime).Round(time.Second).String(),
		FTSEnabled:    s.db.HasFTS(),
	}
	writeOK(w, resp)
}

type chatJSON struct {
	JID           string `json:"jid"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	LastMessageTS string `json:"last_message_ts"`
}

type chatsResponse struct {
	OK    bool       `json:"ok"`
	Chats []chatJSON `json:"chats"`
}

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	query := r.URL.Query().Get("query")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	chats, err := s.db.ListChats(query, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]chatJSON, len(chats))
	for i, c := range chats {
		out[i] = chatJSON{
			JID:           c.JID,
			Kind:          c.Kind,
			Name:          c.Name,
			LastMessageTS: c.LastMessageTS.Format(time.RFC3339),
		}
	}

	writeOK(w, chatsResponse{OK: true, Chats: out})
}

type messageJSON struct {
	ChatJID     string `json:"chat_jid"`
	ChatName    string `json:"chat_name"`
	MsgID       string `json:"msg_id"`
	SenderJID   string `json:"sender_jid"`
	Timestamp   string `json:"timestamp"`
	FromMe      bool   `json:"from_me"`
	Text        string `json:"text"`
	DisplayText string `json:"display_text"`
	MediaType   string `json:"media_type,omitempty"`
}

type messagesResponse struct {
	OK       bool          `json:"ok"`
	Messages []messageJSON `json:"messages"`
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	chatJID := r.URL.Query().Get("chat_jid")
	if chatJID == "" {
		writeError(w, http.StatusBadRequest, "chat_jid is required")
		return
	}

	limit := 50
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}

	var before, after *time.Time
	if beforeStr := r.URL.Query().Get("before"); beforeStr != "" {
		if t, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			before = &t
		}
	}
	if afterStr := r.URL.Query().Get("after"); afterStr != "" {
		if t, err := time.Parse(time.RFC3339, afterStr); err == nil {
			after = &t
		}
	}

	msgs, err := s.db.ListMessages(store.ListMessagesParams{
		ChatJID: chatJID,
		Limit:   limit,
		Before:  before,
		After:   after,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]messageJSON, len(msgs))
	for i, m := range msgs {
		out[i] = messageJSON{
			ChatJID:     m.ChatJID,
			ChatName:    m.ChatName,
			MsgID:       m.MsgID,
			SenderJID:   m.SenderJID,
			Timestamp:   m.Timestamp.Format(time.RFC3339),
			FromMe:      m.FromMe,
			Text:        m.Text,
			DisplayText: m.DisplayText,
			MediaType:   m.MediaType,
		}
	}

	writeOK(w, messagesResponse{OK: true, Messages: out})
}

type searchRequest struct {
	Query   string `json:"query"`
	ChatJID string `json:"chat_jid"`
	Limit   int    `json:"limit"`
}

type searchResponse struct {
	OK      bool          `json:"ok"`
	Results []messageJSON `json:"results"`
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req searchRequest

	if r.Method == http.MethodGet {
		req.Query = r.URL.Query().Get("query")
		req.ChatJID = r.URL.Query().Get("chat_jid")
		if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
			req.Limit = l
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	}

	if strings.TrimSpace(req.Query) == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}
	if req.Limit <= 0 {
		req.Limit = 50
	}

	msgs, err := s.db.SearchMessages(store.SearchMessagesParams{
		Query:   req.Query,
		ChatJID: req.ChatJID,
		Limit:   req.Limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]messageJSON, len(msgs))
	for i, m := range msgs {
		out[i] = messageJSON{
			ChatJID:     m.ChatJID,
			ChatName:    m.ChatName,
			MsgID:       m.MsgID,
			SenderJID:   m.SenderJID,
			Timestamp:   m.Timestamp.Format(time.RFC3339),
			FromMe:      m.FromMe,
			Text:        m.Text,
			DisplayText: m.DisplayText,
			MediaType:   m.MediaType,
		}
	}

	writeOK(w, searchResponse{OK: true, Results: out})
}

type sendRequest struct {
	To      string `json:"to"`
	Message string `json:"message"`
	ChatJID string `json:"chat_jid"` // alias for 'to'
}

type sendResponse struct {
	OK        bool   `json:"ok"`
	MessageID string `json:"message_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.mu.RLock()
	waClient := s.wa
	s.mu.RUnlock()

	if waClient == nil || !waClient.IsConnected() {
		writeJSON(w, http.StatusServiceUnavailable, sendResponse{
			OK:    false,
			Error: "WhatsApp not connected",
		})
		return
	}

	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, sendResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	to := strings.TrimSpace(req.To)
	if to == "" {
		to = strings.TrimSpace(req.ChatJID)
	}
	if to == "" {
		writeJSON(w, http.StatusBadRequest, sendResponse{
			OK:    false,
			Error: "to or chat_jid is required",
		})
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeJSON(w, http.StatusBadRequest, sendResponse{
			OK:    false,
			Error: "message is required",
		})
		return
	}

	toJID, err := wa.ParseUserOrJID(to)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, sendResponse{
			OK:    false,
			Error: "invalid recipient: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	msgID, err := waClient.SendText(ctx, toJID, req.Message)
	if err != nil {
		s.log.Error().Err(err).Str("to", to).Msg("failed to send message via RPC")
		writeJSON(w, http.StatusInternalServerError, sendResponse{
			OK:    false,
			Error: "send failed: " + err.Error(),
		})
		return
	}

	s.log.Info().Str("to", to).Str("msg_id", string(msgID)).Msg("message sent via RPC")

	// Store the sent message in DB.
	now := time.Now().UTC()
	chatName := waClient.ResolveChatName(ctx, toJID, "")
	kind := "dm"
	if toJID.Server == types.GroupServer {
		kind = "group"
	} else if toJID.IsBroadcastList() {
		kind = "broadcast"
	}
	_ = s.db.UpsertChat(toJID.String(), kind, chatName, now)
	_ = s.db.UpsertMessage(store.UpsertMessageParams{
		ChatJID:    toJID.String(),
		ChatName:   chatName,
		MsgID:      string(msgID),
		SenderJID:  "",
		SenderName: "me",
		Timestamp:  now,
		FromMe:     true,
		Text:       req.Message,
	})

	writeJSON(w, http.StatusOK, sendResponse{
		OK:        true,
		MessageID: string(msgID),
	})
}
