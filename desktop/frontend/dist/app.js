(() => {
  'use strict';
  const $ = (q) => document.querySelector(q);
  const $$ = (q) => [...document.querySelectorAll(q)];
  const views = {overview:['工作站总览','掌握游戏进程、节点分配和这台电脑的运行情况。'],processes:['游戏进程','将检测到的 PID 分配到可用代理节点，并监控实际执行状态。'],nodes:['代理节点','查看 Controller 下发的可用节点、容量和最近健康状态。'],logs:['运行日志','查看本机客户端运行、连接和配置变化记录。'],settings:['连接设置','管理此电脑与 NodeSwarm Controller 的连接方式。']};
  const bridge = window.go?.main?.App;
  const demo = new URLSearchParams(location.search).get('preview') === '1' && !bridge;
  let tab = 'overview', busy = false, lastToast = 0, lastSettingsVersion = '';
  let state = {settings:{controller:'',name:'',targets:'Client.exe',engine:'observe',pidHardLimit:0,autoConnect:true},running:false,connected:false,status:'disconnected',credentialReady:false,lastSync:'',lastError:'',client:{processes:[],routes:[],backend:'observe'},remote:{clientBindings:[],maxProxyPids:8,revision:0},nodes:[],logs:[]};
  if (demo) {
    state={settings:{controller:'https://console.nodeswarm.local',name:'Game-PC-01',targets:'Client.exe',engine:'observe',pidHardLimit:8,autoConnect:true},running:true,connected:true,status:'online',credentialReady:true,lastSync:new Date().toISOString(),lastError:'',client:{processes:[{pid:18320,name:'Client.exe',path:'D:\\Game\\Client.exe',startTime:'2026-11-03T08:43:12Z'},{pid:18496,name:'Client.exe',path:'D:\\Game\\Client.exe',startTime:'2026-11-03T08:44:10Z'},{pid:19180,name:'Client.exe',path:'D:\\Game\\Client.exe',startTime:'2026-11-03T09:01:26Z'}],routes:[{pid:18320,status:'planned_only',nodeId:'n1'}],backend:'observe',observedAt:new Date().toISOString()},remote:{maxProxyPids:8,revision:2,clientBindings:[{pid:18320,startTime:'2026-11-03T08:43:12Z',nodeId:'n1',name:'Client.exe'}]},nodes:[{id:'n1',name:'华东 · 上海 01',host:'203.0.113.10',port:1080,region:'上海',capacity:8,enabled:true,lastStatus:'healthy',latencyMs:34},{id:'n2',name:'华南 · 广州 02',host:'198.51.100.22',port:1080,region:'广州',capacity:8,enabled:true,lastStatus:'healthy',latencyMs:52},{id:'n3',name:'华北 · 北京 03',host:'192.0.2.34',port:1080,region:'北京',capacity:4,enabled:true,lastStatus:'untested',latencyMs:0}],logs:['09:35:22  已同步 Controller 配置（修订 2）','09:35:21  主控连接已建立','09:34:58  本机进程采集完成：3 个 Client.exe']};
  }
  const esc = (v) => String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const fmtTime = (t) => {if(!t)return '—';const d=new Date(t);return Number.isNaN(d.getTime())?'—':d.toLocaleTimeString('zh-CN',{hour12:false});};
  const tag=(txt,kind='muted')=>`<span class="tag tag-${kind}">${esc(txt)}</span>`;
  function quota() {const server=state.remote?.maxProxyPids||8, local=state.settings?.pidHardLimit||0;return local>0?Math.min(server,local):server;}
  function toast(text,error=false){const el=$('#toast');el.textContent=text;el.classList.toggle('error',error);el.classList.add('show');const n=++lastToast;setTimeout(()=>{if(n===lastToast)el.classList.remove('show');},3800);}
  async function call(method,...args){if(!bridge){throw Error('当前是界面预览模式，请使用安装的 Windows App 操作。');}if(busy)throw Error('上一操作尚未完成');busy=true;try{return await bridge[method](...args);}finally{busy=false;}}
  function switchView(key){if(!views[key])return;tab=key;$$('.nav-item').forEach(n=>n.classList.toggle('active',n.dataset.view===key));$$('.view').forEach(v=>v.classList.toggle('active',v.id===`view-${key}`));$('#breadcrumb-current').textContent=key==='overview'?'总览':views[key][0];$('#page-title').textContent=views[key][0];$('#page-description').textContent=views[key][1];}
  function getBinding(pid){return (state.remote?.clientBindings||[]).find(b=>b.pid===pid);}
  function processTable(processes,summary=false){if(!processes?.length)return `<div class="empty"><div class="empty-symbol">▤</div><strong>暂无符合条件的进程</strong><small>打开目标游戏客户端后，程序会自动发现该进程。</small></div>`;
    const list=summary?processes.slice(0,5):processes;
    let head=summary?'<th style="width:49%">程序与路径</th><th style="width:22%">PID</th><th>绑定状态</th>':'<th style="width:28%">游戏进程</th><th style="width:10%">PID</th><th style="width:22%">代理节点</th><th style="width:16%">执行状态</th><th style="width:24%">操作</th>';
    return `<table><thead><tr>${head}</tr></thead><tbody>${list.map(p=>{
      const b=getBinding(p.pid), node=(state.nodes||[]).find(n=>n.id===b?.nodeId);const route=(state.client?.routes||[]).find(r=>r.pid===p.pid && r.startTime===p.startTime);
      const status=route?.status==='process_name_unverified'?tag('进程名规则 · 未验证','warn'):route?.status==='planned_only'||b?tag('策略已规划','blue'):tag('未分配');
      if(summary)return `<tr><td><div class="process-name">${esc(p.name)}</div><div class="process-sub" title="${esc(p.path)}">${esc(p.path)}</div></td><td class="pid">${esc(p.pid)}</td><td>${status}</td></tr>`;
      const opts=(state.nodes||[]).filter(n=>n.enabled).map(n=>`<option value="${esc(n.id)}" ${b?.nodeId===n.id?'selected':''}>${esc(n.name)}</option>`).join('');
      return `<tr><td><div class="process-name">${esc(p.name)}</div><div class="process-sub" title="${esc(p.path)}">${esc(p.path)}</div></td><td class="pid">${esc(p.pid)}</td><td>${b?esc(node?.name||'已失效节点'):'未绑定'}</td><td>${status}</td><td><div class="assign-controls"><select class="assign-select" aria-label="为 PID ${esc(p.pid)} 选择节点" data-node-for="${p.pid}"><option value="">选择节点</option>${opts}</select><button class="action-link" data-assign="${p.pid}" data-start="${esc(p.startTime)}">分配</button>${b?`<button class="action-link danger" data-unassign="${p.pid}" data-start="${esc(p.startTime)}">解绑</button>`:''}</div></td></tr>`;
    }).join('')}</tbody></table>`;
  }
  function renderNodes(){const nodes=state.nodes||[];$('#nodes-table').innerHTML=nodes.length?`<table><thead><tr><th style="width:32%">节点</th><th style="width:27%">SOCKS5 地址</th><th style="width:14%">地区</th><th style="width:13%">容量</th><th>健康状态</th></tr></thead><tbody>${nodes.map(n=>`<tr><td><div class="node-name"><span class="node-icon">◫</span>${esc(n.name)}</div></td><td class="node-host">${esc(n.host)}:${esc(n.port)}</td><td>${esc(n.region||'—')}</td><td>${esc((state.remote?.clientBindings||[]).filter(b=>b.nodeId===n.id).length)} / ${esc(n.capacity)}</td><td>${n.enabled?tag(n.lastStatus==='healthy'||n.lastStatus==='ok'?'正常':n.lastStatus==='untested'?'待检测':n.lastStatus==='error'?'异常':'已启用',n.lastStatus==='error'?'warn':n.lastStatus==='healthy'||n.lastStatus==='ok'?'success':'muted'):tag('已停用')}</td></tr>`).join('')}</tbody></table>`:'<div class="empty"><div class="empty-symbol">◫</div><strong>暂无代理节点</strong><small>请在云端 Controller 中配置或导入 SOCKS5 节点。</small></div>'}
  function render(){const procs=state.client?.processes||[],routes=state.client?.routes||[],bound=state.remote?.clientBindings||[],nodes=state.nodes||[];const connected=!!state.connected, running=!!state.running;
    $('#machine-name').textContent=state.settings?.name||'本机客户端';$('#side-title').textContent=connected?'主控已连接':running?'正在重连':'未连接';$('#side-subtitle').textContent=state.settings?.controller||'等待连接 Controller';$('#side-dot').className='status-dot '+(connected?'':running?'warn':'muted');$('#sync-label').textContent=connected?'上次同步 '+fmtTime(state.lastSync):running?'正在连接…':'未连接';$('#process-count').textContent=procs.length;
    $('#connect-btn').textContent=running?'断开连接':'连接主控';$('#connect-btn').className='btn '+(running?'btn-danger':'btn-primary');$('#metric-procs').textContent=procs.length;$('#metric-proc-note').textContent='目标进程 '+(state.settings?.targets||'Client.exe');$('#metric-bound').textContent=bound.length;$('#metric-limit').textContent=quota();$('#metric-quota-note').textContent=bound.length>=quota()?'已达到本机配额上限':`还可分配 ${Math.max(0,quota()-bound.length)} 个 PID`;$('#metric-nodes').textContent=nodes.filter(n=>n.enabled).length;$('#metric-engine').textContent=state.settings?.engine==='proxybridge'?'ProxyBridge':'Observe';$('#metric-mode-note').textContent=state.settings?.engine==='proxybridge'?'进程名规则 · 出口未验证':'仅监控，不接管网络连接';$('#setup-notice').hidden=!!state.settings?.controller;
    $('#overview-processes').innerHTML=processTable(procs,true);$('#all-processes').innerHTML=processTable(procs);$('#quota-badge').textContent=`${bound.length} / ${quota()} PID`;$('#autofill-btn').disabled=!connected || procs.length===0 || !nodes.length;
    $('#overview-connect').textContent=connected?'心跳正常，连接到 '+state.settings.controller:running?'正在建立安全连接':'未运行';// Keep status badges as stable DOM nodes across refreshes
    const stepBadge=(id,html)=>{let el=$('#'+id);if(el)el.outerHTML=html.replace('<span','<span id="'+id+'"')};
    // Restore the stable label node after switching its semantic status.

    stepBadge('overview-connect-badge',tag(connected?'在线':running?'连接中':'离线',connected?'success':running?'warn':'muted'));
    $('#overview-scan').textContent=state.client?.scanError||`最近发现 ${procs.length} 个目标进程`;stepBadge('overview-scan-badge',tag(state.client?.scanError?'采集异常':running?'正常':'未启动',state.client?.scanError?'warn':running?'success':'muted'));
    $('#overview-policy').textContent=state.client?.error||`${bound.length} 个 PID 已规划，${routes.length} 条执行状态`;stepBadge('overview-policy-badge',tag(state.client?.error?'异常':bound.length?'已规划':'未应用',state.client?.error?'warn':bound.length?'blue':'muted'));
    $('#overview-logs').innerHTML=(state.logs||[]).slice(-3).reverse().map(s=>`<div class="log-line">${esc(s)}</div>`).join('')||'<div class="empty" style="padding:15px">暂无事件记录</div>';
    $('#log-list').innerHTML=(state.logs||[]).slice().reverse().map(s=>`<div class="log-item">${esc(s)}</div>`).join('')||'<div class="empty">暂无运行日志</div>';
    renderNodes();updateSettings();
  }
  function updateSettings(){const form=$('#settings-form');const settings=state.settings||{};const fingerprint=JSON.stringify(settings);if(fingerprint===lastSettingsVersion)return;lastSettingsVersion=fingerprint;
    for(const k of ['controller','name','targets','engine','proxyBridgeCli','pidHardLimit'])if(form.elements.namedItem(k))form.elements.namedItem(k).value=settings[k]??'';
    form.elements.namedItem('autoConnect').checked=!!settings.autoConnect;
  }
  function readSettings(){const els=$('#settings-form').elements;return {controller:els.namedItem('controller').value.trim(),name:els.namedItem('name').value.trim(),targets:els.namedItem('targets').value.trim(),engine:els.namedItem('engine').value,proxyBridgeCli:els.namedItem('proxyBridgeCli').value.trim(),pidHardLimit:Number(els.namedItem('pidHardLimit').value)||0,autoConnect:els.namedItem('autoConnect').checked};}
  async function refreshRemote(){if(!bridge)return;try{state=await bridge.GetState();if(state.credentialReady){await bridge.Refresh();state=await bridge.GetState()}render()}catch(e){toast(String(e),true)}}
  async function operation(fn,after){try{await fn();if(after)await after();if(bridge){state=await bridge.GetState();render()}}catch(e){toast(String(e),true)}}
  document.addEventListener('click',async e=>{
    const nav=e.target.closest('[data-view]');if(nav){switchView(nav.dataset.view);return}
    const jump=e.target.closest('[data-switch]');if(jump){switchView(jump.dataset.switch);return}
    const bind=e.target.closest('[data-assign]');if(bind){const pid=Number(bind.dataset.assign),start=bind.dataset.start,nodeID=document.querySelector(`[data-node-for="${pid}"]`)?.value;if(!nodeID){toast('请先选择代理节点',true);return}await operation(()=>call('Assign',pid,start,nodeID),async()=>{await bridge.Refresh();toast('绑定策略已保存，等待客户端报告实际状态')});return}
    const unbind=e.target.closest('[data-unassign]');if(unbind){await operation(()=>call('Unassign',Number(unbind.dataset.unassign),unbind.dataset.start),async()=>{await bridge.Refresh();toast('已解除绑定')});return}
  });
  $('#refresh-btn').addEventListener('click',async()=>{await operation(async()=>{if(bridge)await bridge.Refresh()},()=>toast('状态已刷新'))});
  $('#nodes-refresh').addEventListener('click',refreshRemote);
  $('#autofill-btn').addEventListener('click',()=>operation(()=>call('AutoFill'),async()=>{await bridge.Refresh();toast('自动补齐策略已提交')}));
  $('#connect-btn').addEventListener('click',()=>operation(async()=>{if(state.running){await call('Stop');toast('已断开连接')}else{if(!state.settings.controller){switchView('settings');toast('请先填写连接设置');return}const key=$('#settings-form').elements.namedItem('enrollKey').value;await call('Start',key);toast('正在启动客户端')}},()=>{}));
  $('#settings-form').addEventListener('submit',e=>{e.preventDefault();operation(()=>call('SaveSettings',readSettings()),()=>{toast('本机设置已保存');lastSettingsVersion='';})});
  $('#settings-start').addEventListener('click',()=>operation(async()=>{await call('SaveSettings',readSettings());const key=$('#settings-form').elements.namedItem('enrollKey').value;await call('Start',key);$('#settings-form').elements.namedItem('enrollKey').value='';switchView('overview');toast('正在连接 Controller')}));
  $('#clear-log-btn').addEventListener('click',()=>operation(()=>call('ClearLogs'),()=>toast('已清除本机显示日志')));
  if(!demo && !bridge){$('#connect-btn').disabled=true;$('#refresh-btn').disabled=true;}
  render();if(!demo && bridge){setInterval(async()=>{if(busy)return;try{state=await bridge.GetState();render();if(state.credentialReady && (state.running||state.connected) && (Date.now()-new Date(state.lastSync||0).getTime()<60000)){/* refresh remote separately below */}}catch(e){/* connection will retry */}},4000);setInterval(async()=>{if(busy)return;try{if(state.credentialReady){await bridge.Refresh();state=await bridge.GetState();render()}}catch(e){}},13000)}
})();
