package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestSDKError_Error(t *testing.T) {
	err := New(CodeMissingSigner, "signer is required")
	expected := "[AUTH-001] signer is required"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestSDKError_Is(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{
			name:   "same error",
			err:    ErrMissingSigner,
			target: ErrMissingSigner,
			want:   true,
		},
		{
			name:   "different error",
			err:    ErrMissingSigner,
			target: ErrMissingCreds,
			want:   false,
		},
		{
			name:   "same code different instance",
			err:    New(CodeMissingSigner, "signer is required"),
			target: ErrMissingSigner,
			want:   true,
		},
		{
			name:   "wrapped SDK error",
			err:    errors.Join(ErrInsufficientFunds, errors.New("additional context")),
			target: ErrInsufficientFunds,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errors.Is(tt.err, tt.target)
			if got != tt.want {
				t.Errorf("errors.Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorDefinitions(t *testing.T) {
	// Test that all errors are non-nil and have meaningful messages
	errorTests := []struct {
		name string
		err  *SDKError
		code ErrorCode
	}{
		// Authentication and Authorization errors
		{"ErrMissingSigner", ErrMissingSigner, CodeMissingSigner},
		{"ErrMissingCreds", ErrMissingCreds, CodeMissingCreds},
		{"ErrInvalidSignature", ErrInvalidSignature, CodeInvalidSignature},
		{"ErrUnauthorized", ErrUnauthorized, CodeUnauthorized},

		// Wallet derivation errors
		{"ErrProxyWalletUnsupported", ErrProxyWalletUnsupported, CodeProxyWalletUnsupported},
		{"ErrSafeWalletUnsupported", ErrSafeWalletUnsupported, CodeSafeWalletUnsupported},

		// CLOB API errors
		{"ErrInsufficientFunds", ErrInsufficientFunds, CodeInsufficientFunds},
		{"ErrRateLimitExceeded", ErrRateLimitExceeded, CodeRateLimitExceeded},
		{"ErrOrderNotFound", ErrOrderNotFound, CodeOrderNotFound},
		{"ErrMarketClosed", ErrMarketClosed, CodeMarketClosed},
		{"ErrGeoblocked", ErrGeoblocked, CodeGeoblocked},
		{"ErrInvalidPrice", ErrInvalidPrice, CodeInvalidPrice},
		{"ErrInvalidSize", ErrInvalidSize, CodeInvalidSize},

		// HTTP and Network errors
		{"ErrInternalServerError", ErrInternalServerError, CodeInternalServerError},
		{"ErrBadRequest", ErrBadRequest, CodeBadRequest},
		{"ErrCircuitOpen", ErrCircuitOpen, CodeCircuitOpen},
		{"ErrTooManyRequests", ErrTooManyRequests, CodeTooManyRequests},

		// Data API errors
		{"ErrMissingRequest", ErrMissingRequest, CodeMissingRequest},
		{"ErrMissingUser", ErrMissingUser, CodeMissingUser},
		{"ErrInvalidMarketFilter", ErrInvalidMarketFilter, CodeInvalidMarketFilter},
		{"ErrInvalidTradeFilter", ErrInvalidTradeFilter, CodeInvalidTradeFilter},

		// WebSocket errors
		{"ErrInvalidSubscription", ErrInvalidSubscription, CodeInvalidSubscription},

		// CTF errors
		{"ErrMissingU256Value", ErrMissingU256Value, CodeMissingU256Value},
		{"ErrMissingBackend", ErrMissingBackend, CodeMissingBackend},
		{"ErrMissingTransactor", ErrMissingTransactor, CodeMissingTransactor},
		{"ErrNegRiskAdapter", ErrNegRiskAdapter, CodeNegRiskAdapter},
		{"ErrConfigNotFound", ErrConfigNotFound, CodeConfigNotFound},

		// Bridge errors
		{"ErrMissingFromAddress", ErrMissingFromAddress, CodeMissingFromAddress},
		{"ErrMissingDepositAddress", ErrMissingDepositAddress, CodeMissingDepositAddress},
		{"ErrWithdrawUnsupported", ErrWithdrawUnsupported, CodeWithdrawUnsupported},
		{"ErrMissingWithdrawRequest", ErrMissingWithdrawRequest, CodeMissingWithdrawRequest},
		{"ErrMissingWithdrawAddress", ErrMissingWithdrawAddress, CodeMissingWithdrawAddress},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s is nil", tt.name)
			}
			if tt.err.Code != tt.code {
				t.Errorf("%s.Code = %s, want %s", tt.name, tt.err.Code, tt.code)
			}
			if tt.err.Message == "" {
				t.Errorf("%s has empty message", tt.name)
			}
			if tt.err.Error() == "" {
				t.Errorf("%s.Error() returns empty string", tt.name)
			}
			// Verify error string contains code
			if !strings.Contains(tt.err.Error(), string(tt.code)) {
				t.Errorf("%s.Error() = %q, should contain code %s", tt.name, tt.err.Error(), tt.code)
			}
		})
	}
}

func TestErrorCodeUniqueness(t *testing.T) {
	// Test that all error codes are unique
	allCodes := []ErrorCode{
		CodeMissingSigner,
		CodeMissingCreds,
		CodeInvalidSignature,
		CodeUnauthorized,
		CodeProxyWalletUnsupported,
		CodeSafeWalletUnsupported,
		CodeInsufficientFunds,
		CodeRateLimitExceeded,
		CodeOrderNotFound,
		CodeMarketClosed,
		CodeGeoblocked,
		CodeInvalidPrice,
		CodeInvalidSize,
		CodeInternalServerError,
		CodeBadRequest,
		CodeCircuitOpen,
		CodeTooManyRequests,
		CodeMissingRequest,
		CodeMissingUser,
		CodeInvalidMarketFilter,
		CodeInvalidTradeFilter,
		CodeInvalidSubscription,
		CodeMissingU256Value,
		CodeMissingBackend,
		CodeMissingTransactor,
		CodeNegRiskAdapter,
		CodeConfigNotFound,
		CodeMissingFromAddress,
		CodeMissingDepositAddress,
		CodeWithdrawUnsupported,
		CodeMissingWithdrawRequest,
		CodeMissingWithdrawAddress,
	}

	seen := make(map[ErrorCode]bool)
	for _, code := range allCodes {
		if seen[code] {
			t.Errorf("Duplicate error code: %s", code)
		}
		seen[code] = true
	}
}

func TestErrorMessageUniqueness(t *testing.T) {
	// Test that all error messages are unique
	allErrors := []*SDKError{
		ErrMissingSigner,
		ErrMissingCreds,
		ErrInvalidSignature,
		ErrUnauthorized,
		ErrProxyWalletUnsupported,
		ErrSafeWalletUnsupported,
		ErrInsufficientFunds,
		ErrRateLimitExceeded,
		ErrOrderNotFound,
		ErrMarketClosed,
		ErrGeoblocked,
		ErrInvalidPrice,
		ErrInvalidSize,
		ErrInternalServerError,
		ErrBadRequest,
		ErrCircuitOpen,
		ErrTooManyRequests,
		ErrMissingRequest,
		ErrMissingUser,
		ErrInvalidMarketFilter,
		ErrInvalidTradeFilter,
		ErrInvalidSubscription,
		ErrMissingU256Value,
		ErrMissingBackend,
		ErrMissingTransactor,
		ErrNegRiskAdapter,
		ErrConfigNotFound,
		ErrMissingFromAddress,
		ErrMissingDepositAddress,
		ErrWithdrawUnsupported,
		ErrMissingWithdrawRequest,
		ErrMissingWithdrawAddress,
	}

	seen := make(map[string]bool)
	for _, err := range allErrors {
		msg := err.Message
		if seen[msg] {
			t.Errorf("Duplicate error message: %s", msg)
		}
		seen[msg] = true
	}
}

func TestErrorCodeFormat(t *testing.T) {
	// Test that error codes follow the expected format
	tests := []struct {
		code   ErrorCode
		prefix string
	}{
		{CodeMissingSigner, "AUTH-"},
		{CodeMissingCreds, "AUTH-"},
		{CodeProxyWalletUnsupported, "WALLET-"},
		{CodeInsufficientFunds, "CLOB-"},
		{CodeInternalServerError, "NET-"},
		{CodeMissingRequest, "DATA-"},
		{CodeInvalidSubscription, "WS-"},
		{CodeMissingU256Value, "CTF-"},
		{CodeMissingFromAddress, "BRIDGE-"},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if !strings.HasPrefix(string(tt.code), tt.prefix) {
				t.Errorf("Code %s should start with %s", tt.code, tt.prefix)
			}
		})
	}
}
