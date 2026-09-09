# Scan benchmarking and Windows compatibility baseline

This document defines repeatable local measurements for script extraction and
end-to-end scan operations. Keep raw test output, profiles, source snapshots,
and machine-specific paths in an external evidence directory. Commit only the
synthetic benchmark sources and this method.

## Capture the test environment

Before each run, record the following values in the evidence directory:

- UTC timestamp, operating system, architecture, CPU, logical processor count,
  and physical memory when the host exposes it;
- `go version`, `goregraph version`, and the effective `GOCACHE`, `GOMOD`, and
  `GOTMPDIR` values;
- `git rev-parse HEAD`, `git status --short --branch`, and a source snapshot ID;
- the exact command, arguments, working directory, workload identity, and output
  path.

Use a fresh evidence directory for each candidate. Do not place retained test
JSON, CPU profiles, allocation profiles, or private source in the repository.
If another build or test changes the checkout or consumes CPU during a run,
record that fact and exclude the run from performance comparisons.

## Correctness baseline

Run the full suite once and retain the complete JSON stream:

```powershell
go test ./... -json -timeout 20m |
  Tee-Object -FilePath $evidenceDirectory/go-test-all.json
```

Re-run only failed test names, one package at a time, with `-count=1 -v`. A
Windows runner must use directories owned by the account that launches `go` and
`git`. Set per-run `TEMP`, `TMP`, and `GOCACHE` directories when the host uses an
isolated execution account. Do not add a global Git `safe.directory`, relax
source-path confinement, or change ACLs merely to make the suite pass.

The baseline investigation at commit `081b405383692c27cbae254e949c6ea947f4d6f8`
used Go 1.26.4 on Windows amd64 (Windows 10.0.26200) and GoreGraph 1.4.0. The
first attempt could not use `%LOCALAPPDATA%\go-build`: cached objects returned
`Access is denied`. A task-local `GOCACHE` allowed compilation, but the sandbox
account still could not call `filepath.EvalSymlinks` on files created below the
normal user's `%TEMP%` tree. This produced two downstream symptom families:

- context output reported `source path escapes project root`, including
  `TestResolveSourcePathUsesSelectedIndexScope` and
  `TestSourceDerivedGeneralityWorkspace`;
- Git update fixtures were classified as `not_git`, including
  `TestRunPreviewReportsWouldUpdateWithoutChangingRepository` and
  `TestRunGitUpdateDefaultsToPreview`.

The same Git update tests and the generality acceptance test passed when run by
the normal Windows user with a separate temporary Go cache. A live sandbox-owned
fixture also completed `git rev-parse --show-toplevel`, while the direct
`filepath.EvalSymlinks` assertion returned `Access is denied`. These results
classify the failures as runner temporary-path ownership/confinement failures,
not a defect in GoreGraph's path-boundary comparison or Git update state
machine. Configure an owned temporary root for the isolated runner and repeat
the full suite before claiming a green baseline.

That investigation ran while other implementation tasks were changing the
working tree. Its partial full-suite result is useful only for failure
classification; its elapsed time is not a performance baseline.

## Script symbol extraction benchmark

The benchmark workload is generated source rather than copied application code.
For each requested size, generate independent named functions, calls to one
imported symbol, nested arrows, destructuring scopes, and TSX constructs. Pad
with comments to the exact byte target. Use stable workload identities for
16,384, 32,768, 65,536, and 131,072 bytes, plus a separate correctness case near
the 512 KiB input limit.

Run five timing/allocation samples sequentially, with no other test or build
load:

```powershell
go test ./internal/scan -run '^$' `
  -bench '^BenchmarkExtractScriptSymbolFactsLexical$' -benchmem -count=5
```

Run CPU and allocation profiles separately from the five final samples. Record
the profile command and benchmark selector beside each profile. If `go tool
pprof` is unavailable, record that limitation and repeat with a supported Go
toolchain; do not infer percentages from stack samples.

## End-to-end protocols

Use one frozen synthetic workspace snapshot for all operations. Run processes
sequentially and record wall time, peak resident memory when available, input
file count and bytes, emitted fact and diagnostic counts, and output bytes.
Preserve stdout, stderr, the manifest, and the exact command in the external
evidence directory.

### Cold build

Start from a copied frozen source snapshot with no GoreGraph output. Use a fresh
process, empty per-run Go cache when measuring compilation separately, and cold
filesystem state only when the operating system provides a documented cache
reset. Run `goregraph workspace build all <path> --workspace <workspace>
--no-update-gitignore`. Do not label an ordinary first process as a cold
filesystem run.

### Unchanged update

After a successful build, leave sources and generated output untouched. Start a
new process and run `goregraph workspace update <path> --workspace <workspace>
--target all --no-update-gitignore`. Verify that every project is reported
unchanged and that output content remains identical where the format promises
determinism.

### One-file update

Restore the built snapshot, change one designated synthetic source file by a
fixed byte sequence, and start a new update process with the same arguments.
Record the changed project set and verify that unrelated projects remain
unchanged. Restore the source snapshot after the measurement.

### Output loading

Measure loading independently from scanning. Start a new process against the
already generated output and use the read-only command under test, such as
`goregraph doctor <path>` or a fixed `goregraph query` invocation. Record the
output snapshot ID and bytes read so different candidates consume identical
artifacts.

### Context command

Use one fixed query, token budget, and file limit against the same generated
agent index:

```powershell
goregraph context <path> --query '<fixed query>' `
  --budget-tokens 4000 --max-files 12
```

Run each sample in a fresh process. Record the context ID, estimated tokens,
source coverage, source section and omission counts, output bytes, elapsed time,
and peak memory. Do not retry unless the returned pack explicitly allows it; a
retry is a separate workload and must not be mixed into first-call results.


## Local 1.4.1 implementation observations

The lexical-scope benchmark retained the same synthetic facts as the previous
implementation. A 100-group extraction workload previously took a median of
3.586 seconds. The indexed implementation measured 36–44 ms without competing
work and a median of 131.5 ms under concurrent test/fuzz load. The latter is a
conservative 27.3x extraction-only observation, not an end-to-end scan or token
saving. A subsequent five-second fuzz run executed 16,548 inputs without failure.

`BenchmarkProjectAgentBuild` generates 24 TypeScript modules with 40 functions
each and exercises extraction, projection, validation and transactional output.
A local Windows run (`-benchtime=3x`) measured 1.603 seconds/op, 52.45 MB/op and
145,170 allocations/op. Its separate CPU profile attributed approximately 3.34%
of samples cumulatively to `ExtractScriptSymbolFactsContext`, 9.89% to project
extraction, and 65.46% to transactional output (overlapping cumulative costs,
not additive percentages). System calls dominated the profile. Other agents
were active; these are bottleneck observations, not an uncontended release gate.
The profile was decoded with the installed standard-library `go run cmd/pprof`
because that installation did not include a prebuilt `go tool pprof` executable.

Decision for B3/B4: retain no persistent fact cache or speculative loading map.
The measured extraction share does not justify another invalidation and disk-I/O
layer. Existing per-operation symbol lookup maps and lazy usage shards remain.
The unchanged-update path instead verifies input identities and skips publication
when all requested projections are current. Broader cold/warm measurements on
representative workspaces remain necessary before claiming a 50% one-file-update
or complete-scan improvement. The historical 86% token result is independent of
these scan timings; no new paid agent trials were run for this local candidate.
