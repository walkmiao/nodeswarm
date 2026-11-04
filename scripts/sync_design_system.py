#!/usr/bin/env python3
"""Keep Controller and Windows Desktop copies of shared NodeLink tokens in sync.

Usage: python3 scripts/sync_design_system.py [--check]
"""
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
source = ROOT / 'design-system'
targets = (ROOT / 'web/design-system', ROOT / 'desktop/frontend/dist/design-system')
check = '--check' in sys.argv
mismatches = []
for target in targets:
    for name in ('tokens.css', 'components.css'):
        want = (source / name).read_bytes()
        file = target / name
        if not file.is_file() or file.read_bytes() != want:
            mismatches.append(str(file.relative_to(ROOT)))
            if not check:
                file.parent.mkdir(parents=True, exist_ok=True)
                file.write_bytes(want)
if mismatches and check:
    raise SystemExit('Stale design-system copies: ' + ', '.join(mismatches))
print('Design system synchronized' if not check else 'Design system copies verified')
