#!/usr/bin/env python3
"""Collect exactly four checked desktop archives from this CI run's artifacts."""

import argparse
import hashlib
from pathlib import Path
import re
import shutil


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True)
    parser.add_argument("--input-path", default="dist/artifacts")
    parser.add_argument("--output-path", default="dist/release")
    args = parser.parse_args()
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", args.version):
        parser.error("invalid version")
    source, output = Path(args.input_path).resolve(), Path(args.output_path).resolve()
    expected = {"mihomo-smart-selector-" + args.version + "-" + platform + "-" + arch + ext
                for platform, ext in [("windows", ".zip"), ("darwin", ".tar.gz")]
                for arch in ["amd64", "arm64"]}
    verified = {}
    for manifest in source.glob("*/SHA256SUMS.txt"):
        for line in manifest.read_text(encoding="utf-8").splitlines():
            match = re.fullmatch(r"([0-9a-f]{64})  (.+)", line)
            if not match:
                raise RuntimeError("Malformed checksum in " + str(manifest))
            digest, name = match.groups()
            if name not in expected or name in verified:
                raise RuntimeError("Unexpected or duplicate release archive: " + name)
            archive = manifest.parent / name
            if archive.is_symlink() or hashlib.sha256(archive.read_bytes()).hexdigest() != digest:
                raise RuntimeError("Checksum mismatch or symlink: " + name)
            verified[name] = (archive, digest)
    if set(verified) != expected:
        raise RuntimeError("Missing release archives: " + ", ".join(sorted(expected - set(verified))))
    output.mkdir(parents=True, exist_ok=True)
    if any(output.iterdir()):
        raise RuntimeError("Release output must be empty")
    checksums = []
    for name, (archive, digest) in sorted(verified.items()):
        shutil.copyfile(archive, output / name)
        checksums.append(digest + "  " + name)
    (output / "SHA256SUMS.txt").write_bytes(("\n".join(checksums) + "\n").encode("utf-8"))
    print("Verified and collected all four desktop archives into " + str(output))


if __name__ == "__main__":
    main()
