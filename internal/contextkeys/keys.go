package contextkeys

// CtxKey is a custom type for context keys to avoid collisions.
type CtxKey string

// RequestBodyKey is the key for storing the raw request body in the context.
const RequestBodyKey CtxKey = "requestBody"
