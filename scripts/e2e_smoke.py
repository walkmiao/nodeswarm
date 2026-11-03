#!/usr/bin/env python3
"""Controller + real Cloud Agent + actual SOCKS5 socket end-to-end smoke.
No external internet target is required: it validates authentication,
SOCKS5 UDP-associate negotiation, and denial of private destinations.
"""
import json, os, pathlib, signal, socket, subprocess, tempfile, time, urllib.error, urllib.request
ROOT=pathlib.Path(__file__).resolve().parents[1]
BIN=str(ROOT/'node-console-linux-amd64')

def free_udp_port():
    with socket.socket(socket.AF_INET,socket.SOCK_DGRAM) as s:
        s.bind(('127.0.0.1',0))
        return s.getsockname()[1]

def free_port():
    with socket.socket() as s:s.bind(('127.0.0.1',0));return s.getsockname()[1]

def http(base,path,method='GET',token='',payload=None):
    req=urllib.request.Request(base+path,method=method,headers={'Authorization':'Bearer '+token,'Content-Type':'application/json'},data=json.dumps(payload).encode() if payload is not None else None)
    try:
        with urllib.request.urlopen(req,timeout=5) as r:return r.status,json.load(r)
    except urllib.error.HTTPError as e:
        return e.code,json.loads(e.read())

