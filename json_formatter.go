package logrus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strconv"
	"unicode"
	"unicode/utf8"
)

type fieldKey string

// FieldMap allows customization of the key names for default fields.
type FieldMap map[fieldKey]string

func (f FieldMap) resolve(key fieldKey) string {
	if k, ok := f[key]; ok {
		return k
	}

	return string(key)
}

// JSONFormatter formats logs into parsable json
type JSONFormatter struct {
	// TimestampFormat sets the format used for marshaling timestamps.
	// The format to use is the same than for time.Format or time.Parse from the standard
	// library.
	// The standard Library already provides a set of predefined format.
	TimestampFormat string

	// DisableTimestamp allows disabling automatic timestamps in output
	DisableTimestamp bool

	// DisableHTMLEscape allows disabling html escaping in output
	DisableHTMLEscape bool

	// DataKey allows users to put all the log entry parameters into a nested dictionary at a given key.
	DataKey string

	// FieldMap allows users to customize the names of keys for default fields.
	// As an example:
	// formatter := &JSONFormatter{
	//   	FieldMap: FieldMap{
	// 		 FieldKeyTime:  "@timestamp",
	// 		 FieldKeyLevel: "@level",
	// 		 FieldKeyMsg:   "@message",
	// 		 FieldKeyFunc:  "@caller",
	//    },
	// }
	FieldMap FieldMap

	// CallerPrettyfier can be set by the user to modify the content
	// of the function and file keys in the json data when ReportCaller is
	// activated. If any of the returned value is the empty string the
	// corresponding key will be removed from json fields.
	CallerPrettyfier func(*runtime.Frame) (function string, file string)

	// PrettyPrint will indent all json logs
	PrettyPrint bool
}

// Format renders a single log entry
func (f *JSONFormatter) Format(entry *Entry) ([]byte, error) {
	data := make(Fields, len(entry.Data)+4)
	for k, v := range entry.Data {
		switch v := v.(type) {
		case error:
			// Otherwise errors are ignored by `encoding/json`
			// https://github.com/bnulwh/logrus/issues/137
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

	timestampFormat := f.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = defaultTimestampFormat
	}

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

	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	if f.PrettyPrint {
		// Keep encoding/json for the rare pretty-printed output.
		encoder := json.NewEncoder(b)
		encoder.SetEscapeHTML(!f.DisableHTMLEscape)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(data); err != nil {
			return nil, fmt.Errorf("failed to marshal fields to JSON, %w", err)
		}
		return b.Bytes(), nil
	}

	// Fast path: hand-rolled JSON writer that is byte-identical to
	// encoding/json (same key order, string escaping and number formatting),
	// avoiding the per-call Encoder allocation and reflection overhead. Keys
	// are sorted like encoding/json does, so the output is unchanged. Complex
	// values fall back to encoding/json.
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		appendJSONString(b, k, !f.DisableHTMLEscape)
		b.WriteByte(':')
		if err := appendJSONValue(b, data[k], f.DisableHTMLEscape); err != nil {
			return nil, fmt.Errorf("failed to marshal fields to JSON, %w", err)
		}
	}
	b.WriteString("}\n")
	return b.Bytes(), nil
}

const jsonHexDigits = "0123456789abcdef"

