# Documentation index: webscope

**Reader:** people

Before writing a new doc, check here whether the topic already exists; never explain the same thing twice.

| Document | Question it answers |
|---|---|
| [README.md](README.md) | Where is each document of `docs/`, and who is it for? |
| [usage.md](usage.md) | What does each tool guarantee, and in what order are they used together to test a screen or migrate a module? |
| [install.md](install.md) | How is webscope installed in Claude Code (plugin or not) and in other agents? |
| [adr/0002-plugin-builds-the-binary.md](adr/0002-plugin-builds-the-binary.md) | How does the Go binary reach the machine of someone who installs the plugin? (The plugin builds it on first start.) |
| [module-testing.md](module-testing.md) | What is the full procedure to test a module as a user and as an attacker, and how are findings classified? |
| [patterns.md](patterns.md) | Which known findings show up when testing screens, how are they recognized, and what to do with each? |
| [adr/0001-page-observes.md](adr/0001-page-observes.md) | Should `page` stop driving the screen and only observe, now that Claude in Chrome has native tools? (No: measured, the native path costs more.) |
| [research/README.md](research/README.md) | What has already been looked up (with source, date and confidence) before searching again? |
