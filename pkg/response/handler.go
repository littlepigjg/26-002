package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// WriteJSON writes a JSON response with proper headers.
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// WriteSuccess writes a success response.
func WriteSuccess(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	})
}

// WriteCreated writes a created response.
func WriteCreated(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusCreated, Response{
		Code:    CodeSuccess,
		Message: "created",
		Data:    data,
	})
}

// WritePaginated writes a paginated response.
func WritePaginated(w http.ResponseWriter, data interface{}, total int64, page, pageSize int) {
	hasMore := int64(page*pageSize) < total
	WriteJSON(w, http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
		Extra: map[string]interface{}{
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"has_more":  hasMore,
		},
	})
}

// ParseErrorHandler handles JSON parse errors gracefully.
func ParseErrorHandler(w http.ResponseWriter, err error) {
	errMsg := err.Error()
	var statusCode int
	var code int
	var message string

	switch {
	case strings.Contains(errMsg, "unexpected end of JSON input"):
		statusCode = http.StatusBadRequest
		code = CodeBadRequest
		message = "Invalid JSON: unexpected end of input"
	case strings.Contains(errMsg, "cannot unmarshal"):
		statusCode = http.StatusBadRequest
		code = CodeBadRequest
		message = "Invalid JSON format"
	default:
		statusCode = http.StatusBadRequest
		code = CodeBadRequest
		message = fmt.Sprintf("Invalid request body: %s", errMsg)
	}

	WriteJSON(w, statusCode, Response{
		Code:    code,
		Message: message,
	})
}
