package logger

import (
	"fmt"
	"strconv"
	"time"
)

type serializer struct {
	buf []byte
}

func newSerializer() *serializer {
	return &serializer{
		buf: make([]byte, 0, 1024),
	}
}

func (s *serializer) reset() {
	s.buf = s.buf[:0]
}

func (s *serializer) serialize(level int64, args []any) []byte {
	s.reset()
	s.buf = append(s.buf, '{')

	// Time is always first
	s.buf = append(s.buf, `"time":"`...)
	s.buf = append(s.buf, time.Now().Format(time.RFC3339Nano)...)
	s.buf = append(s.buf, '"')

	// Level is always second
	s.buf = append(s.buf, `,"level":"`...)
	s.buf = append(s.buf, levelToString(level)...)
	s.buf = append(s.buf, '"')

	// Fields as ordered array
	s.buf = append(s.buf, `,"fields":[`...)

	for i, arg := range args {
		if i > 0 {
			s.buf = append(s.buf, ',')
		}
		s.writeValue(arg)
	}

	s.buf = append(s.buf, ']', '}', '\n')
	return s.buf
}

// levelToString converts the numeric levels to string to be written in the file.
func levelToString(level int64) string {
	switch level {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "unknown"
	}
}

func (s *serializer) writeString(str string) {
	for i := 0; i < len(str); i++ {
		if str[i] < 0x20 || str[i] == '"' || str[i] == '\\' {
			s.buf = append(s.buf, '\\')
		}
		s.buf = append(s.buf, str[i])
	}
}

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