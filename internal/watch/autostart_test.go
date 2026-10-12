package watch

import (
	"encoding/binary"
	"encoding/xml"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestScheduledTaskXMLUsesUTF16LE(t *testing.T) {
	data := scheduledTaskXML(`C:\Arbeit\Wälder & Co\O'Brien\autostart.js`, `DOMAIN\Änne`)
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
	if task.UserID != `DOMAIN\Änne` || task.Exec.Command != `%SystemRoot%\System32\wscript.exe` {
		t.Fatalf("task identity or executable changed: %+v", task)
	}
	if !strings.Contains(body, "<RestartOnFailure><Interval>PT1M</Interval><Count>255</Count></RestartOnFailure>") {
		t.Fatal("scheduled task does not recover a failed supervisor")
	}
	for _, want := range []string{"<DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>", "<StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>", "<ExecutionTimeLimit>PT0S</ExecutionTimeLimit>"} {
		if !strings.Contains(body, want) {
			t.Fatalf("scheduled watcher can stop because of battery or time limits: missing %q", want)
		}
	}
	if want := `//B //NoLogo //E:JScript "C:\Arbeit\Wälder & Co\O'Brien\autostart.js"`; task.Exec.Arguments != want {
		t.Fatalf("task arguments = %q, want %q", task.Exec.Arguments, want)
	}
}

func TestLaunchAgentSupervisesFailuresAndPreservesExplicitStop(t *testing.T) {
	t.Setenv("GOREGRAPH_WATCH_HOME", t.TempDir())
	root := Root{Path: "/projects/A & B", ID: "test", Workspace: true}
	body, err := launchAgentBody(root, "/bin/goregraph")
	if err != nil {
		t.Fatal(err)
	}
	if err := xml.Unmarshal(body, new(any)); err != nil {
		t.Fatalf("invalid launch agent XML: %v", err)
	}
	for _, want := range []string{
		"<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>",
		"<string>supervise</string>", "<string>/projects/A &amp; B</string>",
		"<key>ExitTimeOut</key><integer>0</integer>", "<key>GOREGRAPH_WATCH_HOME</key>",
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("launch agent missing %q", want)
		}
	}
}

func TestSystemdSupervisesWithoutCancellingCurrentUpdate(t *testing.T) {
	t.Setenv("GOREGRAPH_WATCH_HOME", t.TempDir())
	body := systemdServiceBody(Root{ID: "test"}, []string{"\"/bin/goregraph\"", "\"watch\"", "\"supervise\""})
	for _, want := range []string{"Restart=on-failure", "RestartSec=2", "StartLimitIntervalSec=0", "KillMode=process", "TimeoutStopSec=infinity", "GOREGRAPH_WATCH_HOME="} {
		if !strings.Contains(body, want) {
			t.Fatalf("service missing %q", want)
		}
	}
}
