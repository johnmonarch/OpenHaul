package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/apperrors"
	"github.com/openhaulguard/openhaulguard/internal/domain"
	"github.com/openhaulguard/openhaulguard/internal/packet"
	"github.com/openhaulguard/openhaulguard/internal/version"
)

const (
	defaultProtocolVersion = "2024-11-05"
	safetyLanguage         = "Do not use this tool to declare a carrier fraudulent. Use returned evidence and risk flags for manual review only."
)

var webKeyPattern = regexp.MustCompile(`(?i)(webKey=)[^&\s"']+`)

// Service is the application surface exposed through MCP.
type Service interface {
	Lookup(context.Context, domain.LookupRequest) (domain.LookupResult, error)
	Diff(context.Context, string, string, string, bool) (domain.DiffResult, error)
}

type Server struct {
	service        Service
	in             io.Reader
	out            io.Writer
	defaultOffline bool
}

type Option func(*Server)

func WithDefaultOffline(offline bool) Option {
	return func(s *Server) {
		s.defaultOffline = offline
	}
}

func NewServer(service Service, in io.Reader, out io.Writer, opts ...Option) *Server {
	s := &Server{service: service, in: in, out: out}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Server) Run(ctx context.Context) error {
	if s.service == nil {
		return errors.New("mcp server requires a service")
	}
	reader := bufio.NewReader(s.in)
	for {
		body, err := readMessage(reader)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var req rpcRequest
		if err := json.Unmarshal(body, &req); err != nil {
			if writeErr := writeMessage(s.out, errorResponse(nil, rpcError{Code: -32700, Message: "parse error"})); writeErr != nil {
				return writeErr
			}
			continue
		}
		if !req.hasID {
			continue
		}
		result, callErr := s.handleRequest(ctx, req)
		var resp rpcResponse
		if callErr != nil {
			resp = errorResponse(req.ID, *callErr)
		} else {
			resp = resultResponse(req.ID, result)
		}
		if err := writeMessage(s.out, resp); err != nil {
			return err
		}
	}
}

func (s *Server) handleRequest(ctx context.Context, req rpcRequest) (any, *rpcError) {
	if req.JSONRPC != "2.0" || strings.TrimSpace(req.Method) == "" {
		return nil, invalidRequest("invalid JSON-RPC request")
	}
	switch req.Method {
	case "initialize":
		return initializeResult(req.Params), nil
	case "tools/list":
		return map[string]any{"tools": tools()}, nil
	case "tools/call":
		return s.callTool(ctx, req.Params)
	case "notifications/initialized":
		return map[string]any{}, nil
	default:
		return nil, &rpcError{Code: -32601, Message: "method not found"}
	}
}

func initializeResult(params json.RawMessage) map[string]any {
	protocolVersion := defaultProtocolVersion
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(bytes.TrimSpace(params)) != 0 {
		_ = json.Unmarshal(params, &p)
	}
	if strings.TrimSpace(p.ProtocolVersion) != "" {
		protocolVersion = p.ProtocolVersion
	}
	return map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "openhaulguard",
			"version": version.Version,
		},
		"instructions": safetyLanguage,
	}
}

