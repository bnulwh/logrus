package logrus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"testing"
	"time"
)

// This file pins the hand-rolled JSON fast path to encoding/json semantics:
// JSONFormatter.Format must stay byte-identical to what json.Encoder would
// produce (key order, string escaping, number formatting, HTML escaping).

var compatTestFrame = runtime.Frame{
	Function: "github.com/bnulwh/logrus.ExampleFunc",
	File:     "C:/src/example/main.go",
	Line:     42,
}

func compatTestEntry(t *testing.T, value interface{}, caller bool) *Entry {
	logger := New()
	e := NewEntry(logger)
	e.Time = time.Date(2026, 8, 16, 9, 30, 0, 0, time.UTC)
	e.Level = InfoLevel
	e.Message = "hello world"
	e.err = "format error"
	if caller {
		e.Caller = &compatTestFrame
	}
	if value != nil {
		e = e.WithField("field", value)
	}
	return e
}

// jsonReference builds the expected output with the production behaviour the
// fast path replaces: a plain encoding/json Encoder over the same data map.
func jsonReference(entry *Entry, f *JSONFormatter) []byte {
	timestampFormat := f.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = defaultTimestampFormat
	}
	data := make(Fields, len(entry.Data)+4)
	for k, v := range entry.Data {
		switch v := v.(type) {
		case error:
			data[k] = v.Error()
		default:
			data[k] = v
		}
	}
	if f.DataKey != "" {
		newData := make(Fields, 4)
		newData[f.DataKey] = data
		data = newData
	}
	prefixFieldClashes(data, f.FieldMap, entry.HasCaller())
	if entry.err != "" {
		data[f.FieldMap.resolve(FieldKeyLogrusError)] = entry.err
	}
	if !f.DisableTimestamp {
		data[f.FieldMap.resolve(FieldKeyTime)] = entry.Time.Format(timestampFormat)
	}
	data[f.FieldMap.resolve(FieldKeyMsg)] = entry.Message
	data[f.FieldMap.resolve(FieldKeyLevel)] = entry.Level.String()
	if entry.HasCaller() {
		funcVal := entry.Caller.Function
		fileVal := fmt.Sprintf("%s:%d", entry.Caller.File, entry.Caller.Line)
		if f.CallerPrettyfier != nil {
			funcVal, fileVal = f.CallerPrettyfier(entry.Caller)
		}
		if funcVal != "" {
			data[f.FieldMap.resolve(FieldKeyFunc)] = funcVal
		}
		if fileVal != "" {
			data[f.FieldMap.resolve(FieldKeyFile)] = fileVal
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(!f.DisableHTMLEscape)
	if err := enc.Encode(data); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func TestJSONFormatterMatchesEncodingJSON(t *testing.T) {
	values := []interface{}{
		// strings
		"plain",
		"with space",
		`quote"back\slash`,
		"html&<>\u2028\u2029",
		"ctrl\x01\x1f\n\r\t",
		"中文🎉",
		"",
		"\x7f",
		// integers
		0, 42, -42,
		int8(-8), int16(1600), int32(32000),
		int64(1) << 40, int64(-1) << 40,
		uint(7), uint8(8), uint16(1600), uint32(32000), uint64(1) << 63,
		// floats
		1.0, 0.5, -0.0,
		123456789.123456789,
		3.141592653589793,
		1e21, 1e-7, 1e-6, 1e21 - 1e10,
		float32(0.1),
		float32(1e-7),
		float32(1e21),
		math.NaN(), math.Inf(1), math.Inf(-1), // must error like encoding/json
		// other scalars
		true, false, nil,
		json.Number("123.45"),
		// complex values (fallback path)
		map[string]interface{}{"k": "v", "n": 1},
		map[string]interface{}{"html": "&<>", "nested": map[string]interface{}{"x": "<y>"}},
		[]interface{}{"a", 1, true},
		[]string{"x", "y"},
		[]byte("binary\x00data"),
		time.Date(2026, 8, 16, 9, 30, 0, 0, time.UTC),
		errors.New("boom"),
	}

	variants := []struct {
		name string
		mod  func(*JSONFormatter)
	}{
		{"default", func(f *JSONFormatter) {}},
		{"noescape", func(f *JSONFormatter) { f.DisableHTMLEscape = true }},
		{"nots", func(f *JSONFormatter) { f.DisableTimestamp = true }},
		{"datakey", func(f *JSONFormatter) { f.DataKey = "fields" }},
		{"remap", func(f *JSONFormatter) {
			f.FieldMap = FieldMap{FieldKeyTime: "@t", FieldKeyMsg: "@m", FieldKeyLevel: "@l", FieldKeyLogrusError: "@e"}
		}},
	}

	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			for _, caller := range []bool{false, true} {
				for _, value := range values {
					// NaN/Inf must produce the same error both ways.
					if isNanOrInf(value) {
						continue // covered by TestJSONFormatterRejectsNaN
					}
					f := &JSONFormatter{}
					variant.mod(f)
					e1 := compatTestEntry(t, value, caller)
					e2 := compatTestEntry(t, value, caller)
					got, err1 := f.Format(e1)
					want := jsonReference(e2, f)
					if err1 != nil {
						t.Fatalf("value=%v caller=%v: %v", value, caller, err1)
					}
					if !bytes.Equal(got, want) {
						t.Errorf("value=%#v caller=%v\n got=%s\nwant=%s", value, caller, got, want)
					}
				}
			}
		})
	}
}

