#!/usr/bin/env python3
"""Run real Controller API smoke tests with an ephemeral data directory.

No third-party dependencies. Provide NODE_CONSOLE_TEST_BIN to use an existing
binary, or the script builds one in the temporary directory using Go.
"""
import json
import os
import pathlib
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

ROOT=pathlib.Path(__file__).resolve().parents[1]


def query(base,path,method='GET',token='',data=None):
    payload=json.dumps(data).encode() if data is not None else None
    request=urllib.request.Request(base+path,data=payload,method=method,
         headers={'Authorization':'Bearer '+token,'Content-Type':'application/json'})
    try:
        with urllib.request.urlopen(request,timeout=5) as r:
            return r.status,json.loads(r.read())
    except urllib.error.HTTPError as err:
        return err.code,json.loads(err.read())


def run():
    with tempfile.TemporaryDirectory(prefix='nodeswarm-api-') as d:
        d=pathlib.Path(d)
        binary=os.environ.get('NODE_CONSOLE_TEST_BIN')
        if not binary:
            binary=str(d/'nodeswarm')
            subprocess.run(['go','build','-o',binary,'.'],cwd=ROOT,check=True)
        with socket.socket(socket.AF_INET,socket.SOCK_STREAM) as s:
            s.bind(('127.0.0.1',0));port=s.getsockname()[1]
        url=f'http://127.0.0.1:{port}'
        env={**os.environ,'NODE_MANAGER_DATA':str(d/'data')}
        proc=subprocess.Popen([binary,'-mode','controller','-listen',f'127.0.0.1:{port}'],cwd=ROOT,env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        try:
            for _ in range(50):
                try:
                    if query(url,'/api/health')[0]==200:break
                except Exception:
                    time.sleep(.1)
            else:raise AssertionError('controller not ready')
            token=(d/'data'/'admin.token').read_text().strip()
            assert query(url,'/api/dashboard')[0]==401
            assert query(url,'/api/dashboard',token=token)[0]==200
            status,enroll=query(url,'/api/enrollments','POST',token,{'label':'Smoke','ttlMinutes':10})
            assert status==201 and len(enroll['token'])>40
            status,node=query(url,'/api/nodes','POST',token,{'name':'smoke','host':'127.0.0.1','port':50000,'capacity':2})
            assert status==201,node
            status,data=query(url,'/api/dashboard',token=token)
            assert status==200 and len(data['nodes'])==1
            assert len(data['enrollments'])==1 and enroll['token'] not in json.dumps(data)
            assert 'alerts' in data and isinstance(data['alerts'],list)
            status,_=query(url,'/api/nodes/'+node['id'],'DELETE',token)
            assert status==200
            assert query(url,'/api/dashboard',token=token)[1]['nodes']==[]
            print('API PASS: authentication, one-time grant issuance, node create/delete, alerts dashboard')
        finally:
            proc.terminate()
            try:proc.wait(timeout=4)
            except subprocess.TimeoutExpired:proc.kill()

if __name__=='__main__':run()
