package agent

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var assetJSONLine = regexp.MustCompile(`^\s*"([^"]+)"\s*:\s*(.*?)\s*,?\s*$`)
var assetYAMLLine = regexp.MustCompile(`^(\s*(?:-\s*)?)([A-Za-z_][A-Za-z0-9_]*)\s*:\s*(.*)$`)
var assetFileReference = regexp.MustCompile(`^\{fileID: -?[0-9]+(?:, guid: [a-fA-F0-9]{32})?(?:, type: [0-9]+)?\}$`)
var assetSafeScalarKeys = func() map[string]bool {
	safe := map[string]bool{}
	for _, key := range strings.Fields("schema_version engine producer_version source source_sha256 id name kind property target object frame vertices polygons triangles degenerate_polygons bounds_min bounds_max location position scale rotation_mode hidden_render hierarchy active indices animation_length deform slot index interpolation keyframes_truncated m_Name m_IsActive m_Enabled m_Layer m_TagString m_LoopTime m_Speed m_IsTrigger m_Weight m_ApplyRootMotion fileID guid type x y z w") {
		safe[key] = true
	}
	return safe
}()

func renderAssetSourceCandidate(candidate sourceCandidate, file sourceFile) ContextSourceSection {
	start := clampSourceLine(candidate.StartLine, len(file.Lines))
	state := "indexed_range_current"
	if at := strings.LastIndex(candidate.Qualified, "#"); at >= 0 {
		identity := candidate.Qualified[at+1:]
		encoded, _ := json.Marshal(identity)
		for i, line := range file.Lines {
			jsonIdentity := strings.Contains(line, "\"id\"") && strings.Contains(line, string(encoded))
			yamlIdentity := strings.HasPrefix(line, "--- !u!") && (strings.HasSuffix(strings.TrimSpace(line), "&"+identity) || strings.HasSuffix(strings.TrimSpace(line), "&"+identity+" stripped"))
			if jsonIdentity || yamlIdentity {
				if i+1 != start {
					state = "relocated_current"
				}
				start = i + 1
				break
			}
		}
	}
	end := min(len(file.Lines), start+30)
	start = max(1, start-30)
	lines := append([]string(nil), file.Lines[start-1:end]...)
	lines = redactAssetSourceLines(lines)
	return ContextSourceSection{Project: candidate.Project, Path: candidate.Path, StartLine: start, EndLine: end, Role: candidate.Role, RenderMode: "focused", SourceState: state, Content: renderAssetNumberedSource(lines, start)}
}

func redactAssetSourceLines(lines []string) []string {
	result := make([]string, len(lines))
	var numericArrays []bool
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		result[i] = redactAssetSourceLine(line)
		if match := assetJSONLine.FindStringSubmatch(line); len(match) > 0 && strings.TrimSpace(match[2]) == "[" {
			numeric := false
			switch match[1] {
			case "head", "tail", "bone_head", "bone_tail", "bone_matrix", "bounds_min", "bounds_max", "location", "position", "scale", "frame_value", "frame_range":
				numeric = true
			}
			numericArrays = append(numericArrays, numeric)
			continue
		}
		if trimmed == "[" {
			numeric := len(numericArrays) > 0 && numericArrays[len(numericArrays)-1]
			numericArrays = append(numericArrays, numeric)
			if numeric {
				result[i] = line
			}
			continue
		}
		if trimmed == "]" || trimmed == "]," {
			if len(numericArrays) > 0 {
				numericArrays = numericArrays[:len(numericArrays)-1]
			}
			continue
		}
		if len(numericArrays) > 0 && numericArrays[len(numericArrays)-1] {
			var value any
			if json.Unmarshal([]byte(strings.TrimSuffix(trimmed, ",")), &value) == nil {
				switch value.(type) {
				case float64, nil:
					result[i] = line
				}
			}
		}
	}
	return result
}

func redactAssetSourceLine(line string) string {
	if match := assetJSONLine.FindStringSubmatch(line); len(match) > 0 {
		value := strings.TrimSuffix(strings.TrimSpace(match[2]), ",")
		var scalar any
		if assetSafeScalarKeys[match[1]] && json.Unmarshal([]byte(value), &scalar) == nil {
			switch scalar.(type) {
			case string, float64, bool, nil:
				return line
			}
		}
		if value == "{" || value == "[" || value == "{}" || value == "[]" {
			return line
		}
		return strings.Replace(line, match[2], "\"<redacted>\"", 1)
	}
	if match := assetYAMLLine.FindStringSubmatch(line); len(match) > 0 {
		value := strings.TrimSpace(match[3])
		if value == "" {
			return line
		}
		if assetSafeScalarKeys[match[2]] {
			if _, err := strconv.ParseFloat(value, 64); err == nil {
				return line
			}
			if match[2] == "m_Name" || match[2] == "guid" || match[2] == "m_TagString" {
				return line
			}
		}
		if assetFileReference.MatchString(value) {
			return line
		}
		return match[1] + match[2] + ": <redacted>"
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "{" || trimmed == "}" || trimmed == "}," || trimmed == "]" || trimmed == "]," || strings.HasPrefix(trimmed, "--- !u!") || strings.HasPrefix(trimmed, "%YAML") || strings.HasPrefix(trimmed, "%TAG") {
		return line
	}
	if trimmed == "" {
		return line
	}
	return "<redacted>"
}

func renderAssetNumberedSource(lines []string, start int) string {
	var result strings.Builder
	for i, line := range lines {
		if i > 0 {
			result.WriteByte('\n')
		}
		result.WriteString(strconv.Itoa(start + i))
		result.WriteByte('\t')
		result.WriteString(line)
	}
	return result.String()
}
