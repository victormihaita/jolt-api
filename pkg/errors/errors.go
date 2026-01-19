package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError represents an application-level error
type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
	Err        error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// Error codes
const (
	CodeBadRequest          = "BAD_REQUEST"
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeForbidden           = "FORBIDDEN"
	CodeNotFound            = "NOT_FOUND"
	CodeConflict            = "CONFLICT"
	CodeInternalError       = "INTERNAL_ERROR"
	CodeValidationError     = "VALIDATION_ERROR"
	CodeTokenExpired        = "TOKEN_EXPIRED"
	CodeInvalidToken        = "INVALID_TOKEN"
	CodeDeviceNotFound      = "DEVICE_NOT_FOUND"
	CodeReminderNotFound    = "REMINDER_NOT_FOUND"
	CodeUserNotFound        = "USER_NOT_FOUND"
	CodeSyncConflict        = "SYNC_CONFLICT"
	CodePremiumRequired     = "PREMIUM_REQUIRED"
	CodeDeviceLimitExceeded = "DEVICE_LIMIT_EXCEEDED"
)

// Common errors
var (
	ErrBadRequest = &AppError{
		Code:       CodeBadRequest,
		Message:    "Bad request",
		StatusCode: http.StatusBadRequest,
	}

	ErrUnauthorized = &AppError{
		Code:       CodeUnauthorized,
		Message:    "Unauthorized",
		StatusCode: http.StatusUnauthorized,
	}

	ErrForbidden = &AppError{
		Code:       CodeForbidden,
		Message:    "Forbidden",
		StatusCode: http.StatusForbidden,
	}

	ErrNotFound = &AppError{
		Code:       CodeNotFound,
		Message:    "Resource not found",
		StatusCode: http.StatusNotFound,
	}

	ErrInternalError = &AppError{
		Code:       CodeInternalError,
		Message:    "Internal server error",
		StatusCode: http.StatusInternalServerError,
	}

	ErrTokenExpired = &AppError{
		Code:       CodeTokenExpired,
		Message:    "Token has expired",
		StatusCode: http.StatusUnauthorized,
	}

	ErrInvalidToken = &AppError{
		Code:       CodeInvalidToken,
		Message:    "Invalid token",
		StatusCode: http.StatusUnauthorized,
	}

	ErrReminderNotFound = &AppError{
		Code:       CodeReminderNotFound,
		Message:    "Reminder not found",
		StatusCode: http.StatusNotFound,
	}

	ErrUserNotFound = &AppError{
		Code:       CodeUserNotFound,
		Message:    "User not found",
		StatusCode: http.StatusNotFound,
	}

	ErrDeviceNotFound = &AppError{
		Code:       CodeDeviceNotFound,
		Message:    "Device not found",
		StatusCode: http.StatusNotFound,
	}

	ErrPremiumRequired = &AppError{
		Code:       CodePremiumRequired,
		Message:    "This feature requires a premium subscription",
		StatusCode: http.StatusForbidden,
	}

	ErrDeviceLimitExceeded = &AppError{
		Code:       CodeDeviceLimitExceeded,
		Message:    "Device limit exceeded. Upgrade to premium for unlimited devices.",
		StatusCode: http.StatusForbidden,
	}
)

// New creates a new AppError
func New(code string, message string, statusCode int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// Wrap wraps an error with an AppError
func Wrap(err error, code string, message string, statusCode int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Err:        err,
	}
}

// ValidationError creates a validation error
func ValidationError(message string) *AppError {
	return &AppError{
		Code:       CodeValidationError,
		Message:    message,
		StatusCode: http.StatusBadRequest,
	}
}

// SyncConflictError creates a sync conflict error
func SyncConflictError(message string) *AppError {
	return &AppError{
		Code:       CodeSyncConflict,
		Message:    message,
		StatusCode: http.StatusConflict,
	}
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

// GetAppError extracts AppError from an error
func GetAppError(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}
