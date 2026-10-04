package scan

import (
	"path"
	"strings"
	"unicode"
)

type swiftTarget struct {
	module       string
	files        map[string]bool
	dependencies map[string]bool
}

func xcodeSwiftTargets(file string, body string, sources []swiftSource) ([]swiftTarget, bool) {
	project, ok := parseOpenStep(body)
	if !ok {
		return nil, false
	}
	objects := plistMap(project["objects"])
	root := plistMap(objects[plistString(project["rootObject"])])
	base := path.Dir(path.Dir(file))
	groupPaths := map[string]string{}
	seen := map[string]bool{}
	validGroups := true
	var walk func(string, string)
	walk = func(id, parent string) {
		object := plistMap(objects[id])
		name := plistString(object["path"])
		tree := plistString(object["sourceTree"])
		if tree != "" && tree != "<group>" && tree != "SOURCE_ROOT" {
			return
		}
		if tree == "SOURCE_ROOT" {
			parent = base
		}
		if strings.Contains(name, "$") || path.IsAbs(name) {
			return
		}
		location := path.Clean(path.Join(parent, name))
		if location == ".." || strings.HasPrefix(location, "../") {
			return
		}
		if seen[id] {
			if groupPaths[id] != location {
				validGroups = false
			}
			return
		}
		seen[id] = true
		groupPaths[id] = location
		for _, child := range plistList(object["children"]) {
			walk(plistString(child), location)
		}
	}
	walk(plistString(root["mainGroup"]), base)
	if !validGroups {
		return nil, false
	}
	modules := map[string]string{}
	for _, item := range plistList(root["targets"]) {
		id := plistString(item)
		target := plistMap(objects[id])
		if plistString(target["isa"]) != "PBXNativeTarget" {
			continue
		}
		name := plistString(target["name"])
		configurationList := plistMap(objects[plistString(target["buildConfigurationList"])])
		moduleName := ""
		for _, configuration := range plistList(configurationList["buildConfigurations"]) {
			settings := plistMap(plistMap(objects[plistString(configuration)])["buildSettings"])
			for key := range settings {
				if strings.HasPrefix(key, "PRODUCT_MODULE_NAME[") {
					return nil, false
				}
			}
			configured := plistString(settings["PRODUCT_MODULE_NAME"])
			if configured == "" {
				configured = name
			}
			if strings.Contains(configured, "$") || moduleName != "" && moduleName != configured {
				return nil, false
			}
			moduleName = configured
		}
		if moduleName != "" {
			name = moduleName
		}
		if !swiftLiteralModuleName(name) {
			return nil, false
		}
		modules[id] = path.Join(path.Dir(file), name)
	}
	var targets []swiftTarget
	for _, item := range plistList(root["targets"]) {
		id := plistString(item)
		module, exists := modules[id]
		if !exists {
			continue
		}
		target := plistMap(objects[id])
		result := swiftTarget{module: module, files: map[string]bool{}, dependencies: map[string]bool{module: true}}
		for _, groupID := range plistList(target["fileSystemSynchronizedGroups"]) {
			group := plistMap(objects[plistString(groupID)])
			location := groupPaths[plistString(groupID)]
			if location == "" || plistString(group["isa"]) != "PBXFileSystemSynchronizedRootGroup" {
				return nil, false
			}
			excluded := map[string]bool{}
			for _, exceptionID := range plistList(group["exceptions"]) {
				exception := plistMap(objects[plistString(exceptionID)])
				if plistString(exception["target"]) != id {
					continue
				}
				if plistString(exception["isa"]) != "PBXFileSystemSynchronizedBuildFileExceptionSet" || exception["platformFiltersByRelativePath"] != nil || exception["additionalCompilerFlagsByRelativePath"] != nil {
					return nil, false
				}
				for _, name := range plistList(exception["membershipExceptions"]) {
					excluded[path.Clean(path.Join(location, plistString(name)))] = true
				}
			}
			for _, source := range sources {
				if strings.HasPrefix(source.file, location+"/") && !excluded[source.file] {
					result.files[source.file] = true
				}
			}
		}
		for _, phaseID := range plistList(target["buildPhases"]) {
			phase := plistMap(objects[plistString(phaseID)])
			if plistString(phase["isa"]) != "PBXSourcesBuildPhase" {
				continue
			}
			for _, buildID := range plistList(phase["files"]) {
				build := plistMap(objects[plistString(buildID)])
				if build["platformFilter"] != nil || build["platformFilters"] != nil {
					return nil, false
				}
				ref := plistString(build["fileRef"])
				if location := groupPaths[ref]; location != "" {
					result.files[location] = true
				}
			}
		}
		for _, dependencyID := range plistList(target["dependencies"]) {
			dependency := plistMap(objects[plistString(dependencyID)])
			if targetModule := modules[plistString(dependency["target"])]; targetModule != "" {
				result.dependencies[targetModule] = true
			}
		}
		targets = append(targets, result)
	}
	return targets, len(targets) > 0
}

func assignSwiftTargets(sources []swiftSource, metadata []assetExportSource) {
	var targets []swiftTarget
	var failedScopes []string
	for _, file := range metadata {
		var parsed []swiftTarget
		var ok bool
		if path.Base(file.file.Path) == "Package.swift" {
			parsed, ok = swiftPackageTargets(file.file.Path, file.body, sources)
		} else {
			parsed, ok = xcodeSwiftTargets(file.file.Path, file.body, sources)
		}
		if !ok {
			scope := path.Dir(file.file.Path)
			if strings.HasSuffix(file.file.Path, ".pbxproj") {
				scope = path.Dir(scope)
			}
			failedScopes = append(failedScopes, scope)
		}
		targets = append(targets, parsed...)
	}
	if len(targets) == 0 && len(failedScopes) == 0 {
		return
	}
	for i := range sources {
		s := &sources[i]
		if path.Base(s.file) == "Package.swift" {
			continue
		}
		var owners []swiftTarget
		failed := false
		for _, scope := range failedScopes {
			if scope == "." || strings.HasPrefix(s.file, scope+"/") {
				failed = true
			}
		}
		for _, target := range targets {
			if target.files[s.file] {
				owners = append(owners, target)
			}
		}
		old := s.module
		if len(owners) == 1 && !failed {
			s.module = owners[0].module
			s.visibleModules = owners[0].dependencies
		} else {
			s.module = "unassigned/" + s.file
			s.limitations = append(s.limitations, "Swift source target is unassigned, shared, conditional or unsupported")
		}
		rewrite := func(name string) string {
			if name == old {
				return s.module
			}
			return strings.Replace(name, old+".", s.module+".", 1)
		}
		symbol := func(record *RichSymbolRecord) {
			record.Module = s.module
			record.Owner = rewrite(record.Owner)
			record.QualifiedName = rewrite(record.QualifiedName)
			record.ID = StableWorkspaceSymbolID(record.Kind, "", "", "swift", record.QualifiedName, record.File)
			if len(s.limitations) > 0 {
				record.Limitations = append(record.Limitations, "Swift target membership could not be uniquely verified")
			}
		}
		for j := range s.types {
			symbol(&s.types[j].symbol)
		}
		for j := range s.members {
			s.members[j].owner = rewrite(s.members[j].owner)
			symbol(&s.members[j].symbol)
		}
	}
}

func swiftLiteralModuleName(name string) bool {
	if name == "" {
		return false
	}
	for i, value := range name {
		if value != '_' && !unicode.IsLetter(value) && (i == 0 || !unicode.IsDigit(value)) {
			return false
		}
	}
	return true
}
