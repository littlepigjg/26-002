// Package response provides a unified API response format for HTTP handlers.
// It wraps all API responses in a consistent JSON structure with code, message,
// data, and optional pagination fields.
package response

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Response is the standard API response wrapper.
type Response struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    interface{}            `json:"data,omitempty"`
	Extra   map[string]interface{} `json:"extra,omitempty"`
}

// PaginatedResponse wraps a list response with pagination metadata.
type PaginatedResponse struct {
	Code       int                    `json:"code"`
	Message    string                 `json:"message"`
	Data       interface{}            `json:"data,omitempty"`
	Pagination *PaginationInfo        `json:"pagination,omitempty"`
	Extra      map[string]interface{} `json:"extra,omitempty"`
}

// PaginationInfo contains pagination metadata.
type PaginationInfo struct {
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	HasMore  bool `json:"has_more"`
}

// Common API response codes
const (
	CodeSuccess         = 0
	CodeBadRequest      = 400
	CodeUnauthorized    = 401
	CodeForbidden       = 403
	CodeNotFound        = 404
	CodeConflict        = 409
	CodeInternalError   = 500
	CodeServiceUnavail  = 503
)

// Success writes a successful JSON response.
func Success(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	})
}

// SuccessWithMessage writes a successful JSON response with a custom message.
func SuccessWithMessage(w http.ResponseWriter, message string, data interface{}) {
	writeJSON(w, http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: message,
		Data:    data,
	})
}

// Created writes a 201 Created response.
func Created(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusCreated, Response{
		Code:    CodeSuccess,
		Message: "created",
		Data:    data,
	})
}

// NoContent writes a 204 No Content response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// BadRequest writes a 400 Bad Request response.
func BadRequest(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadRequest, Response{
		Code:    CodeBadRequest,
		Message: message,
	})
}

// Unauthorized writes a 401 Unauthorized response.
func Unauthorized(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusUnauthorized, Response{
		Code:    CodeUnauthorized,
		Message: message,
	})
}

// Forbidden writes a 403 Forbidden response.
func Forbidden(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusForbidden, Response{
		Code:    CodeForbidden,
		Message: message,
	})
}

// NotFound writes a 404 Not Found response.
func NotFound(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusNotFound, Response{
		Code:    CodeNotFound,
		Message: message,
	})
}

// Conflict writes a 409 Conflict response.
func Conflict(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusConflict, Response{
		Code:    CodeConflict,
		Message: message,
	})
}

// InternalError writes a 500 Internal Server Error response.
func InternalError(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusInternalServerError, Response{
		Code:    CodeInternalError,
		Message: message,
	})
}

// ServiceUnavailable writes a 503 Service Unavailable response.
func ServiceUnavailable(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusServiceUnavailable, Response{
		Code:    CodeServiceUnavail,
		Message: message,
	})
}

// WriteError writes an appropriate error response based on the error type.
func WriteError(w http.ResponseWriter, err error) {
	if err == nil {
		Success(w, nil)
		return
	}

	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "not found"):
		NotFound(w, errStr)
	case strings.Contains(errStr, "bad request"), strings.Contains(errStr, "invalid"):
		BadRequest(w, errStr)
	case strings.Contains(errStr, "unauthorized"):
		Unauthorized(w, errStr)
	case strings.Contains(errStr, "forbidden"):
		Forbidden(w, errStr)
	case strings.Contains(errStr, "conflict"):
		Conflict(w, errStr)
	default:
		InternalError(w, errStr)
	}
}

// Paginated writes a paginated response.
func Paginated(w http.ResponseWriter, data interface{}, total, page, pageSize int) {
	hasMore := page*pageSize < total
	writeJSON(w, http.StatusOK, PaginatedResponse{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
		Pagination: &PaginationInfo{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			HasMore:  hasMore,
		},
	})
}

// PaginatedWithExtra writes a paginated response with extra data.
func PaginatedWithExtra(w http.ResponseWriter, data interface{}, total, page, pageSize int, extra map[string]interface{}) {
	hasMore := page*pageSize < total
	writeJSON(w, http.StatusOK, PaginatedResponse{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
		Pagination: &PaginationInfo{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			HasMore:  hasMore,
		},
		Extra: extra,
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(data)
}
