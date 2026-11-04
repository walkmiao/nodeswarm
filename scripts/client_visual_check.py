#!/usr/bin/env python3
"""Real authenticated Controller + browser check for Windows Client management.
The inventory is seeded with a fixture heartbeat; no game or driver is faked.
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
from playwright.sync_api import sync_playwright

ROOT = pathlib.Path(__file__).resolve().parents[1]

def port():
    with socket.socket() as s:
        s.bind(("127.0.0.1",0))
        return s.getsockname()[1]

def request(base,path,method="GET",token="",payload=None):
    b=json.dumps(payload).encode() if payload is not None else None
    req=urllib.request.Request(base+path,method=method,data=b,headers={"Authorization":"Bearer "+token,"Content-Type":"application/json"})
    try:
        with urllib.request.urlopen(req,timeout=8) as r:return r.status,json.load(r)
    except urllib.error.HTTPError as e:return e.code,json.loads(e.read())

def main():
    with tempfile.TemporaryDirectory(prefix="nodeswarm-client-ui-") as tmp:
        portno=port();data_dir=pathlib.Path(tmp)/"controller";base=f"http://127.0.0.1:{portno}"
        bin=str(ROOT/"node-console-linux-amd64")
        process=subprocess.Popen([bin,"-mode","controller","-listen",f"127.0.0.1:{portno}"],env={**os.environ,"NODE_MANAGER_DATA":str(data_dir)},stdout=subprocess.DEVNULL)
        try:
            for _ in range(60):
                try:
                    if request(base,"/api/health")[0]==200:break
                except Exception:time.sleep(.1)
            else:raise RuntimeError("controller not ready")
            admin=(data_dir/"admin.token").read_text().strip()
            enroll=(data_dir/"enroll.token").read_text().strip()
            code,credential=request(base,"/api/agent/enroll","POST",enroll,{"kind":"client","name":"Windows Lab 01","os":"windows","arch":"amd64"})
            assert code==201,(code,credential)
            code,_=request(base,"/api/agent/heartbeat","POST",credential["token"],{
                "id":credential["id"],"os":"windows","hostname":"GAME-PC-01","version":"1.2.0",
                "client":{"backend":"observe","revision":0,"observedAt":time.strftime("%Y-%m-%dT%H:%M:%SZ",time.gmtime()),"processes":[
                    {"pid":12340,"name":"Client.exe","path":"C:\\Games\\Yulgang\\Client.exe","startTime":"2026-11-03T05:00:00Z"},
                    {"pid":12341,"name":"Client.exe","path":"C:\\Games\\Yulgang\\Client.exe","startTime":"2026-11-03T05:02:00Z"}],"routes":[]}})
            assert code==200
            code,node=request(base,"/api/nodes","POST",admin,{"name":"Tokyo VPS 01","host":"203.0.113.23","port":11080,"username":"demo","password":"private-pw","capacity":3,"enabled":True,"region":"Tokyo","provider":"Cloud"})
            assert code==201,(code,node)
            html=(ROOT/"web/index.html").read_text().replace('<link rel="stylesheet" href="/style.css" />','').replace('<script defer src="/app.js"></script>','')
            css='\n'.join([(ROOT/'web/design-system/tokens.css').read_text(),(ROOT/'web/design-system/components.css').read_text(),(ROOT/'web/style.css').read_text()]);css=__import__('re').sub(r'@import url\([^)]*\);','',css);js=(ROOT/"web/app.js").read_text()
            def backend(path,opts):
                headers=opts.get('headers') or {};method=opts.get('method','GET');b=opts.get('body')
                r=urllib.request.Request(base+path,method=method,data=b.encode() if b else None,headers=headers)
                try:
                    with urllib.request.urlopen(r,timeout=10) as response:return {'status':response.status,'body':response.read().decode(),'contentType':response.headers.get('content-type','application/json')}
                except urllib.error.HTTPError as e:return {'status':e.code,'body':e.read().decode(),'contentType':'application/json'}
            with sync_playwright() as p:
                browser=p.chromium.launch(headless=True,executable_path='/usr/bin/chromium',args=['--no-sandbox','--disable-dev-shm-usage'])
                page=browser.new_page(viewport={'width':1536,'height':960},device_scale_factor=1)
                errors=[];page.on('pageerror',lambda err:errors.append(str(err)))
                page.expose_function('testBackend',backend);page.set_content(html);page.add_style_tag(content=css)
                page.add_script_tag(content="""Object.defineProperty(window,'sessionStorage',{configurable:true,value:{getItem:()=>null,setItem:()=>{},removeItem:()=>{}}});window.fetch=async function(path,opts={}){const d=await window.testBackend(path,opts);return new Response(d.body,{status:d.status,headers:{'content-type':d.contentType}})}""")
                page.add_script_tag(content=js)
                page.locator('#token').fill(admin)
                page.locator('#login-form button').click()
                page.locator('[data-page="clients"]').wait_for(timeout=10000)
                page.locator('[data-page="clients"]').click()
                page.get_by_text('Windows Lab 01').wait_for()
                page.screenshot(path=str(ROOT.parent/'nodeswarm-v1.3-clients.png'),animations='disabled',full_page=True)
                page.locator('[data-action="client-detail"]').click()
                page.locator('.client-node-select').first.wait_for()
                assert page.locator('#client-limit').input_value()=='8'
                page.locator('#client-limit').fill('1')
                page.locator('[data-action="save-client-limit"]').click()
                page.get_by_text('1 个 PID 已规划').count()  # quota pane rendered from server state
                page.locator('#client-limit').wait_for()
                assert page.locator('#client-limit').input_value()=='1'
                code,view=request(base,'/api/clients/'+credential['id'],token=admin)
                assert code==200 and view.get('maxProxyPids')==1,(code,view)
                page.screenshot(path=str(ROOT.parent/'nodeswarm-v1.3-client-detail.png'),animations='disabled')
                page.locator('.client-node-select').first.select_option(node['id'])
                page.locator('.client-node-select').nth(1).select_option(node['id'])
                assert page.locator('[data-action="save-client-bindings"]').is_disabled(), 'over limit selection not blocked in UI'
                page.locator('.client-node-select').nth(1).select_option('')
                page.locator('[data-action="save-client-bindings"]').click()
                page.wait_for_timeout(400)
                code,view=request(base,'/api/clients/'+credential['id'],token=admin)
                assert code==200 and len(view.get('clientBindings',[]))==1,(code,view)
                # Save keeps the detail drawer open so the latest binding can
                # be reviewed without another navigation.
                page.locator('.client-node-select').first.wait_for()
                assert page.locator('.client-node-select').first.input_value()==node['id']
                # Expand this machine's quota then auto-fill its remaining slot.
                page.locator('#client-limit').fill('2')
                page.locator('[data-action="save-client-limit"]').click()
                page.locator('#client-limit').wait_for()
                assert page.locator('#client-limit').input_value()=='2'
                page.locator('[data-action="auto-client-bindings"]').click()
                page.wait_for_timeout(450)
                code,view=request(base,'/api/clients/'+credential['id'],token=admin)
                assert code==200 and len(view.get('clientBindings',[]))==2,(code,view)
                assert len({b['pid'] for b in view['clientBindings']})==2
                assert not errors,errors
                print('CLIENT VISUAL PASS: auth, quota, manual + auto assignment, persistence')
                browser.close()
        finally:
            process.terminate()
            try:process.wait(timeout=4)
            except subprocess.TimeoutExpired:process.kill()

if __name__ == '__main__':main()