// appendJSONString writes s as a JSON string, byte-identical to encoding/json
// (HTML escaping of & < > plus always-escaping of non-printable runes,
// including U+2028/U+2029 and invalid UTF-8 replacement with U+FFFD).
func appendJSONString(b *bytes.Buffer, s string, escapeHTML bool) {
	b.WriteByte('"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			// encoding/json's safeSet marks 0x20..0x7e and 0x7f (DEL) as safe;
			// only bytes < 0x20 and the HTML/quote/backslash bytes escape.
			if c >= 0x20 && c <= 0x7f && c != '"' && c != '\\' &&
				(!escapeHTML || (c != '&' && c != '<' && c != '>')) {
				i++
				continue
			}
			b.WriteString(s[start:i])
			switch c {
			case '"':
				b.WriteString(`\"`)
			case '\\':
				b.WriteString(`\\`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				b.WriteString(`\u00`)
				b.WriteByte(jsonHexDigits[c>>4])
				b.WriteByte(jsonHexDigits[c&0xF])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			// invalid UTF-8: encoding/json replaces the byte with U+FFFD
			b.WriteString(s[start:i])
			b.WriteString(`\ufffd`)
			i++
			start = i
			continue
		}
		if !unicode.IsPrint(r) || (escapeHTML && (r == '&' || r == '<' || r == '>' || r == '\u2028' || r == '\u2029')) {
			b.WriteString(s[start:i])
			b.WriteString(`\u`)
			for shift := 12; shift >= 0; shift -= 4 {
				b.WriteByte(jsonHexDigits[(r>>uint(shift))&0xF])
			}
			i += size
			start = i
			continue
		}
		i += size
	}
	b.WriteString(s[start:])
	b.WriteByte('"')
}

// appendJSONFloat writes f as a JSON number following encoding/json's rules
// (ES2015 NumberToString with 'f'/'e' exponent cutoffs, e-09 cleanup and a
// round-trip check), so the output is byte-identical. bits is 32 or 64 and
// must match the source type: float32 values print with float32 precision.
func appendJSONFloat(b *bytes.Buffer, f float64, bits int) error {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return fmt.Errorf("json: unsupported value: %v", f)
	}
	abs := math.Abs(f)
	fmtCh := byte('f')
	if abs != 0 {
		if (bits == 32 && (float32(abs) < 1e-6 || float32(abs) >= 1e21)) ||
			(bits == 64 && (abs < 1e-6 || abs >= 1e21)) {
			fmtCh = 'e'
		}
	}
	var tmp [64]byte
	n := strconv.AppendFloat(tmp[:0], f, fmtCh, -1, bits)
	if fmtCh == 'e' {
		// encoding/json cleans up e-09 to e-9
		if len(n) >= 4 && n[len(n)-4] == 'e' && n[len(n)-3] == '-' && n[len(n)-2] == '0' {
			n[len(n)-2] = n[len(n)-1]
			n = n[:len(n)-1]
		}
	}
	// round-trip check, like encoding/json's eqfloat
	if fv, err := strconv.ParseFloat(string(n), bits); err != nil || fv != f {
		n = strconv.AppendFloat(tmp[:0], f, 'g', -1, bits)
	}
	b.Write(n)
	return nil
}

// appendJSONValue writes v as a JSON value with fast paths for the common
// scalar types, falling back to encoding/json for anything else.
func appendJSONValue(b *bytes.Buffer, v interface{}, disableHTMLEscape bool) error {
	switch v := v.(type) {
	case nil:
		b.WriteString("null")
	case string:
		appendJSONString(b, v, !disableHTMLEscape)
	case bool:
		if v {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int:
		var tmp [20]byte
		b.Write(strconv.AppendInt(tmp[:0], int64(v), 10))
	case int8:
		var tmp [20]byte
		b.Write(strconv.AppendInt(tmp[:0], int64(v), 10))
	case int16:
		var tmp [20]byte
		b.Write(strconv.AppendInt(tmp[:0], int64(v), 10))
	case int32:
		var tmp [20]byte
		b.Write(strconv.AppendInt(tmp[:0], int64(v), 10))
	case int64:
		var tmp [20]byte
		b.Write(strconv.AppendInt(tmp[:0], v, 10))
	case uint:
		var tmp [20]byte
		b.Write(strconv.AppendUint(tmp[:0], uint64(v), 10))
	case uint8:
		var tmp [20]byte
		b.Write(strconv.AppendUint(tmp[:0], uint64(v), 10))
	case uint16:
		var tmp [20]byte
		b.Write(strconv.AppendUint(tmp[:0], uint64(v), 10))
	case uint32:
		var tmp [20]byte
		b.Write(strconv.AppendUint(tmp[:0], uint64(v), 10))
	case uint64:
		var tmp [20]byte
		b.Write(strconv.AppendUint(tmp[:0], v, 10))
	case float32:
		return appendJSONFloat(b, float64(v), 32)
	case float64:
		return appendJSONFloat(b, v, 64)
	case json.Number:
		b.WriteString(v.String())
	default:
		// Complex values (maps, slices, structs, time.Time, []byte, ...): fall
		// back to encoding/json for exactness.
		enc, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if disableHTMLEscape {
			// json.Marshal always HTML-escapes; undo it to match the
			// formatter's SetEscapeHTML(false) semantics.
			enc = bytes.ReplaceAll(enc, []byte(`\u003c`), []byte("<"))
			enc = bytes.ReplaceAll(enc, []byte(`\u003e`), []byte(">"))
			enc = bytes.ReplaceAll(enc, []byte(`\u0026`), []byte("&"))
		}
		b.Write(enc)
	}
	return nil
}
