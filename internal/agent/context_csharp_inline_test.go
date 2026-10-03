package agent

import (
	"strings"
	"testing"
)

func TestCSharpInlineClassSourceCanBeDeliveredWithoutChoosingAnOverload(t *testing.T) {
	body := "using UnityEngine;\npublic class Receiver:MonoBehaviour { public int count; public void Press(){} public void Press(int value){} }\n"
	file := sourceFile{Path: "Receiver.cs", Lines: strings.Split(body, "\n")}
	section, err := renderSourceCandidate(sourceCandidate{Path: file.Path, Name: "Receiver", Qualified: "Receiver", Kind: "symbol", StartLine: 2}, file, "focused")
	if err != nil || !strings.Contains(section.Content, "Press(int value)") {
		t.Fatal(err, section)
	}
}
