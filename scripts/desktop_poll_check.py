#!/usr/bin/env python3
"""Desktop WebView regression: unchanged heartbeat data must not remount select controls.

Run on a developer machine with Python Playwright and a local Chromium. A small
Wails bridge fixture is used only for DOM lifecycle testing, not proxy egress.
"""
from pathlib import Path
from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parents[1]
ASSETS = ROOT / 'desktop/frontend/dist'
BRIDGE = r'''
  window.__fixture = {
    settings: {controller:'https://controller.example.com', name:'Windows QA', targets:'Client.exe', engine:'observe', pidHardLimit:8, autoConnect:true},
    running:true, connected:true, status:'online', credentialReady:false, lastSync:'2026-11-04T12:00:00Z',
    remote:{maxProxyPids:8, revision:1, clientBindings:[]},
    client:{processes:[{pid:12345,name:'Client.exe',path:'C:\\Games\\Client.exe',startTime:'2026-11-04T10:00:00Z'}], routes:[], backend:'observe'},
    nodes:[{id:'node-a',name:'Tokyo 01',host:'203.0.113.4',port:1080,capacity:8,enabled:true,region:'Tokyo'},
           {id:'node-b',name:'Tokyo 02',host:'198.51.100.7',port:1080,capacity:8,enabled:true,region:'Tokyo'}], logs:[]
  };
  window.__bindCalls = [];
  window.go = {main:{App:{GetState:async()=>structuredClone(window.__fixture), Refresh:async()=>{},
    Assign:async(pid,start,nodeID)=>{window.__bindCalls.push({pid,start,nodeID});window.__fixture.remote.clientBindings=[{pid,startTime:start,nodeId:nodeID,name:"Client.exe"}];}
  }}};
  window.setInterval = (callback,ms) => {if(ms===4000)window.__tick=callback;return 0;};
'''
with sync_playwright() as playwright:
    browser=playwright.chromium.launch(headless=True,executable_path='/usr/bin/chromium',args=['--no-sandbox','--disable-dev-shm-usage'])
    page=browser.new_page(viewport={'width':1500,'height':950})
    errors=[]
    page.on('pageerror',lambda error:errors.append(str(error)))
    # Restricted CI browsers often block file:// navigation. Render the real
    # assets in a blank page and expose a local Wails mock before app.js.
    html=(ASSETS/'index.html').read_text().replace('<link rel="stylesheet" href="./style.css" />','').replace('<script src="./app.js"></script>','')
    page.set_content(html)
    css='\n'.join((ASSETS/'design-system'/name).read_text() for name in ('tokens.css','components.css'))+'\n'+(ASSETS/'style.css').read_text()
    import re
    page.add_style_tag(content=re.sub(r'@import url\([^)]*\);','',css))
    page.evaluate(BRIDGE)
    page.add_script_tag(content=(ASSETS/'app.js').read_text())
    page.evaluate('async()=>await window.__tick()')
    page.locator('[data-view="processes"]').click()
    select=page.locator('[data-node-for="12345"]')
    select.select_option('node-b')
    select.focus()
    page.evaluate('window.__originalSelect=document.querySelector("[data-node-for=\\"12345\\"]")')
    page.evaluate('async()=>await window.__tick()')
    assert page.evaluate('document.querySelector("[data-node-for=\\"12345\\"]")===window.__originalSelect'), 'unchanged poll remounted the selector'
    assert select.input_value()=='node-b','unchanged poll reset the unsaved selection'
    assert page.evaluate('document.activeElement===window.__originalSelect'),'unchanged poll stole focus'
    page.evaluate('window.__fixture.client.processes.push({pid:12346,name:"Client.exe",path:"C:\\\\Games\\\\Client.exe",startTime:"2026-11-04T10:01:00Z"})')
    page.evaluate('async()=>await window.__tick()')
    assert page.locator('[data-node-for="12346"]').count()==1,'changed inventory was not rendered'
    assert page.locator('[data-node-for="12345"]').input_value()=='node-b','changed inventory reset unsaved selection'
    page.locator('[data-assign="12345"]').click()
    page.wait_for_function('window.__bindCalls.length === 1')
    assert page.evaluate('window.__bindCalls[0].nodeID')=='node-b','assignment ignored selected node'
    assert not errors,errors
    print('DESKTOP POLL PASS: stable selectors, focus, inventory changes, and real binding action dispatch')
    browser.close()
