package apperrors

import (
	"errors"
	"fmt"
)

const (
	CodeGeneric           = "OHG_GENERIC"
	CodeInvalidArgs       = "OHG_INVALID_ARGS"
	CodeSetupIncomplete   = "OHG_SETUP_INCOMPLETE"
	CodeAuthFMCSAMissing  = "OHG_AUTH_FMCSA_MISSING"
	CodeAuthFMCSAInvalid  = "OHG_AUTH_FMCSA_INVALID"
	CodeSourceUnavailable = "OHG_SOURCE_UNAVAILABLE"
	CodeSourceNotFound    = "OHG_SOURCE_NOT_FOUND"
	CodeDatabase          = "OHG_DB_ERROR"
	CodePacketParseFailed = "OHG_PACKET_PARSE_FAILED"
	CodeOfflineCacheMiss  = "OHG_OFFLINE_CACHE_MISS"
	CodeSourceRateLimited = "OHG_SOURCE_RATE_LIMITED"
)

type OHGError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Cause      error  `json:"-"`
	UserAction string `json:"user_action,omitempty"`
	Retryable  bool   `json:"retryable"`
}

func (e *OHGError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
}

func (e *OHGError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func New(code, message, action string) *OHGError {
	return &OHGError{Code: code, Message: message, UserAction: action}
}

func Wrap(code, message, action string, cause error) *OHGError {
	return &OHGError{Code: code, Message: message, UserAction: action, Cause: cause}
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ohg *OHGError
	if !errors.As(err, &ohg) {
		return 1
	}
	switch ohg.Code {
	case CodeInvalidArgs:
		return 2
	case CodeSetupIncomplete:
		return 3
	case CodeAuthFMCSAMissing, CodeAuthFMCSAInvalid:
		return 4
	case CodeSourceUnavailable, CodeSourceRateLimited:
		return 5
	case CodeSourceNotFound:
		return 6
	case CodeDatabase:
		return 7
	case CodePacketParseFailed:
		return 8
	case CodeOfflineCacheMiss:
		return 10
	default:
		return 1
	}
}
