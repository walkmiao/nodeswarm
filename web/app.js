'use strict';
// NodeSwarm 1.3: self-contained SPA. All displayed remote data is escaped before HTML insertion.
const $ = s => document.querySelector(s);
const e = v => String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const icons = {
  dashboard:'<rect x="3" y="3" width="7" height="9" rx="1"/><rect x="14" y="3" width="7" height="5" rx="1"/><rect x="14" y="12" width="7" height="9" rx="1"/><rect x="3" y="16" width="7" height="5" rx="1"/>',
  server:'<rect x="3" y="3" width="18" height="7" rx="2"/><rect x="3" y="14" width="18" height="7" rx="2"/><path d="M7 6.5h.01M7 17.5h.01M11 6.5h7M11 17.5h7"/>',
  cloud:'<path d="M7 17a4 4 0 0 1-.4-8A6 6 0 0 1 18 10a3.5 3.5 0 0 1-.5 7H7Z"/>',
  activity:'<path d="M3 12h4l3-8 4 16 3-8h4"/>',
  cpu:'<rect x="5" y="5" width="14" height="14" rx="2"/><path d="M9 9h6v6H9zM9 1v4m6-4v4M9 19v4m6-4v4M1 9h4m-4 6h4m14-6h4m-4 6h4"/>',
  list:'<path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/>',
  settings:'<path d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7Z"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-1.9 1.9-.06-.06a1.7 1.7 0 0 0-1.88-.34 1.7 1.7 0 0 0-1.02 1.57V21h-2.7v-.09a1.7 1.7 0 0 0-1.02-1.57 1.7 1.7 0 0 0-1.88.34l-.06.06-1.9-1.9.06-.06A1.7 1.7 0 0 0 7.78 15a1.7 1.7 0 0 0-1.57-1.02H6v-2.7h.21A1.7 1.7 0 0 0 7.78 10a1.7 1.7 0 0 0-.34-1.88l-.06-.06 1.9-1.9.06.06A1.7 1.7 0 0 0 11.22 6a1.7 1.7 0 0 0 1.02-1.57V4h2.7v.43A1.7 1.7 0 0 0 15.96 6a1.7 1.7 0 0 0 1.88-.34l.06-.06 1.9 1.9-.06.06A1.7 1.7 0 0 0 19.4 10a1.7 1.7 0 0 0 1.57 1.02H21v2.7h-.03A1.7 1.7 0 0 0 19.4 15Z"/>',
  refresh:'<path d="M20 7v5h-5M4 17v-5h5"/><path d="M6.1 9A7 7 0 0 1 18 7l2 5M4 12l2 5a7 7 0 0 0 11.9-2"/>',
  plus:'<path d="M12 5v14M5 12h14"/>',
  search:'<circle cx="11" cy="11" r="7"/><path d="m16 16 5 5"/>',
  chevron:'<path d="m9 18 6-6-6-6"/>',
  down:'<path d="m6 9 6 6 6-6"/>',
  arrow:'<path d="M5 12h14m-6-6 6 6-6 6"/>',
  external:'<path d="M14 3h7v7M10 14 21 3"/><path d="M21 13v7H4V3h7"/>',
  upload:'<path d="M12 16V4m-5 5 5-5 5 5M4 16v4h16v-4"/>',
  download:'<path d="M12 4v12m-5-5 5 5 5-5M4 17v3h16v-3"/>',
  check:'<path d="m4 12 5 5L20 6"/>',
  alert:'<path d="M12 3 2 21h20L12 3ZM12 9v5m0 4h.01"/>',
  info:'<circle cx="12" cy="12" r="9"/><path d="M12 11v5m0-8h.01"/>',
  clock:'<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  power:'<path d="M12 2v10m6.4-7.4a9 9 0 1 1-12.8 0"/>',
  play:'<path d="m8 5 11 7-11 7V5Z"/>',
  stop:'<rect x="5" y="5" width="14" height="14" rx="2"/>',
  rotate:'<path d="M2 11a10 10 0 0 1 17-6l2 2M21 2v5h-5M22 13a10 10 0 0 1-17 6l-2-2M3 22v-5h5"/>',
  dot:'<circle cx="12" cy="12" r="4"/>',
  x:'<path d="M18 6 6 18M6 6l12 12"/>',
  trash:'<path d="M3 6h18M8 6V4h8v2m3 0-1 15H6L5 6m5 4v7m4-7v7"/>',
  edit:'<path d="m4 16 12-12 4 4-12 12-5 1 1-5ZM14 6l4 4"/>',
  copy:'<rect x="8" y="8" width="13" height="13" rx="2"/><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"/>',
  shield:'<path d="M12 22s8-4 8-11V5l-8-3-8 3v6c0 7 8 11 8 11Z"/><path d="m9 12 2 2 4-4"/>',
  globe:'<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a14 14 0 0 1 0 18 14 14 0 0 1 0-18"/>',
  terminal:'<rect x="3" y="4" width="18" height="16" rx="2"/><path d="m7 9 3 3-3 3m6 0h4"/>',
  chart:'<path d="M3 3v18h18M7 15l4-5 3 3 6-8"/>',
  link:'<path d="M10 13a5 5 0 0 0 7 .4l3-3a5 5 0 0 0-7-7l-2 2M14 11a5 5 0 0 0-7-.4l-3 3a5 5 0 1 0 7 7l2-2"/>',
  menu:'<path d="M3 6h18M3 12h18M3 18h18"/>',
  filter:'<path d="M4 4h16l-6 8v7l-4 2v-9L4 4Z"/>',
  user:'<circle cx="12" cy="8" r="4"/><path d="M4 21c0-5 3-8 8-8s8 3 8 8"/>',
  log:'<path d="M6 3h9l4 4v14H6V3ZM15 3v5h4M9 12h7M9 16h7"/>',
  harddrive:'<rect x="3" y="5" width="18" height="14" rx="3"/><path d="M7 14h.01M12 14h6"/>',
  help:'<circle cx="12" cy="12" r="9"/><path d="M9 9a3 3 0 1 1 5 2c-1 .8-2 1.2-2 3m0 3h.01"/>',
  layers:'<path d="m12 2 9 5-9 5-9-5 9-5Zm-9 10 9 5 9-5M3 17l9 5 9-5"/>'
};
function I(name,cls=''){return `<svg class="lucide ${cls}" viewBox="0 0 24 24" aria-hidden="true">${icons[name]||icons.dot}</svg>`}
const PAGE_LABELS={overview:'基础设施总览',agents:'云端 Agent',nodes:'代理节点池',clients:'Windows 客户端',processes:'主控本机规划',alerts:'告警中心',events:'事件日志',settings:'系统设置'};
const state={token:sessionStorage.getItem('nodeConsoleAuth')||'',page:'overview',data:{agents:[],clients:[],nodes:[],assignments:[],events:[],alerts:[],enrollments:[],activeAlerts:0},processes:[],filter:'',agentFilter:'all',selected:new Set(),drawer:'',clientDetail:'',modal:'',busy:false,navOpen:false,modalData:null,bootstrapToken:'',lastUpdate:0,processFilter:'Client.exe'};
function compact(v){let n=Number(v)||0;return n>=1e9?(n/1e9).toFixed(1)+' GB':n>=1e6?(n/1e6).toFixed(1)+' MB':n>=1e3?(n/1e3).toFixed(1)+' KB':n+' B'}
function cpu(v){return Math.max(0,Math.min(100,Number(v)||0))}
function tformat(v){if(!v)return '—';const d=new Date(v);return Number.isNaN(d.getTime())?'—':d.toLocaleString('zh-CN',{month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false})}
function relative(v){if(!v)return '尚未上报';let s=(Date.now()-new Date(v).getTime())/1000;if(!Number.isFinite(s))return '—';if(s<30)return '刚刚';if(s<90)return '约 1 分钟前';if(s<3600)return Math.round(s/60)+' 分钟前';if(s<86400)return Math.round(s/3600)+' 小时前';return tformat(v)}
function usedCapacity(node){return state.data.assignments.filter(a=>a.nodeId===node.id).length+state.data.clients.reduce((n,c)=>n+(c.clientBindings||[]).filter(b=>b.nodeId===node.id).length,0)}
function sortedAgents(){return [...state.data.agents].sort((a,b)=>((a.status==='online')?0:1)-((b.status==='online')?0:1)||a.name.localeCompare(b.name))}
function agentById(id){return state.data.agents.find(a=>a.id===id)}
function nodeById(id){return state.data.nodes.find(a=>a.id===id)}
function status(text,kind=''){return `<span class="status ${kind}">${e(text)}</span>`}
function agentStatus(a){return a.status==='online'?status('在线','good'):a.status==='pending'?status('待接入','wait'):status('离线')}
function runtimeStatus(a){const r=a.runtime||{};if(a.status!=='online')return status('未确认','wait');if(a.desired?.enabled&&r.revision!==a.revision)return status('等待确认','wait');return r.status==='running'?status('运行中','good'):r.status==='error'?status('异常','bad'):a.desired?.enabled?status('等待同步','wait'):status('已停止')}
function nodeStatus(n){if(!n.enabled)return status('已停用');if(n.lastStatus==='tcp_ok')return status('TCP 已验证','good');if(n.lastStatus==='failed')return status('连接失败','bad');return status('待检测','wait')}
function track(n,kind=''){const v=cpu(n);return `<div class="resource-track"><div class="resource-fill ${kind}" style="width:${v}%"></div></div>`}
let chartID=0;
function smoothPath(points){
 if(points.length<2)return points.length?`M${points[0][0]} ${points[0][1]}`:'';
 let path=`M${points[0][0]} ${points[0][1]}`;
 for(let i=0;i<points.length-1;i++){
  const p0=points[Math.max(0,i-1)],p1=points[i],p2=points[i+1],p3=points[Math.min(points.length-1,i+2)];
  const c1x=p1[0]+(p2[0]-p0[0])/6,c1y=p1[1]+(p2[1]-p0[1])/6;
  const c2x=p2[0]-(p3[0]-p1[0])/6,c2y=p2[1]-(p3[1]-p1[1])/6;
  path+=` C${c1x.toFixed(1)} ${c1y.toFixed(1)} ${c2x.toFixed(1)} ${c2y.toFixed(1)} ${p2[0]} ${p2[1]}`;
 }
 return path;
}
function sparkline(data,key='cpu'){
 const vals=(data||[]).slice(-30).map(p=>cpu(p[key]||0));
 if(!vals.length)return '<span class="subtle">等待 Agent 上报采样</span>';
 const points=vals.map((v,i)=>[2+(i/Math.max(1,vals.length-1))*146,31-v*.27]);
 const line=smoothPath(points),area=`${line} L148 33 L2 33 Z`,id='sw-chart-'+(++chartID);
 return `<svg class="spark" viewBox="0 0 150 34" role="img" aria-label="CPU 使用率趋势"><defs><linearGradient id="${id}" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="var(--accent)" stop-opacity=".18"/><stop offset="1" stop-color="var(--accent)" stop-opacity="0"/></linearGradient></defs><path d="M2 10H148M2 20H148M2 30H148" stroke="var(--grid)" stroke-width=".5"/><path d="${area}" fill="url(#${id})"/><path d="${line}" stroke="var(--accent)" stroke-width="2" stroke-linecap="round" fill="none"/></svg>`;
}
function toast(msg,error=false){let wrap=$('#toast-region');if(!wrap)return;const item=document.createElement('div');item.className='toast'+(error?' error':'');item.innerHTML=I(error?'alert':'check')+`<span>${e(msg)}</span>`;wrap.appendChild(item);setTimeout(()=>item.remove(),4300)}
async function api(path,method='GET',body=undefined){const headers={'Authorization':'Bearer '+state.token};if(body!==undefined)headers['Content-Type']='application/json';let r;try{r=await fetch(path,{method,headers,body:body===undefined?undefined:JSON.stringify(body),cache:'no-store'})}catch(err){throw new Error('无法连接主控服务：'+err.message)};const type=r.headers.get('content-type')||'';const data=type.includes('json')?await r.json():await r.text();if(r.status===401){state.token='';sessionStorage.removeItem('nodeConsoleAuth');renderLogin();throw new Error('会话已失效，请重新输入访问令牌')};if(!r.ok)throw new Error(data?.error||`请求失败 ${r.status}`);return data}
async function connect(token){state.token=token;try{await api('/api/auth/check');sessionStorage.setItem('nodeConsoleAuth',token);await refresh();toast('已连接到控制平面')}catch(err){state.token='';sessionStorage.removeItem('nodeConsoleAuth');renderLogin(err.message)}}
async function refresh(silent=false){if(!state.token)return;try{const d=await api('/api/dashboard');state.data={agents:d.agents||[],clients:d.clients||[],nodes:d.nodes||[],assignments:d.assignments||[],events:d.events||[],alerts:d.alerts||[],enrollments:d.enrollments||[],activeAlerts:d.activeAlerts||0,proxyEngine:d.proxyEngine};state.lastUpdate=Date.now();if(state.page==='processes')await fetchProcesses();if(!state.modal&&!state.drawer&&!state.clientDetail)render();else {patchDrawer();if(!silent)toast('已更新最新节点状态')}}catch(err){if(state.token){if(!silent)toast(err.message,true);$('#connection-label')?.replaceChildren(document.createTextNode('连接异常'))}}}
async function fetchProcesses(){try{const d=await api('/api/processes?name='+encodeURIComponent(state.processFilter));state.processes=d.processes||[]}catch(err){state.processes=[];toast(err.message,true)}}
function patchDrawer(){
 if(!state.drawer)return;const a=agentById(state.drawer);if(!a)return;const r=a.runtime||{},m=a.metrics||{};
 const put=(name,text)=>{const x=document.querySelector('[data-live="'+name+'"]');if(x)x.textContent=text};
 put('cpu',cpu(m.cpu).toFixed(1)+'%');put('mem',m.memoryTotal?Math.round((m.memoryUsed||0)/m.memoryTotal*100)+'%':'—');put('connections',String(r.connections||0));put('traffic',compact(r.bytesUp)+' / '+compact(r.bytesDown));
 const statusBox=document.querySelector('[data-live="agent-status"]');if(statusBox)statusBox.innerHTML=agentStatus(a)+' '+runtimeStatus(a)+' <span class="muted small" style="margin-left:auto">'+relative(a.lastSeen)+'</span>';
}
function renderLogin(err=''){$('#root').innerHTML=`<div class="login-wrap"><div class="login-left"><div class="login-logo"><div class="brand-emblem">N</div>NodeSwarm <span class="login-suffix">/ CONTROL</span></div><div class="login-pitch"><div class="page-eyebrow" class="login-eyebrow">INFRASTRUCTURE · REIMAGINED</div><h1>一处掌控，<br>所有云端节点。</h1><p>从 Agent 接入、SOCKS5 配置到运行监控，让分散的基础设施变得简单、清晰、可控。</p><div class="login-feature"><span>${I('shield')}令牌认证</span><span>${I('cloud')}跨平台 Agent</span><span>${I('activity')}实时指标</span></div></div><div class="login-legal">NODESWARM · SELF-HOSTED</div></div><div class="login-right"><form id="login-form" class="login-form"><div class="page-eyebrow" style="margin-bottom:12px">WELCOME BACK</div><h2>登录控制台</h2><p class="intro">请输入主控端管理员访问令牌，安全连接你的节点。</p><div class="field"><label for="token">管理员访问令牌</label><input class="input" id="token" name="token" type="password" required minlength="24" autocomplete="off" placeholder="粘贴 64 位访问令牌" /></div><button class="btn btn-primary btn-block" type="submit" style="margin-top:21px">${I('arrow')}连接控制台</button><p class="login-error">${e(err)}</p><p class="login-help">首次启动后可在主控服务器的数据目录找到 <code>admin.token</code>。请勿向任何人分享该文件或将令牌写入公开脚本。</p></form></div></div>`;setTimeout(()=>$('#token')?.focus(),0)}
function sidebar(){const item=(name,ico)=>`<button class="nav-item ${state.page===name?'active':''}" data-page="${name}" type="button">${I(ico)}${PAGE_LABELS[name]}${name==='agents'?`<span class="nav-badge">${state.data.agents.filter(a=>a.status==='online').length}</span>`:name==='alerts'&&state.data.activeAlerts?`<span class="nav-badge alert-count">${state.data.activeAlerts}</span>`:''}</button>`;return `<aside class="sidebar"><div class="brand"><div class="brand-emblem">N</div><div><strong>NodeSwarm</strong><span>INFRASTRUCTURE PLATFORM</span></div></div><div class="workspace-switch"><div class="brand-emblem">N</div><div><strong>生产环境</strong><small>workspace / self-hosted</small></div>${I('down')}</div><div class="nav-group">工作空间</div>${item('overview','dashboard')}${item('agents','server')}${item('clients','harddrive')}${item('nodes','globe')}<div class="nav-group">诊断工具</div>${item('processes','layers')}<div class="nav-group">管理</div>${item('alerts','alert')}${item('events','list')}${item('settings','settings')}<div class="sidebar-bottom"><div class="sidebar-status"><span class="status-pulse"></span>控制平面在线</div><div class="sidebar-footnote">${state.data.agents.length} 台 Agent 已登记</div><div class="sidebar-version"><span>NodeSwarm v1.7.0</span><button class="btn btn-quiet btn-sm" style="color:var(--t3)" data-action="logout">退出</button></div></div></aside>`}
function heading(label,description,actions='',eyebrow='WORKSPACE'){return `<div class="page-heading"><div class="heading-copy"><div class="page-eyebrow">${eyebrow}</div><h1>${label}</h1><p>${description}</p></div><div class="heading-actions">${actions}</div></div>`}
const refreshButton=()=>`<button class="btn" data-action="refresh">${I('refresh')}刷新</button>`;
function metricCard(name,value,sub,ico,color=''){return `<div class="metric-card"><div class="metric-top"><span>${name}</span><div class="metric-icon ${color}">${I(ico)}</div></div><div class="metric-value numeric">${value}</div><div class="metric-bottom">${sub}</div></div>`}
function empty(ico,title,desc,action=''){return `<div class="empty"><div class="empty-art">${I(ico)}</div><h3>${title}</h3><p>${desc}</p>${action}</div>`}
function agentRow(a,compactView=false,checkbox=false){const cpuValue=cpu(a.metrics?.cpu),memTotal=a.metrics?.memoryTotal||0,memPct=memTotal?Math.round((a.metrics?.memoryUsed||0)/memTotal*100):0;
 return `<tr>${checkbox?`<td class="td-check"><input type="checkbox" data-check="agent" value="${e(a.id)}" ${state.selected.has(a.id)?'checked':''} aria-label="选择 ${e(a.name)}"></td>`:''}<td><div class="table-primary"><div class="avatar">${I(a.os==='windows'?'harddrive':'server')}</div><div><div class="name">${e(a.name)}</div><div class="description mono">${e(a.hostname||a.observedIp||a.id.slice(0,8))}</div></div></div></td><td>${agentStatus(a)}</td><td>${runtimeStatus(a)}</td>${!compactView?`<td><div class="resource-figure numeric">${cpuValue.toFixed(0)}% <em>CPU</em></div>${track(cpuValue)}</td><td><div class="resource-figure numeric">${memPct}% <em>内存</em></div>${track(memPct,'memory')}</td>`:''}<td class="muted table-nowrap">${relative(a.lastSeen)}</td><td><div class="table-actions"><button class="btn btn-quiet btn-sm" data-action="agent-detail" data-id="${e(a.id)}">管理 ${I('chevron')}</button></div></td></tr>`}
