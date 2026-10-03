package scan

type csharpParameter struct {
	name, typeName, modifier string
	optional, variadic       bool
}

func csharpParameterInfo(tokens []csharpToken) []csharpParameter {
	var result []csharpParameter
	for _, part := range csharpSplit(tokens, ",") {
		parameter := csharpParameter{}
		for i, token := range part {
			if token.text == "=" {
				parameter.optional = true
				part = part[:i]
				break
			}
		}
		for _, token := range part {
			if token.text == "params" {
				parameter.variadic = true
			}
			if token.text == "ref" || token.text == "out" || token.text == "in" {
				parameter.modifier = token.text
			}
		}
		part = csharpWithoutModifiers(part)
		if len(part) < 2 {
			parameter.typeName = "unknown"
		} else {
			parameter.name = part[len(part)-1].text
			parameter.typeName = csharpJoined(part[:len(part)-1])
		}
		result = append(result, parameter)
	}
	return result
}

func csharpBindArguments(member csharpMember, tokens []csharpToken) ([][]csharpToken, bool) {
	if csharpParameterCount(tokens) < 0 {
		return nil, false
	}
	args := csharpSplit(tokens, ",")
	bound := make([][]csharpToken, len(member.parameterInfo))
	for ordinal, arg := range args {
		index := ordinal
		if len(arg) > 1 && arg[1].text == ":" {
			index = -1
			for i, parameter := range member.parameterInfo {
				if parameter.name == arg[0].text {
					index = i
					break
				}
			}
			arg = arg[2:]
		}
		if index < 0 || index >= len(bound) || bound[index] != nil || len(arg) == 0 {
			return nil, false
		}
		parameter := member.parameterInfo[index]
		if parameter.variadic {
			return nil, false
		}
		modifier := ""
		if arg[0].text == "ref" || arg[0].text == "out" || arg[0].text == "in" {
			modifier = arg[0].text
			arg = arg[1:]
		}
		if modifier != parameter.modifier && !(modifier == "" && parameter.modifier == "in") {
			return nil, false
		}
		bound[index] = arg
	}
	for i, parameter := range member.parameterInfo {
		if bound[i] == nil && !parameter.optional {
			return nil, false
		}
	}
	return bound, true
}
