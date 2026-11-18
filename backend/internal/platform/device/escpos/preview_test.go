package escpos

import (
	"strings"
	"testing"
)

// TestPreview renders the sample ticket and logs a human-readable approximation
// of the paper output. It asserts nothing; it exists to *see* the layout.
//
//	go test ./internal/platform/device/escpos/ -run Preview -v
//
// The preview decodes text back from the device codepage and honors alignment
// (ESC a). Control sequences (size, bold, cut, feed) are stripped, so the bold
// TOTAL and double-width business name look normal here — only spacing/wrapping
// reflect the real ticket.
func TestPreview(t *testing.T) {
	cases := []struct {
		name string
		opts Options
	}{
		{"80mm (48 cols)", Options{Width: 48, Codepage: CodepageCP850, Cut: CutPartial}},
		{"58mm (32 cols)", Options{Width: 32, Codepage: CodepageCP850, Cut: CutPartial}},
	}
	for _, tc := range cases {
		raw, err := Render(sampleTicket(), tc.opts)
		if err != nil {
			t.Fatalf("render %s: %v", tc.name, err)
		}
		t.Logf("\n%s\n%s", tc.name, preview(raw, tc.opts))
	}
}

// preview turns ESC/POS bytes back into an ASCII paper mock: it decodes text via
// the codepage, applies ESC a alignment, and frames each line at the paper width.
func preview(raw []byte, opts Options) string {
	cm, _ := charmapFor(opts.Codepage)
	width := opts.width()

	var out strings.Builder
	border := "+" + strings.Repeat("=", width) + "+\n"
	out.WriteString(border)

	align := 0
	var line []byte
	flush := func() {
		decoded, _ := cm.NewDecoder().Bytes(line)
		line = line[:0]
		text := string(decoded)
		pad := width - len([]rune(text))
		if pad > 0 {
			switch align {
			case 1: // center
				left := pad / 2
				text = strings.Repeat(" ", left) + text + strings.Repeat(" ", pad-left)
			case 2: // right
				text = strings.Repeat(" ", pad) + text
			default: // left
				text = text + strings.Repeat(" ", pad)
			}
		}
		out.WriteString("|")
		out.WriteString(text)
		out.WriteString("|\n")
	}

	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case escByte: // ESC: @ is 2 bytes; a/t/E/d are 3 bytes (track alignment for 'a')
			if i+1 < len(raw) && raw[i+1] == '@' {
				i++
				continue
			}
			if i+1 < len(raw) && raw[i+1] == 'a' && i+2 < len(raw) {
				align = int(raw[i+2])
			}
			i += 2
		case gsByte: // GS: ! n and V n are both 3 bytes
			i += 2
		case lfByte:
			flush()
		default:
			line = append(line, raw[i])
		}
	}
	out.WriteString(border)
	return out.String()
}
