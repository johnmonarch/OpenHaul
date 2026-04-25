package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/app"
	"github.com/openhaulguard/openhaulguard/internal/apperrors"
	"github.com/openhaulguard/openhaulguard/internal/domain"
	"github.com/openhaulguard/openhaulguard/internal/packet"
	"github.com/openhaulguard/openhaulguard/internal/version"
)

const maxRequestBodyBytes = 1 << 20

var webKeyPattern = regexp.MustCompile(`(?i)(webKey=)[^&\s"']+`)

type Service interface {
	Lookup(context.Context, domain.LookupRequest) (domain.LookupResult, error)
	Diff(context.Context, string, string, string, bool) (domain.DiffResult, error)
	WatchExport(context.Context) (app.WatchExportResult, error)
}

type Server struct {
	service        Service
	defaultOffline bool
	token          string
}

type Option func(*Server)

func WithDefaultOffline(offline bool) Option {
	return func(s *Server) {
		s.defaultOffline = offline
	}
}

func WithToken(token string) Option {
	return func(s *Server) {
		s.token = strings.TrimSpace(token)
	}
}

func NewServer(service Service, opts ...Option) *Server {
	s := &Server{service: service}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Server) Run(ctx context.Context, listen string) error {
	if s.service == nil {
		return errors.New("http api server requires a service")
	}
	if err := ValidateListen(listen, s.token != ""); err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              listen,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/v1/carrier/lookup", s.withAuth(s.carrierLookup))
	mux.HandleFunc("/v1/carrier/diff", s.withAuth(s.carrierDiff))
	mux.HandleFunc("/v1/packet/extract", s.withAuth(s.packetExtract))
	mux.HandleFunc("/v1/packet/check", s.withAuth(s.packetCheck))
	mux.HandleFunc("/v1/watch/export", s.withAuth(s.watchExport))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgs, "endpoint not found", ""), http.StatusNotFound)
	})
	return mux
}

func ValidateListen(listen string, authConfigured bool) error {
	host, _, err := net.SplitHostPort(strings.TrimSpace(listen))
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInvalidArgs, "invalid --listen address", "Use host:port, for example 127.0.0.1:8787", err)
	}
	if isLoopbackHost(host) {
		return nil
	}
	if authConfigured {
		return nil
	}
	return apperrors.New(apperrors.CodeInvalidArgs, "refusing to serve without an API token on a non-loopback address", "Use --api-token or OHG_API_TOKEN, or bind to 127.0.0.1")
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"service":       "openhaulguard_http",
		"version":       version.Version,
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
		"auth_required": s.token != "",
	})
}

func (s *Server) carrierLookup(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var req lookupRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.IdentifierType) == "" || strings.TrimSpace(req.IdentifierValue) == "" {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgs, "identifier_type and identifier_value are required", ""), http.StatusBadRequest)
		return
	}
	maxAge, ok := parseDurationField(w, req.MaxAge, "max_age")
	if !ok {
		return
	}
	offline := s.defaultOffline
	if req.Offline != nil {
		offline = *req.Offline
	}
	result, err := s.service.Lookup(r.Context(), domain.LookupRequest{
		IdentifierType:  req.IdentifierType,
		IdentifierValue: req.IdentifierValue,
		ForceRefresh:    req.ForceRefresh,
		Offline:         offline,
		MaxAge:          maxAge,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sanitizeLookupResult(result))
}

