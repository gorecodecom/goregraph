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

The default source path is a static pattern adapter, not an MSBuild evaluation. Conditional
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

Workspace discovery respects directory-scoped `.gitignore` rules, including
negation. Ignored scratch trees and archived Unity/Blender copies are not
registered as independent projects.

Unique script classes also connect serialized field names to their source members,
including supported inherited members. Unique prefab component correspondence can
retain the source script for stripped scene components. Supported saved UnityEvent
callbacks use the target component/script, method name and fixed listener signature;
supported dynamic event signatures come from a verified `UnityEvent` source field.
Only uniquely matched instance methods returning `void` are bound. Unknown object
arguments, unsupported nested event paths, conditional providers and ambiguous
signatures remain open. These are `serialized_field`/`persistent_callback` use
relations, never executable callgraph edges. Adaptive context can deliver both the
saved prefab and explicitly named receiver code, within its normal budgets and
redaction/read-receipt rules. It does not prove the event is active or invoked.

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
counts, shape keys, armature bone hierarchy, action ranges, bounded animation
channels/keyframes and driver variable/target links. Materials and node groups retain
image/group/object references and node connections; collections and scenes retain
membership. Pose constraints and per-object pose-bone identities preserve instance
ownership even when multiple armature objects share one data block.
Selected scene frames contribute evaluated mesh counts, degenerate-polygon counts
and world bounds, plus world-space pose-bone endpoints and matrices. It restores the
scene frame and releases evaluated meshes.
Blender evaluates selected frames only; viewport settings, external dependencies,
unsampled frames, containment and exhaustive collision correctness are not proven.

Optional surfaces make detailed downstream geometry inspection possible:

```sh
blender --background --disable-autoexec /path/to/Art/Knight.blend \
  --python /tmp/goregraph-blender.py -- \
  --root /path/to/Art --output /path/to/Art/Knight.goregraph-blender.json \
  --frames 1,10 --geometry
```

`--geometry` includes evaluated world-space vertex positions and triangle indices
only for samples marked `geometry_complete: true`. Shared export budgets default
to 20,000 vertices and 40,000 triangles across the selected objects/frames. Explicit
`--max-geometry-vertices`/`--max-geometry-triangles` settings are capped at
100,000/200,000; the report must fit 16 MiB. Samples exceeding the remaining budget
retain counts/bounds and mark geometry incomplete. The exporter does not silently
present a partial vertex subset as a complete surface, nor perform collision proofs.

The source and owned linked-library SHA-256 hashes must match indexed files before
report objects become active facts. Outside-root libraries and textures remain
unverified dependencies and are named as limitations. Invalid coordinates, surface
indices, duplicate object/frame identities, duplicate IDs, out-of-project and stale
reports contribute diagnostics
instead of current object facts. Source hashing confirms freshness, not the accuracy
or authenticity of a third-party report. The scanner never executes the exporter.
Adaptive context queries also verify the original asset hash before delivering an
export as current source evidence, including owned linked libraries and changes
since the last watcher update. No external dependency is refreshed or exported.
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
  -goregraph-include-dependencies true \
  -goregraph-output /path/to/Unity/Evidence/Knight.goregraph-unity.json
```

The template reads persistent imported assets, hierarchy, components, serialized
references, mesh counts/bounds, skinning bones, blend shapes and animation metadata.
The optional `-goregraph-include-dependencies true` includes persistent owned
imported sub-assets such as referenced meshes and materials. It does not load
external project files. Regardless of this option, reports record SHA-256 hashes
for owned import dependencies and their `.meta` files, package configuration and
the Editor version. Changed dependencies invalidate exported evidence, even before
the next watcher update. Built-in and unowned references remain limited evidence.
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
