package logger

import (
	"fmt"
	"strconv"
	"time"
)

// Log format variables
var (
	format string
)

// serializer manages the buffered writing of log entries in different formats
type serializer struct {
	buf    []byte
	format string
}

// newSerializer creates a serializer instance to be used by processor
func newSerializer() *serializer {
	return &serializer{
		buf:    make([]byte, 0, 1024),
		format: format,
	}
}

// reset clears the serializer buffer for reuse
func (s *serializer) reset() {
	s.buf = s.buf[:0]
}

// serialize converts log entries to either JSON or text format based on configuration
func (s *serializer) serialize(level int64, args []any) []byte {
	s.reset()

	if s.format == "json" {
		return s.serializeJSON(level, args)
	}
	return s.serializeText(level, args)
}

// serializeJSON formats log entries as JSON with time, level and fields
func (s *serializer) serializeJSON(level int64, args []any) []byte {
	s.buf = append(s.buf, '{')

	// Time is always first
	s.buf = append(s.buf, `"time":"`...)
	s.buf = append(s.buf, time.Now().Format(time.RFC3339Nano)...)
	s.buf = append(s.buf, '"')

	// Level is always second
	s.buf = append(s.buf, `,"level":"`...)
	s.buf = append(s.buf, levelToString(level)...)
	s.buf = append(s.buf, '"')

	// Fields as ordered array. if enabled, trace is the first field.
	s.buf = append(s.buf, `,"fields":[`...)

	for i, arg := range args {
		if i > 0 {
			s.buf = append(s.buf, ',')
		}
		s.writeJSONValue(arg)
	}

	s.buf = append(s.buf, ']', '}', '\n')
	return s.buf
}

// serializeText formats log entries as plain text with time, level and space-separated fields
func (s *serializer) serializeText(level int64, args []any) []byte {
	// Time stamp
	s.buf = append(s.buf, time.Now().Format(time.RFC3339Nano)...)
	s.buf = append(s.buf, ' ')

	// Level in uppercase
	s.buf = append(s.buf, levelToString(level)...)
	s.buf = append(s.buf, ' ')

	// Fields as space-separated values
	for i, arg := range args {
		if i > 0 {
			s.buf = append(s.buf, ' ')
		}
		s.writeTextValue(arg)
	}

	s.buf = append(s.buf, '\n')
	return s.buf
}

// writeTextValue converts any value to its text representation with appropriate quoting
func (s *serializer) writeTextValue(v any) {
	switch val := v.(type) {
	case string:
		if needsQuotes(val) {
			s.buf = append(s.buf, '"')
			s.writeString(val)
			s.buf = append(s.buf, '"')
		} else {
			s.writeString(val)
		}
	case int:
		s.buf = strconv.AppendInt(s.buf, int64(val), 10)
	case int64:
		s.buf = strconv.AppendInt(s.buf, val, 10)
	case float64:
		s.buf = strconv.AppendFloat(s.buf, val, 'f', -1, 64)
	case bool:
		s.buf = strconv.AppendBool(s.buf, val)
	case nil:
		s.buf = append(s.buf, "null"...)
	default:
		str := stringifyMessage(val)
		if needsQuotes(str) {
			s.buf = append(s.buf, '"')
			s.writeString(str)
			s.buf = append(s.buf, '"')
		} else {
			s.writeString(str)
		}
	}
}

// writeJSONValue converts any value to its JSON representation with proper type handling
func (s *serializer) writeJSONValue(v any) {
	switch val := v.(type) {
	case string:
		s.buf = append(s.buf, '"')
		s.writeString(val)
		s.buf = append(s.buf, '"')
	case int:
		s.buf = strconv.AppendInt(s.buf, int64(val), 10)
	case int64:
		s.buf = strconv.AppendInt(s.buf, val, 10)
	case float64:
		s.buf = strconv.AppendFloat(s.buf, val, 'f', -1, 64)
	case bool:
		s.buf = strconv.AppendBool(s.buf, val)
	case nil:
		s.buf = append(s.buf, "null"...)
	default:
		s.buf = append(s.buf, '"')
		s.writeString(stringifyMessage(val))
		s.buf = append(s.buf, '"')
	}
}

// needsQuotes checks if a string needs to be quoted in text format
func needsQuotes(s string) bool {
	if len(s) == 0 {
		return true
	}
	for _, c := range s {
		if c <= ' ' || c == '"' || c == '\\' {
			return true
		}
	}
	return false
}

// levelToString converts the numeric levels to string to be written in the file.
func levelToString(level int64) string {
	switch level {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return fmt.Sprintf("UNKNOWN (%d)", level)
	}
}

// stringifyMessage converts any type to a string representation
func stringifyMessage(msg any) string {
	switch m := msg.(type) {
	case string:
		return m
	case error:
		return m.Error()
	case fmt.Stringer:
		return m.String()
	default:
		return fmt.Sprintf("%+v", m)
	}
}

// writeString appends a string to the buffer with proper escape sequence handling
func (s *serializer) writeString(str string) {
	for i := 0; i < len(str); i++ {
		if str[i] < 0x20 || str[i] == '"' || str[i] == '\\' {
			s.buf = append(s.buf, '\\')
		}
		s.buf = append(s.buf, str[i])
	}
}

// writeValue converts any value to its string representation (deprecated - use format-specific writers)
func (s *serializer) writeValue(v any) {
	switch val := v.(type) {
	case string:
		s.buf = append(s.buf, '"')
		s.writeString(val)
		s.buf = append(s.buf, '"')
	case int:
		s.buf = strconv.AppendInt(s.buf, int64(val), 10)
	case int64:
		s.buf = strconv.AppendInt(s.buf, val, 10)
	case float64:
		s.buf = strconv.AppendFloat(s.buf, val, 'f', -1, 64)
	case bool:
		s.buf = strconv.AppendBool(s.buf, val)
	case nil:
		s.buf = append(s.buf, "null"...)
	default:
		s.buf = append(s.buf, '"')
		s.writeString(stringifyMessage(val))
		s.buf = append(s.buf, '"')
	}
}