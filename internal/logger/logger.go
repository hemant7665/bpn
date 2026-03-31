package logger

import (
	"encoding/json"
	"log"
	"time"
)

type entry struct {
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Time    string         `json:"time"`
	Fields  map[string]any `json:"fields,omitempty"`
}

func Info(message string, fields map[string]any) {
	write("INFO", message, fields)
}

func Error(message string, fields map[string]any) {
	write("ERROR", message, fields)
}

func write(level string, message string, fields map[string]any) {
	payload, err := json.Marshal(entry{
		Level:   level,
		Message: message,
		Time:    time.Now().UTC().Format(time.RFC3339Nano),
		Fields:  fields,
	})
	if err != nil {
		log.Printf("{\"level\":\"ERROR\",\"message\":\"logger_marshal_failed\",\"time\":\"%s\"}", time.Now().UTC().Format(time.RFC3339Nano))
		return
	}
	log.Println(string(payload))
}
