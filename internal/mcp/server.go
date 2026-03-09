// Package mcp implements a minimal MCP (Model Context Protocol) server
// exposing revise review comments as tools for Claude Code.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/justincampbell/revise/internal/comments"
)

// ConnectedMsg is sent to the TUI when an MCP client connects or disconnects.
type ConnectedMsg struct{ Connected bool }

// jsonrpc types
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type toolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema inputSchema `json:"inputSchema"`
}

type inputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

type property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// Server is a minimal MCP server exposing revise comment file operations as tools.
type Server struct {
	filePath string
	submitCh chan struct{}     // signaled when user submits in TUI (for fast wait_for_review)
	notify   func(interface{}) // optional callback to send messages to TUI
}

// New creates a new MCP server for the given comment file path.
func New(filePath string) *Server {
	return &Server{
		filePath: filePath,
		submitCh: make(chan struct{}, 1),
	}
}

// SetNotify sets a callback that is called when the server wants to send
// a message to the TUI (e.g. ConnectedMsg when a client connects).
func (s *Server) SetNotify(fn func(interface{})) {
	s.notify = fn
}

// NotifySubmit signals that the user has submitted comments in the TUI,
// allowing wait_for_review to return immediately instead of waiting for a poll.
func (s *Server) NotifySubmit() {
	select {
	case s.submitCh <- struct{}{}:
	default:
	}
}

// ServeStdio runs the MCP server on stdin/stdout (standalone mode).
func (s *Server) ServeStdio() error {
	return s.Serve(os.Stdin, os.Stdout)
}

// ListenUnix listens on a Unix domain socket, handling each connection in a goroutine.
// This is the embedded mode used when the TUI is running.
func (s *Server) ListenUnix(socketPath string) error {
	if err := os.MkdirAll("/tmp/revise", 0755); err != nil {
		return err
	}
	// Remove stale socket from a previous run.
	os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", socketPath, err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	if s.notify != nil {
		s.notify(ConnectedMsg{Connected: true})
	}
	defer func() {
		if s.notify != nil {
			s.notify(ConnectedMsg{Connected: false})
		}
	}()
	s.Serve(conn, conn)
}

// Serve handles MCP requests from r, writing responses to w.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	enc := json.NewEncoder(w)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		// Notifications have no ID and need no response.
		if req.ID == nil && isNotification(req.Method) {
			continue
		}

		resp := s.dispatch(req)
		if resp == nil {
			continue
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func isNotification(method string) bool {
	switch method {
	case "notifications/initialized", "notifications/cancelled":
		return true
	}
	return false
}

func (s *Server) dispatch(req request) *response {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return &response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: "method not found: " + req.Method},
		}
	}
}

func (s *Server) handleInitialize(req request) *response {
	return &response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]interface{}{
				"name":    "revise",
				"version": "1.0",
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
		},
	}
}

func (s *Server) handleToolsList(req request) *response {
	tools := []toolDef{
		{
			Name:        "get_comments",
			Description: "Get all open review comments from the current revise session.",
			InputSchema: inputSchema{Type: "object"},
		},
		{
			Name:        "reply_comment",
			Description: "Add a reply to an existing review comment.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"id":   {Type: "string", Description: "Comment ID"},
					"body": {Type: "string", Description: "Reply text"},
				},
				Required: []string{"id", "body"},
			},
		},
		{
			Name:        "close_comment",
			Description: "Mark a review comment as addressed (closed), optionally with an explanation.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"id":   {Type: "string", Description: "Comment ID"},
					"body": {Type: "string", Description: "How the comment was addressed"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "wait_for_review",
			Description: "Block until the user submits new review comments. Returns open comments and resets the submitted flag.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"timeout_seconds": {Type: "number", Description: "Timeout in seconds (default: 86400 = 24h). Omit or pass 0 for the default."},
				},
			},
		},
	}
	return &response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"tools": tools},
	}
}

