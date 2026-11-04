#!/usr/bin/env python3
"""Static assertions for the NodeSwarm design contract (stdlib only)."""
from pathlib import Path
import re
from html.parser import HTMLParser
import subprocess

ROOT=Path(__file__).resolve().parents[1]
subprocess.run(['python3', str(ROOT/'scripts/sync_design_system.py'), '--check'], check=True)
required=['--accent: #5bcfc5','--w-rail: 76px','--w-sidebar: 240px','--h-row: 60px','--h-titlebar: 44px','--h-kpi: 76px']
tokens=(ROOT/'design-system/tokens.css').read_text()
for token in required:
    assert token in tokens, f'Missing design token: {token}'
for name in ('web/style.css','desktop/frontend/dist/style.css'):
    css=(ROOT/name).read_text()
    assert css.startswith("@import url('./design-system/tokens.css');"),name
    assert not re.search(r'#[0-9a-fA-F]{3,8}\b',css), f'Raw hex in {name}'
    assert not re.search(r'prefers-color-scheme\s*:\s*light|color-scheme\s*:\s*light',css,re.I),name
for name in ('web/app.js','desktop/frontend/dist/app.js'):
    js=(ROOT/name).read_text()
    assert not re.search(r'#[0-9a-fA-F]{3,8}\b',js),name

class IDs(HTMLParser):
    def __init__(self): super().__init__(); self.ids=set(); self.nav=[]
    def handle_starttag(self,tag,attrs):
        d=dict(attrs)
        if d.get('id'): self.ids.add(d['id'])
        if d.get('data-view'): self.nav.append(d['data-view'])

client=IDs();client.feed((ROOT/'desktop/frontend/dist/index.html').read_text())
needed={'root','style.css'} # only ID expectations below; root is on server
required_client={'page-title','settings-form','metric-bound','metric-limit','process-count','all-processes','nodes-table','connect-btn','top-connection-dot','side-dot','clear-log-btn'}
assert required_client <= client.ids, f'Missing desktop IDs: {required_client - client.ids}'
assert set(client.nav)=={'overview','processes','nodes','logs','settings'},'Desktop nav mismatch'
server=(ROOT/'web/index.html').read_text()
assert '/style.css' in server and '/app.js' in server
assert (ROOT/'desktop/main.go').read_text().find('R: 10, G: 10, B: 11') >= 0,'Wails initial background must be dark'
print('NodeSwarm UI design audit PASS: tokens, dark-only, both shells, native app IDs')