func isNanOrInf(v interface{}) bool {
	switch v := v.(type) {
	case float64:
		return math.IsNaN(v) || math.IsInf(v, 0)
	case float32:
		return math.IsNaN(float64(v)) || math.IsInf(float64(v), 0)
	}
	return false
}

func TestJSONFormatterRejectsNaN(t *testing.T) {
	for _, value := range []interface{}{math.NaN(), math.Inf(1), float32(math.NaN())} {
		f := &JSONFormatter{}
		_, err := f.Format(compatTestEntry(t, value, false))
		if err == nil || !strings.Contains(err.Error(), "unsupported value") {
			t.Errorf("value=%v: want unsupported-value error, got %v", value, err)
		}
	}
}

func TestJSONFormatterPrettyPrintStillWorks(t *testing.T) {
	f := &JSONFormatter{PrettyPrint: true}
	out, err := f.Format(compatTestEntry(t, "v", false))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got["field"] != "v" {
		t.Errorf("field = %v, want v", got["field"])
	}
}

// TestTextFormatterAppendValue pins the value-writing edge cases: integers and
// booleans must never be quoted, while NaN/Inf (rendered as NaN/+Inf/-Inf)
// must stay quoted exactly like the pre-optimization implementation.
func TestTextFormatterAppendValue(t *testing.T) {
	f := &TextFormatter{DisableColors: true}
	for _, tc := range []struct {
		val  interface{}
		want string
	}{
		{42, "k=42"},
		{int64(-7), "k=-7"},
		{uint(9), "k=9"},
		{true, "k=true"},
		{false, "k=false"},
		{1.5, "k=1.5"},
		// NaN/Inf render as NaN/+Inf/-Inf and, like all letters/+/-, never
		// trigger quoting; pin this so it cannot silently change.
		{math.NaN(), "k=NaN"},
		{math.Inf(1), "k=+Inf"},
		{math.Inf(-1), "k=-Inf"},
		{"hello", "k=hello"},
		{"with space", `k="with space"`},
		{"quote\"back\\", `k="quote\"back\\"`},
		// empty values are only quoted when QuoteEmptyFields is enabled
		{"", "k="},
	} {
		e := &Entry{Level: InfoLevel, Message: "m", Data: Fields{"k": tc.val}}
		out, err := f.Format(e)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), tc.want) {
			t.Errorf("val=%#v: want substring %q in %q", tc.val, tc.want, out)
		}
	}
}
