package app

// contextKey is an unexported type used for context keys to prevent collisions
// with keys defined in other packages.
type contextKey string

// SessionIDKey is the typed context key for passing ai_session_id between packages.
// Used by AI App's CompositeToolExecutor (injection) and ITSM tool handlers (reading).
const SessionIDKey = contextKey("ai_session_id")

// UserMessageKey is the typed context key for passing the latest user message
// into tool handlers. Tools should prefer this as the original user request
// when model-supplied tool arguments are abbreviated.
const UserMessageKey = contextKey("ai_latest_user_message")