def run():
    with tempfile.TemporaryDirectory(prefix='nodeconsole-e2e-') as tmp:
        base=pathlib.Path(tmp);controllerDir=base/'controller';agentDir=base/'agent';port=free_port();socksPort=free_port();udpPort=free_udp_port();url=f'http://127.0.0.1:{port}';ps=[]
        try:
            control_env={**os.environ,'NODE_MANAGER_DATA':str(controllerDir)}
            proc=subprocess.Popen([BIN,'-mode','controller','-listen',f'127.0.0.1:{port}'],env=control_env,stdout=open('/tmp/nc-e2e-controller.log','w'),stderr=subprocess.STDOUT);ps.append(proc)
            for _ in range(30):
                try:
                    if http(url,'/api/health')[0]==200:break
                except Exception:time.sleep(.2)
            else:raise AssertionError('controller did not start')
            admin=(controllerDir/'admin.token').read_text().strip()
            code,issued=http(url,'/api/enrollments','POST',admin,{'label':'e2e-one-time-agent','ttlMinutes':60})
            assert code==201 and len(issued['token'])>=48
            enroll=issued['token']
            code,listed=http(url,'/api/enrollments',token=admin)
            assert code==200 and len(listed['enrollments'])==1 and enroll not in json.dumps(listed)
            assert http(url,'/api/agents')[0]==401,'unprotected admin API'
            agent_env={**os.environ,'NODE_MANAGER_CONTROLLER':url,'NODE_MANAGER_ENROLL_KEY':enroll,'NODE_MANAGER_AGENT_NAME':'E2E-Cloud-01','NODE_MANAGER_AGENT_DATA':str(agentDir)}
            agent=subprocess.Popen([BIN,'-mode','agent'],env=agent_env,stdout=open('/tmp/nc-e2e-agent.log','w'),stderr=subprocess.STDOUT);ps.append(agent)
            def wait_for(pred,timeout=25):
                until=time.time()+timeout
                lastcheck=0
                while time.time()<until:
                    r=http(url,'/api/dashboard',token=admin)[1]
                    if pred(r):return r
                    if time.time()-lastcheck>5:
                        a=r['agents'][0] if r['agents'] else None
                        print('WAIT', 'rev', a.get('revision') if a else None, 'runtime', a.get('runtime') if a else None, flush=True)
                        lastcheck=time.time()
                    time.sleep(.4)
                raise AssertionError('timed out waiting for Agent state')
            d=wait_for(lambda x:len(x['agents'])==1 and x['agents'][0]['status']=='online')
            agentID=d['agents'][0]['id'];print('PASS one-time enrollment + independent Agent auth')
            assert http(url,'/api/enrollments',token=admin)[1]['enrollments']==[], 'grant not consumed'
            code,_=http(url,'/api/agent/enroll','POST',enroll,{'name':'e2e-replay'})
            assert code==401,'single-use grant can be replayed'
            cfg={'name':'E2E-Cloud-01','proxy':{'enabled':True,'listen':'127.0.0.1','publicHost':'127.0.0.1','port':socksPort,'username':'node','password':'e2e_long_strong_password','capacity':8,'maxConnections':32,'udp':True,'udpPortStart':udpPort,'udpPortEnd':udpPort,'region':'QA','provider':'self-hosted','allowedCidrs':['127.0.0.1/32']}}
            code,res=http(url,'/api/agents/'+agentID+'/config','PUT',admin,cfg);assert code==200,(code,res);assert 'password' not in res['desired']
            d=wait_for(lambda x:x['agents'][0]['runtime']['status']=='running' and x['agents'][0]['runtime']['revision']==1)
            assert any(n.get('managedAgentId')==agentID for n in d['nodes'])
            print('PASS controller config -> Agent SOCKS5 startup + inventory sync')
            with socket.create_connection(('127.0.0.1',socksPort),timeout=3) as s:
                s.settimeout(3);s.sendall(b'\x05\x01\x02');assert s.recv(2)==b'\x05\x02'
                user=b'node';password=b'e2e_long_strong_password'
                s.sendall(bytes([1,len(user)])+user+bytes([len(password)])+password);assert s.recv(2)==b'\x01\x00'
                s.sendall(bytes([5,1,0,1,127,0,0,1,0,80]));ans=s.recv(10);assert ans[1]==2,ans
            with socket.create_connection(('127.0.0.1',socksPort),timeout=3) as s:
                s.settimeout(3);s.sendall(b'\x05\x01\x02');assert s.recv(2)==b'\x05\x02'
                user=b'node';password=b'WRONG'
                s.sendall(bytes([1,len(user)])+user+bytes([len(password)])+password);assert s.recv(2)==b'\x01\x01'
            print('PASS SOCKS5 authenticated handshake / wrong password / private target blocked')
            with socket.create_connection(('127.0.0.1',socksPort),timeout=3) as s:
                s.settimeout(3);s.sendall(b'\x05\x01\x02');assert s.recv(2)==b'\x05\x02';u=b'node';p=b'e2e_long_strong_password';s.sendall(bytes([1,len(u)])+u+bytes([len(p)])+p);assert s.recv(2)==b'\x01\x00'
                s.sendall(bytes([5,3,0,1,0,0,0,0,0,0]));reply=s.recv(32);assert reply[0:2]==b'\x05\x00',reply;assert int.from_bytes(reply[-2:],'big')==udpPort,(reply,udpPort)
                print('PASS SOCKS5 UDP ASSOCIATE with fixed cloud-firewall port pool')
            code,_=http(url,'/api/agents/'+agentID+'/action','POST',admin,{'action':'stop'});assert code==200
            d=wait_for(lambda x:x['agents'][0]['runtime']['status']=='stopped' and x['agents'][0]['runtime']['revision']==2)
            try:socket.create_connection(('127.0.0.1',socksPort),timeout=1);raise AssertionError('stopped SOCKS still listening')
            except (ConnectionRefusedError,TimeoutError,OSError):pass
            code,_=http(url,'/api/agents/'+agentID+'/action','POST',admin,{'action':'start'});assert code==200
            wait_for(lambda x:x['agents'][0]['runtime']['status']=='running' and x['agents'][0]['runtime']['revision']==3)
            code,_=http(url,'/api/agents/'+agentID+'/action','POST',admin,{'action':'restart'});assert code==200
            wait_for(lambda x:x['agents'][0]['runtime']['status']=='running' and x['agents'][0]['runtime']['revision']==4)
            print('PASS remote stop / start / restart and reported revision')
            code,alerts=http(url,'/api/alerts',token=admin)
            assert code==200 and 'active' in alerts and isinstance(alerts['alerts'],list)
            print('PASS authenticated fleet alert endpoint')
            code,profile=http(url,'/api/agents/'+agentID+'/profile?process=Client.exe',token=admin)
            assert code==200 and profile['ProxyRules'][0]['ProcessName']=='Client.exe' and profile['ProxyConfigs'][0]['Host']=='127.0.0.1'
            print('PASS ProxyBridge process-name profile generation')
            return True
        finally:
            for p in reversed(ps):
                p.terminate()
                try:p.wait(timeout=3)
                except subprocess.TimeoutExpired:p.kill()

if __name__=='__main__':
    run();print('ALL END-TO-END TESTS PASS')
