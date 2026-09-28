package watch

import (
	"encoding/binary"
	"encoding/xml"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestScheduledTaskXMLUsesUTF16LE(t *testing.T) {
	root := Root{Path: `C:\Arbeit\Wälder & Co`, Workspace: true}
	data := scheduledTaskXML(root, `C:\Programme\GoreGraph.exe`, `DOMAIN\Änne`)
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xfe {
		t.Fatalf("scheduled task XML lacks UTF-16LE BOM: %x", data[:min(len(data), 2)])
	}
	if (len(data)-2)%2 != 0 {
		t.Fatalf("scheduled task XML has an odd number of UTF-16LE bytes: %d", len(data)-2)
	}
	units := make([]uint16, (len(data)-2)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[2+i*2:])
	}
	body := string(utf16.Decode(units))
	const declaration = "<?xml version=\"1.0\" encoding=\"UTF-16\"?>"
	if !strings.HasPrefix(body, declaration) {
		t.Fatalf("unexpected XML declaration: %.60q", body)
	}
	var task struct {
		UserID string `xml:"Principals>Principal>UserId"`
		Exec   struct {
			Command   string `xml:"Command"`
			Arguments string `xml:"Arguments"`
		} `xml:"Actions>Exec"`
	}
	if err := xml.Unmarshal([]byte(strings.TrimPrefix(body, declaration)), &task); err != nil {
		t.Fatalf("decoded task XML is invalid: %v", err)
	}
	if task.UserID != `DOMAIN\Änne` || task.Exec.Command != `C:\Programme\GoreGraph.exe` {
		t.Fatalf("task identity or executable changed: %+v", task)
	}
	if want := `watch run "C:\Arbeit\Wälder & Co" --workspace`; task.Exec.Arguments != want {
		t.Fatalf("task arguments = %q, want %q", task.Exec.Arguments, want)
	}
}
