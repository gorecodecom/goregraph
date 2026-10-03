"""Explicit SourceKit export: python3 sourcekit.py REQUEST.json NEW.goregraph-swift.json.

Uses source.request.indexsource directly. Does not launch SourceKit-LSP, SwiftPM,
Xcode, project builds, or applications. Compiler arguments are explicitly supplied.
"""
import ctypes
import hashlib
import json
import re
import sys
from pathlib import Path


class Variant(ctypes.Structure):
    _fields_ = [("data", ctypes.c_uint64 * 3)]


class SourceKit:
    def __init__(self, library):
        self.lib = ctypes.CDLL(library)
        pointer = ctypes.c_void_p
        signatures = {
            "sourcekitd_initialize": (None, []),
            "sourcekitd_shutdown": (None, []),
            "sourcekitd_uid_get_from_cstr": (pointer, [ctypes.c_char_p]),
            "sourcekitd_request_uid_create": (pointer, [pointer]),
            "sourcekitd_request_string_create": (pointer, [ctypes.c_char_p]),
            "sourcekitd_request_dictionary_create": (pointer, [pointer, pointer, ctypes.c_size_t]),
            "sourcekitd_request_dictionary_set_value": (None, [pointer, pointer, pointer]),
            "sourcekitd_request_array_create": (pointer, [pointer, ctypes.c_size_t]),
            "sourcekitd_request_array_set_value": (None, [pointer, ctypes.c_size_t, pointer]),
            "sourcekitd_request_release": (None, [pointer]),
            "sourcekitd_send_request_sync": (pointer, [pointer]),
            "sourcekitd_response_is_error": (ctypes.c_bool, [pointer]),
            "sourcekitd_response_error_get_description": (ctypes.c_char_p, [pointer]),
            "sourcekitd_response_get_value": (Variant, [pointer]),
            "sourcekitd_variant_json_description_copy": (pointer, [Variant]),
            "sourcekitd_response_dispose": (None, [pointer]),
        }
        for name, (result, args) in signatures.items():
            function = getattr(self.lib, name)
            function.restype, function.argtypes = result, args
        self.free = ctypes.CDLL(None).free
        self.free.argtypes, self.free.restype = [pointer], None
        self.lib.sourcekitd_initialize()

    def uid(self, value):
        return self.lib.sourcekitd_uid_get_from_cstr(value.encode())

    def index(self, file, args, request_kind="source.request.indexsource"):
        request = self.lib.sourcekitd_request_dictionary_create(None, None, 0)
        values = [self.lib.sourcekitd_request_uid_create(self.uid(request_kind)),
                  self.lib.sourcekitd_request_string_create(str(file).encode()),
                  self.lib.sourcekitd_request_array_create(None, 0)]
        try:
            for arg in args:
                value = self.lib.sourcekitd_request_string_create(arg.encode())
                self.lib.sourcekitd_request_array_set_value(values[2], ctypes.c_size_t(-1).value, value)
                self.lib.sourcekitd_request_release(value)
            for key, value in zip(["key.request", "key.sourcefile", "key.compilerargs"], values):
                self.lib.sourcekitd_request_dictionary_set_value(request, self.uid(key), value)
            response = self.lib.sourcekitd_send_request_sync(request)
            try:
                if self.lib.sourcekitd_response_is_error(response):
                    raise RuntimeError(self.lib.sourcekitd_response_error_get_description(response).decode())
                raw = self.lib.sourcekitd_variant_json_description_copy(self.lib.sourcekitd_response_get_value(response))
                try:
                    return json.loads(ctypes.string_at(raw))
                finally:
                    self.free(raw)
            finally:
                self.lib.sourcekitd_response_dispose(response)
        finally:
            for value in values:
                self.lib.sourcekitd_request_release(value)
            self.lib.sourcekitd_request_release(request)


def safe_input(root, name):
    candidate = root / name
    resolved = candidate.resolve(strict=True)
    resolved.relative_to(root)
    if candidate.is_symlink() or Path(name).is_absolute() or any(parent.is_symlink() for parent in candidate.parents if parent != root and root in parent.parents):
        raise ValueError("Only project-relative regular inputs are accepted")
    return resolved


def digest(file):
    return hashlib.sha256(file.read_bytes()).hexdigest()


def declaration_kind(kind):
    suffix = kind.removeprefix("source.lang.swift.decl.")
    if suffix in {"class", "struct", "enum", "protocol", "typealias"}:
        return suffix
    if suffix == "enumelement":
        return "enum_case"
    if suffix == "function.constructor":
        return "constructor"
    if suffix.startswith("function."):
        return "function" if suffix == "function.free" else "method"
    if suffix in {"var.instance", "var.static", "var.class"}:
        return "property"
    return None


