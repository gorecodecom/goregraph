# C#, Unity and Blender evidence

These adapters add to the existing Schema 3 symbols, relations, callgraph, test-map,
agent context and workspace indexes. They do not replace Java, Spring, or script
resolution. Normal scans, watcher refreshes and MCP queries never launch Unity or
Blender, compile C#, execute project code, or regenerate exports.

## C#

The source adapter tokenizes comments and literals separately. It records namespaces,
using aliases, type declarations, inheritance, nested types, constructors, methods,
parameter signatures, fields and properties. Unique supported static receiver/arity
bindings, including supported inherited methods and known literal overloads, produce callgraph and direct-usage edges. NUnit, xUnit and Unity test
attributes contribute declaration-based test-to-code links; no execution or passing
result is inferred. Literal ASP.NET controller/HTTP attributes and typed HttpClient
literal requests contribute route/contract facts. `.csproj` project/package references
and `.asmdef` / `.asmref` assembly dependencies are separate metadata evidence.
Unique indexed `.meta` GUIDs resolve assembly references; `.asmref` folders retain
the referenced assembly's code boundary. Duplicate GUIDs remain ambiguous.

This is a static pattern adapter, not Roslyn or an MSBuild evaluation. Conditional
compilation is not evaluated. Ambiguous overloads, unknown receivers, reflection,
extension methods, virtual dispatch, dynamic strings and unresolved external
assemblies remain uncertain. Generic constraints, complex receiver expressions,
local functions, complex top-level statements and compiler-generated members may be absent. Supported top-level DI and minimal API declarations are indexed as source facts.
Method source declarations can be navigation entrypoints; this does not establish
that Unity invokes a callback or that the method runs successfully.

See [language analysis](LANGUAGE-ANALYSIS.md) for deeper C# and Swift coverage, conservative binding rules and compiler-fixture validation.

## Native Unity assets

Unity YAML documents contribute object names, class kinds, selected non-secret
properties and source-line-backed GUID/fileID references. This covers scenes,
prefabs, materials, serialized assets, animations, Animator controllers and playable
assets. A unique MonoScript GUID can connect an asset to its C# class. Duplicate
GUIDs remain ambiguous; missing local fileIDs are diagnosed. External/package GUIDs
are unresolved evidence and are not automatically classified as broken references.

`index/assets.json` contains the detailed nodes, references and diagnostics. Objects
also enter the existing full-symbol and relation outputs and the agent context.
Asset diagnostics enter the existing canonical diagnostic families. There is no
new automatic runtime validation and no claim of complete 3D contact detection.

Authored asset selection uses separate limits in `goregraph.yml`:

```yaml
max_asset_file_size_kb: 16384
max_binary_asset_size_kb: 262144
```

The ordinary code-file limit remains separate. Supported binary Unity models,
images and audio files are inventoried and hashed incrementally, not interpreted as
source. Binary `.blend` files are likewise inventoried. Scan and watcher snapshots
use the same selection and hashes. Oversized/ignored files remain outside coverage.
Unity-generated root `Library`, `Temp`, `obj`, `Logs`, `UserSettings` and generated
IL2CPP project trees are excluded. No cache or package installation is triggered.

Unity projects can be discovered from `ProjectSettings/ProjectVersion.txt` and
`Packages/manifest.json` without an IDE-generated `.csproj`. Directories containing
`.blend` files can be discovered as separate asset collections. A sibling art
collection remains a separate workspace source, not a backend service or a made-up
runtime dependency. This preserves the Unity code root and lets the workspace query
include both actual registered source collections.

## Explicit Blender export

Write the bundled template to a new file (existing files are never overwritten):

```sh
goregraph assets exporter blender --output /tmp/goregraph-blender.py
```

Run Blender explicitly, with auto-execution of project scripts disabled:

```sh
blender --background --factory-startup --disable-autoexec /path/to/Art/Knight.blend \
  --python /tmp/goregraph-blender.py -- \
  --root /path/to/Art --output /path/to/Art/Knight.goregraph-blender.json \
  --frames 1,10,20
```

The root must contain the source `.blend`. Put the report within the same indexed
project/asset collection. The exporter never saves the source blend. It atomically
writes objects, parent/material/modifier/constraint/action references, mesh topology
counts, shape-key names, armature bone hierarchy, action ranges, bounded animation
channels/keyframes and driver metadata.
Selected scene frames contribute evaluated mesh counts, degenerate-polygon counts
and world bounds, plus world-space pose-bone endpoints and matrices. It restores the
scene frame and releases evaluated meshes.
Blender evaluates selected frames only; viewport settings, external dependencies,
unsampled frames, containment and exhaustive collision correctness are not proven.

The source SHA-256 must match an indexed file before report objects become active
facts. Invalid, duplicate-ID, out-of-project and stale reports contribute diagnostics
instead of current object facts. Source hashing confirms freshness, not the accuracy
or authenticity of a third-party report. The scanner never executes the exporter.
Adaptive context queries also verify the original asset hash before delivering an
export as current source evidence, including changes since the last watcher update.
Asset source sections are bounded and redact unsupported serialized values; source
read receipts preserve the original file hash and actual line numbers.
Frame samples have separate source-backed navigation identities. Queries naming an
object or bone and a frame can select that sample; supported numeric vectors and
matrices remain visible while unknown serialized values stay redacted.

## Explicit Unity editor export

```sh
goregraph assets exporter unity --output /path/to/Unity/Assets/Editor/GoreGraphAssetExporter.cs
```

Adding an Editor script is an explicit project change. Invoke it manually in your
chosen Editor, for example:

```sh
unity run /path/to/Unity -- -executeMethod GoreGraphAssetExporter.Export \
  -goregraph-source Assets/Knight.prefab \
  -goregraph-output /path/to/Unity/Evidence/Knight.goregraph-unity.json
```

The template reads persistent imported assets, hierarchy, components, serialized
references, mesh counts/bounds, skinning bones, blend shapes and animation metadata.
It does not instantiate prefabs, open scenes, save assets or run gameplay. Starting
an Editor can itself refresh its library/importers; that is why this is a manual
operation and is never part of an AI query or normal scan. Reports are SHA-256-checked
like Blender reports. Exported GlobalObjectIds can link to native indexed GUID/fileID
objects; unindexed external targets remain unresolved.

## Validation and activation

Regression fixtures cover source literals/comments, typed calls, ambiguous overloads,
conditional providers, dynamic HTTP expressions, test links, native Unity references,
stale Blender reports, source-scope restrictions, metadata dependencies and identical
scan/watch inventories. Real exporter smoke tests use disposable Blender and Unity
projects; they do not touch game projects. Agent-context tests verify bounded source
receipts and explicitly named C# sources.

Changes to these analyzers invalidate old generations via the extractor/agent
revision. Install/activate the new binary only as an explicit user operation. A
current user-enabled watcher can then regenerate the indexes with the new analyzer;
old MCP processes need restarting to use that binary. No publication is required.
