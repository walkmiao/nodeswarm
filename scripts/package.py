#!/usr/bin/env python3
"""Reproducible source + binaries ZIP, no secrets, no nested project folder."""
from pathlib import Path
import zipfile
import sys

root=Path(__file__).resolve().parents[1]
target=root.parent/'node-console-v1.0.0.zip'
files=['README.md','Makefile','go.mod','main.go','main_test.go','api_agents.go','api_agents_test.go']
files+= [str(p.relative_to(root)) for d in ['internal','web','deploy','docs','scripts','cmd'] for p in (root/d).rglob('*') if p.is_file() and not any(x in ('__pycache__','node_modules','.git') for x in p.parts) and p.suffix not in ('.pyc','.log')]
for n in ['node-console-linux-amd64','node-console-linux-arm64','node-console-windows-amd64.exe']:
    if not (root/n).is_file():sys.exit(f'Missing required build artifact {n}; run make all')
    files.append(n)
with zipfile.ZipFile(target,'w',compression=zipfile.ZIP_DEFLATED,compresslevel=7) as z:
    for f in sorted(set(files)):
        full=root/f
        assert full.is_file(),full
        z.write(full,arcname=f)
print(f'PACKAGED {len(set(files))} files: {target} ({target.stat().st_size/1024/1024:.1f} MiB)')
