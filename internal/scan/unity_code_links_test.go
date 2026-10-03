package scan

import (
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/gitignore"
)

func TestUnityFieldsAndPersistentEventsRetainSignatureBoundaries(t *testing.T) {
	for _, mode := range []string{"1", "3", "0"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "Assets/Handler.cs", "public class Handler { public int count; public void Press(){} public void Press(int count){} }")
			writeFile(t, root, "Assets/Handler.cs.meta", "guid: 11111111111111111111111111111111\n")
			writeFile(t, root, "Assets/Button.prefab", `%YAML 1.1
--- !u!114 &200
MonoBehaviour:
  m_Script: {fileID: 11500000, guid: 11111111111111111111111111111111, type: 3}
  count: 42
  onClick:
    m_PersistentCalls:
      m_Calls:
      - m_Target: {fileID: 200}
        m_MethodName: Press
        m_Mode: `+mode+`
        m_CallState: 2
`)
			index, _, err := scanProject(root, config.Defaults(), gitignore.Matcher{})
			if err != nil {
				t.Fatal(err)
			}
			fields, callbacks := 0, 0
			for _, ref := range index.SymbolFacts.References {
				if ref.Type == "serialized_field" && ref.Resolution == SymbolResolutionExact {
					fields++
				}
				if ref.Type == "persistent_callback" {
					callbacks++
					if mode == "0" {
						if ref.Resolution == SymbolResolutionExact {
							t.Fatal(ref)
						}
					} else if ref.Resolution != SymbolResolutionExact || mode == "3" && !strings.HasSuffix(ref.TargetQualifiedName, "(int)") {
						t.Fatal(ref)
					}
				}
			}
			if fields != 1 || callbacks != 1 || len(index.CSharp.graph.Edges) != 0 {
				t.Fatal(fields, callbacks, index.CSharp.graph)
			}
		})
	}
}

func TestUnityInheritedSavedMembersRequireAnInstanceVoidCallback(t *testing.T) {
	for _, declaration := range []string{"public void Press(){}", "public int Press(){return 1;}", "public static void Press(){}"} {
		root := t.TempDir()
		writeFile(t, root, "Assets/Base.cs", "public class Base { public int count; "+declaration+" }")
		writeFile(t, root, "Assets/Handler.cs", "public class Handler:Base {}")
		writeFile(t, root, "Assets/Handler.cs.meta", "guid: 11111111111111111111111111111111\n")
		writeFile(t, root, "Assets/Button.prefab", `%YAML 1.1
--- !u!114 &200
MonoBehaviour:
  m_Script: {fileID: 11500000, guid: 11111111111111111111111111111111, type: 3}
  count: 42
  clicked:
    m_PersistentCalls:
      m_Calls:
      - m_Target: {fileID: 200}
        m_MethodName: Press
        m_Mode: 1
`)
		index, _, err := scanProject(root, config.Defaults(), gitignore.Matcher{})
		if err != nil {
			t.Fatal(err)
		}
		field, callback := false, false
		for _, ref := range index.SymbolFacts.References {
			field = field || ref.Type == "serialized_field" && ref.Resolution == SymbolResolutionExact && ref.To == "Assets/Base.cs"
			callback = callback || ref.Type == "persistent_callback" && ref.Resolution == SymbolResolutionExact && ref.To == "Assets/Base.cs"
		}
		if !field || callback != (declaration == "public void Press(){}") {
			t.Fatal(declaration, index.SymbolFacts.References)
		}
	}
}

func TestUnityStrippedPrefabComponentRetainsUniqueScriptCorrespondence(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "Assets/Handler.cs", "public class Handler {public void Press(){}}")
	writeFile(t, root, "Assets/Handler.cs.meta", "guid: 11111111111111111111111111111111\n")
	writeFile(t, root, "Assets/Base.prefab.meta", "guid: 22222222222222222222222222222222\n")
	writeFile(t, root, "Assets/Base.prefab", `%YAML 1.1
--- !u!114 &100
MonoBehaviour:
  m_Script: {fileID: 11500000, guid: 11111111111111111111111111111111, type: 3}
`)
	writeFile(t, root, "Assets/Scene.unity", `%YAML 1.1
--- !u!114 &200 stripped
MonoBehaviour:
  m_CorrespondingSourceObject: {fileID: 100, guid: 22222222222222222222222222222222, type: 3}
--- !u!114 &300
MonoBehaviour:
  clicked:
    m_PersistentCalls:
      m_Calls:
      - m_Target: {fileID: 200}
        m_MethodName: Press
        m_Mode: 1
`)
	index, _, err := scanProject(root, config.Defaults(), gitignore.Matcher{})
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range index.SymbolFacts.References {
		if ref.Type == "persistent_callback" && ref.Resolution == SymbolResolutionExact && ref.To == "Assets/Handler.cs" {
			return
		}
	}
	t.Fatal("prefab correspondence lost the callback target", index.SymbolFacts.References)
}

func TestUnityDynamicInheritedEventNeedsAnUnconditionalFieldSignature(t *testing.T) {
	for _, conditional := range []bool{false, true} {
		root := t.TempDir()
		field := "public UnityEvent<int> clicked;"
		if conditional {
			field = "\n#if OPTIONAL_EVENT\n" + field + "\n#endif\n"
		}
		writeFile(t, root, "Assets/Base.cs", "using UnityEngine.Events;\npublic class Base {"+field+"}")
		writeFile(t, root, "Assets/Handler.cs", "public class Handler:Base {public void Press(int value){} public void Press(string value){}}")
		writeFile(t, root, "Assets/Handler.cs.meta", "guid: 11111111111111111111111111111111\n")
		writeFile(t, root, "Assets/Button.prefab", `%YAML 1.1
--- !u!114 &200
MonoBehaviour:
  m_Script: {fileID: 11500000, guid: 11111111111111111111111111111111, type: 3}
  clicked:
    m_PersistentCalls:
      m_Calls:
      - m_Target: {fileID: 200}
        m_MethodName: Press
        m_Mode: 0
`)
		index, _, err := scanProject(root, config.Defaults(), gitignore.Matcher{})
		if err != nil {
			t.Fatal(err)
		}
		callback := false
		for _, ref := range index.SymbolFacts.References {
			if ref.Type == "persistent_callback" && ref.Resolution == SymbolResolutionExact {
				callback = true
				if !strings.HasSuffix(ref.TargetQualifiedName, "(int)") {
					t.Fatal(ref)
				}
			}
		}
		if callback == conditional || len(index.CSharp.graph.Edges) != 0 {
			t.Fatal("dynamic event signature was misclassified", conditional, callback, index.CSharp.graph)
		}
	}
}