func tools() []tool {
	return []tool{
		{
			Name:        "carrier_lookup",
			Description: "Lookup a carrier by MC, MX, FF, DOT, or name. Returns public-record facts, local freshness, and risk flags for manual review. " + safetyLanguage,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"identifier_type":  map[string]any{"type": "string", "enum": []string{"mc", "mx", "ff", "dot", "name"}},
					"identifier_value": map[string]any{"type": "string"},
					"force_refresh":    map[string]any{"type": "boolean", "default": false},
					"offline":          map[string]any{"type": "boolean", "default": false},
					"max_age":          map[string]any{"type": "string", "default": "24h"},
				},
				"required": []string{"identifier_type", "identifier_value"},
			},
		},
		{
			Name:        "carrier_diff",
			Description: "Compare local carrier observations over time and return material field changes. " + safetyLanguage,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"identifier_type":  map[string]any{"type": "string", "enum": []string{"mc", "dot"}},
					"identifier_value": map[string]any{"type": "string"},
					"since":            map[string]any{"type": "string", "default": "90d"},
					"strict":           map[string]any{"type": "boolean", "default": false},
				},
				"required": []string{"identifier_type", "identifier_value"},
			},
		},
		{
			Name:        "packet_extract",
			Description: "Extract structured carrier fields from a text or text-based PDF packet. " + safetyLanguage,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "packet_check",
			Description: "Compare a carrier packet against a carrier lookup and return packet/source field differences. " + safetyLanguage,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":             map[string]any{"type": "string"},
					"identifier_type":  map[string]any{"type": "string", "enum": []string{"mc", "mx", "ff", "dot", "name"}},
					"identifier_value": map[string]any{"type": "string"},
					"force_refresh":    map[string]any{"type": "boolean", "default": false},
					"offline":          map[string]any{"type": "boolean", "default": false},
					"max_age":          map[string]any{"type": "string", "default": "24h"},
				},
				"required": []string{"path", "identifier_type", "identifier_value"},
			},
		},
	}
}

func (s *Server) callTool(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if len(bytes.TrimSpace(params)) == 0 {
		return nil, invalidParams("tools/call params are required")
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, invalidParams("invalid tools/call params")
	}
	switch p.Name {
	case "carrier_lookup":
		return s.callCarrierLookup(ctx, p.Arguments)
	case "carrier_diff":
		return s.callCarrierDiff(ctx, p.Arguments)
	case "packet_extract":
		return s.callPacketExtract(ctx, p.Arguments)
	case "packet_check":
		return s.callPacketCheck(ctx, p.Arguments)
	default:
		return nil, invalidParams("unknown tool")
	}
}

func (s *Server) callCarrierLookup(ctx context.Context, raw json.RawMessage) (any, *rpcError) {
	var args struct {
		IdentifierType  string `json:"identifier_type"`
		IdentifierValue string `json:"identifier_value"`
		ForceRefresh    bool   `json:"force_refresh"`
		Offline         *bool  `json:"offline"`
		MaxAge          string `json:"max_age"`
	}
	if err := decodeArguments(raw, &args); err != nil {
		return nil, invalidParams("invalid carrier_lookup arguments")
	}
	if strings.TrimSpace(args.IdentifierType) == "" || strings.TrimSpace(args.IdentifierValue) == "" {
		return nil, invalidParams("carrier_lookup requires identifier_type and identifier_value")
	}
	var maxAge time.Duration
	if strings.TrimSpace(args.MaxAge) != "" {
		parsed, err := time.ParseDuration(args.MaxAge)
		if err != nil {
			return nil, invalidParams("invalid max_age duration")
		}
		maxAge = parsed
	}
	offline := s.defaultOffline
	if args.Offline != nil {
		offline = *args.Offline
	}
	result, err := s.service.Lookup(ctx, domain.LookupRequest{
		IdentifierType:  args.IdentifierType,
		IdentifierValue: args.IdentifierValue,
		ForceRefresh:    args.ForceRefresh,
		Offline:         offline,
		MaxAge:          maxAge,
	})
	if err != nil {
		return toolErrorResult(err), nil
	}
	result = sanitizeLookupResult(result)
	out, err := jsonToolResult(result)
	if err != nil {
		return nil, internalError("could not encode carrier_lookup result")
	}
	return out, nil
}

func (s *Server) callCarrierDiff(ctx context.Context, raw json.RawMessage) (any, *rpcError) {
	var args struct {
		IdentifierType  string `json:"identifier_type"`
		IdentifierValue string `json:"identifier_value"`
		Since           string `json:"since"`
		Strict          bool   `json:"strict"`
	}
	if err := decodeArguments(raw, &args); err != nil {
		return nil, invalidParams("invalid carrier_diff arguments")
	}
	if strings.TrimSpace(args.IdentifierType) == "" || strings.TrimSpace(args.IdentifierValue) == "" {
		return nil, invalidParams("carrier_diff requires identifier_type and identifier_value")
	}
	since := strings.TrimSpace(args.Since)
	if since == "" {
		since = "90d"
	}
	result, err := s.service.Diff(ctx, args.IdentifierType, args.IdentifierValue, since, args.Strict)
	if err != nil {
		return toolErrorResult(err), nil
	}
	out, err := jsonToolResult(result)
	if err != nil {
		return nil, internalError("could not encode carrier_diff result")
	}
	return out, nil
}

