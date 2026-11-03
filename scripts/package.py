#!/usr/bin/env python3
"""Make source-and-binaries release zip; never include runtime secrets or git history."""
from pathlib import Path
import hashlib
import zipfile
import sys

root = Path(__file__).resolve().parents[1]
version = 'v1.1.0'
target = root.parent / f'nodeswarm-{version}-release.zip'
binaries = (
    'node-console-linux-amd64',
    'node-console-linux-arm64',
    'node-console-windows-amd64.exe',
)
for name in binaries:
    if not (root / name).is_file():
        sys.exit(f'Missing build artifact: {name}; run make all')

files = ['README.md', 'Makefile', 'go.mod', 'Dockerfile', '.dockerignore', '.gitignore', '.gitattributes', 'main.go', 'main_test.go', 'api_agents.go', 'api_agents_test.go', 'probe_batch_test.go']
include = ('internal', 'web', 'deploy', 'docs', 'scripts', '.github')
exclude = {'__pycache__', 'node_modules', '.git', 'agent.env', 'data', 'runtime', 'logs'}
for folder in include:
    for candidate in (root / folder).rglob('*'):
        if not candidate.is_file() or any(part in exclude for part in candidate.relative_to(root).parts):
            continue
        if candidate.suffix in ('.pyc', '.log', '.key', '.pem', '.token', '.exe'):
            continue
        files.append(str(candidate.relative_to(root)))
files.extend(binaries)
manifest = ''.join(f'{hashlib.sha256((root / f).read_bytes()).hexdigest()}  {f}\n' for f in binaries)
with zipfile.ZipFile(target, 'w', compression=zipfile.ZIP_DEFLATED, compresslevel=7) as z:
    for name in sorted(set(files)):
        path = root / name
        if not path.is_file():
            sys.exit(f'Missing source file: {name}')
        z.write(path, arcname=name)
    z.writestr('SHA256SUMS', manifest)
print(f'PACKAGED {len(set(files))} files: {target} ({target.stat().st_size / 1024 / 1024:.1f} MiB)')
