"""Record the real binaries in temporary, isolated config and task directories."""

import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading

from fixture import make_server


def write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content)


def prepare(root, endpoint):
    project = root / "hexai-demo"
    project.mkdir()
    subprocess.run(["git", "init", "-q", str(project)], check=True)
    config = f'''[provider]
name = "openai"
[openai]
base_url = "{endpoint}/v1"
model = "demo-fast"
[models.completion]
provider = "openai"
model = "demo-fast"
[completion]
completion_debounce_ms = 100
'''
    write(root / "config/hexai/config.toml", config)
    write(project / "single.toml", config)
    write(project / "multi.toml", config + '''
[[models.cli]]
provider = "openai"
model = "demo-fast"
[[models.cli]]
provider = "openai"
model = "demo-careful"
''')
    prepare_editor(root, project)
    prepare_actions(project, config)
    prepare_fallback(project, endpoint)
    write(root / "taskrc", f"data.location={root}/tasks\nconfirmation=no\n")
    (root / "tasks").mkdir()
    return project


def prepare_editor(root, project):
    write(root / "config/helix/languages.toml", '''[language-server.hexai]
command = "hexai-lsp-server"
[[language]]
name = "go"
language-servers = ["hexai"]
[[language]]
name = "markdown"
language-servers = ["hexai"]
''')
    write(root / "config/helix/config.toml", '''theme = "base16_default_dark"
[editor]
auto-completion = false
[editor.lsp]
auto-signature-help = false
[keys.select]
"A-a" = ":pipe hexai-tmux-action"
''')
    write(project / "add.go", "package demo\n\nfunc Add(a, b int) int {\n\treturn \n}\n")
    write(project / "chat.md", "# Ask Hexai\n\n")
    write(project / "inline.go", "package demo\n\nfunc Add(a, b int) int {\n\treturn \n}\n")
    write(project / "selection.go", "func Add(a, b int) int { return a + b }\n")
    write(project / "action.go", "package demo\n\nfunc Add(a, b int) int { return a + b }\n")
    write(project / "rewrite.go", "// simplify this function\nfunc Add(a, b int) int {\n\tresult := a + b\n\treturn result\n}\n")
    write(project / "notes.txt", "The functoin retunrs teh sum of two intergers.\n")


def prepare_actions(project, config):
    write(project / "custom.toml", config + '''
[[prompts.code_action.custom]]
id = "compact"
title = "Compact function"
hotkey = "e"
instruction = "Simplify the selection. Return only code."
[[tmux_action.menu]]
kind = "custom"
custom_id = "compact"
hotkey = "e"
[[tmux_action.menu]]
kind = "skip"
hotkey = "s"
''')
    write(project / "tmux.conf", '''set -g status-right '#{@hexai_status}'
set -g status-right-length 100
set -g status-left ' Hexai demo | '
set -g status-interval 1
set -g default-shell /bin/bash
set -g default-command 'bash --noprofile --rcfile "$BASH_ENV"'
''')


def prepare_fallback(project, endpoint):
    write(project / "fallback.toml", f'''[providers.primary]
type = "openai"
base_url = "{endpoint}/outage/v1"
model = "demo-fast"
[providers.backup]
type = "openai"
base_url = "{endpoint}/v1"
model = "demo-careful"
[[models.cli]]
provider = "primary"
fallback_provider = "backup"
''')


def environment(root, binaries, project):
    # Keep the user's HOME untouched. Only temporary XDG and Taskwarrior paths
    # are used. Drop inherited provider keys/config overrides from recordings.
    keep = ("PATH", "HOME", "DISPLAY", "LANG", "TERM", "FONTCONFIG_PATH")
    env = {key: os.environ[key] for key in keep if key in os.environ}
    env.update({
        "PATH": str(binaries) + os.pathsep + os.environ["PATH"],
        "XDG_CONFIG_HOME": str(root / "config"),
        "XDG_CACHE_HOME": str(root / "cache"),
        "XDG_STATE_HOME": str(root / "state"),
        "XDG_DATA_HOME": str(root / "data"),
        "TASKRC": str(root / "taskrc"),
        "TASKDATA": str(root / "tasks"),
        "HEXAI_OPENAI_API_KEY": "recording-fixture-only",
        "HEXAI_DEMO": str(project),
        "HEXAI_EDITOR": "hx",
        "HEXAI_TMUX_STATUS": "0",
        "PS1": "$ ",
        "PROMPT_COMMAND": "",
        "HISTFILE": "/dev/null",
        "BASH_ENV": str(root / "bash-env"),
        "VHS_NO_SANDBOX": "true",
    })
    write(root / "bash-env", "PS1='\\n$ '\nPROMPT_COMMAND=''\nHISTFILE=/dev/null\n")
    return env


def record(tape, binaries, repo):
    server = make_server()
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        with tempfile.TemporaryDirectory(prefix="hexai-guide-") as directory:
            root = Path(directory)
            project = prepare(root, f"http://127.0.0.1:{server.server_port}")
            env = environment(root, binaries, project)
            print(f"Recording {tape.name}", flush=True)
            try:
                subprocess.run(["vhs", str(tape)], cwd=repo, env=env, check=True)
            finally:
                subprocess.run(
                    ["tmux", "-S", str(project / "tmux.sock"), "kill-server"],
                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                )
    finally:
        server.shutdown()
        server.server_close()


def main():
    repo = Path(__file__).resolve().parents[3]
    binaries = Path(sys.argv[1]).resolve()
    tapes = Path(__file__).parent.glob("[0-9]*.tape")
    selected = set(sys.argv[2:])
    tapes = [t for t in sorted(tapes) if not selected or t.stem in selected]
    if not tapes or selected - {t.stem for t in tapes}:
        raise SystemExit("Unknown recording name; use a tape basename without .tape")
    (repo / "docs/guide/images").mkdir(parents=True, exist_ok=True)
    for tape in tapes:
        record(tape.resolve(), binaries, repo)


if __name__ == "__main__":
    main()
