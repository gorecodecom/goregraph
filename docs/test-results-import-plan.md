# Plan: optional test-result evidence

Status: explicit local JUnit import and dashboard display implemented in
unreleased 1.4.3. CI connections and credential storage are out of scope.

## Goal and existing boundary

GoreGraph 1.4.3 inventories Storybook, Vitest, Playwright and related CI sources. Its
Tests & Tooling dashboard shows static declarations and links, not executed-test
results. Add optional, read-only JUnit import without changing normal scans,
`task_context`, the agent projection or the meaning of the existing inventory.
GoreGraph must never start application tests or contact a CI server during a scan.

The current Weka `frontends` project is a useful acceptance case: GitLab has a
Storybook build job, 20 sequential interaction shards with JUnit XML, two
separate browser checks with JUnit XML, and conditional, non-blocking Playwright
E2E jobs. Screenshot comparisons are prepared but not active in CI. Reports in
the local ignored `output/` directory may belong to different dates and do not
by themselves identify the current Git revision.

## Evidence model

Keep source configuration and execution evidence as separate records. A result
record needs project identity, suite/job identity, report format, origin
(`local` or an explicitly supplied CI artifact), import time, run time if known,
test/pass/failure/error/skip counts, and provenance such as commit, branch,
pipeline and job IDs. Mark metadata supplied by a caller as declared; only a
trusted artifact handoff can establish stronger provenance. Unknown fields stay unknown.
Do not infer a successful pipeline from a passing JUnit file or a current result
from a filename or file modification time.

Dashboard states are deliberately neutral unless a report proves a run:

| Evidence | Display |
| --- | --- |
| Tooling source found, no report imported | Configured / no result imported |
| Local report without trustworthy commit identity | Historical local result, revision unknown |
| Report with a caller-declared commit | Result for the imported report set; current-revision match unverified |
| Expected shards missing, malformed or far apart in time | Incomplete result; no aggregate pass |
| CI job configured but conditional or non-blocking | Preserve the condition/policy as source evidence; do not invent a run |

“Not run” is reserved for an explicit run record stating that status. A missing
file means only that GoreGraph has no imported result. Counts are test-run counts,
not code coverage. Keep the existing dashboard's static snapshot usable when no
result data exists or when an older export lacks the new field.

## Delivery stages

1. **Contract and fixtures — implemented.** A versioned result record is separate
   from `index/tooling.json` and the agent Context Pack. Tests cover Storybook and
   Playwright-style JUnit, 20 shards, skipped tests, XML errors and absent shards.
   Shard numbering, count and timestamps establish structural completeness only;
   they do not authenticate that files came from one pipeline.
2. **Opt-in local import — implemented.** The CLI accepts selected JUnit
   files/directories. It reads reports without executing tests or scanning
   source; it has bounded file, XML and testcase limits, rejects DTD/external
   entities, and publishes validated data atomically. A bad report yields a
   useful import error while the last valid dashboard/index remains intact.
   No report discovery or import runs by default during `scan` or `update`.
3. **Dashboard projection — implemented.** A compact “Results” area within Tests & Tooling
   visually separate from the source inventory. Show source, date and revision
   next to any status. Provide neutral empty and incomplete states, clear
   per-suite details, and no cross-project or cross-revision aggregation. Refresh
   the offline dashboard from the imported sidecar and existing index without a
   full source rescan; leave the agent projection unchanged.
4. **CI handoff — out of scope.** Reports can be downloaded separately and
   explicitly imported as local files. Caller-supplied CI labels remain
   unverified. GoreGraph stores no credentials, downloads no artifacts and does
   not add a dependency to any CI or version-control provider. HTML reports,
   screenshots and coverage files are not pass/fail evidence.

## Regression gates

- With no imported reports, `scan`, `update`, `task_context`, existing dashboards
  and generated agent files keep their current behavior; no additional source
  reads, test executions, network calls or meaningful scan-time regression.
- Malformed or oversized XML, duplicate files, widely separated report times, missing shards,
  skipped tests and reports with both failures and errors cannot produce green.
  Escaped test names render as text and cannot inject dashboard HTML.
- An imported result cannot overwrite source-inventory facts or turn a static
  `allow_failure` declaration into a pipeline verdict. Older dashboard exports
  continue to load with a neutral “results unavailable” state.
- Unit and CLI tests cover parsing and atomic publication. Offline browser checks
  should cover empty, historical, incomplete, pass/fail, responsive and accessible
  states where the browser runtime is available; scripted rendering assertions
  cover the neutral and escaped-text states independently. Compare unchanged
  scan/update outputs and timing before release.

The README documents the implemented import and its evidence limits. Automatic
CI refresh is intentionally excluded.
