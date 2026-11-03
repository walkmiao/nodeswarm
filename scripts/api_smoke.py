"""End-to-end HTTP smoke test; requires a built ./nodemgr binary. No external Python dependencies."""
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
BIN = ROOT / 'nodemgr'
if not BIN.is_file():
    raise SystemExit('First run: go build -o nodemgr .')
with socket.socket() as sock:
    sock.bind(('127.0.0.1', 0))
    port=sock.getsockname()[1]
base=f'http://127.0.0.1:{port}'
def request(path, data=None, method=None):
    body=None if data is None else json.dumps(data).encode('utf-8')
    req=urllib.request.Request(base+path,data=body,method=method,headers={'Content-Type':'application/json'} if body else {})
    with urllib.request.urlopen(req,timeout=5) as res:
        if path == '/': return res.read().decode()
        return json.load(res)
with tempfile.TemporaryDirectory() as d:
    env=dict(os.environ,NODE_MANAGER_PORT=str(port),NODE_MANAGER_DATA=d)
    p=subprocess.Popen([str(BIN)],env=env,cwd=str(ROOT),stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    try:
        for _ in range(30):
            try:
                health=request('/api/health')
                break
            except Exception:
                time.sleep(0.1)
        else:raise RuntimeError('server did not start')
        assert health['proxyEngine']=='not_integrated'
        assert '<title>Node Console' in request('/')
        node=request('/api/nodes',{'name':'smoke','host':'127.0.0.1','port':1080,'capacity':1},'POST')
        assert node['name']=='smoke'
        state=request('/api/state')
        assert len(state['nodes'])==1
        assert 'password' not in state['nodes'][0]
        assert isinstance(request('/api/processes')['processes'],list)
        request('/api/nodes/'+node['id'],method='DELETE')
        assert request('/api/state')['nodes']==[]
        print('HTTP smoke PASS: health, static UI, node CRUD, process API')
    finally:
        p.terminate()
        try:p.wait(timeout=3)
        except subprocess.TimeoutExpired:p.kill()
