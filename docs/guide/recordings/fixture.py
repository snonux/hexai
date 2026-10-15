"""Local, deterministic OpenAI-compatible responses for guide recordings only."""

import json
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def answer(messages, model):
    system = messages[0].get("content", "").lower()
    prompt = messages[-1].get("content", "")
    if "proofreader" in system:
        return "The function returns the sum of two integers."
    if "documentation engine" in system:
        return "// Add returns the sum of a and b.\nfunc Add(a, b int) int { return a + b }"
    if "unit test generator" in system:
        return ('func TestAdd(t *testing.T) {\n'
                '\tif got := Add(2, 3); got != 5 {\n'
                '\t\tt.Fatalf("Add(2, 3) = %d; want 5", got)\n\t}\n}')
    if "completion" in system:
        if "return the sum" in prompt.lower():
            return "a + b"
        if "hello" in prompt.lower():
            return 'fmt.Println("Hello, Hexai!")'
        return "a + b"
    if "refactoring engine" in system or "improvement engine" in system:
        return "func Add(a, b int) int { return a + b }"
    if "slice" in prompt.lower():
        return "A slice is a view of an array, with a length and capacity."
    if "capacity" in prompt.lower():
        return "Capacity is the number of elements available before a new allocation."
    if "command" in prompt.lower() or "ripgrep" in prompt.lower():
        return "rg --files -g '*.go'"
    if model == "demo-careful":
        return "Add accepts two integers and returns their sum."
    return "Add returns the sum of two integers."


class FixtureHandler(BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def do_POST(self):
        if self.path == "/outage/v1/chat/completions":
            self.send_response(503)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"error":{"message":"Demo primary unavailable"}}')
            return
        if self.path != "/v1/chat/completions":
            self.send_error(404)
            return
        request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        reply = answer(request.get("messages", []), request.get("model", ""))
        streaming = request.get("stream", False)
        self.send_response(200)
        content_type = "text/event-stream" if streaming else "application/json"
        self.send_header("Content-Type", content_type)
        self.end_headers()
        try:
            if streaming:
                self.stream_reply(reply)
            else:
                body = {"choices": [{"message": {"role": "assistant", "content": reply}}]}
                self.wfile.write(json.dumps(body).encode())
        except (BrokenPipeError, ConnectionResetError):
            pass  # The editor may cancel obsolete completions while typing.

    def stream_reply(self, reply):
        for start in range(0, len(reply), 5):
            chunk = {"choices": [{"delta": {"content": reply[start:start + 5]}}]}
            self.wfile.write(f"data: {json.dumps(chunk)}\n\n".encode())
            self.wfile.flush()
            time.sleep(0.035)
        self.wfile.write(b"data: [DONE]\n\n")


def make_server():
    return ThreadingHTTPServer(("127.0.0.1", 0), FixtureHandler)