func (s *Server) carrierDiff(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var req diffRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.IdentifierType) == "" || strings.TrimSpace(req.IdentifierValue) == "" {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgs, "identifier_type and identifier_value are required", ""), http.StatusBadRequest)
		return
	}
	since := strings.TrimSpace(req.Since)
	if since == "" {
		since = "90d"
	}
	result, err := s.service.Diff(r.Context(), req.IdentifierType, req.IdentifierValue, since, req.Strict)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) packetExtract(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var req packetPathRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgs, "path is required", ""), http.StatusBadRequest)
		return
	}
	result, err := packet.ExtractReport(r.Context(), req.Path)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) packetCheck(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var req packetCheckRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgs, "path is required", ""), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.IdentifierType) == "" || strings.TrimSpace(req.IdentifierValue) == "" {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgs, "identifier_type and identifier_value are required", ""), http.StatusBadRequest)
		return
	}
	maxAge, ok := parseDurationField(w, req.MaxAge, "max_age")
	if !ok {
		return
	}
	offline := s.defaultOffline
	if req.Offline != nil {
		offline = *req.Offline
	}
	lookup, err := s.service.Lookup(r.Context(), domain.LookupRequest{
		IdentifierType:  req.IdentifierType,
		IdentifierValue: req.IdentifierValue,
		ForceRefresh:    req.ForceRefresh,
		Offline:         offline,
		MaxAge:          maxAge,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	result, err := packet.Check(r.Context(), req.Path, sanitizeLookupResult(lookup))
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) watchExport(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	result, err := s.service.WatchExport(r.Context())
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" {
			next(w, r)
			return
		}
		if tokenFromRequest(r) != s.token {
			writeError(w, apperrors.New(apperrors.CodeAuthAPIInvalid, "invalid or missing API token", "Send Authorization: Bearer <token> or X-OpenHaul-Token"), http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func tokenFromRequest(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):])
	}
	return strings.TrimSpace(r.Header.Get("X-OpenHaul-Token"))
}

func method(w http.ResponseWriter, r *http.Request, allowed string) bool {
	if r.Method == allowed {
		return true
	}
	w.Header().Set("Allow", allowed)
	writeError(w, apperrors.New(apperrors.CodeInvalidArgs, "method not allowed", ""), http.StatusMethodNotAllowed)
	return false
}

func decodeJSONRequest(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, apperrors.Wrap(apperrors.CodeInvalidArgs, "invalid JSON request body", "", err), http.StatusBadRequest)
		return false
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgs, "invalid JSON request body", ""), http.StatusBadRequest)
		return false
	}
	return true
}

func parseDurationField(w http.ResponseWriter, value, field string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, true
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		writeError(w, apperrors.Wrap(apperrors.CodeInvalidArgs, "invalid "+field+" duration", "Use a Go duration like 24h or 30m", err), http.StatusBadRequest)
		return 0, false
	}
	return duration, true
}

func writeAppError(w http.ResponseWriter, err error) {
	writeError(w, err, statusForError(err))
}

func writeError(w http.ResponseWriter, err error, status int) {
	writeJSON(w, status, map[string]any{"error": safeError(err)})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
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

func statusForError(err error) int {
	var ohg *apperrors.OHGError
	if !errors.As(err, &ohg) {
		return http.StatusInternalServerError
	}
	switch ohg.Code {
	case apperrors.CodeInvalidArgs:
		return http.StatusBadRequest
	case apperrors.CodeAuthFMCSAMissing, apperrors.CodeAuthFMCSAInvalid, apperrors.CodeAuthAPIInvalid:
		return http.StatusUnauthorized
	case apperrors.CodeSourceNotFound, apperrors.CodeOfflineCacheMiss:
		return http.StatusNotFound
	case apperrors.CodeSourceRateLimited:
		return http.StatusTooManyRequests
	case apperrors.CodeSourceUnavailable:
		return http.StatusServiceUnavailable
	case apperrors.CodePacketParseFailed:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
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

func redactSecrets(s string) string {
	if s == "" {
		return ""
	}
	return webKeyPattern.ReplaceAllString(s, "${1}REDACTED")
}

type lookupRequest struct {
	IdentifierType  string `json:"identifier_type"`
	IdentifierValue string `json:"identifier_value"`
	ForceRefresh    bool   `json:"force_refresh"`
	Offline         *bool  `json:"offline"`
	MaxAge          string `json:"max_age"`
}

type diffRequest struct {
	IdentifierType  string `json:"identifier_type"`
	IdentifierValue string `json:"identifier_value"`
	Since           string `json:"since"`
	Strict          bool   `json:"strict"`
}

type packetPathRequest struct {
	Path string `json:"path"`
}

type packetCheckRequest struct {
	Path            string `json:"path"`
	IdentifierType  string `json:"identifier_type"`
	IdentifierValue string `json:"identifier_value"`
	ForceRefresh    bool   `json:"force_refresh"`
	Offline         *bool  `json:"offline"`
	MaxAge          string `json:"max_age"`
}

type safeErrorPayload struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	UserAction string `json:"user_action,omitempty"`
	Retryable  bool   `json:"retryable"`
}

func ListenURL(listen string) string {
	if strings.HasPrefix(listen, "http://") || strings.HasPrefix(listen, "https://") {
		return listen
	}
	return fmt.Sprintf("http://%s", listen)
}
