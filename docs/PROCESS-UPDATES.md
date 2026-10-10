# Process updates

The executable at the installation path is authoritative. GoreGraph does not
download releases or choose a different branch automatically. Installing a build
does not itself enable a watcher or start a stopped watcher.

## MCP on macOS and Linux

The stdio server checks its installation path between requests and approximately
once per second while idle. Package-manager symlink changes are followed. A
replacement must support the MCP handoff protocol and remain unchanged during
validation. Missing, incomplete or invalid replacements are reported on stderr;
the working process and connection remain available.

An active request completes and writes its response before replacement. The server
then executes the installed binary in place, preserving its PID, standard streams,
command-line options, and all unread input. This includes multiple queued requests
and an incomplete JSON message. No second initialization is required. The server
is stateless between calls; the existing input-size limit remains in effect.

Installers should stage the complete executable and atomically rename it into
place, or atomically switch the package-manager symlink. The installation directory
must remain trusted, just as for launching GoreGraph normally.

MCP processes started with a build predating this feature cannot acquire it in
memory. Reconnect those clients once after installing this build. Starting a
standalone server with unrelated stdin does not reconnect an existing client.

## Watchers

Existing supervised watchers validate changed executables, allow active updates
to drain, and start the new worker. On macOS/Linux the supervisor also replaces
itself in place. Autostart settings and selected roots remain intact. Unsupervised
legacy watchers require an explicit `goregraph watch restart <root>` once.

## Windows

Windows cannot replace a running process in place. On the next request after a
validated installation change, the MCP server exits with a reconnect diagnostic
instead of evaluating that request with an outdated analyzer. The MCP client must
reconnect. Idle detection and connection-preserving replacement are currently
macOS/Linux features. The Windows watcher supervisor continues to replace workers;
restarting the supervisor loads its new executable too.

## Verification

`goregraph version` describes the executable launched by the current shell, not
every previously started process. Inspect both workspace roots with
`goregraph watch status <root>` and check the reported commit, last successful
update, current activity and errors. An old output timestamp alone does not prove
that unchanged sources are stale. `doctor` checks output integrity separately.

The integration test builds two distinct binaries, replaces the installed file
while one stdio session stays open, rejects an invalid candidate, and verifies
responses from the new binary with preserved partial/pipelined input and protocol
selection. Watcher tests separately cover draining, invalid replacements and
installation symlinks.
