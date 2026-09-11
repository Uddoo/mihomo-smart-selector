#!/usr/bin/env python3
"""Run an extracted package on native macOS using an isolated HTTP Controller."""

import argparse
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import platform
import socket
import struct
import subprocess
import tarfile
import tempfile
import threading
import time
from urllib.error import URLError
from urllib.request import ProxyHandler, Request, build_opener


class Controller(BaseHTTPRequestHandler):
    def do_GET(self):
        payload = {"/version": {"version": "package-test"},
                   "/proxies": {"proxies": {}},
                   "/providers/proxies": {"providers": {}}}.get(self.path)
        data = json.dumps(payload or {"error": "unknown path"}).encode()
        self.send_response(200 if payload is not None else 404)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *args):
        pass


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--archive-path", required=True)
    parser.add_argument("--expected-version", required=True)
    args = parser.parse_args()
    if platform.system() != "Darwin":
        parser.error("This smoke test requires native macOS")
    arch = {"arm64": "arm64", "x86_64": "amd64"}.get(platform.machine())
    if not arch:
        parser.error("Unsupported host architecture")
    archive = Path(args.archive_path).resolve()
    run = Path(tempfile.mkdtemp(prefix="mss-macos-test-")).resolve()
    install, other = run / "中文安装 with spaces", run / "other working directory"
    install.mkdir()
    other.mkdir()
    checks, children, logs = [], [], []
    fixture = ThreadingHTTPServer(("127.0.0.1", 0), Controller)
    threading.Thread(target=fixture.serve_forever, daemon=True).start()
    fixture_running = True
    env = dict(os.environ)
    env.pop("MIHOMO_SECRET", None)
    opener = build_opener(ProxyHandler({}))

    def check(condition, name):
        if not condition:
            raise RuntimeError("Failed: " + name + "; evidence: " + str(run))
        checks.append(name)

    def start(arguments, name):
        log = (run / (name + ".log")).open("wb")
        logs.append(log)
        child = subprocess.Popen(arguments, cwd=other, env=env, stdout=log, stderr=log)
        children.append(child)
        return child

    def request(path, payload=None, method=None):
        data = None if payload is None else json.dumps(payload).encode()
        req = Request(base + path, data=data, method=method,
                      headers={"Content-Type": "application/json"} if data else {})
        with opener.open(req, timeout=3) as response:
            body = response.read()
            return response.status, json.loads(body) if path.startswith("/api/") else body.decode()

    def boot(previous=""):
        deadline = time.monotonic() + 25
        while time.monotonic() < deadline:
            if app.poll() is not None:
                raise RuntimeError("Packaged executable exited: " + (run / "app.log").read_text())
            try:
                _, state = request("/api/v1/service")
                if state["status"] == "running" and state.get("instance_id") and state["instance_id"] != previous:
                    return state
            except (URLError, TimeoutError, ConnectionError):
                pass
            time.sleep(0.1)
        raise RuntimeError("Packaged service did not become ready; evidence: " + str(run))

    try:
        expected = {"mihomo-smart-selector", "start.command", "config.example.yaml", "LICENSE", "README-macOS.md"}
        with tarfile.open(archive) as bundle:
            members = bundle.getmembers()
            check(len(members) == 5 and {m.name for m in members} == expected and
                  all(m.isfile() for m in members), "archive contains only the five intended public files")
            for member in members:
                destination = install / member.name
                destination.write_bytes(bundle.extractfile(member).read())
                destination.chmod(member.mode)
        binary, launcher = install / "mihomo-smart-selector", install / "start.command"
        check(os.access(binary, os.X_OK) and os.access(launcher, os.X_OK), "archive retains executable permissions")
        magic, cpu = struct.unpack("<II", binary.read_bytes()[:8])
        check(magic == 0xFEEDFACF and cpu == {"amd64": 0x01000007, "arm64": 0x0100000C}[arch],
              "Mach-O architecture matches the native host")
        version = subprocess.check_output([str(binary), "-version"], cwd=other, env=env, text=True).strip()
        check(version.startswith("Mihomo Smart Selector " + args.expected_version + " (darwin/" + arch + "; go")
              and version.endswith(")"), "native binary prints the exact release version and architecture")
        check(subprocess.check_output([str(launcher), "-version"], cwd=other, env=env, text=True).strip() == version,
              "launcher handles Unicode and spaces from another working directory")
        subprocess.run([str(binary), "-init"], cwd=other, env=env, check=True)
        config = install / "config.yaml"
        check(config.is_file() and not (other / "config.yaml").exists(), "first configuration is beside the executable")
        original = config.read_bytes()
        subprocess.run([str(binary), "-init"], cwd=other, env=env, check=True)
        check(config.read_bytes() == original, "initialization preserves existing configuration bytes")
        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            app_port = listener.getsockname()[1]
        config.write_bytes(original.replace(b"127.0.0.1:8788", ("127.0.0.1:" + str(app_port)).encode())
                           .replace(b"127.0.0.1:9090", ("127.0.0.1:" + str(fixture.server_port)).encode()))
        configured = config.read_bytes()
        base = "http://127.0.0.1:" + str(app_port)
        app = start([str(binary)], "app")
        first = boot()
        check(request("/api/v1/health")[1]["mihomo_connected"], "no-argument binary connects to the isolated Controller")
        check("/assets/" in request("/")[1], "packaged binary serves the embedded dashboard")
        state = request("/api/v1/connection")[1]
        saved = request("/api/v1/connection", {"revision": state["revision"], "controller": state["saved"]["controller"],
                        "request_timeout_seconds": 13, "secret_action": "none"}, "PUT")[1]
        check(saved["restart_required"], "connection API persists pending settings")
        check(request("/api/v1/service/restart", {"instance_id": first["instance_id"], "confirm": True}, "POST")[0] == 202,
              "web restart accepts the current service instance")
        second = boot(first["instance_id"])
        state = request("/api/v1/connection")[1]
        check(not state["restart_required"] and state["active"]["request_timeout_seconds"] == 13,
              "web restart applies the saved connection")
        check(app.poll() is None, "restart retains the original native process")
        check(config.read_bytes() == configured, "startup and restart preserve YAML bytes")
        check((install / "data/selector.db.connection.json").is_file() and not (other / "data").exists(),
              "SQLite and connection settings resolve beside configuration")
        duplicate = start([str(binary)], "duplicate")
        check(duplicate.wait(timeout=10) != 0, "second instance exits promptly with failure")
        check("cannot listen" in (run / "duplicate.log").read_text(), "port conflict has an actionable diagnosis")
        check(request("/api/v1/service")[1]["instance_id"] == second["instance_id"], "second instance leaves the service intact")
        fixture.shutdown()
        fixture.server_close()
        fixture_running = False
        request("/api/v1/service/restart", {"instance_id": second["instance_id"], "confirm": True}, "POST")
        offline = boot(second["instance_id"])
        check(bool(offline["instance_id"]), "service restart recovers with an offline Controller")
        check(bool(request("/api/v1/connection")[1]["active"]["controller"]), "offline settings remain accessible")
        report = {"archive": str(archive), "sha256": hashlib.sha256(archive.read_bytes()).hexdigest(),
                  "version": version, "architecture": "darwin/" + arch, "system": platform.mac_ver()[0],
                  "checks": checks, "count": len(checks)}
        (run / "report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
        print(str(len(checks)) + " macOS package checks passed. Evidence: " + str(run), flush=True)
    finally:
        for child in children:
            if child.poll() is None:
                child.terminate()
                try:
                    child.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    child.kill()
                    child.wait(timeout=5)
        for log in logs:
            log.close()
        if fixture_running:
            fixture.shutdown()
            fixture.server_close()


if __name__ == "__main__":
    main()
