package logging

import "go.uber.org/zap"

// TypeField returns a zap.Field that records the log entry's origin type.
// Use one of the LogType constants (TypeApplication, TypeSIPRequest, etc.).
func TypeField(t LogType) zap.Field {
	return zap.String("type", string(t))
}