func (s *Server) callPacketExtract(ctx context.Context, raw json.RawMessage) (any, *rpcError) {
	var args struct {
		Path string `json:"path"`
	}
	if err := decodeArguments(raw, &args); err != nil {
		return nil, invalidParams("invalid packet_extract arguments")
	}
	if strings.TrimSpace(args.Path) == "" {
		return nil, invalidParams("packet_extract requires path")
	}
	result, err := packet.ExtractReport(ctx, args.Path)
	if err != nil {
		return toolErrorResult(err), nil
	}
	out, err := jsonToolResult(result)
	if err != nil {
		return nil, internalError("could not encode packet_extract result")
	}
	return out, nil
}

func (s *Server) callPacketCheck(ctx context.Context, raw json.RawMessage) (any, *rpcError) {
	var args struct {
		Path            string `json:"path"`
		IdentifierType  string `json:"identifier_type"`
		IdentifierValue string `json:"identifier_value"`
		ForceRefresh    bool   `json:"force_refresh"`
		Offline         *bool  `json:"offline"`
		MaxAge          string `json:"max_age"`
	}
	if err := decodeArguments(raw, &args); err != nil {
		return nil, invalidParams("invalid packet_check arguments")
	}
	if strings.TrimSpace(args.Path) == "" {
		return nil, invalidParams("packet_check requires path")
	}
	if strings.TrimSpace(args.IdentifierType) == "" || strings.TrimSpace(args.IdentifierValue) == "" {
		return nil, invalidParams("packet_check requires identifier_type and identifier_value")
	}
	var maxAge time.Duration
	if strings.TrimSpace(args.MaxAge) != "" {
		parsed, err := time.ParseDuration(args.MaxAge)
		if err != nil {
			return nil, invalidParams("invalid max_age duration")
		}
		maxAge = parsed
	}
	offline := s.defaultOffline
	if args.Offline != nil {
		offline = *args.Offline
	}
	lookup, err := s.service.Lookup(ctx, domain.LookupRequest{
		IdentifierType:  args.IdentifierType,
		IdentifierValue: args.IdentifierValue,
		ForceRefresh:    args.ForceRefresh,
		Offline:         offline,
		MaxAge:          maxAge,
	})
	if err != nil {
		return toolErrorResult(err), nil
	}
	result, err := packet.Check(ctx, args.Path, sanitizeLookupResult(lookup))
	if err != nil {
		return toolErrorResult(err), nil
	}
	out, err := jsonToolResult(result)
	if err != nil {
		return nil, internalError("could not encode packet_check result")
	}
	return out, nil
}

func decodeArguments(raw json.RawMessage, dst any) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	return json.Unmarshal(trimmed, dst)
}

func jsonToolResult(v any) (toolCallResult, error) {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return toolCallResult{}, err
	}
	return toolCallResult{
		Content:           []toolContent{{Type: "text", Text: string(body)}},
		StructuredContent: v,
	}, nil
}

func toolErrorResult(err error) toolCallResult {
	payload := map[string]any{"error": safeError(err)}
	body, marshalErr := json.MarshalIndent(payload, "", "  ")
	if marshalErr != nil {
		body = []byte(`{"error":{"code":"OHG_GENERIC","message":"request failed"}}`)
	}
	return toolCallResult{
		Content:           []toolContent{{Type: "text", Text: string(body)}},
		StructuredContent: payload,
		IsError:           true,
	}
}

