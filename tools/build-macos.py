#!/usr/bin/env python3
"""Build portable macOS archives on any Go development host (Python 3.9+)."""

import argparse
import hashlib
import io
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", default="dev")
    parser.add_argument("--arch", nargs="+", choices=["amd64", "arm64"], default=["amd64", "arm64"])
    parser.add_argument("--output-path", default="dist/macos")
    args = parser.parse_args()
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", args.version):
        parser.error("version must contain only letters, numbers, dots, underscores or hyphens")
    root = Path(__file__).resolve().parent.parent
    portable_go = root / ".tools/go/bin/go.exe"
    go = str(portable_go) if os.name == "nt" and portable_go.is_file() else shutil.which("go")
    if not go:
        parser.error("Go was not found on PATH or under .tools/go")
    output = (root / args.output_path).resolve()
    output.mkdir(parents=True, exist_ok=True)
    stage = Path(tempfile.mkdtemp(prefix="mss-macos-build-")).resolve()
    checksums = []
    try:
        for arch in dict.fromkeys(args.arch):
            binary = stage / "mihomo-smart-selector"
            env = dict(os.environ, CGO_ENABLED="0", GOOS="darwin", GOARCH=arch,
                       GOAMD64="v1", GOARM64="v8.0")
            subprocess.run([go, "build", "-trimpath", "-ldflags", "-s -w -X main.version=" + args.version,
                            "-o", str(binary), "./cmd/mihomo-smart-selector"], cwd=root, env=env, check=True)
            subprocess.run([go, "version", "-m", str(binary)], cwd=root, check=True)
            archive = output / ("mihomo-smart-selector-" + args.version + "-darwin-" + arch + ".tar.gz")
            # Explicit public files only. Set archive permissions even when building on Windows.
            files = [(binary, "mihomo-smart-selector", 0o755),
                     (root / "deploy/macos/start.command", "start.command", 0o755),
                     (root / "config.example.yaml", "config.example.yaml", 0o644),
                     (root / "LICENSE", "LICENSE", 0o644),
                     (root / "docs/macos.md", "README-macOS.md", 0o644)]
            with tarfile.open(archive, "w:gz", format=tarfile.USTAR_FORMAT) as bundle:
                for source, name, mode in files:
                    data = source.read_bytes()
                    if name == "start.command":
                        data = data.replace(b"\r\n", b"\n")
                    info = tarfile.TarInfo(name)
                    info.size, info.mode, info.mtime = len(data), mode, 0
                    bundle.addfile(info, io.BytesIO(data))
            checksums.append(hashlib.sha256(archive.read_bytes()).hexdigest() + "  " + archive.name)
            print("Packaged " + str(archive), flush=True)
        (output / "SHA256SUMS.txt").write_bytes(("\n".join(checksums) + "\n").encode("utf-8"))
    finally:
        # Validate the exact absolute recursive-cleanup target on every host.
        if stage.parent != Path(tempfile.gettempdir()).resolve() or not stage.name.startswith("mss-macos-build-"):
            raise RuntimeError("Unexpected staging path")
        shutil.rmtree(stage)


if __name__ == "__main__":
    main()
