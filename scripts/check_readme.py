#!/usr/bin/env python3
"""Compiles every ```go block in README.md against the SDK (go vet), so documented snippets cannot rot.

Fragment blocks are wrapped in a function with `client`, `secret` and `ctx` in scope; blocks that start with
`package main` are compiled as written. Usage: python3 scripts/check_readme.py
"""
import os
import re
import shutil
import subprocess
import sys

root = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
src = open(os.path.join(root, "README.md"), encoding="utf-8").read()
blocks = re.findall(r"```go\n(.*?)```", src, re.S)
out = [
    "package readmecheck\n",
    'import (\n"context"\n"errors"\n"fmt"\n"io"\n"log"\n"net/http"\n stampmitra "github.com/Bani-Global-Industries-LLP/stampmitra-developer-go"\n)\n',
    "var _ = errors.New\nvar _ = fmt.Sprint\nvar _ = io.EOF\nvar _ = log.Println\nvar _ http.Handler\nvar _ context.Context\n",
]
for i, b in enumerate(blocks):
    if b.startswith("package main"):
        b = b.replace("package main", "")
        b = re.sub(r"import \(.*?\)\n", "", b, flags=re.S)
        out.append(b.replace("func main()", f"func main{i}()"))
        continue
    b = b.replace("ctx := context.Background()\n", "")
    names = set()
    for m in re.finditer(r"^([a-zA-Z_][\w, ]*?) :?= ", b, re.M):
        for n in m.group(1).split(","):
            n = n.strip()
            if n and n not in ("_", "err", "if", "for") and re.fullmatch(r"\w+", n):
                names.add(n)
    errline = " _ = err\n" if any(re.match(r"^\w[\w, ]*, err :?= ", x) for x in b.split("\n")) else ""
    used = "".join(f" _ = {n}\n" for n in sorted(names))
    out.append(f"func snippet{i}(client *stampmitra.Client, secret string) {{\n ctx := context.Background()\n _ = ctx\n{b}\n{errline}{used}}}\n")

d = os.path.join(root, "examples", "readmecheck")
os.makedirs(d, exist_ok=True)
try:
    open(os.path.join(d, "check.go"), "w", encoding="utf-8").write("\n".join(out))
    r = subprocess.run(["go", "vet", "./readmecheck"], cwd=os.path.join(root, "examples"))
    print(f"{len(blocks)} Go blocks from README.md {'compile' if r.returncode == 0 else 'FAILED to compile'}")
    sys.exit(r.returncode)
finally:
    shutil.rmtree(d, ignore_errors=True)
