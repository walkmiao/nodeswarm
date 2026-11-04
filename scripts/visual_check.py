"""Browser UI visual regression / interaction check with the REAL controller API.
The sandbox browser blocks loopback navigation, so use Playwright's exposed Python
bridge to issue actual API calls from Python while rendering the actual app JS.
Provide NODE_CONSOLE_TEST_URL and NODE_CONSOLE_TEST_TOKEN_FILE.
"""
import os, urllib.request, urllib.error
from pathlib import Path
from playwright.sync_api import sync_playwright
ROOT=Path(__file__).resolve().parents[1]
URL=os.environ.get('NODE_CONSOLE_TEST_URL','http://127.0.0.1:18088')
TOKEN_FILE=os.environ.get('NODE_CONSOLE_TEST_TOKEN_FILE','/tmp/nodeconsole-qa-controller/admin.token')
TOKEN=Path(TOKEN_FILE).read_text().strip()
html=(ROOT/'web/index.html').read_text().replace('<link rel="stylesheet" href="/style.css" />','').replace('<script defer src="/app.js"></script>','')
css='\n'.join([(ROOT/'web/design-system/tokens.css').read_text(),(ROOT/'web/design-system/components.css').read_text(),(ROOT/'web/style.css').read_text()]);css=__import__('re').sub(r'@import url\([^)]*\);','',css);js=(ROOT/'web/app.js').read_text()
def backend(path,options):
    headers=options.get('headers') or {};body=options.get('body');req=urllib.request.Request(URL+path,data=body.encode() if body else None,headers=headers,method=options.get('method','GET'))
    try:
        with urllib.request.urlopen(req,timeout=10) as res:return {'status':res.status,'body':res.read().decode(),'type':res.headers.get('content-type','application/json')}
    except urllib.error.HTTPError as exc:return {'status':exc.code,'body':exc.read().decode(),'type':'application/json'}
with sync_playwright() as p:
    browser=p.chromium.launch(headless=True,executable_path='/usr/bin/chromium',args=['--no-sandbox','--disable-dev-shm-usage'])
    page=browser.new_page(viewport={'width':1500,'height':1000},device_scale_factor=1)
    errors=[];page.on('pageerror',lambda err:errors.append(str(err)))
    page.expose_function('testBackend',backend);page.set_content(html);page.add_style_tag(content=css)
    page.add_script_tag(content="""Object.defineProperty(window,'sessionStorage',{configurable:true,value:{getItem:()=>null,setItem:()=>{},removeItem:()=>{}}});window.fetch=async function(path,opts={}){const d=await window.testBackend(path,opts);return new Response(d.body,{status:d.status,headers:{'content-type':d.type}})}""")
    page.add_script_tag(content=js)
    page.locator('#login-form').wait_for()
    page.screenshot(path=str(ROOT.parent/'node-console-v1-login.png'),animations='disabled')
    page.locator('#token').fill(TOKEN);page.locator('#login-form button').click()
    page.locator('.metric-card').first.wait_for(timeout=12000)
    page.screenshot(path=str(ROOT.parent/'node-console-v1-overview.png'),animations='disabled',full_page=True)
    page.locator('[data-page="agents"]').first.click();page.locator('.table-wrap tbody tr').first.wait_for()
    page.screenshot(path=str(ROOT.parent/'node-console-v1-agents.png'),animations='disabled')
    page.locator('[data-action="agent-detail"]').first.click();page.locator('#agent-form').wait_for()
    page.screenshot(path=str(ROOT.parent/'node-console-v1-detail.png'),animations='disabled')
    page.locator('[data-action="close-overlay"]').first.click()
    page.locator('[data-page="nodes"]').first.click()
    page.screenshot(path=str(ROOT.parent/'node-console-v1-nodes.png'),animations='disabled')
    page.locator('[data-action="new-node"]').first.click();page.locator('#node-form').wait_for()
    page.screenshot(path=str(ROOT.parent/'node-console-v1-new-node.png'),animations='disabled')
    # Real browser -> real authenticated controller: create, edit and delete.
    testname=f'UI-QA-Temporary-{os.getpid()}'
    page.locator('#field-name').fill(testname)
    page.locator('#field-host').fill('127.0.0.3')
    page.locator('#field-port').fill('12181')
    page.locator('#field-capacity').fill('3')
    page.locator('[data-action="save-node"]').click()
    qa=page.locator('tbody tr').filter(has_text=testname)
    qa.wait_for(timeout=10000)
    qa.locator('[data-action="edit-node"]').click()
    page.locator('#field-region').fill('Test region')
    page.locator('[data-action="save-node"]').click()
    qa=page.locator('tbody tr').filter(has_text=testname)
    qa.get_by_text('Test region').wait_for(timeout=10000)
    qa.locator('[data-action="delete-node"]').click()
    page.locator('[data-danger-confirm] [data-confirm-proceed]').click()
    qa.wait_for(state='detached',timeout=10000)
    page.locator('[data-page="settings"]').first.click()
    assert page.locator('.codebox').count()>=3
    page.locator('[data-action="new-enrollment"]').click()
    page.locator('#enroll-form').wait_for()
    page.locator('#field-label').fill('Playwright-Bootstrap')
    page.locator('#field-ttlMinutes').fill('15')
    page.locator('[data-action="save-enrollment"]').click()
    page.get_by_text('注册密钥已生成').wait_for()
    assert page.locator('.overlay .codebox').count()==1
    page.screenshot(path=str(ROOT.parent/'nodeswarm-grant.png'),animations='disabled')
    page.locator('[data-action="close-overlay"]').last.click()
    page.get_by_text('Playwright-Bootstrap').wait_for()
    page.locator('[data-action="revoke-grant"]').click()
    page.locator('[data-danger-confirm] [data-confirm-proceed]').click()
    page.get_by_text('Playwright-Bootstrap').wait_for(state='detached')
    page.locator('[data-page="alerts"]').first.click()
    page.get_by_text('实时告警列表').wait_for()
    page.screenshot(path=str(ROOT.parent/'nodeswarm-alerts.png'),animations='disabled')
    page.locator('[data-page="events"]').first.click()
    assert page.locator('.log-row').count()>=1
    assert not errors, errors
    print('VISUAL PASS: login, overview, agents, UDP fields, node CRUD, enrollment grant issue/revoke, alerts, events')
    browser.close()
