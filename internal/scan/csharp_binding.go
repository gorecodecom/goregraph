package scan

import (
	"strconv"
	"strings"
)

func csharpExpressionType(tokens []csharpToken, variables map[string]string) string {
	if len(tokens) == 1 {
		switch {
		case tokens[0].kind == "literal":
			return "string"
		case tokens[0].kind == "char_literal":
			return "char"
		case tokens[0].text == "true" || tokens[0].text == "false":
			return "bool"
		case tokens[0].kind == "identifier":
			return variables[tokens[0].text]
		}
	}
	if len(tokens) > 1 && tokens[0].text == "new" && tokens[1].kind == "identifier" {
		return tokens[1].text
	}
	value := csharpJoined(tokens)
	if _, err := strconv.ParseInt(value, 10, 32); err == nil {
		return "int"
	}
	if strings.Contains(value, ".") {
		if _, err := strconv.ParseFloat(value, 64); err == nil {
			return "double"
		}
	}
	return ""
}

func csharpPrimitiveType(name string) string {
	name = strings.TrimSuffix(name, "?")
	name = strings.TrimPrefix(name, "System.")
	switch name {
	case "Int32", "int":
		return "int"
	case "SByte", "sbyte":
		return "sbyte"
	case "Byte", "byte":
		return "byte"
	case "Int16", "short":
		return "short"
	case "UInt16", "ushort":
		return "ushort"
	case "UInt32", "uint":
		return "uint"
	case "UInt64", "ulong":
		return "ulong"
	case "Int64", "long":
		return "long"
	case "String", "string":
		return "string"
	case "Boolean", "bool":
		return "bool"
	case "Char", "char":
		return "char"
	case "Double", "double":
		return "double"
	case "Single", "float":
		return "float"
	case "Decimal", "decimal":
		return "decimal"
	}
	return ""
}

func csharpMatchingArguments(member csharpMember, args [][]csharpToken, variables map[string]string) bool {
	if len(args) != len(member.parameterTypes) {
		return false
	}
	for i, arg := range args {
		actualName := csharpExpressionType(arg, variables)
		if strings.HasSuffix(actualName, "?") && !strings.HasSuffix(member.parameterTypes[i], "?") && csharpPrimitiveType(actualName) != "string" {
			return false
		}
		actual := csharpPrimitiveType(actualName)
		expected := csharpPrimitiveType(member.parameterTypes[i])
		if actual != "" && expected != "" && csharpConversionRank(actual, expected) < 0 && !csharpConstantConversion(arg, expected) {
			return false
		}
	}
	return true
}

func csharpConversionRank(actual, expected string) int {
	if actual == expected {
		return 0
	}
	conversions := map[string]string{
		"sbyte": "short int long float double decimal", "byte": "short ushort int uint long ulong float double decimal",
		"short": "int long float double decimal", "ushort": "int uint long ulong float double decimal", "int": "long float double decimal",
		"uint": "long ulong float double decimal", "long": "float double decimal", "ulong": "float double decimal",
		"char": "ushort int uint long ulong float double decimal", "float": "double",
	}
	if strings.Contains(" "+conversions[actual]+" ", " "+expected+" ") {
		return 1
	}
	return -1
}

func csharpConstantConversion(tokens []csharpToken, expected string) bool {
	value, err := strconv.ParseInt(csharpJoined(tokens), 10, 32)
	if err != nil {
		return false
	}
	switch expected {
	case "sbyte":
		return value >= -128 && value <= 127
	case "byte":
		return value >= 0 && value <= 255
	case "short":
		return value >= -32768 && value <= 32767
	case "ushort":
		return value >= 0 && value <= 65535
	case "uint", "ulong":
		return value >= 0
	}
	return false
}

func csharpTargetBetter(actual, left, right string) (bool, bool) {
	if left == right {
		return false, true
	}
	if actual == left {
		return true, true
	}
	if actual == right {
		return false, false
	}
	forward := csharpConversionRank(left, right) >= 0
	reverse := csharpConversionRank(right, left) >= 0
	if forward && !reverse {
		return true, true
	}
	signed := map[string]string{"sbyte": "byte ushort uint ulong", "short": "ushort uint ulong", "int": "uint ulong", "long": "ulong"}
	if strings.Contains(" "+signed[left]+" ", " "+right+" ") {
		return true, true
	}
	return false, false
}

func csharpBetterOverloads(candidates []csharpMember, args [][]csharpToken, variables map[string]string) []csharpMember {
	var retained []csharpMember
	for i, candidate := range candidates {
		dominated := false
		for j, other := range candidates {
			if i == j {
				continue
			}
			better, comparable := false, true
			for k, arg := range args {
				actual := csharpPrimitiveType(csharpExpressionType(arg, variables))
				left, right := csharpPrimitiveType(other.parameterTypes[k]), csharpPrimitiveType(candidate.parameterTypes[k])
				if actual == "" || left == "" || right == "" {
					comparable = false
					break
				}
				preferred, compatible := csharpTargetBetter(actual, left, right)
				if !compatible {
					comparable = false
					break
				}
				better = better || preferred
			}
			if comparable && better {
				dominated = true
				break
			}
		}
		if !dominated {
			retained = append(retained, candidate)
		}
	}
	return retained
}

func csharpInheritedMembers(source csharpSource, owner, name string, members map[string][]csharpMember, sources []csharpSource, types map[string][]RichSymbolRecord, seen map[string]bool) []csharpMember {
	if seen[owner] {
		return nil
	}
	seen[owner] = true
	if declared := members[owner+"."+name]; len(declared) > 0 {
		return declared
	}
	var result []csharpMember
	for _, s := range sources {
		for _, typ := range s.types {
			if typ.symbol.QualifiedName != owner {
				continue
			}
			for _, base := range typ.bases {
				for _, target := range csharpFindTypes(s, owner, base, types) {
					result = append(result, csharpInheritedMembers(source, target.QualifiedName, name, members, sources, types, seen)...)
				}
			}
		}
	}
	return result
}
