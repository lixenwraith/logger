package quick

import (
	"fmt"
	"github.com/LixenWraith/logger"
	"reflect"
	"strconv"
	"strings"
)

// config parses configuration strings into a LoggerConfig.
// Each argument should be in "key=value" format where key matches LoggerConfig field names.
// The function handles type conversion and validation for each field.
func config(args ...string) (*logger.LoggerConfig, error) {
	cfg := &logger.LoggerConfig{}
	for _, arg := range args {
		key, value, err := parseKeyValue(arg)
		if err != nil {
			return nil, fmt.Errorf("invalid config format: %s", arg)
		}

		if err := setValue(cfg, key, value); err != nil {
			return nil, fmt.Errorf("config error: %s", err)
		}
	}
	return cfg, nil
}

// parseKeyValue splits a configuration string into key and value parts.
// Input format must be "key=value". Leading and trailing spaces are removed from both parts.
// Returns error if format is invalid.
func parseKeyValue(arg string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(arg), "=")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid format")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

// setValue updates a LoggerConfig field using reflection.
// Field matching is case-insensitive. Values are converted to appropriate types.
// Special handling is provided for the "level" field to accept string values.
// Returns error if field is unknown or value cannot be converted to required type.
func setValue(cfg *logger.LoggerConfig, key, value string) error {
	// Convert key to lowercase for case-insensitive matching with lower-case LoggerConfig tags
	key = strings.ToLower(key)

	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if tag := field.Tag.Get("toml"); tag == key {
			f := v.Field(i)
			if !f.IsValid() {
				return fmt.Errorf("unknown config key: %s", key)
			}

			switch f.Kind() {
			case reflect.Int64:
				if strings.EqualFold(key, "level") {
					// Special handling for level
					level, err := parseLevel(value)
					if err != nil {
						return err
					}
					f.SetInt(level)
				} else {
					// Other int64 fields
					val, err := strconv.ParseInt(value, 10, 64)
					if err != nil {
						return fmt.Errorf("invalid int64 value for %s: %s", key, value)
					}
					f.SetInt(val)
				}

			case reflect.Float64:
				val, err := strconv.ParseFloat(value, 64)
				if err != nil {
					return fmt.Errorf("invalid float64 value for %s: %s", key, value)
				}
				f.SetFloat(val)

			case reflect.String:
				// Special handling for format field
				if key == "format" {
					f.SetString(strings.ToLower(value))
				} else {
					// Keep original case for name and directory
					f.SetString(value)
				}

			case reflect.Bool:
				val, err := strconv.ParseBool(value)
				if err != nil {
					return fmt.Errorf("invalid bool value for %s: %s", key, value)
				}
				f.SetBool(val)

			default:
				return fmt.Errorf("unsupported config type for %s", key)
			}

			return nil
		}
	}
	return fmt.Errorf("unknown config key: %s", key)
}

// parseLevel converts level string to corresponding int64 constant.
// Accepts both format variants: "debug"/"leveldebug", "info"/"levelinfo" etc.
// Returns error if level string is invalid.
func parseLevel(level string) (int64, error) {
	switch strings.ToLower(level) {
	case "debug", "leveldebug":
		return logger.LevelDebug, nil
	case "info", "levelinfo":
		return logger.LevelInfo, nil
	case "warn", "levelwarn":
		return logger.LevelWarn, nil
	case "error", "levelerror":
		return logger.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid level: %s", level)
	}
}