function overview(){const agents=state.data.agents,nodes=state.data.nodes,online=agents.filter(a=>a.status==='online').length,running=agents.filter(a=>a.runtime?.status==='running'&&a.status==='online').length,probed=nodes.filter(n=>n.lastStatus==='tcp_ok').length;

 return `${heading('基础设施总览','查看你的云端主机、SOCKS5 服务和网络连接情况。',`<button class="btn" data-action="setup">${I('terminal')}部署 Agent</button><button class="btn btn-primary" data-action="new-node">${I('plus')}添加代理节点</button>`,'OVERVIEW')}
 <section class="metric-grid">${metricCard('在线 Agent',online,`${agents.length} 台已注册设备`, 'server')}${metricCard('运行中服务',running,'由 Agent 实时上报','activity','teal')}${metricCard('代理节点总数',nodes.length,`${probed} 条 TCP 检测通过`,'globe','purple')}${metricCard('未处理告警',state.data.activeAlerts||0,'点击告警中心查看详情','alert','amber')}</section>
 <div class="dash-grid"><div class="stack"><section class="card"><div class="card-head"><div><h2>Agent 运行状态</h2><div class="minor">远程主机在线情况与代理服务状态</div></div><button class="link" data-page="agents">查看所有 Agent ${I('arrow')}</button></div>${agents.length?`<div class="table-wrap"><table><thead><tr><th>主机</th><th>连接</th><th>SOCKS5</th><th>最近心跳</th><th style="text-align:right">操作</th></tr></thead><tbody>${sortedAgents().slice(0,5).map(a=>agentRow(a,true)).join('')}</tbody></table></div>`:empty('cloud','还没有云端 Agent','在 Linux / Windows 服务器部署 Agent，即可在这里集中管理。',`<button class="btn btn-primary" data-action="setup">${I('plus')}查看部署步骤</button>`)}</section>
 <section class="card"><div class="card-head"><div><h2>代理节点池</h2><div class="minor">独立代理与云端 Agent 生成的托管出口</div></div><button class="link" data-page="nodes">管理节点 ${I('arrow')}</button></div>${nodes.length?`<div class="table-wrap"><table><thead><tr><th>节点</th><th>验证状态</th><th>容量</th><th style="text-align:right">详情</th></tr></thead><tbody>${nodes.slice(0,4).map(n=>`<tr><td><div class="table-primary"><div class="avatar">${I('globe')}</div><div><div class="name">${e(n.name)}</div><div class="description mono">${e(n.host)}:${n.port}</div></div></div></td><td>${nodeStatus(n)}</td><td><span class="numeric">${usedCapacity(n)} / ${n.capacity}</span></td><td><div class="table-actions"><button class="btn btn-quiet btn-sm" data-action="test-node" data-id="${e(n.id)}">检测 ${I('arrow')}</button></div></td></tr>`).join('')}</tbody></table></div>`:empty('globe','节点池还是空的','可以手工导入 SOCKS5，也可以由已接入 Agent 托管生成。',`<button class="btn btn-sm" data-action="new-node">添加代理</button>`)}</section></div>
 <div class="stack"><section class="card"><div class="card-head"><div><h2>运行资源</h2><div class="minor">在线 Agent 最新 CPU 使用率</div></div>${I('activity')}</div><div class="card-body">${agents.filter(a=>a.status==='online').length?agents.filter(a=>a.status==='online').slice(0,4).map(a=>`<div class="insight"><div class="insight-icon">${I('server')}</div><div style="flex:1;min-width:0"><div style="display:flex;justify-content:space-between;gap:6px"><h3>${e(a.name)}</h3><span class="numeric muted small">${cpu(a.metrics?.cpu).toFixed(1)}%</span></div><div style="display:flex;justify-content:space-between;align-items:center;gap:7px;margin-top:5px">${sparkline(a.history)}</div></div></div>`).join(''):empty('cpu','还没有资源样本','Agent 接入后自动采集 CPU、内存等指标。')}</div></section>
 <section class="card"><div class="card-head"><div><h2>最近操作</h2><div class="minor">代理配置和节点变更</div></div><button class="link" data-page="events">事件日志 ${I('arrow')}</button></div><div class="card-body">${recentEvents(5)||`<div class="empty small"><div class="empty-art">${I('list')}</div><h3>还没有事件</h3><p>执行操作后会自动显示在这里。</p></div>`}</div></section>
 <section class="card"><div class="card-body" style="display:flex;gap:12px;align-items:flex-start"><div class="insight-icon">${I('shield')}</div><div><h3 style="font-size:12px;margin-bottom:6px">安全连接</h3><p class="muted" style="font-size:11px">Agent 使用独立认证凭据连接主控。公网部署请启用 HTTPS，并限制 SOCKS5 端口的访问来源。</p><button class="link" data-page="settings" style="margin-top:8px">部署与安全说明 ${I('arrow')}</button></div></div></section></div></div>`
}
function agentsPage(){const total=state.data.agents.length,online=state.data.agents.filter(a=>a.status==='online').length,running=state.data.agents.filter(a=>a.runtime?.status==='running').length;
 const items=sortedAgents().filter(a=>(state.agentFilter==='all'||(state.agentFilter==='online'&&a.status==='online')||(state.agentFilter==='offline'&&a.status!=='online'))&&(`${a.name} ${a.hostname} ${a.observedIp} ${a.id}`).toLowerCase().includes(state.filter.toLowerCase()));
 return `${heading('云端 Agent', '集中控制所有 Linux / Windows 节点：接入、监控、启停、配置和诊断。',`${refreshButton()}<button class="btn btn-primary" data-action="setup">${I('plus')}部署 Agent</button>`,'AGENT FLEET')}
 <section class="metric-grid">${metricCard('已接入节点',total,'登记的云端主机','server')}${metricCard('当前在线',online,'心跳延迟小于 32 秒','activity','teal')}${metricCard('运行中的代理',running,'实际 SOCKS5 服务状态','globe','purple')}${metricCard('累计会话数',state.data.agents.reduce((n,a)=>n+(a.runtime?.totalConnections||0),0),'由 Agent 统计上报','layers','amber')}</section>
 <section class="card"><div class="toolbar"><div class="toolbar-left"><div class="search-wrap">${I('search')}<input class="input" data-filter="agents" placeholder="搜索名称、IP 或 Agent ID" value="${e(state.filter)}"></div><div class="tabs"><button class="tab ${state.agentFilter==='all'?'active':''}" data-action="agent-filter" data-value="all">全部</button><button class="tab ${state.agentFilter==='online'?'active':''}" data-action="agent-filter" data-value="online">在线</button><button class="tab ${state.agentFilter==='offline'?'active':''}" data-action="agent-filter" data-value="offline">离线</button></div></div><div class="toolbar-right"><button class="btn btn-sm" data-action="batch-start" ${state.selected.size?'':'disabled'}>${I('play')}批量启动</button><button class="btn btn-sm" data-action="batch-stop" ${state.selected.size?'':'disabled'}>${I('stop')}批量停止</button><span class="muted small">已选 ${state.selected.size} 台</span></div></div>
 ${items.length?`<div class="table-wrap"><table><thead><tr><th class="td-check"><input type="checkbox" data-check="all" aria-label="全选 Agent"></th><th>节点 / 系统</th><th>连接状态</th><th>代理服务</th><th>CPU</th><th>内存</th><th>最近心跳</th><th style="text-align:right">操作</th></tr></thead><tbody>${items.map(a=>agentRow(a,false,true)).join('')}</tbody></table></div>`:empty('server','没有符合条件的 Agent','部署并注册 Agent 后，就可以在这里远程管理。',`<button class="btn btn-primary" data-action="setup">部署新 Agent</button>`)}<div class="pagination-caption">显示 ${items.length} / ${total} 台 Agent · 每 7 秒心跳</div></section>
 <div class="info-strip" style="margin-top:var(--s-9)">${I('info')}<div><strong>关于控制范围</strong><p>云端 Agent 可远程运行和管理 SOCKS5 服务。它不是本地游戏客户端的透明代理驱动；Windows 客户端具有可选的 PID TCP 实验引擎，需单独安装官方驱动并实际验证。</p></div></div>`
}
function nodeRow(n){const used=usedCapacity(n);return `<tr><td><div class="table-primary"><div class="avatar">${I(n.managedAgentId?'server':'globe')}</div><div><div class="name">${e(n.name)}</div><div class="description mono">${e(n.host)}:${n.port}</div></div></div></td><td>${n.managedAgentId?`<span class="tag">Agent 托管</span>`:'<span class="tag">手动导入</span>'}</td><td>${nodeStatus(n)}</td><td><div class="row-number"><span class="numeric">${used} / ${n.capacity}</span><div class="progress-bar"><span style="width:${cpu(used/Math.max(1,n.capacity)*100)}%"></span></div></div></td><td>${n.lastStatus==='tcp_ok'?`<span class="numeric">${n.latencyMs} ms</span>`:'<span class="muted">—</span>'}</td><td><span class="muted">${e(n.region||n.provider||'—')}</span></td><td style="text-align:right"><div class="table-actions"><button class="btn btn-sm" data-action="test-node" data-id="${e(n.id)}">${I('activity')}检测</button>${n.managedAgentId?`<button class="btn btn-quiet btn-icon" data-action="agent-detail" data-id="${e(n.managedAgentId)}" title="打开托管 Agent">${I('arrow')}</button>`:`<button class="btn btn-quiet btn-icon" data-action="edit-node" data-id="${e(n.id)}" title="编辑节点">${I('edit')}</button><button class="btn btn-quiet btn-icon" data-action="delete-node" data-id="${e(n.id)}" title="删除节点">${I('trash')}</button>`}</div></td></tr>`}
function nodesPage(){const nodes=state.data.nodes.filter(n=>(n.name+' '+n.host+' '+n.region+' '+n.provider).toLowerCase().includes(state.filter.toLowerCase()));return `${heading('代理节点池','集中管理自建 SOCKS5 与第三方代理节点，维护检测状态和使用容量。',`${refreshButton()}<button class="btn" data-action="probe-all">${I('activity')}批量检测</button><button class="btn" data-action="import">${I('upload')}批量导入</button><button class="btn btn-primary" data-action="new-node">${I('plus')}新增节点</button>`,'PROXY INVENTORY')}
 <section class="card"><div class="toolbar"><div class="toolbar-left"><div class="search-wrap">${I('search')}<input class="input" data-filter="nodes" placeholder="搜索节点、地址、地区" value="${e(state.filter)}"/></div></div><div class="toolbar-right"><span class="muted small">当前 ${nodes.length} 个节点</span></div></div>
 ${nodes.length?`<div class="table-wrap"><table><thead><tr><th>代理节点</th><th>来源</th><th>状态</th><th>规划容量</th><th>延迟</th><th>位置 / 服务商</th><th style="text-align:right">管理</th></tr></thead><tbody>${nodes.map(nodeRow).join('')}</tbody></table></div>`:empty('globe','还没有代理节点','从 SOCKS5 服务商导入已有代理，或者配置云端 Agent 自动生成托管节点。',`<button class="btn btn-primary" data-action="new-node">${I('plus')}添加代理节点</button>`)}<div class="pagination-caption">TCP 检测通过不等于 UDP 或目标游戏连接已验证。验证由主控服务器发起。</div></section>
 <div class="section-grid" style="margin-top:20px"><div class="card"><div class="card-head"><h2>使用提示</h2></div><div class="card-body"><div class="insight"><div class="insight-icon">${I('activity')}</div><div><h3>真实代理探测</h3><p>通过 SOCKS5 握手、账密验证与 TCP CONNECT 检查线路是否通畅。</p></div></div><div class="insight"><div class="insight-icon">${I('layers')}</div><div><h3>节点容量</h3><p>可以为每条代理设置计划分配容量，避免将多个进程误分配至已满节点。</p></div></div><div class="insight"><div class="insight-icon">${I('shield')}</div><div><h3>凭据保护</h3><p>节点密码经过加密保存，管理接口只展示是否已配置密码。</p></div></div></div></div><div class="card"><div class="card-head"><h2>网络连接概览</h2></div><div class="card-body"><div class="util-grid"><div class="mini-stat"><span class="label">TCP 已验证</span><strong>${state.data.nodes.filter(n=>n.lastStatus==='tcp_ok').length}</strong></div><div class="mini-stat"><span class="label">连接失败</span><strong>${state.data.nodes.filter(n=>n.lastStatus==='failed').length}</strong></div><div class="mini-stat"><span class="label">Agent 托管</span><strong>${state.data.nodes.filter(n=>!!n.managedAgentId).length}</strong></div><div class="mini-stat"><span class="label">手动节点</span><strong>${state.data.nodes.filter(n=>!n.managedAgentId).length}</strong></div></div></div></div></div>`}