def require_source_only_syntax(file):
    # This is deliberately conservative, including strings/comments: an
    # interpolation can contain a macro, and macro expansion loads compiler code.
    # Arbitrary attributes can resolve to attached macros in imported modules.
    text = file.read_text()
    attributes = set(re.findall(r"@\s*([^\W\d]\w*)", text))
    intrinsics = {"available", "objc", "nonobjc", "escaping", "autoclosure",
                  "convention", "discardableResult", "dynamicCallable",
                  "dynamicMemberLookup", "propertyWrapper", "resultBuilder",
                  "globalActor", "preconcurrency", "retroactive", "unchecked", "main"}
    directives = set(re.findall(r"#\s*([^\W\d]\w*)", text))
    if (attributes - intrinsics or
            directives - {"if", "elseif", "else", "endif", "available", "unavailable", "sourceLocation"} or
            re.search(r"\bmacro\s+[^\W\d]|[@#]\s*`", text)):
        raise ValueError("Macro/attribute syntax requires explicit allow_macro_expansion=true; otherwise use static GoreGraph evidence")


def main():
    if len(sys.argv) != 3 or not sys.argv[2].endswith(".goregraph-swift.json"):
        raise ValueError("Usage: sourcekit.py REQUEST.json NEW.goregraph-swift.json")
    request = json.loads(Path(sys.argv[1]).read_text())
    root = Path(request["root"]).resolve(strict=True)
    inputs = {name: digest(safe_input(root, name)) for name in request["inputs"]}
    report = {"schema_version": 1, "language": "swift", "producer": "SourceKit indexsource",
              "inputs": inputs, "covered_files": [], "declarations": [], "references": [],
              "configuration": {"macro_expansion_allowed": request.get("allow_macro_expansion") is True}}
    if request.get("allow_macro_expansion") is not True:
        for module in request["modules"]:
            for name in module["files"]:
                require_source_only_syntax(safe_input(root, name))
    kit = SourceKit(request["sourcekit_library"])
    try:
        for module in request["modules"]:
            files = [safe_input(root, name) for name in module["files"]]
            arguments = module.get("compiler_args", [])
            if any("plugin" in arg or arg.startswith("@") for arg in arguments):
                raise ValueError("Plugin loading and response-file compiler arguments are unsupported")
            arguments = ["-module-name", module["name"], *arguments, *map(str, files)]
            for name, file in zip(module["files"], files):
                if name not in inputs or name in report["covered_files"]:
                    raise ValueError("Source omitted from inputs or assigned to multiple modules")
                lines = file.read_bytes().splitlines()
                diagnostics = kit.index(file, arguments, "source.request.diagnostics")
                errors = [item.get("key.description", "compiler error") for item in diagnostics.get("key.diagnostics", [])
                          if item.get("key.severity") == "source.diagnostic.severity.error"]
                if errors:
                    raise RuntimeError("SourceKit rejected " + name + ": " + "; ".join(errors))
                response = kit.index(file, arguments)
                report["covered_files"].append(name)

                def walk(entities, caller="", owner=""):
                    for entity in entities:
                        kind = entity.get("key.kind", "")
                        usr = entity.get("key.usr", "")
                        line, column = entity.get("key.line", 0), entity.get("key.column", 0)
                        token, after = "", b""
                        if 0 < line <= len(lines) and column > 0:
                            fragment = lines[line - 1][column - 1:].decode("utf-8")
                            match = re.match(r"`[^`]+`|[^\W\d]\w*|[/=\-+!*%<>&|^?~.]+", fragment)
                            if match:
                                token = match.group()
                                after = fragment[len(token):].lstrip().encode()
                        nested_caller, nested_owner = caller, owner
                        if kind.startswith("source.lang.swift.decl.extension."):
                            nested_owner = entity.get("key.name", owner)
                        declared = declaration_kind(kind)
                        implicit_constructor = declared == "constructor" and entity.get("key.is_implicit", False)
                        if declared and token and usr and (not entity.get("key.is_implicit", False) or implicit_constructor):
                            report["declarations"].append({"usr": usr, "name": token, "kind": declared,
                                "file": name, "line": line, "column": column, "owner": owner, "module": module["name"]})
                            if declared in {"method", "function", "constructor", "property"}:
                                nested_caller = usr
                            else:
                                nested_owner = entity.get("key.name", token)
                        elif kind.startswith("source.lang.swift.ref.") and token and usr:
                            # Operator values such as (+) remain symbol uses. SourceKit's
                            # bound operator expression and explicit argument/closure syntax
                            # provide static calls, without proving runtime activation.
                            operator = ".function.operator." in kind and not after.startswith(b")")
                            call = ".function." in kind and (operator or after.startswith(b"(") or after.startswith(b"{"))
                            report["references"].append({"usr": usr, "caller": caller, "name": token,
                                "file": name, "line": line, "column": column,
                                "kind": "calls_method_owner" if call else "uses_symbol"})
                        walk(entity.get("key.entities", []), nested_caller, nested_owner)
                walk(response.get("key.entities", []))
        if any(digest(safe_input(root, name)) != value for name, value in inputs.items()):
            raise RuntimeError("Input changed during export")
        with Path(sys.argv[2]).open("x", encoding="utf-8") as output:
            json.dump(report, output, indent=2, ensure_ascii=False)
            output.write("\n")
    finally:
        kit.lib.sourcekitd_shutdown()


if __name__ == "__main__":
    main()
