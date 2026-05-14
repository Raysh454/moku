package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// StdoutLogger is a tiny, structured logger used during development.
// It implements Logger and prints JSON lines to stdout.
type StdoutLogger struct {
	component string
	fields    map[string]any
}

// NewStdoutLogger creates a new simple StdoutLogger. component is optional and
// will be included as a persistent field on With().
func NewStdoutLogger(component string) *StdoutLogger {
	return &StdoutLogger{
		component: component,
		fields:    make(map[string]any),
	}
}

func (s *StdoutLogger) log(level string, msg string, fields ...Field) {
	type outEntry struct {
		Level     string         `json:"level"`
		Msg       string         `json:"msg"`
		Component string         `json:"component,omitempty"`
		Time      string         `json:"time"`
		Fields    map[string]any `json:"fields,omitempty"`
	}
	m := make(map[string]any)
	for k, v := range s.fields {
		m[k] = v
	}
	for _, f := range fields {
		m[f.Key] = f.Value
	}
	entry := outEntry{
		Level:     level,
		Msg:       msg,
		Component: s.component,
		Time:      time.Now().UTC().Format(time.RFC3339),
		Fields:    m,
	}
	enc, err := json.Marshal(entry)
	if err != nil {
		// Fallback simple formatting to stdout if JSON marshal fails
		fmt.Fprintf(os.Stdout, "%s %s %v\n", level, msg, m)
		return
	}
	fmt.Fprintln(os.Stdout, string(enc))
}

func (s *StdoutLogger) Debug(msg string, fields ...Field) {
	s.log("debug", msg, fields...)
}

func (s *StdoutLogger) Info(msg string, fields ...Field) {
	s.log("info", msg, fields...)
}

func (s *StdoutLogger) Warn(msg string, fields ...Field) {
	s.log("warn", msg, fields...)
}

func (s *StdoutLogger) Error(msg string, fields ...Field) {
	s.log("error", msg, fields...)
}

func (s *StdoutLogger) With(fields ...Field) Logger {
	newFields := make(map[string]any)
	for k, v := range s.fields {
		newFields[k] = v
	}

	child := &StdoutLogger{
		component: s.component,
		fields:    newFields,
	}
	for _, f := range fields {
		if f.Key == "component" {
			if str, ok := f.Value.(string); ok {
				child.component = str
			}
		} else {
			child.fields[f.Key] = f.Value
		}
	}
	return child
}
