#!/usr/bin/env python3
"""Exercise deployed citizenship MCP without recording fabricated learner progress."""

import argparse
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid


class Client:
    def __init__(self, url, token):
        self.url = url.rstrip("/")
        self.token = token
        self.request_id = 0

    def request(self, path, payload=None, auth=None):
        headers = {"Accept": "application/json, text/event-stream"}
        if auth:
            headers["Authorization"] = auth
        data = None if payload is None else json.dumps(payload).encode()
        if data:
            headers["Content-Type"] = "application/json"
        request = urllib.request.Request(self.url + path, data=data, headers=headers)
        try:
            response = urllib.request.urlopen(request, timeout=30)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            return response.status, response.headers, response.read().decode()

    def rpc(self, method, params=None, auth=None, path="/mcp"):
        self.request_id += 1
        payload = {"jsonrpc": "2.0", "id": self.request_id, "method": method}
        if params is not None:
            payload["params"] = params
        status, _, body = self.request(path, payload, auth or "Bearer " + self.token)
        assert status == 200, f"{method}: HTTP {status}"
        if body.startswith("event:") or body.startswith("data:"):
            body = next(line[5:].strip() for line in body.splitlines() if line.startswith("data:"))
        result = json.loads(body)
        assert "error" not in result, f"{method}: JSON-RPC error"
        return result["result"]

    def tool(self, name, arguments=None, expect_error=False):
        result = self.rpc("tools/call", {"name": name, "arguments": arguments or {}})
        assert bool(result.get("isError")) == expect_error, f"{name}: unexpected tool result"
        if expect_error:
            return result
        if "structuredContent" in result:
            return result["structuredContent"]
        return json.loads(next(item["text"] for item in result["content"] if item["type"] == "text"))


def verify_auth(client):
    assert client.request("/healthz")[0] == 200, "health check"
    payload = {"jsonrpc": "2.0", "id": 1, "method": "tools/list"}
    for auth in (None, "Bearer wrong-token", "Bearer ", "Basic " + client.token):
        status, headers, _ = client.request("/mcp", payload, auth)
        assert status == 401 and headers.get("WWW-Authenticate", "").startswith("Bearer"), "authentication rejection"
    assert client.request("/mcp")[0] == 401, "unauthenticated GET"
    client.rpc("tools/list", auth="bearer " + client.token)
    client.rpc("tools/list", path="/mcp?token=" + urllib.parse.quote(client.token))
    print("PASS health, authentication, header and query token support")


def verify_content(client, version):
    initialized = client.rpc("initialize", {
        "protocolVersion": "2025-03-26", "capabilities": {},
        "clientInfo": {"name": "citizenship-deployment-smoke", "version": "1"},
    })
    info = initialized["serverInfo"]
    assert info["name"] == "bulgarian-citizenship-tutor", "server mode"
    assert info["version"] == version, "release version"
    assert "ONE question and wait" in initialized["instructions"], "live teaching instructions"
    names = {tool["name"] for tool in client.rpc("tools/list")["tools"]}
    expected = {"list_citizenship_materials", "get_citizenship_plan", "get_citizenship_lesson",
                "record_citizenship_progress", "get_citizenship_question", "check_citizenship_answer",
                "get_citizenship_exam", "grade_citizenship_exam", "save_vocabulary",
                "list_vocabulary", "delete_vocabulary"}
    assert names == expected, "tool discovery"
    catalog = client.tool("list_citizenship_materials")
    rules = [item for item in catalog["materials"] if item["material_id"].startswith("study/grammar-rules/") and not item["material_id"].endswith("/README")]
    assert len(rules) >= 36, "grammar curriculum"
    assert len(catalog["tests"]) >= 6 and all(item["ready"] for item in catalog["tests"]), "exam readiness"
    plan = client.tool("get_citizenship_plan")
    assert plan["total_sections"] == len(plan["sections"]) and plan["total_sections"] >= 120, "coverage plan"
    for material in rules:
        lesson = client.tool("get_citizenship_lesson", {"material_id": material["material_id"], "index": 1})
        assert lesson["markdown"] and lesson["is_last"], "one chapter per rule"
    videos = [item for item in catalog["materials"] if item.get("media_path")]
    assert len(videos) >= 3, "listening assets"
    for material in videos:
        lesson = client.tool("get_citizenship_lesson", {"material_id": material["material_id"], "index": 2})
        assert lesson["markdown"] and lesson["media_path"].endswith(".mp4"), "video transcript"
    for test in catalog["tests"]:
        exam = client.tool("get_citizenship_exam", {"test_id": test["test_id"]})
        assert len(exam["questions"]) == 20 and exam["duration_minutes"] == 60 and exam["pass_mark"] == 12, "mock format"
        assert "answers" not in exam, "exam keys must be withheld"
        assert all("answer" not in question and "explanation" not in question for question in exam["questions"]), "question keys must be withheld"
        arguments = {"test_id": test["test_id"], "index": 1}
        question = client.tool("get_citizenship_question", arguments)
        assert question["reading"] and len(question["question"]["choices"]) == 4, "reading practice"
        feedback = client.tool("check_citizenship_answer", {**arguments, "answer": ""})
        assert not feedback["correct"], "blank answer grading"
        correct = client.tool("check_citizenship_answer", {**arguments, "answer": feedback["correct_answer"]})
        assert correct["correct"], "correct answer grading"
        client.tool("grade_citizenship_exam", {"test_id": test["test_id"], "answers": []}, expect_error=True)
    after = client.tool("get_citizenship_plan")
    assert after["latest_exams"] == plan["latest_exams"], "incomplete submissions must not record scores"
    client.tool("list_vocabulary")
    print(f"PASS v{version}, 11 tools, {len(catalog['materials'])} documents, {len(rules)} grammar chapters, {len(catalog['tests'])} exams, {plan['total_sections']} study sections")


def verify_notebook(client):
    term = "deployment-smoke-" + uuid.uuid4().hex
    try:
        client.tool("save_vocabulary", {"term": term, "kind": "word", "translation": "temporary deployment check"})
        found = client.tool("list_vocabulary", {"query": term})
        assert term in json.dumps(found), "vocabulary persistence"
    finally:
        client.tool("delete_vocabulary", {"term": term})
    assert term not in json.dumps(client.tool("list_vocabulary", {"query": term})), "temporary term cleanup"
    print("PASS vocabulary save/list/delete; temporary entry removed")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("url", nargs="?", default="https://bgtutor-mcp.f3s.buetow.org")
    parser.add_argument("--version", default="0.30.0")
    parser.add_argument("--write", action="store_true", help="verify vocabulary writes with a temporary entry")
    args = parser.parse_args()
    token = os.environ.get("BGTUTOR_TOKEN")
    if not token:
        parser.error("set BGTUTOR_TOKEN in the environment")
    client = Client(args.url, token)
    verify_auth(client)
    verify_content(client, args.version)
    if args.write:
        verify_notebook(client)
    print("All citizenship deployment checks passed")


if __name__ == "__main__":
    try:
        main()
    except AssertionError as error:
        # Assertions use fixed diagnostic messages, never authenticated URLs.
        print(f"FAIL {error}", file=sys.stderr)
        sys.exit(1)
    except (urllib.error.URLError, KeyError, ValueError) as error:
        # Do not echo URLs or HTTP bodies, which may contain connector secrets.
        print(f"FAIL {type(error).__name__}: deployment check failed", file=sys.stderr)
        sys.exit(1)
