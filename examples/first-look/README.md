# First look: follow a function call

This workspace contains a tiny Go project. Its entry point calls
`greeting.Message`; no network access, dependencies, or AI setup is needed.

From this directory, with GoreGraph installed:

```sh
goregraph workspace build dashboard . --workspace . --no-update-gitignore
goregraph workspace dashboard open . --workspace .
```

1. Select **greeter** in the sidebar and open **Service Code** (*Service-Code*).
2. Search for `Message` and select the function in `greeting/message.go`.
3. Under **Used by** (*Wird verwendet von*), open the evidence for `main.go`.
   The file and line number show where the message is created.
4. Open **Data quality** (*Datenqualität*) to see the snapshot and what the
   index can cover. An empty usage list does not prove that a symbol is unused.

You can also open the HTML file printed by the build. Keep its
`workspace-map-assets` directory beside it so the usage evidence can load.