func (s *Server) handleToolsCall(req request) *response {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResp(req.ID, -32602, "invalid params")
	}

	var result *toolResult
	var err error

	switch params.Name {
	case "get_comments":
		result, err = s.toolGetComments()
	case "reply_comment":
		result, err = s.toolReplyComment(params.Arguments)
	case "close_comment":
		result, err = s.toolCloseComment(params.Arguments)
	case "wait_for_review":
		result, err = s.toolWaitForReview(params.Arguments)
	default:
		return errorResp(req.ID, -32601, "unknown tool: "+params.Name)
	}

	if err != nil {
		return &response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  &toolResult{Content: []toolContent{{Type: "text", Text: err.Error()}}, IsError: true},
		}
	}
	return &response{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s *Server) toolGetComments() (*toolResult, error) {
	rf, err := comments.Read(s.filePath)
	if err != nil {
		return nil, err
	}
	if rf == nil {
		return textResult("No review session found."), nil
	}

	var open []comments.Comment
	for _, c := range rf.Comments {
		if c.Status == "open" {
			open = append(open, c)
		}
	}
	if len(open) == 0 {
		return textResult("No open comments."), nil
	}

	data, err := json.MarshalIndent(open, "", "  ")
	if err != nil {
		return nil, err
	}
	return textResult(string(data)), nil
}

func (s *Server) toolReplyComment(args json.RawMessage) (*toolResult, error) {
	var p struct {
		ID   string `json:"id"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if p.ID == "" || p.Body == "" {
		return nil, fmt.Errorf("id and body are required")
	}

	rf, err := comments.Read(s.filePath)
	if err != nil {
		return nil, err
	}
	if rf == nil {
		return nil, fmt.Errorf("no review session found")
	}

	found := false
	for i, c := range rf.Comments {
		if c.ID == p.ID {
			rf.Comments[i].Replies = append(rf.Comments[i].Replies, comments.Reply{
				Body:      p.Body,
				Timestamp: comments.Timestamp(),
			})
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("comment %q not found", p.ID)
	}

	if err := comments.Write(s.filePath, rf); err != nil {
		return nil, err
	}
	return textResult("Reply added."), nil
}

func (s *Server) toolCloseComment(args json.RawMessage) (*toolResult, error) {
	var p struct {
		ID   string `json:"id"`
		Body string `json:"body,omitempty"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if p.ID == "" {
		return nil, fmt.Errorf("id is required")
	}

	rf, err := comments.Read(s.filePath)
	if err != nil {
		return nil, err
	}
	if rf == nil {
		return nil, fmt.Errorf("no review session found")
	}

	found := false
	for i, c := range rf.Comments {
		if c.ID == p.ID {
			rf.Comments[i].Status = "closed"
			if p.Body != "" {
				rf.Comments[i].Replies = append(rf.Comments[i].Replies, comments.Reply{
					Body:      p.Body,
					Timestamp: comments.Timestamp(),
				})
			}
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("comment %q not found", p.ID)
	}

	if err := comments.Write(s.filePath, rf); err != nil {
		return nil, err
	}
	return textResult("Comment closed."), nil
}

func (s *Server) toolWaitForReview(args json.RawMessage) (*toolResult, error) {
	var p struct {
		TimeoutSeconds float64 `json:"timeout_seconds"`
	}
	if len(args) > 2 {
		json.Unmarshal(args, &p) //nolint:errcheck
	}

	timeout := time.Duration(p.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 24 * time.Hour
	}

	deadline := time.Now().Add(timeout)
	pollInterval := 500 * time.Millisecond

	for time.Now().Before(deadline) {
		// Check for TUI notification (fast path when embedded).
		select {
		case <-s.submitCh:
			return s.readSubmittedComments()
		default:
		}

		// Poll the file (works for both embedded and standalone mode).
		rf, err := comments.Read(s.filePath)
		if err == nil && rf != nil && rf.Submitted {
			return s.readSubmittedComments()
		}

		time.Sleep(pollInterval)
	}

	return textResult("Timeout: no review submitted within the allotted time."), nil
}

func (s *Server) readSubmittedComments() (*toolResult, error) {
	rf, err := comments.Read(s.filePath)
	if err != nil {
		return nil, err
	}
	if rf == nil {
		return textResult("No review file found."), nil
	}

	// Reset the submitted flag so subsequent wait_for_review calls work.
	rf.Submitted = false
	if err := comments.Write(s.filePath, rf); err != nil {
		return nil, fmt.Errorf("resetting submitted flag: %w", err)
	}

	var open []comments.Comment
	for _, c := range rf.Comments {
		if c.Status == "open" {
			open = append(open, c)
		}
	}
	if len(open) == 0 {
		return textResult("Review submitted with no open comments."), nil
	}

	data, err := json.MarshalIndent(open, "", "  ")
	if err != nil {
		return nil, err
	}
	return textResult(string(data)), nil
}

func textResult(text string) *toolResult {
	return &toolResult{Content: []toolContent{{Type: "text", Text: text}}}
}

func errorResp(id interface{}, code int, msg string) *response {
	return &response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: msg},
	}
}