function clientsPage(){
 const clients=state.data.clients||[],online=clients.filter(c=>c.status==='online').length;
 const active=clients.reduce((n,c)=>n+(c.client?.processes||[]).length,0),bound=clients.reduce((n,c)=>n+(c.clientBindings||[]).length,0);
 const filtered=clients.filter(c=>`${c.name} ${c.hostname} ${c.id}`.toLowerCase().includes(state.filter.toLowerCase()));
 const rows=filtered.map(c=>`<tr><td><div class="table-primary"><div class="avatar">${I('harddrive')}</div><div><div class="name">${e(c.name)}</div><div class="description mono">${e(c.hostname||c.observedIp||c.id)}</div></div></div></td><td>${agentStatus(c)}</td><td>${status((c.client?.backend||'observe')==='proxybridge'?'ProxyBridge':'观测模式',(c.client?.backend==='proxybridge')?'wait':'')}</td><td class="numeric">${(c.client?.processes||[]).length}</td><td class="numeric"><strong>${(c.clientBindings||[]).length}</strong> <span class="muted">/ ${c.maxProxyPids||8}</span></td><td class="muted">${relative(c.lastSeen)}</td><td><div class="table-actions"><button class="btn btn-sm" data-action="client-detail" data-id="${e(c.id)}">${I('settings')}管理进程</button></div></td></tr>`).join('');
 return `${heading('Windows 客户端','独立部署在 Windows 游戏主机，集中查看进程实例和配置 SOCKS5 节点策略。',`${refreshButton()}<button class="btn btn-primary" data-page="settings">${I('plus')}部署客户端</button>`,'CLIENT FLEET')}
 <section class="metric-grid">${metricCard('已注册客户端',clients.length,'Windows 工作站','harddrive')}${metricCard('在线客户端',online,'最近 32 秒收到心跳','activity','teal')}${metricCard('已发现目标进程',active,'按目标 EXE 名称过滤','layers','purple')}${metricCard('已规划节点绑定',bound,'不代表流量已代理','link','amber')}</section>
 <div class="info-strip warning">${I('alert')}<div><strong>绑定不等于真实出站已验证</strong><p>Windows Client 会核验 PID 与进程启动时间。默认只观察；可选 ProxyBridge 进程名规则，但不支持同名不同 PID 的独立出口。界面只展示已确认的执行阶段。</p></div></div>
 <section class="card"><div class="toolbar"><div class="search-wrap">${I('search')}<input class="input" data-filter="clients" value="${e(state.filter)}" placeholder="搜索电脑名称 / 主机名"></div><div class="toolbar-right muted small">${clients.length} 台工作站</div></div>${rows?`<div class="table-wrap"><table><thead><tr><th>设备</th><th>在线状态</th><th>客户端模式</th><th>发现进程</th><th>PID 配额（已绑定/上限）</th><th>最近心跳</th><th style="text-align:right">操作</th></tr></thead><tbody>${rows}</tbody></table></div>`:empty('harddrive','暂无 Windows Client','在 Windows 电脑安装 NodeSwarm Client，注册后会显示实际进程与心跳。',`<button class="btn btn-primary" data-page="settings">查看部署方法</button>`)}</section>`
}
function clientDrawerMarkup(){
 const c=(state.data.clients||[]).find(c=>c.id===state.clientDetail);if(!c)return '';
 const ps=c.client?.processes||[],bindings=c.clientBindings||[],routes=c.client?.routes||[];
 const quota=c.maxProxyPids||8, used=bindings.length, localCap=c.client?.maxProxyPids||0;
 const nodes=state.data.nodes||[];
 const rows=ps.map(p=>{
   const assigned=bindings.find(b=>b.pid===p.pid&&b.startTime===p.startTime);
   const route=routes.find(r=>r.pid===p.pid&&r.startTime===p.startTime);
   const options=`<option value="">不绑定</option>`+nodes.filter(n=>n.enabled||n.id===assigned?.nodeId).map(n=>`<option value="${e(n.id)}" ${assigned?.nodeId===n.id?'selected':''}>${e(n.name)} (${e(n.region||n.host)}) · ${usedCapacity(n)}/${n.capacity}</option>`).join('');
   const msg=route?`${route.status} · ${route.detail||''}`:(assigned?'待下发策略':'未绑定');
   const kind=route?.status==='process_name_unverified'?'wait':route?.status==='planned_only'?'wait':route?'bad':'';
   return `<tr><td><strong>${e(p.name)}</strong><div class="mono-muted">PID ${p.pid} · ${e(p.startTime)}</div><div class="mono-muted" title="${e(p.path)}">${e(p.path||'')}</div></td><td><select class="select client-node-select" data-client-bind="1" data-pid="${p.pid}" data-start="${e(p.startTime)}" aria-label="为 PID ${p.pid} 选择节点">${options}</select></td><td>${status(msg,kind)}</td></tr>`;
 }).join('');
 return `<div class="drawer-overlay" data-overlay="true"><aside class="drawer client-drawer" role="dialog" aria-modal="true" aria-label="Windows Client ${e(c.name)}"><div class="drawer-head"><div class="drawer-head-top"><div><div class="page-eyebrow">WINDOWS CLIENT</div><h2>${e(c.name)}</h2><p>${e(c.hostname||c.observedIp)} · ${e(c.id)}</p></div><button class="btn btn-icon btn-quiet" data-action="close-overlay" aria-label="关闭">${I('x')}</button></div><div class="client-status-row">${agentStatus(c)}${status((c.client?.backend||'observe')==='proxybridge'?'应用名规则模式':'仅观测','wait')}<span class="muted">最近心跳 ${relative(c.lastSeen)}</span></div></div>
 <div class="drawer-content"><section class="drawer-section"><h3>策略执行状态</h3><div class="modal-notice">${I('info')} ${e(c.client?.error||'目前只报告策略和进程名规则状态，不代表 PID 级流量已验证。')}</div><div class="drawer-kv"><span>目标 EXE 数量</span><span>${ps.length}</span></div><div class="drawer-kv"><span>期望修订 / 已回报</span><span>${c.revision} / ${c.client?.revision||0}</span></div></section>
 <section class="drawer-section"><h3>本机可代理 PID 数量上限</h3>
 <p class="muted" style="font-size:12px;margin:0 0 13px">每台 Windows 机器独立设置。保存后主控和客户端会共同校验；超额绑定直接拒绝执行，不会被标记为代理成功。</p>
 <div class="pid-quota-header"><div><strong id="client-quota-used">${used}</strong> <span class="muted">/ ${quota} 个 PID 已规划</span></div>${status(used>=quota?'配额已满':'可继续分配',used>=quota?'wait':'good')}</div>
 <div class="pid-quota-track"><span style="width:${Math.min(100,Math.round(used/quota*100))}%"></span></div>
 <div class="pid-quota-form"><label for="client-limit">允许代理的 PID 上限</label><input class="input" id="client-limit" type="number" min="1" max="128" step="1" value="${quota}"/><button class="btn" data-action="save-client-limit">保存上限</button></div>
 <p class="muted" style="font-size:11px;margin:10px 0 0">有效上限：${localCap?`${localCap}（客户端已确认）`:`等待客户端下一次心跳回报`}。如果客户端设置了更低的本机硬上限，以较低值为准。</p>
 <div id="client-quota-preview" class="muted" style="font-size:11px;margin-top:8px">选择节点后会实时检查本次计划数量。</div></section>
 <section class="drawer-section"><h3>目标进程与代理节点</h3><p class="muted" style="font-size:12px;margin-bottom:12px">每条记录按 PID + 启动时间识别。已退出或重启的进程自动失效。</p><div class="table-wrap"><table><thead><tr><th>目标进程</th><th>节点</th><th>实际状态</th></tr></thead><tbody>${rows||`<tr><td colspan="3">当前没有识别到目标进程。请检查 Client.exe 是否启动以及客户端过滤配置。</td></tr>`}</tbody></table></div></section>
 <section class="drawer-section"><h3>客户端记录</h3>${(c.logs||[]).slice(-6).reverse().map(l=>`<div class="log-row"><div class="log-symbol">${I('activity')}</div><div><strong>${e(l.message)}</strong><time>${tformat(l.at)}</time></div></div>`).join('')||'<p class="muted">尚无事件记录</p>'}</section><section class="drawer-section"><button class="btn btn-danger btn-sm" data-action="delete-client" data-id="${e(c.id)}">${I('trash')}注销此客户端</button></section></div>
 <div class="drawer-actions"><button class="btn" data-action="auto-client-bindings">自动填充空位</button><button class="btn btn-quiet" data-action="close-overlay">取消</button><button class="btn btn-primary" data-action="save-client-bindings">${I('check')}保存并下发绑定</button></div></aside></div>`;
}
function processesPage(){const assign=state.data.assignments||[];return `${heading('本机进程分配','查看运行主控的这台设备上的进程，为后续透明代理引擎规划节点分配。',`<button class="btn" data-action="refresh-process">${I('refresh')}刷新进程</button>`,'LOCAL PROCESS PLANNING')}
 <div class="info-strip warning">${I('alert')}<div><strong>规划模式 · 不会修改真实网络流量</strong><p>此页面只有 PID 绑定规划。云端 Agent 代理服务已可实际运行，但客户端的透明分流引擎尚未接入。这里的“分配”不会改变 Client.exe 的出口。</p></div></div><div class="card"><div class="toolbar"><div class="toolbar-left"><div class="search-wrap">${I('search')}<input class="input" id="process-filter" placeholder="例如 Client.exe" value="${e(state.processFilter)}"></div><button class="btn" data-action="filter-process">搜索进程</button></div><div class="toolbar-right muted small">找到 ${state.processes.length} 条进程</div></div><div class="table-wrap"><table><thead><tr><th>进程名 / 路径</th><th>PID</th><th>启动时间</th><th>规划绑定</th><th style="text-align:right">操作</th></tr></thead><tbody>${state.processes.length?state.processes.slice(0,120).map(p=>{const a=assign.find(x=>x.pid===p.pid&&x.startTime===p.startTime);return `<tr><td><div class="table-primary"><div class="avatar">${I('layers')}</div><div style="min-width:0"><div class="name">${e(p.name)}</div><div class="description mono" style="max-width:320px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="${e(p.path)}">${e(p.path||'—')}</div></div></div></td><td class="mono">${p.pid}</td><td>${tformat(p.startTime)}</td><td>${a?`<span class="tag">${e(nodeById(a.nodeId)?.name||'节点已移除')}</span>`:'<span class="muted">未规划</span>'}</td><td><div class="table-actions">${a?`<button class="btn btn-quiet btn-sm" data-action="unassign" data-pid="${p.pid}" data-start="${e(p.startTime)}">解除规划</button>`:`<select class="select" style="width:155px;min-height:31px;padding:4px" id="assign-${p.pid}"><option value="">自动选择</option>${state.data.nodes.filter(n=>n.enabled).map(n=>`<option value="${e(n.id)}">${e(n.name)}</option>`).join('')}</select><button class="btn btn-sm" data-action="assign" data-pid="${p.pid}" data-start="${e(p.startTime)}" ${state.data.nodes.length?'':'disabled'}>规划</button>`}</div></td></tr>`}).join(''):`<tr><td colspan="5">${empty('layers','未找到匹配进程','请确认进程正在运行，或修改搜索关键词。')}</td></tr>`}</tbody></table></div></div>`}
function allEvents(){const local=(state.data.events||[]).map(x=>({...x,source:'主控'}));const remote=(state.data.agents||[]).flatMap(a=>(a.logs||[]).map(l=>({...l,source:a.name})));return [...local,...remote].sort((a,b)=>new Date(b.at)-new Date(a.at))}
function recentEvents(n){return allEvents().slice(0,n).map(x=>`<div class="log-row"><div class="log-symbol ${e(x.level)}">${I(x.level==='success'?'check':x.level==='warning'||x.level==='error'?'alert':'activity')}</div><div><strong>${e(x.message)}</strong><time>${e(x.source)} · ${tformat(x.at)}</time></div></div>`).join('')}
function eventsPage(){return `${heading('事件日志','集中查看主控配置操作和远端 Agent 上报的诊断信息。',refreshButton(),'EVENT STREAM')}<div class="card"><div class="toolbar"><h2>最近事件</h2><span class="muted small">最多展示最新 150 条</span></div><div class="card-body">${recentEvents(150)||empty('list','还没有可用事件','当节点开始运行或配置被修改，事件会显示在这里。')}</div></div>`}
function alertsPage(){
 const items=state.data.alerts||[],active=items.filter(a=>!a.mutedUntil),critical=active.filter(a=>a.severity==='critical');
 const rows=items.map(a=>`<tr><td>${a.severity==='critical'?status('严重','bad'):status('警告','wait')}</td><td><div class="table-primary"><div class="avatar">${I('alert')}</div><div><div class="name">${e(a.title)}</div><div class="description">${e(a.detail||'暂无详细信息')}</div></div></div></td><td><span class="mono-muted">${e(a.targetName)}</span></td><td>${a.mutedUntil?status('已静音至 '+tformat(a.mutedUntil)) : status('待处理','wait')}</td><td><div class="table-actions">${a.kind==='agent'?`<button class="btn btn-sm" data-action="agent-detail" data-id="${e(a.targetId)}">查看 Agent</button>`:`<button class="btn btn-sm" data-page="nodes">查看节点池</button>`}${a.mutedUntil?`<button class="btn btn-quiet btn-sm" data-action="unmute-alert" data-id="${e(a.id)}">恢复提醒</button>`:`<button class="btn btn-quiet btn-sm" data-action="mute-alert" data-id="${e(a.id)}">静音24h</button>`}</div></td></tr>`).join('');
 return `${heading('告警中心','从 Agent 心跳、代理状态、CPU/内存和节点有效期推导真实告警，可临时静音。',refreshButton(),'FLEET HEALTH')}
 <section class="metric-grid">${metricCard('当前告警',active.length,'不含已静音项目','alert','amber')}${metricCard('严重告警',critical.length,'需要尽快处理','shield')}${metricCard('暂时静音',items.length-active.length,'到期自动恢复提醒','clock')}${metricCard('监控主机',state.data.agents.length,'包含在线、离线和待接入','server')}</section>
 <section class="card"><div class="card-head"><div><h2>实时告警列表</h2><div class="minor">状态由最新心跳与存储数据生成，不代表已向外部发送通知</div></div></div>${items.length?`<div class="table-wrap"><table><thead><tr><th>级别</th><th>问题</th><th>对象</th><th>处理状态</th><th style="text-align:right">操作</th></tr></thead><tbody>${rows}</tbody></table></div>`:empty('check','所有监控指标正常','暂时没有超出阈值的告警。')}</section>`;
}
function enrollmentPanel(){const grants=state.data.enrollments||[];const rows=grants.map(g=>`<tr><td><strong>${e(g.label)}</strong><div class="mono-muted">${e(g.id)}</div></td><td>${tformat(g.expiresAt)}</td><td>${Date.now()<new Date(g.expiresAt).getTime()?status('未使用','good'):status('已过期','wait')}</td><td style="text-align:right"><button class="btn btn-sm btn-quiet" data-action="revoke-grant" data-id="${e(g.id)}">撤销</button></td></tr>`).join('');return `<section class="card" style="margin-top:var(--s-8)"><div class="card-head"><div><h2>Agent 一次性注册令牌</h2><div class="minor">每个令牌只能注册一台 Agent，到期或使用后自动失效；避免把长期密钥发给所有节点。</div></div><button class="btn btn-primary btn-sm" data-action="new-enrollment">${I('plus')}生成令牌</button></div>${grants.length?`<div class="table-wrap"><table><thead><tr><th>用途</th><th>过期时间</th><th>状态</th><th style="text-align:right">操作</th></tr></thead><tbody>${rows}</tbody></table></div>`:`<div class="card-body">${empty('shield','尚未生成一次性密钥','使用“生成令牌”创建单次注册凭据；旧的共享密钥仍可继续使用。')}</div>`}</section>`}
const linuxCommand=`sudo install -m 0755 node-console-linux-amd64 /usr/local/bin/node-console
sudo mkdir -p /var/lib/node-console-agent
sudo chmod 700 /var/lib/node-console-agent
# 使用独立的 root-only 环境文件设置：
# NODE_MANAGER_CONTROLLER=https://console.example.com
# NODE_MANAGER_ENROLL_KEY=<从主控数据目录读取>
# NODE_MANAGER_AGENT_NAME=Cloud-01
sudo systemctl enable --now node-console-agent`;
const windowsCommand=`# PowerShell 管理员会话
$env:NODE_MANAGER_CONTROLLER = "https://console.example.com"
$env:NODE_MANAGER_ENROLL_KEY = "<主控服务器的 enroll.token>"
$env:NODE_MANAGER_AGENT_NAME = "Windows-01"
.\\node-console-windows-amd64.exe -mode agent
# 正式后台运行建议使用 NSSM / WinSW 服务包装，并采用系统权限保护环境变量`;
const clientCommand=`# 在 Windows 11 管理员 PowerShell 中安装 Windows Client
.\deploy\windows\install-client.ps1
notepad "$env:ProgramData\NodeSwarmClient\client.env"
# 设置 HTTPS 主控、一次性注册密钥、目标 EXE
Start-ScheduledTask -TaskName 'NodeSwarmClient'
# 默认 observe：不会更改任何进程网络
# 若已经安装官方 ProxyBridge CLI，可配置 proxybridge 模式，
# 仅对进程名统一分流，不支持同名多个 PID 的独立出口`;
const caddyConfig=`console.example.com {
    encode zstd gzip
    reverse_proxy 127.0.0.1:8088
}`;
function codeBlock(title,code,info=''){return `<div class="codebar"><strong style="font-size:11px">${title}</strong><button class="btn btn-sm" data-action="copy-code" data-code="${e(encodeURIComponent(code))}">${I('copy')}复制</button></div><div class="codebox">${e(code)}</div>${info?`<p class="code-note">${info}</p>`:''}`}
function settingsPage(){return `${heading('部署与安全','部署主控与 Agent、检查访问安全，并了解当前产品的能力边界。',refreshButton(),'DOCUMENTATION')}
 <div class="doc-container"><section class="card"><div class="card-head"><h2>快速部署指南</h2><span class="tag">Self-hosted</span></div><div class="card-body"><div class="doc-section"><div class="doc-step"><span class="step-num">01</span>启动主控 Controller</div><p>在主控服务器运行 Linux 可执行文件，默认仅监听 127.0.0.1:8088。管理员凭据和 Agent 注册密钥会保存在数据目录，权限应为 0600。</p>${codeBlock('Linux Controller','NODE_MANAGER_DATA=/var/lib/node-console /usr/local/bin/node-console -mode controller','# 主控监听本地端口；请在前方配置 TLS 反向代理。')}</div><div class="doc-section"><div class="doc-step"><span class="step-num">02</span>配置 HTTPS 入口</div><p>使用 Caddy / Nginx 为控制平面提供有效 TLS 证书，Agent 默认只允许连接 HTTPS 主控。</p>${codeBlock('Caddyfile',caddyConfig,'请把 console.example.com 替换为自己的实际域名，并将防火墙只开放必要端口。')}</div><div class="doc-section"><div class="doc-step"><span class="step-num">03</span>部署云服务器 Agent</div><p>把相应架构的二进制上传到云主机，配置主控地址和 enrollment token，并以常驻服务运行。Agent 发起出站 HTTPS 连接，不要求主控访问 Agent 管理端口。</p>${codeBlock('Linux Agent',linuxCommand,'完整 systemd service 与环境文件示例参见压缩包 deploy/ 目录。')}${codeBlock('Windows Agent',windowsCommand)}<div class="doc-step"><span class="step-num">04</span>部署 Windows 游戏客户端</div><p>Windows Client 会向 Controller 汇报目标进程，并接收节点绑定策略，默认仅观察模式。</p>${codeBlock('Windows Client',clientCommand)}</div><div class="doc-section" style="margin-bottom:0"><div class="doc-step"><span class="step-num">05</span>管理 SOCKS5 出口</div><p>在“云端 Agent”选中设备，设置对外 IP/域名、SOCKS5 监听端口、认证账号、允许来源网段，然后保存并启动。再到代理节点池检测连接。</p></div></div></section>
 ${enrollmentPanel()}
 <section class="card" style="margin-top:var(--s-8)"><div class="card-head"><h2>上线前安全检查</h2></div><div class="card-body"><div class="insight"><div class="insight-icon">${I('shield')}</div><div><h3>HTTPS 与最小暴露面</h3><p>主控 HTTP 默认仅回环可访问；公开部署应使用 TLS 终止和访问控制。SOCKS5 需设置强密码、云防火墙允许来源 CIDR。</p></div></div><div class="insight"><div class="insight-icon">${I('harddrive')}</div><div><h3>数据文件保护</h3><p>管理员令牌、Agent 注册令牌和加密密钥应限制为运维账号可读，并进行安全备份。</p></div></div><div class="insight"><div class="insight-icon">${I('info')}</div><div><h3>关于客户端透明代理</h3><p>本版本可远程管理真实 SOCKS5 代理服务，已有可选的 PID TCP 实验模式，但尚未完成 UDP / DNS 防泄漏和 Windows 实机出口验收。</p></div></div></div></section></div>`}
function render(){if(!state.token){renderLogin();return}const pages={overview,agents:agentsPage,clients:clientsPage,nodes:nodesPage,processes:processesPage,alerts:alertsPage,events:eventsPage,settings:settingsPage};$('#root').innerHTML=`<div class="shell ${state.navOpen?'nav-open':''}">${sidebar()}<div class="shell-main"><header class="topbar"><div class="breadcrumbs"><button class="top-action mobile-top" data-action="toggle-nav" aria-label="打开导航">${I('menu')}</button><span>工作空间</span>${I('chevron')}<strong>${PAGE_LABELS[state.page]}</strong></div><div class="top-actions"><div class="top-pill"><span class="live-dot"></span><span id="connection-label">主控已连接</span></div><button class="top-action" data-action="refresh" title="刷新所有数据" aria-label="刷新所有数据">${I('refresh')}</button><button class="top-action" data-page="settings" title="部署与设置" aria-label="部署与设置">${I('help')}</button><div class="top-avatar" title="管理员">A</div></div></header><main class="page-content">${pages[state.page]()}<div class="page-footer"><span>NodeSwarm · SELF-HOSTED INFRASTRUCTURE CONTROL</span><span>数据来自真实 Agent / 服务端，无模拟节点</span></div></main></div></div>${state.modal?modalMarkup():''}${state.drawer?drawerMarkup():''}${state.clientDetail?clientDrawerMarkup():''}`;}
function openModal(which,id=''){state.modal=which;state.modalData=id;if(which==='new-node'||which==='edit-node')state.modalData=id?nodeById(id):null;render()}
function closeOverlay(){state.modal='';state.drawer='';state.clientDetail='';state.modalData=null;state.bootstrapToken='';render()}
function field(label,name,value='',extra='',help=''){return `<div class="field"><label for="field-${name}">${label}</label><input class="input" id="field-${name}" name="${name}" value="${e(value)}" ${extra}>${help?`<small>${help}</small>`:''}</div>`}
function nodeModalBody(){const v=state.modalData||{};return `<form id="node-form" data-id="${e(v.id||'')}"><div class="field-grid">${field('节点名称','name',v.name||'','required maxlength="96" placeholder="例如 香港线路 01"')}${field('供应商','provider',v.provider||'','maxlength="96" placeholder="可选"')}${field('服务器 IP / 域名','host',v.host||'','required placeholder="代理公网 IP 或域名"')}${field('SOCKS5 端口','port',v.port||1080,'required type="number" min="1" max="65535"')}${field('账号','username',v.username||'','placeholder="如无需认证可留空"')}${field('密码','password','','type="password" autocomplete="new-password" placeholder="'+(v.id?'留空则保留原密码':'可留空')+'"')}${field('节点地区','region',v.region||'','placeholder="可选"')}${field('规划容量','capacity',v.capacity||1,'required type="number" min="1" max="1000"')}<div class="field field-span">${field('有效期','expiresAt',v.expiresAt?new Date(v.expiresAt).toISOString().slice(0,16):'','type="datetime-local"')}</div><label class="switchrow field-span"><input name="enabled" type="checkbox" ${v.enabled===false?'':'checked'}>启用此节点</label></div></form><div class="modal-notice" style="margin-top:var(--s-9)">${I('shield')} 代理密码不会回传到管理页面。添加后建议立即执行 TCP 检测。</div>`}
function importModalBody(){return `<p class="muted" style="font-size:12px;margin-bottom:15px">每行一个 SOCKS5 节点，使用以下格式。重复的地址、端口和账号会自动跳过。</p><div class="modal-notice" style="margin-bottom:13px"><span class="mono">IP|端口|账号|密码|到期时间</span><br><span class="muted">示例：proxy.example.com|1080|user|password|2027-01-01</span></div><textarea class="textarea mono" id="import-text" rows="8" placeholder="请粘贴节点列表，最多 300 行"></textarea><p class="muted" style="font-size:11px;margin-top:11px">请不要把真实账号密码提交到公开代码仓库。</p>`}
function modalMarkup(){const v=state.modal;
 let name,subtitle,body,footer;
 if(v==='new-node'||v==='edit-node'){name=v==='edit-node'?'编辑代理节点':'新增代理节点';subtitle='配置 SOCKS5 连接信息与节点容量';body=nodeModalBody();footer=`<button class="btn" data-action="close-overlay">取消</button><button class="btn btn-primary" data-action="save-node">${I('check')}保存节点</button>`}
 else if(v==='new-enrollment'){name='创建一次性注册令牌';subtitle='仅允许一台 Agent 使用，超时后失效';body=`<form id="enroll-form"><div class="field-grid">${field('令牌用途','label','Cloud-01','required maxlength="80"')}${field('有效分钟数','ttlMinutes',60,'type="number" min="1" max="1440" required')}</div></form>`;footer=`<button class="btn" data-action="close-overlay">取消</button><button class="btn btn-primary" data-action="save-enrollment">${I('check')}生成密钥</button>`}
 else if(v==='enrollment-issued'){name='注册密钥已生成';subtitle='密钥只会在此刻展示，关闭后无法再次查看';body=`<div class="modal-notice">${I('shield')} 请妥善保管此密钥，只允许注册一台 Agent，注册成功后立即失效。</div>${codeBlock('NODE_MANAGER_ENROLL_KEY',state.bootstrapToken)}`;footer=`<button class="btn btn-primary" data-action="close-overlay">我已保存密钥</button>`}
 else if(v==='import'){name='批量导入节点';subtitle='支持 SOCKS5 地址、端口、账号、密码和到期时间';body=importModalBody();footer=`<button class="btn" data-action="close-overlay">取消</button><button class="btn btn-primary" data-action="save-import">${I('upload')}确认导入</button>`}
 else{return ''}
 return `<div class="overlay" data-overlay="true"><div class="dialog" role="dialog" aria-modal="true" aria-label="${name}"><div class="dialog-head"><div><h2>${name}</h2><p>${subtitle}</p></div><button class="btn btn-quiet btn-icon" data-action="close-overlay" aria-label="关闭">${I('x')}</button></div><div class="dialog-body">${body}</div><div class="dialog-footer">${footer}</div></div></div>`
}
function drawerMarkup(){const a=agentById(state.drawer);if(!a)return '';const c=a.desired||{},m=a.metrics||{},r=a.runtime||{},memPercent=m.memoryTotal?Math.round((m.memoryUsed||0)/m.memoryTotal*100):0;
 const display=(k,v)=>`<div class="drawer-kv"><span>${k}</span><span>${e(v||'—')}</span></div>`;
 return `<div class="drawer-overlay" data-overlay="true"><aside class="drawer" role="dialog" aria-modal="true" aria-label="管理 ${e(a.name)}"><div class="drawer-head"><div class="drawer-head-top"><div><div class="page-eyebrow" style="margin-bottom:9px">NODE DETAILS</div><h2>${e(a.name)}</h2><p>${e(a.hostname||a.id)} · ${e(a.os.toUpperCase())}/${e(a.arch)}</p></div><button class="btn btn-icon btn-quiet" data-action="close-overlay" aria-label="关闭">${I('x')}</button></div><div data-live="agent-status" style="display:flex;gap:7px;align-items:center;margin-top:16px">${agentStatus(a)} ${runtimeStatus(a)}<span class="muted small" style="margin-left:auto">${relative(a.lastSeen)}</span></div></div>
 <div class="drawer-content"><section class="drawer-section"><h3>实时运行状态</h3><div class="util-grid"><div class="mini-stat"><span class="label">CPU 使用率</span><strong class="numeric" data-live="cpu">${cpu(m.cpu).toFixed(1)}%</strong>${track(m.cpu)}</div><div class="mini-stat"><span class="label">内存使用率</span><strong class="numeric" data-live="mem">${memPercent}%</strong>${track(memPercent,'memory')}</div><div class="mini-stat"><span class="label">当前连接数</span><strong class="numeric" data-live="connections">${r.connections||0}</strong></div><div class="mini-stat"><span class="label">上下行流量</span><strong style="font-size:13px" data-live="traffic">${compact(r.bytesUp)} / ${compact(r.bytesDown)}</strong></div></div>${r.error?`<div class="inline-msg" style="margin-top:13px">${I('alert')} ${e(r.error)}</div>`:''}</section>
 <section class="drawer-section"><h3>远程 SOCKS5 配置</h3><p class="muted" style="font-size:11px;margin:-5px 0 16px">保存后配置会在下一次 Agent 心跳（约 7 秒）应用。</p><form id="agent-form" data-id="${e(a.id)}"><div class="field-grid">${field('设备显示名称','name',a.name,'required maxlength="80"')}${field('公网 IP / 域名','publicHost',c.publicHost||'','placeholder="云服务器的公网 IP"','未填写时不会生成代理节点池条目。')}${field('本机监听 IP','listen',c.listen||'0.0.0.0','required','通常为 0.0.0.0')}${field('监听端口','port',c.port||1080,'type="number" min="1" max="65535" required')}${field('SOCKS5 用户名','username',c.username||'node','maxlength="255" required')}${field('SOCKS5 密码','password','','type="password" autocomplete="new-password" placeholder="'+(c.hasPassword?'留空保留现有密码':'留空则生成安全密码')+'"')}${field('规划实例容量','capacity',c.capacity||8,'type="number" min="1" max="1000" required')}${field('最大并发连接','maxConnections',c.maxConnections||200,'type="number" min="1" max="4000" required')}${field('UDP 起始端口','udpPortStart',c.udpPortStart||'','type="number" min="1024" max="65535" placeholder="留空使用动态端口"')}${field('UDP 结束端口','udpPortEnd',c.udpPortEnd||'','type="number" min="1024" max="65535" placeholder="最多128个端口"')}${field('线路地区','region',c.region||'','placeholder="例如 东京"')}${field('服务商','provider',c.provider||'','placeholder="例如 自建 VPS"')}<div class="field field-span"><label for="field-allowedCidrs">允许访问的客户端 CIDR（每行一条）</label><textarea class="textarea mono" id="field-allowedCidrs" name="allowedCidrs" rows="3" placeholder="例如 203.0.113.4/32">${e((c.allowedCidrs||[]).join('\n'))}</textarea><small>留空表示任意公网来源可尝试认证；生产环境强烈建议限制来源网段，且配置云防火墙。</small></div><div class="field-span" style="display:flex;gap:18px;flex-wrap:wrap"><label class="switchrow"><input type="checkbox" name="enabled" ${c.enabled?'checked':''}/>启用代理</label><label class="switchrow"><input type="checkbox" name="udp" ${c.udp?'checked':''}/>启用 SOCKS5 UDP</label></div></div></form><button class="btn btn-primary btn-block" data-action="save-agent" style="margin-top:var(--s-8)">${I('check')}保存并下发配置</button><div class="modal-notice" style="margin-top:12px">${I('shield')} SOCKS5 用户名/密码在控制器上加密保存，Agent 通过 HTTPS 接收。SOCKS5 本身不会加密端到端传输。</div></section>
 <section class="drawer-section"><h3>节点与接入信息</h3>${display('Agent ID',a.id)}${display('公网可见 IP',a.observedIp)}${display('配置公网地址',c.publicHost?`${c.publicHost}:${c.port}`:'尚未设置')}${display('软件版本',a.version)}${display('注册时间',tformat(a.registeredAt))}${display('最新心跳',tformat(a.lastSeen))}${display('配置版本',String(a.revision))}</section>
 <section class="drawer-section"><h3>Agent 最近日志</h3>${(a.logs||[]).length?(a.logs||[]).slice(-8).reverse().map(l=>`<div class="log-row"><div class="log-symbol ${e(l.level)}">${I(l.level==='error'?'alert':'activity')}</div><div><strong>${e(l.message)}</strong><time>${tformat(l.at)}</time></div></div>`).join(''):`<p class="muted" style="font-size:11px">暂无远端事件</p>`}</section>
 <section class="drawer-section"><button class="btn btn-danger btn-sm" data-action="delete-agent" data-id="${e(a.id)}">${I('trash')}撤销 Agent 注册</button><p class="muted" style="margin-top:7px;font-size:10px">仅可在远程代理已停止后执行。撤销后该 Agent 将无法继续认证。</p></section></div>
 <div class="drawer-actions"><button class="btn" data-action="download-profile" data-id="${e(a.id)}" ${!c.publicHost?'disabled':''} title="导出适用于 ProxyBridge 的应用名规则">${I('download')}客户端规则</button><button class="btn ${c.enabled?'':'btn-soft'}" data-action="agent-action" data-value="${c.enabled?'stop':'start'}" data-id="${e(a.id)}">${I(c.enabled?'stop':'play')}${c.enabled?'停止代理':'启动代理'}</button><button class="btn" data-action="agent-action" data-value="restart" data-id="${e(a.id)}" ${c.enabled?'':'disabled'}>${I('rotate')}重启</button></div></aside></div>`
}
async function downloadProfile(id){const r=await fetch('/api/agents/'+encodeURIComponent(id)+'/profile',{headers:{Authorization:'Bearer '+state.token},cache:'no-store'});if(!r.ok){const d=await r.json();throw new Error(d.error||'导出失败')};const blob=await r.blob();const url=URL.createObjectURL(blob);const a=document.createElement('a');a.href=url;a.download='proxybridge-'+id+'.pbprofile';document.body.appendChild(a);a.click();a.remove();setTimeout(()=>URL.revokeObjectURL(url),1500);toast('已导出客户端配置，请保护文件内的代理凭据')}
function confirmDanger(message,verb='确认操作'){
 return new Promise(resolve=>{
  const dialog=document.createElement('div');dialog.className='overlay';dialog.setAttribute('data-danger-confirm','true');
  dialog.innerHTML=`<section class="dialog" role="alertdialog" aria-modal="true" aria-labelledby="danger-title" aria-describedby="danger-message"><header class="dialog-head"><h2 id="danger-title">确认危险操作</h2></header><div class="dialog-body"><p id="danger-message">${e(message)}</p></div><footer class="dialog-footer"><button type="button" class="btn" data-confirm-cancel>取消操作</button><button type="button" class="btn btn-danger" data-confirm-proceed>${e(verb)}</button></footer></section>`;
  const finish=value=>{dialog.remove();document.removeEventListener('keydown',onKey,true);resolve(value)};
  const onKey=ev=>{if(ev.key==='Escape'){ev.stopPropagation();finish(false)}};
  dialog.querySelector('[data-confirm-cancel]').addEventListener('click',()=>finish(false));
  dialog.querySelector('[data-confirm-proceed]').addEventListener('click',()=>finish(true));
  dialog.addEventListener('click',ev=>{if(ev.target===dialog)finish(false)});
  document.addEventListener('keydown',onKey,true);document.body.appendChild(dialog);dialog.querySelector('[data-confirm-cancel]').focus();
 });
}
async function handleAction(action,el){const id=el.dataset.id,which=el.dataset.value;
 switch(action){
 case 'refresh':await refresh();return;
 case 'setup':state.page='settings';state.navOpen=false;render();return;
 case 'toggle-nav':state.navOpen=!state.navOpen;render();return;
 case 'logout':state.token='';state.bootstrapToken='';sessionStorage.removeItem('nodeConsoleAuth');renderLogin();return;
 case 'close-overlay':closeOverlay();return;
 case 'new-node':openModal('new-node');return;
 case 'new-enrollment':openModal('new-enrollment');return;
 case 'probe-all':{toast('批量检测中，最多32个节点/次…');const out=await api('/api/nodes/probe-batch','POST',{ids:state.data.nodes.slice(0,32).map(n=>n.id)});const ok=(out.results||[]).filter(x=>x.probe.ok).length;toast(`检测完成：${ok}/${out.count} 个节点通过 SOCKS5 TCP 检测`,ok!==out.count);break}
 case 'edit-node':openModal('edit-node',id);return;
 case 'import':openModal('import');return;
 case 'agent-detail':state.drawer=id;state.clientDetail='';state.modal='';render();return;
 case 'client-detail':state.clientDetail=id;state.modal='';state.drawer='';render();return;
 case 'agent-filter':state.agentFilter=which;render();return;
 case 'copy-code':{const content=decodeURIComponent(el.dataset.code||'');try{await navigator.clipboard.writeText(content);toast('已复制命令')}catch{toast('复制失败，请手动选择文本',true)};return}
 case 'filter-process':state.processFilter=($('#process-filter')?.value||'').trim();await fetchProcesses();render();return;
 case 'refresh-process':await fetchProcesses();render();return;
 case 'download-profile':await downloadProfile(id);return;
 case 'save-enrollment':{const form=$('#enroll-form');if(!form||!form.reportValidity())return;const fd=new FormData(form);const out=await api('/api/enrollments','POST',{label:String(fd.get('label')).trim(),ttlMinutes:Number(fd.get('ttlMinutes'))});state.bootstrapToken=out.token;state.modal='enrollment-issued';await refresh(true);render();return}
 case 'revoke-grant':{if(!await confirmDanger('撤销后这个注册令牌立即失效，无法恢复。','确认撤销令牌'))return;await api('/api/enrollments/'+encodeURIComponent(id),'DELETE');toast('注册密钥已撤销');break}
 case 'mute-alert':{await api('/api/alerts/mute','POST',{id,hours:24});toast('告警已静音 24 小时');break}
 case 'unmute-alert':{await api('/api/alerts/mute/'+encodeURIComponent(id),'DELETE');toast('告警提醒已恢复');break}
 case 'delete-node':{const node=nodeById(id);if(!node)return;if(!await confirmDanger(`删除代理节点「${node.name}」后无法恢复，已规划使用它的客户端需要重新分配。`,`确认删除节点`))return;await api('/api/nodes/'+encodeURIComponent(id),'DELETE');toast('已删除代理节点');break}
 case 'test-node':{toast('正在验证 SOCKS5 TCP 连接…');const p=await api('/api/nodes/'+encodeURIComponent(id)+'/probe','POST',{});toast(p.ok?`TCP 检测通过 · ${p.latencyMs} ms（不代表 UDP 已验证）`:`检测失败：${p.detail}`,!p.ok);break}
 case 'save-client-limit':{
    const cid=state.clientDetail,c=state.data.clients.find(x=>x.id===cid),raw=$('#client-limit')?.value;
    const limit=Number(raw);
    if(!Number.isInteger(limit)||limit<1||limit>128){toast('PID 上限必须是 1～128 的整数',true);return}
    const updated=await api('/api/clients/'+encodeURIComponent(cid)+'/limits','PUT',{maxProxyPids:limit,expectedRevision:c.revision});
    state.data.clients=state.data.clients.map(x=>x.id===cid?updated:x);
    render();toast(`已设置 ${c.name} 最多代理 ${limit} 个 PID`);return;
 }
 case 'auto-client-bindings':{
    const cid=state.clientDetail,c=state.data.clients.find(x=>x.id===cid);
    if(!c)return;
    if([...document.querySelectorAll('[data-client-bind]')].some(el=>el.value!==((c.clientBindings||[]).find(b=>b.pid===Number(el.dataset.pid)&&b.startTime===el.dataset.start)?.nodeId||''))){toast('请先保存手动修改的绑定，再执行自动分配',true);return}
    const result=await api('/api/clients/'+encodeURIComponent(cid)+'/auto-assign','POST',{expectedRevision:c.revision});
    state.data.clients=state.data.clients.map(x=>x.id===cid?result.client:x);
    render();toast(result.added?`已新增 ${result.added} 条 PID 规划，等待客户端确认策略`:'没有新增绑定：机器配额已满或节点容量不足');return;
 }
 case 'save-client-bindings':{
    const cid=state.clientDetail,c=state.data.clients.find(x=>x.id===cid),sel=[...document.querySelectorAll('[data-client-bind]')];
    const bindings=sel.filter(x=>x.value).map(x=>({pid:Number(x.dataset.pid),startTime:x.dataset.start,nodeId:x.value}));
    if(bindings.length>(c.maxProxyPids||8)){toast(`超出当前机器 PID 配额 ${c.maxProxyPids||8}，请减少绑定`,true);return}
    const updated=await api('/api/clients/'+encodeURIComponent(cid)+'/bindings','PUT',{bindings,expectedRevision:c.revision});
    state.data.clients=state.data.clients.map(x=>x.id===cid?updated:x);
    render();toast('进程绑定已保存，等待客户端回报实际执行结果');return;
 }
 case 'delete-client':{
    const c=state.data.clients.find(c=>c.id===id);if(!c)return;
    if(!await confirmDanger(`注销 Windows Client「${c.name}」后其令牌立即失效。请先停止该客户端的代理服务。`,`确认注销客户端`))return;
    await api('/api/clients/'+encodeURIComponent(id),'DELETE');state.clientDetail='';toast('Windows Client 已注销');break;
 }
 case 'delete-agent':{const a=agentById(id);if(!a)return;if(!await confirmDanger(`撤销「${a.name}」的 Agent 凭据后立即失效，无法恢复。请先停止代理服务。`,`确认撤销 Agent`))return;await api('/api/agents/'+encodeURIComponent(id),'DELETE');state.drawer='';toast('Agent 注册已撤销');break}
 case 'agent-action':{await api('/api/agents/'+encodeURIComponent(id)+'/action','POST',{action:which});toast('操作指令已提交，等待 Agent 下次心跳生效');break}
 case 'batch-start':case 'batch-stop':{const ids=[...state.selected];if(!ids.length)return;const act=action==='batch-start'?'start':'stop';const out=await api('/api/agents/batch','POST',{ids,action:act});const ok=(out.results||[]).filter(x=>x.status==='ok').length;toast(`已提交 ${ok}/${ids.length} 台 Agent 的${act==='start'?'启动':'停止'}指令`,ok!==ids.length);state.selected.clear();break}
 case 'save-node':{
 const form=$('#node-form');if(!form||!form.reportValidity())return;const v=new FormData(form),get=k=>String(v.get(k)||'').trim();const expires=get('expiresAt');const input={name:get('name'),host:get('host'),port:Number(get('port')),username:get('username'),password:String(v.get('password')||''),provider:get('provider'),region:get('region'),capacity:Number(get('capacity')),enabled:v.has('enabled'),expiresAt:expires?new Date(expires).toISOString():''};const nodeID=form.dataset.id;await api(nodeID?'/api/nodes/'+nodeID:'/api/nodes',nodeID?'PUT':'POST',input);state.modal='';toast(nodeID?'节点配置已更新':'代理节点已创建');break
 }
 case 'save-import':{const text=$('#import-text')?.value||'';const result=await api('/api/nodes/import','POST',{text});state.modal='';toast(`成功导入 ${result.imported} 个节点，跳过 ${result.skipped} 个重复节点`);break}
 case 'save-agent':{
 const form=$('#agent-form');if(!form||!form.reportValidity())return;const f=new FormData(form),val=k=>String(f.get(k)||'').trim();const cidrs=val('allowedCidrs').split(/[\s,;]+/).map(x=>x.trim()).filter(Boolean);const proxy={enabled:f.has('enabled'),listen:val('listen'),publicHost:val('publicHost'),port:Number(val('port')),username:val('username'),password:String(f.get('password')||''),capacity:Number(val('capacity')),maxConnections:Number(val('maxConnections')),udpPortStart:Number(val('udpPortStart')||0),udpPortEnd:Number(val('udpPortEnd')||0),udp:f.has('udp'),region:val('region'),provider:val('provider'),allowedCidrs:cidrs};await api('/api/agents/'+encodeURIComponent(form.dataset.id)+'/config','PUT',{name:val('name'),proxy});toast('配置已保存并排队下发，请查看下一次 Agent 心跳状态');state.drawer='';break
 }
 case 'assign':{const pid=Number(el.dataset.pid),startTime=el.dataset.start,nodeId=$('#assign-'+pid)?.value||'';await api('/api/assignments','POST',{pid,startTime,nodeId,auto:nodeId===''});toast('仅保存规划关系，尚未转发进程流量');break}
 case 'unassign':{await api('/api/assignments?pid='+encodeURIComponent(el.dataset.pid)+'&startTime='+encodeURIComponent(el.dataset.start),'DELETE');toast('规划绑定已移除');break}
 default:return;
 }
 await refresh();}

// Event delegation keeps overlays, drawers, and table actions consistent after refresh.
document.addEventListener('click',async ev=>{
 const overlay=ev.target.closest('[data-overlay]');if(overlay&&ev.target===overlay){closeOverlay();return}
 const el=ev.target.closest('[data-action],[data-page]');if(!el||state.busy)return;
 if(el.dataset.page){state.page=el.dataset.page;state.filter='';state.navOpen=false;state.selected.clear();if(state.page==='processes'){await fetchProcesses()}render();return}
 state.busy=true;try{await handleAction(el.dataset.action,el)}catch(err){if(state.token)toast(err.message,true)}finally{state.busy=false}
});
document.addEventListener('submit',async ev=>{if(ev.target.id!=='login-form')return;ev.preventDefault();const form=ev.target,token=String(new FormData(form).get('token')||'').trim();const btn=form.querySelector('button[type=submit]');btn.disabled=true;await connect(token)});
document.addEventListener('change',ev=>{
 if(ev.target.matches('[data-client-bind],#client-limit')){
    const c=(state.data.clients||[]).find(x=>x.id===state.clientDetail);
    if(c){
      const n=[...document.querySelectorAll('[data-client-bind]')].filter(x=>x.value).length;
      const quota=Number($('#client-limit')?.value||c.maxProxyPids||8);
      const hint=$('#client-quota-preview');
      if(hint){hint.textContent=`本次选择 ${n} / ${quota} 个 PID ${n>quota?'· 超出上限，请取消部分绑定':'· 尚未保存'}`;hint.style.color=n>quota?'var(--rose)':''}
      const save=document.querySelector('[data-action="save-client-bindings"]');
      if(save)save.disabled=n>(c.maxProxyPids||8);
    }
    return;
 }
 const el=ev.target;if(el.dataset.check==='agent'){if(el.checked)state.selected.add(el.value);else state.selected.delete(el.value);render()}else if(el.dataset.check==='all'){const all=state.data.agents.filter(a=>`${a.name} ${a.hostname} ${a.observedIp}`.toLowerCase().includes(state.filter.toLowerCase()));if(el.checked)all.forEach(a=>state.selected.add(a.id));else state.selected.clear();render()}});
document.addEventListener('input',ev=>{const el=ev.target;if(el.dataset.filter){state.filter=el.value;const start=el.selectionStart,end=el.selectionEnd;render();const el2=$('[data-filter="'+el.dataset.filter+'"]');if(el2){el2.focus();el2.setSelectionRange(start,end)}}});
document.addEventListener('keydown',ev=>{if(ev.key==='Escape'&&(state.modal||state.drawer)){closeOverlay()};if(ev.key==='Enter'&&ev.target.id==='process-filter'){ev.preventDefault();state.processFilter=ev.target.value.trim();fetchProcesses().then(render)}});
if(state.token){refresh(false)}else{renderLogin()}
setInterval(()=>{if(state.token&&document.visibilityState==='visible'&&!state.busy)refresh(true)},9000);