func safeError(err error) safeErrorPayload {
	var ohg *apperrors.OHGError
	if errors.As(err, &ohg) {
		return safeErrorPayload{
			Code:       ohg.Code,
			Message:    redactSecrets(ohg.Message),
			UserAction: redactSecrets(ohg.UserAction),
			Retryable:  ohg.Retryable,
		}
	}
	return safeErrorPayload{Code: apperrors.CodeGeneric, Message: "request failed"}
}

func sanitizeLookupResult(result domain.LookupResult) domain.LookupResult {
	for i := range result.Sources {
		result.Sources[i].Endpoint = redactSecrets(result.Sources[i].Endpoint)
		result.Sources[i].RequestURLRedacted = redactSecrets(result.Sources[i].RequestURLRedacted)
		result.Sources[i].ErrorMessage = redactSecrets(result.Sources[i].ErrorMessage)
	}
	for i := range result.Freshness.Sources {
		result.Freshness.Sources[i].Notes = redactSecrets(result.Freshness.Sources[i].Notes)
	}
	for i := range result.Warnings {
		result.Warnings[i].Message = redactSecrets(result.Warnings[i].Message)
		result.Warnings[i].Action = redactSecrets(result.Warnings[i].Action)
	}
	return result
}

func redactSecrets(text string) string {
	if text == "" {
		return ""
	}
	return webKeyPattern.ReplaceAllString(text, "${1}REDACTED")
}

func readMessage(r *bufio.Reader) ([]byte, error) {
	for {
		line, err := r.ReadBytes('\n')
		if err != nil && !(errors.Is(err, io.EOF) && len(line) > 0) {
			return nil, err
		}
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			if errors.Is(err, io.EOF) {
				return nil, io.EOF
			}
			continue
		}
		if bytes.HasPrefix(bytes.ToLower(trimmed), []byte("content-length:")) {
			size, parseErr := strconv.Atoi(strings.TrimSpace(string(trimmed[len("content-length:"):])))
			if parseErr != nil || size < 0 {
				return nil, fmt.Errorf("invalid content length")
			}
			for {
				header, headerErr := r.ReadBytes('\n')
				if headerErr != nil {
					return nil, headerErr
				}
				if len(bytes.TrimSpace(header)) == 0 {
					break
				}
			}
			body := make([]byte, size)
			_, readErr := io.ReadFull(r, body)
			return body, readErr
		}
		return trimmed, nil
	}
}

func writeMessage(w io.Writer, msg any) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	_, err = w.Write(body)
	return err
}

func resultResponse(id json.RawMessage, result any) rpcResponse {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return rpcResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func errorResponse(id json.RawMessage, err rpcError) rpcResponse {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return rpcResponse{JSONRPC: "2.0", ID: id, Error: &err}
}

func invalidRequest(message string) *rpcError {
	return &rpcError{Code: -32600, Message: message}
}

func invalidParams(message string) *rpcError {
	return &rpcError{Code: -32602, Message: message}
}

func internalError(message string) *rpcError {
	return &rpcError{Code: -32603, Message: message}
}

type rpcRequest struct {
	JSONRPC string
	Method  string
	Params  json.RawMessage
	ID      json.RawMessage
	hasID   bool
}

func (r *rpcRequest) UnmarshalJSON(data []byte) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*r = rpcRequest{}
	if raw, ok := obj["jsonrpc"]; ok {
		if err := json.Unmarshal(raw, &r.JSONRPC); err != nil {
			return err
		}
	}
	if raw, ok := obj["method"]; ok {
		if err := json.Unmarshal(raw, &r.Method); err != nil {
			return err
		}
	}
	if raw, ok := obj["params"]; ok {
		r.Params = append(json.RawMessage(nil), raw...)
	}
	if raw, ok := obj["id"]; ok {
		r.ID = append(json.RawMessage(nil), raw...)
		r.hasID = true
	}
	return nil
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type toolCallResult struct {
	Content           []toolContent `json:"content"`
	StructuredContent any           `json:"structuredContent,omitempty"`
	IsError           bool          `json:"isError,omitempty"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type safeErrorPayload struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	UserAction string `json:"user_action,omitempty"`
	Retryable  bool   `json:"retryable"`
}
