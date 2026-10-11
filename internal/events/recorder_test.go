package events

import (
	"bytes"
	"encoding/json"
	"log"
	"testing"
)

func TestStructuredLogPreservesFieldTypes(t *testing.T) {
	var out bytes.Buffer
	r := Recorder{Logger: log.New(&out, "", 0), MinLevel: Debug}
	r.Log(Info, "probe", "completed", "节点探测完成", map[string]any{
		"delay_ms": 235.6,
		"success":  true,
		"checks":   42,
		"errors":   []string{},
	})
	var record struct {
		Level  string         `json:"level"`
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.Level != "INFO" || record.Fields["success"] != true || record.Fields["checks"].(float64) != 42 || record.Fields["delay_ms"].(float64) != 235.6 {
		t.Fatalf("typed fields were not preserved: %+v", record)
	}
}

func TestStructuredLogFiltersDebugByDefault(t *testing.T) {
	var out bytes.Buffer
	r := Recorder{Logger: log.New(&out, "", 0), MinLevel: Info}
	r.Trace("probe", "success", true)
	if out.Len() != 0 {
		t.Fatalf("debug record was written at info level: %q", out.String())
	}
}

func TestRecordMapsErrorKindToErrorLevel(t *testing.T) {
	var out bytes.Buffer
	r := Recorder{Logger: log.New(&out, "", 0), MinLevel: Info}
	r.Record("error", "操作失败")
	var record struct {
		Level string `json:"level"`
	}
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.Level != "ERROR" {
		t.Fatalf("error kind was logged at %q", record.Level)
	}
}
