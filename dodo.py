from datetime import datetime
import os
import re
import platform
import sys
from os import path
from typing import List, Set

from subprocess import getoutput

from urllib.request import urlopen, Request
from doit.tools import run_once
from doit.action import CmdAction


ext = ""
if platform.system() == "Windows":
    ext = ".exe"

DOIT_CONFIG = {
    "default_tasks": [platform.system().lower(), "coverage"],
}

windows_binary = "dist/sci-hub_windows_64.exe"
macos_binary = "dist/sci-hub_macos_64"
linux_binary = "dist/sci-hub_linux_64"


def task_windows():
    return {
        "actions": [],
        "file_dep": [windows_binary],
    }


def task_mac():
    return {
        "actions": [],
        "file_dep": [macos_binary],
    }


def task_linux():
    return {
        "actions": [],
        "file_dep": [linux_binary],
    }


def task_test():
    """Fetch test torrent"""
    return {
        "actions": ["go test -failfast ./..."],
        "file_dep": list(
            go | {"testdata/sm_00900000-00999999.torrent", "testdata/big_file.bin"}
        ),
        "clean": True,
        "verbosity": 2,
    }


def task_generate():
    """Generate go files"""
    return {
        "actions": [],
        "file_dep": list(generated),
    }


def should_skip(dir_name: str):
    s = path.normpath(dir_name).split(os.sep)
    for part in s:
        if part.startswith("."):
            return True
    return False


def wildcard(ext) -> Set[str]:
    if not ext.startswith("."):
        ext = "." + ext
    r = set()
    for dir, _, files in os.walk("."):
        if should_skip(dir):
            continue
        for file in files:
            if file.endswith(ext):
                r.add(path.join(dir, file))
    return r


proto = wildcard("proto")
go = wildcard("go") | {x.replace(".proto", ".pb.go") for x in proto}
generated = {x.replace(".proto", ".pb.go") for x in proto}


def task_proto():
    """Generate protobuf go files"""
    for p in proto:
        yield {
            "name": p,
            "file_dep": [p, ".bin/protoc-gen-go" + ext],
            "targets": [p.replace(".proto", ".pb.go")],
            "actions": [CmdAction("protoc --go_out=. %s" % p, env=add_path(".bin"))],
            "verbosity": 2,
        }


def task_test_binary():
    """Generate Test binary"""
    return {
        "targets": ["testdata/big_file.bin"],
        "file_dep": ["scripts/gen_big_file.py"],
        "actions": ["python scripts/gen_big_file.py"],
        "clean": True,
    }


def task_test_torrent():
    """Fetch test torrent"""

    def fetch_torrent():
        url = "https://libgen.rs/scimag/repository_torrent/sm_00900000-00999999.torrent"

        httprequest = Request(url)

        with urlopen(httprequest) as response:
            with open("./testdata/sm_00900000-00999999.torrent", "wb") as file:
                file.write(response.read())

    return {
        "targets": ["testdata/sm_00900000-00999999.torrent"],
        "actions": [fetch_torrent],
        "uptodate": [run_once],
        "clean": True,
    }


def task_Windows():
    return {
        "actions": [
            CmdAction(
                ["go", "build", *build_args(), "-o", windows_binary],
                env=env(GOOS="windows"),
            )
        ],
        "targets": [windows_binary],
        "file_dep": list(go),
        "clean": True,
        "verbosity": 2,
    }


def task_macOS():
    return {
        "actions": [
            CmdAction(
                ["go", "build", "-o", macos_binary],
                env=env(GOOS="darwin"),
            )
        ],
        "targets": [macos_binary],
        "file_dep": list(go),
        "clean": True,
        "verbosity": 2,
    }


def task_Linux():
    return {
        "actions": [
            CmdAction(
                ["go", "build", "-o", linux_binary],
                env=env(GOOS="linux"),
            )
        ],
        "targets": [linux_binary],
        "file_dep": list(go),
        "clean": True,
        "verbosity": 2,
    }


def task_coverage():
    """Generate coverage report"""
    return {
        "actions": [
            "go test -covermode=atomic -coverprofile=coverage.out -count=1 ./..."
        ],
        "targets": ["coverage.out"],
        "file_dep": list(go),
        "clean": True,
        "verbosity": 2,
    }


def task_install():
    return {
        "actions": [
            CmdAction(
                "go install google.golang.org/protobuf/cmd/protoc-gen-go",
                env=env(GOBIN=path.abspath(".bin")),
            )
        ],
        "targets": [".bin/protoc-gen-go" + ext],
        "clean": True,
        "uptodate": [run_once],
    }


def env(**kwargs):
    e = os.environ.copy()
    e.update(
        {
            "CGO_ENABLED": "0",
            "GOARCH": "amd64",
        }
    )
    e.update(kwargs)
    return e


def add_path(*paths):
    e = os.environ.copy()
    e["PATH"] = os.pathsep.join(paths) + os.pathsep + e["PATH"]
    return e


def build_args():
    ref = (
        os.getenv(
            "GITHUB_REF",
        )
        or getoutput("git symbolic-ref --short -q HEAD")
    )
    sha = (
        os.getenv(
            "GITHUB_SHA",
        )
        or getoutput("git rev-parse HEAD")
    )

    if sha:
        sha = sha[:8]

    build_time = datetime.utcnow().replace(microsecond=0).isoformat()

    if ref.startswith(
        (
            "refs/tags/",
            "refs/heads/",
        )
    ):
        ref = "".join(ref.split("/")[2:])
    elif match := re.match("refs/pull/(.*)/merge", ref):
        ref = "pr-" + match.group(1)

    ldflags = [
        f"-X 'sci_hub_p2p/pkg/vars.Ref={ref}'",
        f"-X 'sci_hub_p2p/pkg/vars.Commit={sha}'",
        f"-X 'sci_hub_p2p/pkg/vars.Builder={getoutput('go version')}'",
        f"-X 'sci_hub_p2p/pkg/vars.BuildTime={build_time}'",
    ]

    flags = ["-tags", "disable_libutp", "-ldflags", "-s -w " + " ".join(ldflags)]

    return flags
