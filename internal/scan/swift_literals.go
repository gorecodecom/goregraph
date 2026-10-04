package scan

import (
	"strconv"
	"strings"
)

func swiftNumericLiteralCompatible(tokens []csharpToken, expected string) bool {
	if len(tokens) == 0 {
		return false
	}
	for _, token := range tokens {
		if token.kind != "number" && token.text != "." && token.text != "-" && token.text != "+" {
			return false
		}
	}
	value := strings.ReplaceAll(csharpJoined(tokens), "_", "")
	if _, err := strconv.ParseFloat(value, 64); err != nil {
		return false
	}
	expected = strings.TrimSuffix(expected, "?")
	switch expected {
	case "Double", "Float", "Float16", "Float32", "Float64", "CGFloat":
		return true
	}
	return false
}

// Literal default-type preference is only applied to equal non-defaulted labels.
func swiftBetterLiteralCandidates(candidates []swiftMember, args [][]csharpToken, variables map[string]string) []swiftMember {
	var result []swiftMember
	for index, candidate := range candidates {
		dominated := false
		for otherIndex, other := range candidates {
			if index == otherIndex || len(candidate.parameters) != len(args) || len(other.parameters) != len(args) {
				continue
			}
			better, comparable := false, true
			for at, arg := range args {
				value := swiftArgumentValue(arg)
				actual := swiftExpressionType(arg[value:], variables)
				left, right := other.parameters[at], candidate.parameters[at]
				if actual == "" || left.label != right.label {
					comparable = false
					break
				}
				if left.typeName == right.typeName {
					continue
				}
				if left.typeName == actual && swiftNumericLiteralCompatible(arg[value:], right.typeName) {
					better = true
					continue
				}
				comparable = false
				break
			}
			if comparable && better {
				dominated = true
				break
			}
		}
		if !dominated {
			result = append(result, candidate)
		}
	}
	return result
}
