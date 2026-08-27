package model

// Common filter field names used across the application.
const (
	FilterFieldEventType  = "event_type"
	FilterFieldPageURL    = "page_url"
	FilterFieldCountry    = "country"
	FilterFieldDeviceType = "device_type"
	FilterFieldOS          = "os"
	FilterFieldBrowser    = "browser"
	FilterFieldUserType   = "user_type"
	FilterFieldUserID     = "user_id"
	FilterFieldSessionID  = "session_id"
	FilterFieldReferrer   = "referrer"
	FilterFieldDuration   = "duration_ms"
	FilterFieldTimestamp  = "timestamp"
)

// Common filter operators.
const (
	FilterOpEqual       = "eq"
	FilterOpNotEqual    = "neq"
	FilterOpContains    = "contains"
	FilterOpStartsWith  = "starts_with"
	FilterOpEndsWith    = "ends_with"
	FilterOpGreaterThan = "gt"
	FilterOpLessThan    = "lt"
	FilterOpIn          = "in"
)
