# Godot and GDScript static analysis

GoreGraph recognizes `project.godot` as a project marker, `.gd` as GDScript,
`.tscn` and `.tres` as Godot text resources, and `export_presets.cfg` as Godot
configuration. The generated `.godot/` directory is excluded by default.
No Godot editor, game, import process or test runner is started.

The adapters emit Schema 3 declarations and references in `symbols-full.json`
and `relations-full.json`. GDScript also contributes `callgraph.json` and
`test-map.json`. Their capability level is **partial**: an exact source binding
identifies a declaration, not the target chosen by runtime virtual dispatch.

## Five implementation rounds

Each round adds three related capabilities:

| Round | Improvement 1 | Improvement 2 | Improvement 3 |
| --- | --- | --- | --- |
| 1 | Godot project discovery and cache exclusion | GDScript file detection and capability profile | Scene, resource and configuration detection and profile |
| 2 | Script classes, `class_name` and explicit inheritance | Typed, static and multiline function declarations with source spans | Fields, constants, signals, named enums and annotated fields |
| 3 | Literal `preload`/`load` resource references | Local and explicitly preloaded static method calls | Methods on explicitly constructed locals and unchanged initialized fields |
| 4 | Scene node declarations with parent paths | External/subresource references and attached scripts | Saved scene signal connections to unique script methods |
| 5 | Project main scene and autoload dependencies | Literal signal emission and local callback references | Test-source target maps, integrated output and adapter revision invalidation |

## Binding rules

`res://` paths resolve from the project root. Relative `preload()` paths resolve
from the containing script; relative runtime `load()` paths resolve from the
project root. Only files in the indexed inventory can become
internal references. Empty, escaping, absolute, escaped, `user://`, `uid://` and
computed paths remain unbound. No external files are opened to resolve a path.

A top-level immutable `const Model = preload("res://model.gd")` supplies a
script receiver. A static `Model.create()` can link a unique source method.
A simple explicit construction such as `var model := Model.new()` supplies a
local receiver; an initialized top-level field can supply one when it has no
other assignment in the file. These bindings require the complete initializer
to be a literal preload or simple construction. Conditional initializers and
chained expressions remain unresolved. Local variables, match-pattern captures and loop iterators only
shadow names within their indentation block. Reassignment, parameter/local shadowing, unknown
receivers and complex receiver chains prevent binding. This is source evidence,
not a claim that external code cannot modify an object or field.

Own methods and `self.method()` resolve within the current script. Built-in
engine calls and inherited method dispatch are not guessed. Duplicate global
`class_name` declarations produce ambiguous inheritance references. Nested
classes are isolated from outer declarations, loader bindings and field writes,
and are not indexed; qualified parents such as `extends "res://base.gd".Inner`
remain unresolved rather than binding to the outer class. Comments and ordinary/triple-quoted strings cannot
create declarations or calls. Anonymous function bodies are excluded from call
binding; calls outside those bodies retain their own lexical scope.

Inline and multiline annotations retain top-level declarations. Implicit line
joining preserves function bodies even when a closing delimiter starts at
column zero. Named lambdas inside expressions do not become script methods.

Text scenes/resources preserve external resource IDs, embedded resource IDs,
node parent paths and script properties. Nested property arrays and multiline
header groups preserve section boundaries; instance resources in node headers
contribute resource-use references. Unterminated headers remain unbound. A saved connection links a callback
only when its source and target nodes, target script and method are unique.
Duplicate IDs/nodes, missing resources and inherited scene expansion remain
unresolved. Connections and callback values are references rather than executed
call edges. `emit_signal("name")`, own `name.emit()`, direct callback values and
`Callable(self, "method")` provide supported literal signal evidence.

Functions in `tests/` directories or `*_test.gd` files contribute test-source
links through proven direct calls, including custom `SceneTree` harnesses.
These records do not prove test discovery by Godot, execution, assertions or
passing results. No server routes, HTTP clients, persistence or end-to-end data
flow are claimed by these adapters.

The syntax follows the [Godot GDScript reference](https://docs.godotengine.org/en/stable/tutorials/scripting/gdscript/gdscript_basics.html)
and [text scene format](https://docs.godotengine.org/en/stable/engine_details/file_formats/tscn.html).
