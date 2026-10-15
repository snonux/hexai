"""Small stdio client for recording the real experimental MCP server."""

import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


def request(process, request_id, method, params):
    message = {"jsonrpc": "2.0", "id": request_id, "method": method, "params": params}
    process.stdin.write(json.dumps(message) + "\n")
    process.stdin.flush()
    for line in process.stdout:
        response = json.loads(line)
        if response.get("id") == request_id:
            return response
    raise RuntimeError("MCP server closed stdout before replying")


def operation(name):
    prompt = {
        "name": "guide_demo", "title": "Review Go code",
        "arguments": [{"name": "code", "required": True}],
        "messages": [{"role": "user", "content": {
            "type": "text", "text": "Review this Go code: {{code}}"}}],
        "tags": ["go", "review"],
    }
    operations = {
        "list": ("prompts/list", {}),
        "create": ("tools/call", {"name": "create_prompt", "arguments": prompt}),
        "get": ("prompts/get", {"name": "guide_demo", "arguments": {"code": "return a + b"}}),
        "update": ("tools/call", {"name": "update_prompt", "arguments": {
            "name": "guide_demo", "title": "Review and explain Go code"}}),
        "delete": ("tools/call", {"name": "delete_prompt", "arguments": {"name": "guide_demo"}}),
        "missing": ("prompts/get", {"name": "guide_missing"}),
    }
    return operations[name]


def display(response):
    if "error" in response:
        print(json.dumps(response["error"]))
        return 1
    result = response["result"]
    if "prompts" in result:
        print("Prompts: " + ", ".join(p["name"] for p in result["prompts"]))
    else:
        print(json.dumps(result, indent=2))
    return int(result.get("isError", False))


def run_client(directory, name, workspace):
    binary = shutil.which("hexai-mcp-server")
    if binary is None:
        raise SystemExit("hexai-mcp-server must be installed and on PATH")
    # Stop project-config lookup here and bypass the user's global file.
    (workspace / ".git").mkdir()
    config = workspace / "config.toml"
    config.write_text("[mcp]\nslashcommand_sync = false\n")
    env = dict(os.environ, HEXAI_MCP_SLASHCOMMAND_SYNC="0")
    for key in ("XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME"):
        env[key] = str(workspace / key.lower())
    command = [str(Path(binary).resolve()), "--config", str(config),
               "--prompts-dir", str(directory), "--log", str(directory / "server.log"),
               "--slashcommand-dir", str(directory / "commands")]
    with (directory / "startup.log").open("a") as diagnostics:
        with subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                              stderr=diagnostics, text=True, env=env, cwd=workspace) as process:
            try:
                initialized = request(process, 1, "initialize", {
                    "protocolVersion": "2025-11-25", "capabilities": {},
                    "clientInfo": {"name": "hexai-guide", "version": "1"}})
                if "error" in initialized:
                    return display(initialized)
                process.stdin.write('{"jsonrpc":"2.0","method":"notifications/initialized"}\n')
                process.stdin.flush()
                method, params = operation(name)
                return display(request(process, 2, method, params))
            finally:
                process.stdin.close()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path)
    parser.add_argument("operation", choices=("list", "create", "get", "update", "delete", "missing"))
    args = parser.parse_args()
    directory = args.directory.resolve()
    directory.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="hexai-mcp-client-") as temporary:
        return run_client(directory, args.operation, Path(temporary))


if __name__ == "__main__":
    raise SystemExit(main())
