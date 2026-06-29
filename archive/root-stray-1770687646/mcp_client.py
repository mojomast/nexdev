#!/usr/bin/env python3
"""Simple MCP client for Geoffrey"""
import json
import subprocess
import sys

class MCPClient:
    def __init__(self, geoffrussy_bin, project_path):
        self.process = subprocess.Popen(
            [geoffrussy_bin, "mcp-server", "--project-path", project_path],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1
        )
        self.request_id = 0

    def send_request(self, method, params=None):
        self.request_id += 1
        request = {
            "jsonrpc": "2.0",
            "id": self.request_id,
            "method": method
        }
        if params is not None:
            request["params"] = params

        self.process.stdin.write(json.dumps(request) + "\n")
        self.process.stdin.flush()

        # Read response
        response_line = self.process.stdout.readline()
        if response_line:
            return json.loads(response_line)
        return None

    def send_notification(self, method, params=None):
        request = {
            "jsonrpc": "2.0",
            "method": method
        }
        if params is not None:
            request["params"] = params

        self.process.stdin.write(json.dumps(request) + "\n")
        self.process.stdin.flush()

    def close(self):
        self.process.stdin.close()
        self.process.wait()

if __name__ == "__main__":
    project_path = "/home/mojo/projects/geoffrussy/geoffrussy/game-arcade-mcp"
    geoffrussy_bin = "/home/mojo/projects/geoffrussy/geoffrussy/bin/geoffrussy"

    client = MCPClient(geoffrussy_bin, project_path)

    # Initialize
    print("=" * 80)
    print("MCP CALL #1: Initialize")
    print("=" * 80)
    result = client.send_request("initialize", {
        "protocolVersion": "2024-11-05",
        "capabilities": {},
        "clientInfo": {"name": "claude-mcp-client", "version": "1.0"}
    })
    print(json.dumps(result, indent=2))

    # Send initialized notification
    client.send_notification("initialized")

    # List tools
    print("\n" + "=" * 80)
    print("MCP CALL #2: List Tools")
    print("=" * 80)
    result = client.send_request("tools/list")
    if result and "result" in result:
        tools = result["result"].get("tools", [])
        print(f"Found {len(tools)} tools:")
        for tool in tools:
            print(f"  - {tool['name']}: {tool.get('description', 'No description')}")

    client.close()
