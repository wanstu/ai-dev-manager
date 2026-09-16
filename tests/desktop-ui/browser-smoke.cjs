const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {pathToFileURL} = require('node:url');
const {spawnSync} = require('node:child_process');

const root = path.resolve(__dirname, '..', '..');
const frontend = path.join(root, 'cmd', 'ai-dev-manager-desktop', 'frontend');
const productionHTML = fs.readFileSync(path.join(frontend, 'index.html'), 'utf8');
const browserCandidates = [
  path.join(process.env.ProgramFiles || '', 'Google', 'Chrome', 'Application', 'chrome.exe'),
  path.join(process.env['ProgramFiles(x86)'] || '', 'Microsoft', 'Edge', 'Application', 'msedge.exe'),
  path.join(process.env.ProgramFiles || '', 'Microsoft', 'Edge', 'Application', 'msedge.exe'),
].filter(Boolean);
const browser = browserCandidates.find((candidate) => fs.existsSync(candidate));
if (!browser) throw new Error('No installed Chromium browser found for Desktop UI smoke');

const fakeBridge = String.raw`<script>
(() => {
  const calls = [];
  const confirmations = [];
  const browserErrors = [];
  window.addEventListener('error', (event) => browserErrors.push(event.error?.stack || event.message || String(event.error || 'error')));
  window.addEventListener('unhandledrejection', (event) => browserErrors.push(event.reason?.stack || event.reason?.message || String(event.reason || 'rejection')));
  let activeID = 'profile-a';
  const profiles = [
    {id: 'profile-a', name: 'Profile A', base_url: 'http://127.0.0.1:43137', start_service_on_desktop_launch: true},
    {id: 'profile-b', name: 'Profile B', base_url: 'http://127.0.0.1:43138'},
  ];
  const longRoot = 'C:\\fixtures\\' + 'very-long-segment-'.repeat(18) + 'workspace-a';
  const snapshotA = {
    workspaces: [
      {workspace_id:'ws-a', name:'Workspace A', path:longRoot, enabled_mcp_ids:[], enabled_skill_ids:[]},


      {workspace_id:'ws-b', name:'Workspace B', path:'C:\\fixtures\\plain-non-git-workspace-b', enabled_mcp_ids:[], enabled_skill_ids:[]},
    ],
    environments: [
      {environment_id:'env-a', workspace_id:'ws-a', name:'Environment A', root:longRoot, state:'ready', writer:{owner:'writer-a'}, enabled_mcp_ids:['mcp-a'], explicit_mcp_ids:['mcp-a'], inherited_mcp_ids:[], enabled_skill_ids:['skill-a','skill-broken'], explicit_skill_ids:['skill-a','skill-broken'], inherited_skill_ids:[], private_memory_count:1, retention:{persistence:'temporary', creator_surface:'agent', owner_id:'lifecycle-owner-a', session_id:'session-a', run_id:'run-provenance-a', created_at:'2026-09-12T00:00:00Z', expires_at:'2026-09-12T01:00:00Z'}},
      {environment_id:'env-b', workspace_id:'ws-b', name:'Environment B', root:'C:\\fixtures\\plain-non-git-workspace-b', state:'ready', writer:null, enabled_mcp_ids:[], explicit_mcp_ids:[], inherited_mcp_ids:[], enabled_skill_ids:[], explicit_skill_ids:[], inherited_skill_ids:[], private_memory_count:0, retention:{persistence:'durable', creator_surface:'core', created_at:'2026-09-12T00:00:00Z'}},

    ],
    worktree_settings: {root:'C:\\fixtures\\adm-worktrees', branch_prefix:'adm/', workspace_id:''},
    host_environment: {source:'process_environment', last_refresh_status:'not_refreshed', variable_count:42},
    allowed_executables: ['go'],
    exec_denials: [
      {executable:'blocked_python_fixture', count:5, first_blocked_at:'2026-09-12T03:00:00Z', last_blocked_at:'2026-09-12T04:00:00Z', last_environment_id:'env-a', last_surface:'run_start', last_reason:'executable "blocked_python_fixture" is not allowed'},
      {executable:'blocked_node_fixture', count:2, first_blocked_at:'2026-09-12T02:00:00Z', last_blocked_at:'2026-09-12T03:30:00Z', last_environment_id:'env-b', last_surface:'process_start', last_reason:'executable "blocked_node_fixture" is not allowed'},
    ],
    mcps: [
      {id:'mcp-a', name:'MCP A', transport:'streamable-http', endpoint:'http://127.0.0.1:9900/mcp', default_include_in_environment:false, health_policy:{}},
      {id:'mcp-b', name:'MCP B', transport:'streamable-http', endpoint:'http://127.0.0.1:9901/mcp', auth_mode:'none', header_refs:{Authorization:'Bearer \${MCP_HUB_TOKEN}'}, default_include_in_environment:false, health_policy:{}},
    ],
    skills: [
      {id:'skill-a', name:'Skill A', source_id:'source-a', source_root:'C:\\fixtures\\skills', artifact_path:'C:\\fixtures\\skills\\skill-a\\SKILL.md', relative_artifact_path:'skill-a/SKILL.md', support_roots:[], default_include_in_environment:false},
      {id:'skill-idle', name:'Idle Skill', source_id:'source-a', source_root:'C:\\fixtures\\skills', artifact_path:'C:\\fixtures\\skills\\idle\\SKILL.md', relative_artifact_path:'idle/SKILL.md', support_roots:[], default_include_in_environment:false},
      {id:'skill-broken', name:'Broken Skill', source_id:'source-a', source_root:'C:\\fixtures\\skills', artifact_path:'C:\\fixtures\\skills\\broken\\SKILL.md', relative_artifact_path:'broken/SKILL.md', support_roots:[], default_include_in_environment:false},
      {id:'skill-legacy', name:'Legacy Skill', source_id:'', source_root:'', artifact_path:'', relative_artifact_path:'', support_roots:[], default_include_in_environment:false},
      {id:'skill-legacy-2', name:'Legacy Skill 2', source_id:'', source_root:'', artifact_path:'', relative_artifact_path:'', support_roots:[], default_include_in_environment:false},
    ],
    global_memory_count: 1,
  };
  const snapshotB = {
    workspaces: [{workspace_id:'ws-profile-b', name:'Workspace Profile B', path:'C:\\fixtures\\profile-b', enabled_mcp_ids:[], enabled_skill_ids:[]}],
    environments: [{environment_id:'env-profile-b', workspace_id:'ws-profile-b', name:'Environment Profile B', root:'C:\\fixtures\\profile-b', state:'ready', writer:null, enabled_mcp_ids:[], explicit_mcp_ids:[], inherited_mcp_ids:[], enabled_skill_ids:[], explicit_skill_ids:[], inherited_skill_ids:[], private_memory_count:0}],
    worktree_settings: {root:'C:\\fixtures\\profile-b-worktrees', branch_prefix:'adm/', workspace_id:''},
    host_environment: {source:'process_environment', last_refresh_status:'not_refreshed', variable_count:8},
    allowed_executables: [], mcps: [], skills: [], global_memory_count: 0,
  };
  const skillSourcesA = [{skill_source_id:'source-a', root:'C:\\fixtures\\skills', support_roots:[], last_refresh_status:'ok'}];
  const environmentMemory = {'env-a':[{key:'private-sentinel', value:'env-a-private-visible'}], 'env-b':[{key:'private-sentinel', value:'env-b-private-visible'}]};
  const state = {failSkillSources:false, failVerifiers:false, failProcesses:false, failWorkspaceAdd:true, failEnvironmentCreate:true, hideProcess:false, delayProcessLogs:false, delayEnvironmentAInspection:false, delayMCPProbe:false, delayDiscovery:false, failDiscovery:false, delayTreeDigest:false, failBulkSkillAssignment:false, workspaceRecommendationApplied:false};
  const record = (name, args) => calls.push({name, args});
  const recomputeEnvironment = (env, snapshot=snapshotA) => {
    if(!env) return env;
    const workspace=snapshot.workspaces.find(item=>item.workspace_id===env.workspace_id) || {};
    const explicitMCP=new Set(env.explicit_mcp_ids || []), explicitSkill=new Set(env.explicit_skill_ids || []);
    env.inherited_mcp_ids=[...(workspace.enabled_mcp_ids || [])].sort(); env.inherited_skill_ids=[...(workspace.enabled_skill_ids || [])].sort();
    env.enabled_mcp_ids=[...new Set([...explicitMCP, ...env.inherited_mcp_ids])].sort(); env.enabled_skill_ids=[...new Set([...explicitSkill, ...env.inherited_skill_ids])].sort();
    return env;
  };
  const recomputeWorkspaceEnvironments = (workspaceID) => { for(const env of snapshotA.environments) if(env.workspace_id===workspaceID) recomputeEnvironment(env); };
  const temporaryStatus = (id) => {
    const env=snapshotA.environments.find(item=>item.environment_id===id); if(!env) throw new Error('env not found: '+id);
    const retention=env.retention || {}; const temporary=retention.persistence==='temporary'; const expired=Boolean(retention.expires_at && Date.parse(retention.expires_at)<=Date.now()); const blockers=[]; if(env.writer?.owner) blockers.push('active_writer');
    return {environment_id:id, workspace_id:env.workspace_id, name:env.name, root:env.root, managed_worktree:false, retention:structuredClone(retention), cleanup_state:temporary?(blockers.length?'blocked':expired?'eligible':'not_due'):'durable', cleanup_eligible:Boolean(temporary && expired && blockers.length===0), blockers, uncertainties:[]};
  };
  const adapter = {
    async GetConnectionProfiles(){ record('GetConnectionProfiles',[]); return {profiles, active_id:activeID}; },
    async SelectConnectionProfile(id){ record('SelectConnectionProfile',[id]); activeID=id; return {profiles, active_id:activeID}; },
    async SaveConnectionProfile(profile){ record('SaveConnectionProfile',[profile]); return {profiles, active_id:activeID}; },
    async DeleteConnectionProfile(id){ record('DeleteConnectionProfile',[id]); return {profiles, active_id:activeID}; },
    async DisconnectADM(){ record('DisconnectADM',[]); return true; },
    async ConnectADM(input){ record('ConnectADM',[input]); const base=profiles.find(p=>p.id===activeID)?.base_url || input?.base_url || ''; return {state:'running', base_url:base, health_url:base+'/healthz', agent_mcp_url:base+'/mcp', admin_mcp_url:base+'/admin/mcp', pid:1234, version:'browser-fixture', local_bootstrap_eligible:false}; },
    async StartLocalADM(input){ record('StartLocalADM',[input]); const base=profiles.find(p=>p.id===activeID)?.base_url || input?.base_url || ''; return {state:'running', base_url:base, health_url:base+'/healthz', agent_mcp_url:base+'/mcp', admin_mcp_url:base+'/admin/mcp', pid:4321, version:'browser-fixture', local_bootstrap_eligible:true}; },
    async GetSnapshot(){ record('GetSnapshot',[]); return activeID==='profile-b' ? structuredClone(snapshotB) : structuredClone(snapshotA); },
    async DiscoverWorkspace(id,input){
      record('DiscoverWorkspace',[id,input]); const delayed=state.delayDiscovery; if(delayed) await new Promise(resolve=>setTimeout(resolve,180)); if(state.failDiscovery) throw new Error('fixture discovery failure');
      const limits={max_depth:input?.max_depth||4,max_entries:input?.max_entries||2000,max_candidates:input?.max_candidates||50,max_digest_entries:input?.max_digest_entries||100,max_output_bytes:input?.max_output_bytes||65536};
      const observed_at='2026-09-12T05:00:00Z'; const base={scope:{workspace_id:id},scan_path:input?.path||'.',query:input?.query||'',observed_at,limits,visited_entries:7,read_batches:2,digest:[{path:'.',kind:'directory',observed_children:3,observed_markers:1,children_complete:true},{path:'apps/p2',kind:'directory',observed_children:2,observed_markers:1,children_complete:true}],truncated:false,stop_reasons:[],diagnostics:[],excluded_directories:['node_modules'],skipped_directories:1,skipped_links:0,omitted_candidates:0,omitted_digest_entries:0,omitted_markers:0,omitted_diagnostics:0,coverage:'complete_under_exclusion_policy'};
      if(input?.path==='empty') return {...base,candidates:[],digest:[],visited_entries:0};
      if(input?.query==='nomatch') return {...base,candidates:[]};
      if(input?.max_entries===1) return {...base,candidates:[{root:'apps/p2',name:'P2 '+('very-long-name-'.repeat(8)),suggested_environment_root:'apps/p2',evidence:'marker',query_match:'none',markers:[{path:'apps/p2/package.json',kind:'node'}]}],truncated:true,stop_reasons:['entry_limit'],coverage:'partial; unvisited entries may depend on filesystem enumeration order',omitted_digest_entries:2};
      if(id==='ws-b') return {...base,candidates:[{root:'service-b',name:'Service B',suggested_environment_root:'service-b',evidence:'marker',query_match:'none',markers:[{path:'service-b/go.mod',kind:'go'}]}]};
      return {...base,candidates:[{root:'.',name:'Workspace A root',suggested_environment_root:'.',evidence:'marker',query_match:'none',markers:[{path:'package.json',kind:'node'}]},{root:'apps/p2',name:'P2 '+('very-long-name-'.repeat(8)),suggested_environment_root:'apps/p2',evidence:'marker',query_match:'none',markers:[{path:'apps/p2/package.json',kind:'node'}]},{root:'docs',name:'Docs markerless',suggested_environment_root:'docs',evidence:'directory-only',query_match:'none',markers:[]}]};
    },
    async EnvironmentTreeDigest(id,input){ record('EnvironmentTreeDigest',[id,input]); const delayed=state.delayTreeDigest; if(delayed) await new Promise(resolve=>setTimeout(resolve,180)); return {scope:{environment_id:id},scan_path:input?.path||'.',query:'',observed_at:'2026-09-12T05:05:00Z',limits:{max_depth:4,max_entries:2000,max_candidates:50,max_digest_entries:100,max_output_bytes:65536},visited_entries:4,read_batches:1,candidates:[],digest:[{path:'.',kind:'directory',observed_children:2,observed_markers:1,children_complete:true},{path:'src',kind:'directory',observed_children:1,observed_markers:0,children_complete:true}],truncated:false,stop_reasons:[],diagnostics:[],excluded_directories:['node_modules'],skipped_directories:0,skipped_links:0,omitted_candidates:0,omitted_digest_entries:0,omitted_markers:0,omitted_diagnostics:0,coverage:'complete_under_exclusion_policy'}; },
    async AddWorkspace(input){ record('AddWorkspace',[input]); if(state.failWorkspaceAdd) throw new Error('fixture workspace save failure'); const snapshot=activeID==='profile-b'?snapshotB:snapshotA; const workspace={workspace_id:'ws-added-specific', name:input?.name||'Added Workspace', path:input?.path||'', enabled_mcp_ids:[], enabled_skill_ids:[]}; snapshot.workspaces.push(workspace); return structuredClone(workspace); },
    async CreateEnvironment(input){ record('CreateEnvironment',[input]); if(state.failEnvironmentCreate) throw new Error('fixture environment create failure'); const snapshot=activeID==='profile-b'?snapshotB:snapshotA; const workspace=snapshot.workspaces.find(item=>item.workspace_id===input?.workspace_id); if(!workspace) throw new Error('workspace not found: '+input?.workspace_id); const env={environment_id:'env-created-drift', workspace_id:workspace.workspace_id, name:input?.name||'Created Environment', root:input?.root||workspace.path, state:'ready', writer:null, enabled_mcp_ids:[], explicit_mcp_ids:[], inherited_mcp_ids:[], enabled_skill_ids:[], explicit_skill_ids:[], inherited_skill_ids:[], private_memory_count:0, retention:{persistence:'durable', creator_surface:'desktop', created_at:'2026-09-16T00:00:00Z'}}; snapshot.environments.push(env); return structuredClone(env); },
    async EnvironmentWorkspaceOptions(id){ record('EnvironmentWorkspaceOptions',[id]); const snapshot=activeID==='profile-b'?snapshotB:snapshotA; const env=snapshot.environments.find(item=>item.environment_id===id); if(!env) throw new Error('env not found: '+id); const norm=(value)=>String(value||'').replace(/\\/g,'/').replace(/\/+$/,'').toLowerCase(); const root=norm(env.root); const candidates=snapshot.workspaces.filter(ws=>{ const base=norm(ws.path); return root===base || root.startsWith(base+'/'); }).sort((a,b)=>norm(b.path).length-norm(a.path).length); return {environment_id:id,current_workspace_id:env.workspace_id,recommended_workspace_id:candidates[0]?.workspace_id||'',candidates:structuredClone(candidates),rebind_allowed:true,managed_worktree:false,reason:''}; },
    async EnvironmentWorkspaceRecommendations(){ record('EnvironmentWorkspaceRecommendations',[]); if(activeID==='profile-b') return []; const items=[]; if(!state.workspaceRecommendationApplied) items.push({environment_id:'env-recommendation',environment_name:'Environment Recommendation',root:'C:\\fixtures\\recommendation\\project\\service',current_workspace:{workspace_id:'ws-recommendation-current',name:'Workspace Recommendation Current',path:'C:\\fixtures\\recommendation'},recommended_workspace:{workspace_id:'ws-recommendation-target',name:'Workspace Recommendation Target',path:'C:\\fixtures\\recommendation\\project'}}); const added=snapshotA.workspaces.find(item=>item.workspace_id==='ws-added-specific'); const existing=snapshotA.environments.find(item=>item.environment_id==='env-existing-workspace-drift'); if(added && existing && existing.workspace_id!==added.workspace_id){ const current=snapshotA.workspaces.find(item=>item.workspace_id===existing.workspace_id); if(current) items.push({environment_id:existing.environment_id,environment_name:existing.name,root:existing.root,current_workspace:structuredClone(current),recommended_workspace:structuredClone(added)}); } return items; },
    async SetEnvironmentWorkspace(id,workspaceID){ record('SetEnvironmentWorkspace',[id,workspaceID]); if(id==='env-recommendation' && workspaceID==='ws-recommendation-target'){ state.workspaceRecommendationApplied=true; return {environment_id:id,workspace_id:workspaceID,name:'Environment Recommendation',root:'C:\\fixtures\\recommendation\\project\\service',state:'ready',enabled_mcp_ids:[],explicit_mcp_ids:[],inherited_mcp_ids:[],enabled_skill_ids:[],explicit_skill_ids:[],inherited_skill_ids:[],private_memory_count:0}; } const snapshot=activeID==='profile-b'?snapshotB:snapshotA; const env=snapshot.environments.find(item=>item.environment_id===id); if(!env) throw new Error('env not found: '+id); if(!snapshot.workspaces.some(item=>item.workspace_id===workspaceID)) throw new Error('workspace not found: '+workspaceID); env.workspace_id=workspaceID; recomputeEnvironment(env,snapshot); return structuredClone(env); },
    async GetTemporaryEnvironmentStatus(id){ record('GetTemporaryEnvironmentStatus',[id]); return structuredClone(temporaryStatus(id)); },
    async PromoteTemporaryEnvironment(id,ownerID){ record('PromoteTemporaryEnvironment',[id,ownerID]); const env=snapshotA.environments.find(item=>item.environment_id===id); if(!env) throw new Error('env not found: '+id); if(env.retention?.persistence!=='temporary') throw new Error('environment is not temporary'); if(env.retention?.owner_id!==ownerID) throw new Error('wrong lifecycle owner'); env.retention={persistence:'durable', creator_surface:env.retention.creator_surface || 'agent', created_at:env.retention.created_at || '2026-09-12T00:00:00Z'}; return structuredClone(temporaryStatus(id)); },
    async CleanupTemporaryEnvironment(id,ownerID,execute){ record('CleanupTemporaryEnvironment',[id,ownerID,execute]); const env=snapshotA.environments.find(item=>item.environment_id===id); if(!env) throw new Error('env not found: '+id); if(execute && env.retention?.owner_id!==ownerID) throw new Error('wrong lifecycle owner'); const status=temporaryStatus(id); const item={kind:'environment', id, retention:structuredClone(status.retention), cleanup_state:status.cleanup_state, cleanup_eligible:status.cleanup_eligible, blockers:structuredClone(status.blockers), uncertainties:[]}; const result={dry_run:!execute, report:{generated_at:'2026-09-13T00:00:00Z', resources:[item]}, would_remove:status.cleanup_eligible?[{kind:'environment',id,action:'remove'}]:[], removed:[], skipped:status.cleanup_eligible?[]:[item]}; if(execute){ if(!status.cleanup_eligible) throw new Error('cleanup blocked: '+(status.blockers.join(', ')||status.cleanup_state)); snapshotA.environments=snapshotA.environments.filter(item=>item.environment_id!==id); result.dry_run=false; result.removed=[{kind:'environment',id,action:'remove'}]; result.would_remove=[]; } return structuredClone(result); },
    async AllowExecutable(executable){ record('AllowExecutable',[executable]); if(!snapshotA.allowed_executables.includes(executable)) snapshotA.allowed_executables.push(executable); snapshotA.exec_denials=snapshotA.exec_denials.filter(item=>item.executable!==executable); return structuredClone(snapshotA.allowed_executables); },
    async RemoveExecutable(executable){ record('RemoveExecutable',[executable]); snapshotA.allowed_executables=snapshotA.allowed_executables.filter(item=>item!==executable); return structuredClone(snapshotA.allowed_executables); },
    async ClearExecDenial(executable){ record('ClearExecDenial',[executable]); snapshotA.exec_denials=snapshotA.exec_denials.filter(item=>item.executable!==executable); return structuredClone(snapshotA.exec_denials); },
    async ClearAllExecDenials(){ record('ClearAllExecDenials',[]); snapshotA.exec_denials=[]; return null; },
    async ListSkillSources(){ record('ListSkillSources',[]); if(state.failSkillSources) throw new Error('skill sources unavailable'); return activeID==='profile-b' ? [] : structuredClone(skillSourcesA); },
    async UpdateSkillSource(id,input){ record('UpdateSkillSource',[id,input]); const source=skillSourcesA.find(item=>item.skill_source_id===id); if(!source) throw new Error('source not found: '+id); source.root=input.root; source.support_roots=input.support_roots || []; source.default_include_in_environment=Boolean(input.default_include_in_environment); source.last_refresh_status='pending'; source.last_refresh_error='source settings changed; refresh required'; return structuredClone(source); },
    async InspectEnvironment(id){ record('InspectEnvironment',[id]); if(state.delayEnvironmentAInspection && id==='env-a') await new Promise(resolve=>setTimeout(resolve,180)); const snapshot=activeID==='profile-b'?snapshotB:snapshotA; const env=snapshot.environments.find(e=>e.environment_id===id); const workspace=snapshot.workspaces.find(w=>w.workspace_id===env?.workspace_id); const facts=id==='env-a'?[{key:'skill/skill-broken', kind:'skill', state:'unavailable', reason_code:'artifact_missing', message:'fixture artifact missing', source:'fixture capability report', observed_at:'2026-09-12T03:00:00Z'}]:[]; return {environment:structuredClone(env), workspace:structuredClone(workspace), capability_report:{generated_at:'2026-09-11T15:00:00Z', facts}, unresolved_mcp_ids:[], unresolved_skill_ids:[]}; },
    async ListEnvironmentSkills(id){ record('ListEnvironmentSkills',[id]); const snapshot=activeID==='profile-b'?snapshotB:snapshotA; const env=snapshot.environments.find(e=>e.environment_id===id); const enabled=new Set(env?.enabled_skill_ids || []); return {skills:snapshot.skills.map(skill => ({skill_id:skill.id, source_id:skill.source_id || '', enabled:enabled.has(skill.id), state:!enabled.has(skill.id) ? 'disabled' : skill.id==='skill-broken' ? 'artifact_missing' : skill.id==='skill-legacy' ? 'unconfigured' : 'available', reason:skill.id==='skill-broken' ? 'fixture artifact missing' : skill.id==='skill-legacy' ? 'fixture legacy entry is unconfigured' : !enabled.has(skill.id) ? 'skill is not enabled for this Environment' : ''}))}; },
    async ListSkillAvailability(){ record('ListSkillAvailability',[]); const snapshot=activeID==='profile-b'?snapshotB:snapshotA; return {scope:'catalog', skills:snapshot.skills.map(skill => ({skill_id:skill.id, source_id:skill.source_id || '', enabled:true, state:skill.id==='skill-broken' ? 'artifact_missing' : skill.id.startsWith('skill-legacy') ? 'unconfigured' : 'available', reason:skill.id==='skill-broken' ? 'fixture artifact missing' : skill.id.startsWith('skill-legacy') ? 'fixture legacy entry is unconfigured' : ''}))}; },
    async UpdateMCP(id,input){ record('UpdateMCP',[id,input]); const entry=snapshotA.mcps.find(item=>item.id===id); if(!entry) throw new Error('mcp not found: '+id); if(input?.transport==='streamable-http' && input?.auth_mode==='none' && Object.keys(input?.header_refs || {}).length) throw new Error('mcp auth_mode "none" cannot configure header_refs'); Object.assign(entry, structuredClone(input), {id}); return structuredClone(entry); },
    async SetMCPDefault(id,value){ record('SetMCPDefault',[id,value]); const entry=snapshotA.mcps.find(item=>item.id===id); if(!entry) throw new Error('mcp not found: '+id); entry.default_include_in_environment=Boolean(value); return structuredClone(entry); },
    async SetWorkspaceMCP(workspaceID,id,value){ record('SetWorkspaceMCP',[workspaceID,id,value]); const workspace=snapshotA.workspaces.find(item=>item.workspace_id===workspaceID); if(!workspace) throw new Error('workspace not found: '+workspaceID); const set=new Set(workspace.enabled_mcp_ids || []); if(value) set.add(id); else set.delete(id); workspace.enabled_mcp_ids=[...set].sort(); recomputeWorkspaceEnvironments(workspaceID); return structuredClone(workspace); },
    async SetEnvironmentMCP(environmentID,id,value){ record('SetEnvironmentMCP',[environmentID,id,value]); const env=snapshotA.environments.find(item=>item.environment_id===environmentID); if(!env) throw new Error('env not found: '+environmentID); const set=new Set(env.explicit_mcp_ids || []); if(value) set.add(id); else set.delete(id); env.explicit_mcp_ids=[...set].sort(); recomputeEnvironment(env); return structuredClone(env); },
    async PreviewMCPImport(input){ record('PreviewMCPImport',[input]); return {format: input?.format || 'generic-mcpservers', candidates:[{name:'Imported MCP', transport:'streamable-http', endpoint:'http://127.0.0.1:9902/mcp', reference_requirements:[{field_path:'headers.Authorization', reference_name:'ADM_IMPORTED_TOKEN'}], errors:[], warnings:[]}]}; },
    async ApplyMCPImport(input){ record('ApplyMCPImport',[input]); const definition={id:'mcp-imported', name:'Imported MCP', transport:'streamable-http', endpoint:'http://127.0.0.1:9902/mcp', default_include_in_environment:Boolean(input?.default_include), health_policy:{}}; if(!snapshotA.mcps.some(item=>item.id===definition.id)) snapshotA.mcps.push(definition); return {result:{mutations:[{action:'created', definition}]}}; },
    async ProbeMCPHealth(id){ record('ProbeMCPHealth',[id]); if(state.delayMCPProbe) await new Promise(resolve=>setTimeout(resolve,180)); if(state.mcpProbeError==='connection') return {state:'error', error_kind:'connection_failed', message:'MCP 协议连接失败。请检查 Endpoint、Transport、TLS/代理。'}; return state.mcpProbeError ? {state:'configured', error_kind:'unresolved_secret_reference', message:'mcp connection configuration has an unresolved environment reference'} : {state:'healthy', message:'fixture healthy'}; },
    async RemoveMCP(id){ record('RemoveMCP',[id]); const index=snapshotA.mcps.findIndex(item=>item.id===id); if(index>=0) snapshotA.mcps.splice(index,1); for(const workspace of snapshotA.workspaces) workspace.enabled_mcp_ids=(workspace.enabled_mcp_ids||[]).filter(value=>value!==id); for(const env of snapshotA.environments){ env.explicit_mcp_ids=(env.explicit_mcp_ids||[]).filter(value=>value!==id); recomputeEnvironment(env); } return null; },
    async SetSkillDefault(id,value){ record('SetSkillDefault',[id,value]); const entry=snapshotA.skills.find(item=>item.id===id); if(!entry) throw new Error('skill not found: '+id); entry.default_include_in_environment=Boolean(value); return structuredClone(entry); },
    async SetWorkspaceSkill(workspaceID,id,value){ record('SetWorkspaceSkill',[workspaceID,id,value]); const workspace=snapshotA.workspaces.find(item=>item.workspace_id===workspaceID); if(!workspace) throw new Error('workspace not found: '+workspaceID); const set=new Set(workspace.enabled_skill_ids || []); if(value) set.add(id); else set.delete(id); workspace.enabled_skill_ids=[...set].sort(); recomputeWorkspaceEnvironments(workspaceID); return structuredClone(workspace); },
    async SetEnvironmentSkill(environmentID,id,value){ record('SetEnvironmentSkill',[environmentID,id,value]); if(state.failBulkSkillAssignment && environmentID==='env-b' && id==='skill-a') throw new Error('fixture bulk assignment failure'); const env=snapshotA.environments.find(item=>item.environment_id===environmentID); if(!env) throw new Error('env not found: '+environmentID); const set=new Set(env.explicit_skill_ids || []); if(value) set.add(id); else set.delete(id); env.explicit_skill_ids=[...set].sort(); recomputeEnvironment(env); return structuredClone(env); },
    async RemoveSkill(id){ record('RemoveSkill',[id]); const index=snapshotA.skills.findIndex(skill=>skill.id===id); if(index<0) throw new Error('skill not found: '+id); snapshotA.skills.splice(index,1); for(const workspace of snapshotA.workspaces) workspace.enabled_skill_ids=(workspace.enabled_skill_ids||[]).filter(value=>value!==id); for(const env of snapshotA.environments){ env.explicit_skill_ids=(env.explicit_skill_ids||[]).filter(value=>value!==id); recomputeEnvironment(env); } return null; },
    async ListVerifiers(id){ record('ListVerifiers',[id]); if(state.failVerifiers) throw new Error('verifiers unavailable'); return [{verifier_id:id==='env-b'?'verifier-b':'verifier-a', name:id==='env-b'?'Verifier B':'Verifier A', kind:'test', executable:'go', args:['test','./...'], enabled:true}]; },
    async ListProcesses(id){ record('ListProcesses',[id]); if(state.failProcesses) throw new Error('processes unavailable'); if(state.hideProcess && id==='env-a') return []; return [{id:id==='env-b'?'proc-b':'proc-a', state:'running', pid:id==='env-b'?3333:2222, listening_ports:[8080]}]; },
    async GetProcessLogs(id,processID){ record('GetProcessLogs',[id,processID]); if(state.delayProcessLogs && id==='env-a') await new Promise(resolve=>setTimeout(resolve,180)); return {stdout:id==='env-b'?'PROCESS_B_LOG':'PROCESS_A_LOG', stderr:'', stdout_truncated:id==='env-a', stderr_truncated:false}; },
    async ListRuns(id){ record('ListRuns',[id]); return [{id:id==='env-b'?'run-b':'run-a', state:'succeeded', executable:'go', args:['test'], stdout:id==='env-b'?'RUN_B_OUTPUT':'RUN_A_OUTPUT', stderr:'', exit_code:0, stdout_truncated:false, stderr_truncated:false}]; },
    async ListGlobalMemory(){ record('ListGlobalMemory',[]); return [{key:'sentinel', value:'visible-after-explicit-load'}]; },
    async ListEnvironmentMemory(id){ record('ListEnvironmentMemory',[id]); return structuredClone(environmentMemory[id] || []); },
    async WriteEnvironmentMemory(id,key,value){ record('WriteEnvironmentMemory',[id,key,value]); environmentMemory[id] = (environmentMemory[id] || []).filter(entry=>entry.key!==key); environmentMemory[id].push({key,value}); return null; },
    async GetDesktopPreferences(){ record('GetDesktopPreferences',[]); return {launch_at_login_supported:false, launch_at_login:false}; },
    async SetLaunchAtLogin(value){ record('SetLaunchAtLogin',[value]); return {launch_at_login_supported:false, launch_at_login:false}; },
  };
  window.go = {desktop:{Adapter:adapter}};
  window.runtime = {EventsOn(){}, EventsEmit(){}};
  window.confirm = (message) => { confirmations.push(String(message || '')); return true; };
  Object.defineProperty(navigator, 'clipboard', {configurable:true, value:{writeText:async(text)=>{ window.__copiedText=String(text || ''); }}});
  window.__fakeADM = {calls, confirmations, state, browserErrors, snapshotA, recomputeEnvironment};
})();
</script>`;

const runner = String.raw`<script>
window.addEventListener('DOMContentLoaded', async () => {
  const result = {checks:[], failures:[], width:window.innerWidth, height:window.innerHeight, scale:window.devicePixelRatio};
  const sleep = (ms=30) => new Promise(resolve => setTimeout(resolve, ms));
  const waitFor = async (fn, label, timeout=4000) => {
    const start=Date.now();
    while(Date.now()-start<timeout){ if(fn()) return; await sleep(20); }
    throw new Error('timeout: '+label);
  };
  const check = (condition, label) => {
    result.checks.push(label);
    if(!condition) throw new Error(label);
  };
  const visibleRoute = () => document.querySelector('[data-management-page]:not([hidden])')?.dataset.managementPage || '';
  const routeLink = (route) => document.querySelector('.nav-link[data-route-link="'+route+'"]');
  const clickRoute = async (route) => { routeLink(route).click(); await sleep(); check(visibleRoute()===route, 'route '+route+' visible'); };
  try {
    await waitFor(() => document.getElementById('dashboardDataState')?.dataset.state==='success', 'initial dashboard success');
    check(location.hash==='#/overview', 'default hash canonicalizes to overview');
    check(document.getElementById('workspaceCount').textContent==='2', 'successful snapshot renders count');
    check(window.__fakeADM.calls.some(c=>c.name==='StartLocalADM'), 'startup profile can start local ADM Service with Desktop');
    check(document.querySelectorAll('[data-management-page]:not([hidden])').length===1, 'exactly one page initially visible');

    const beforeRoutes = window.__fakeADM.calls.length;
    for(const route of ['workspaces','environments','runtime','mcp','skills','gateway','diagnostics','exec-allowlist','settings','overview']) await clickRoute(route);
    check(document.getElementById('diagnosticsPageContent').textContent.includes('不会自动 fan-out'), 'Diagnostics route without Environment does not fan out');
    await clickRoute('mcp');
    const hierarchyWorkspace=document.getElementById('managementWorkspace'), hierarchyEnvironment=document.getElementById('managementEnvironment');
    check(hierarchyWorkspace.value==='' && hierarchyEnvironment.disabled, 'Management Context starts with Workspace selection before Environment');
    hierarchyWorkspace.value='ws-b'; hierarchyWorkspace.dispatchEvent(new Event('change',{bubbles:true}));
    await waitFor(() => hierarchyEnvironment.value==='env-b', 'single Environment auto-selects after Workspace choice');
    check([...hierarchyEnvironment.options].some(option=>option.value==='env-b') && ![...hierarchyEnvironment.options].some(option=>option.value==='env-a'), 'Environment selector is scoped to selected Workspace');
    hierarchyWorkspace.value=''; hierarchyWorkspace.dispatchEvent(new Event('change',{bubbles:true})); await waitFor(() => hierarchyEnvironment.value==='' && hierarchyEnvironment.disabled, 'clearing Workspace clears and disables Environment selection');
    await clickRoute('gateway');
    check(document.getElementById('managementContextPanel').hidden, 'ADM connection route hides Management Context');
    check(document.querySelectorAll('#gatewayState').length===1, 'Gateway status id is unique in the document');
    check(document.body.textContent.includes('Saved connection profile') && document.body.textContent.includes('Runtime endpoints') && document.body.textContent.includes('Local service lifecycle'), 'Gateway page groups profile endpoints and local lifecycle controls');
    check(document.getElementById('gatewayStartButton') && document.getElementById('gatewayStopButton') && document.getElementById('gatewayRefreshButton'), 'Gateway grouped page preserves lifecycle buttons');    await clickRoute('exec-allowlist');
    check(document.getElementById('managementContextPanel').hidden, 'Exec allowlist route hides Management Context');
    check(document.body.textContent.includes('Allowed executables') && document.getElementById('execList') && document.querySelector('[data-dialog-open="execDialog"]'), 'Exec allowlist grouping preserves explicit allow/remove surface');
    check(document.getElementById('execBlockedCount').textContent==='7' && document.getElementById('execBlockedList').textContent.includes('blocked_python_fixture'), 'Exec allowlist renders denied executable observations sorted by count');
    check(document.getElementById('execBlockedList').textContent.indexOf('blocked_python_fixture') < document.getElementById('execBlockedList').textContent.indexOf('blocked_node_fixture'), 'Exec denied executable observations are sorted by count');
    document.querySelector('#execBlockedList button[data-action="allow-blocked-executable"][data-id="blocked_python_fixture"]').click();
    await waitFor(() => window.__fakeADM.calls.some(c=>c.name==='AllowExecutable' && c.args[0]==='blocked_python_fixture'), 'Allow blocked executable action');
    await waitFor(() => document.getElementById('execList').textContent.includes('blocked_python_fixture') && !document.getElementById('execBlockedList').textContent.includes('blocked_python_fixture'), 'Allowing blocked executable adds allowlist and clears observation');
    document.querySelector('#execBlockedList button[data-action="clear-blocked-executable"][data-id="blocked_node_fixture"]').click();
    await waitFor(() => window.__fakeADM.calls.some(c=>c.name==='ClearExecDenial' && c.args[0]==='blocked_node_fixture'), 'Clear one blocked executable observation');
    await waitFor(() => document.getElementById('execBlockedList').textContent.includes('暂无被拦截 executable'), 'Clearing blocked executable removes final observation');
    const afterExecAuthorityActions = window.__fakeADM.calls.length;
    await clickRoute('settings');
    check(document.getElementById('managementContextPanel').hidden, 'Settings route hides Management Context');
    check(document.body.textContent.includes('Desktop shell preferences') && document.getElementById('launchAtLogin'), 'Settings grouping preserves Desktop shell preference');
    check(document.body.textContent.includes('Managed worktrees') && document.getElementById('worktreeRoot').value==='C:\\fixtures\\adm-worktrees' && document.getElementById('worktreeBranchPrefix').value==='adm/', 'Settings renders managed worktree Core configuration');
    check(document.body.textContent.includes('Host environment') && document.getElementById('hostEnvironmentSummary').textContent.includes('process_environment') && document.getElementById('hostEnvironmentSummary').textContent.includes('42'), 'Settings renders safe host environment metadata without variable values');
    check(window.__fakeADM.calls.length===afterExecAuthorityActions, 'settings routing after explicit exec actions makes no adapter calls');
    check(document.querySelectorAll('.nav-link[aria-current="page"]').length===1, 'one active menu item');

    const memoryCallsBefore = window.__fakeADM.calls.filter(c=>c.name==='ListGlobalMemory').length;
    await clickRoute('memory');
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='ListGlobalMemory').length>memoryCallsBefore, 'Memory route lazy-loads Global Memory for the management view');
    check(document.getElementById('globalMemoryList').textContent.includes('visible-after-explicit-load'), 'Memory route lazy-load renders value');

    await clickRoute('workspaces');
    check(document.getElementById('workspaceVisibleCount').textContent==='2' && document.getElementById('workspaceListTotalCount').textContent==='2', 'Workspace visible/total counts use loaded snapshot');
    check(document.getElementById('workspaceList').textContent.includes('Environments 1'), 'Workspace rows show joined Environment counts');
    const projectFilterCalls=window.__fakeADM.calls.length;
    const workspaceFilter=document.getElementById('workspaceFilter'); workspaceFilter.value='Workspace B'; workspaceFilter.dispatchEvent(new Event('input',{bubbles:true})); await sleep();
    check(document.getElementById('workspaceVisibleCount').textContent==='1' && document.getElementById('workspaceList').textContent.includes('Workspace B') && !document.getElementById('workspaceList').textContent.includes('Workspace A'), 'Workspace search filters loaded rows locally');
    check(window.__fakeADM.calls.length===projectFilterCalls, 'Workspace filtering makes no adapter calls');
    workspaceFilter.value=''; workspaceFilter.dispatchEvent(new Event('input',{bubbles:true})); await sleep();
    const managementBeforeWorkspaceFilter=document.getElementById('managementEnvironment').value;
    document.querySelector('#workspaceList button[data-action="filter-environments-by-workspace"][data-id="ws-a"]').click(); await sleep();
    check(visibleRoute()==='environments' && document.getElementById('environmentWorkspaceFilter').value==='ws-a', 'Workspace shortcut navigates to filtered Environments');
    check(document.getElementById('environmentVisibleCount').textContent==='1' && document.getElementById('environmentList').textContent.includes('Environment A') && !document.getElementById('environmentList').textContent.includes('Environment B'), 'Workspace shortcut filters Environment rows');
    check(document.getElementById('managementEnvironment').value===managementBeforeWorkspaceFilter, 'Workspace shortcut does not retarget Management Environment');
    check(window.__fakeADM.calls.length===projectFilterCalls, 'Workspace shortcut remains presentation-only');
    document.getElementById('environmentWorkspaceFilter').value=''; document.getElementById('environmentWorkspaceFilter').dispatchEvent(new Event('change',{bubbles:true})); await sleep();
    await clickRoute('workspaces');

    const discoveryCallsBefore=window.__fakeADM.calls.filter(c=>c.name==='DiscoverWorkspace').length;
    const discoverA=document.querySelector('#workspaceList button[data-action="discover-workspace"][data-id="ws-a"]');
    discoverA.focus(); discoverA.click(); await waitFor(() => document.getElementById('workspaceDiscoveryDialog').open, 'Workspace discovery dialog opens');
    check(window.__fakeADM.calls.filter(c=>c.name==='DiscoverWorkspace').length===discoveryCallsBefore, 'opening discovery does not scan');
    check(document.getElementById('workspaceDiscoveryResult').textContent.includes('显式开始'), 'discovery starts unloaded');
    document.getElementById('workspaceDiscoveryPath').value='apps'; document.getElementById('workspaceDiscoveryPath').dispatchEvent(new Event('input',{bubbles:true}));
    document.getElementById('workspaceDiscoveryFilter').value='p2'; document.getElementById('workspaceDiscoveryFilter').dispatchEvent(new Event('input',{bubbles:true})); await sleep();
    check(window.__fakeADM.calls.filter(c=>c.name==='DiscoverWorkspace').length===discoveryCallsBefore, 'typing discovery options and local filter does not scan');
    document.getElementById('workspaceDiscoveryPath').value=''; document.getElementById('workspaceDiscoveryFilter').value='';
    window.__fakeADM.state.delayDiscovery=true;
    document.getElementById('workspaceDiscoveryForm').requestSubmit(); document.getElementById('workspaceDiscoveryForm').requestSubmit();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='DiscoverWorkspace').length===discoveryCallsBefore+1, 'explicit discovery starts once');
    check(document.getElementById('workspaceDiscoveryScanButton').disabled, 'pending discovery disables duplicate scan');
    await waitFor(() => document.getElementById('workspaceDiscoveryResult').textContent.includes('apps/p2'), 'discovery results render');
    window.__fakeADM.state.delayDiscovery=false;
    check(window.__fakeADM.calls.filter(c=>c.name==='DiscoverWorkspace').length===discoveryCallsBefore+1, 'double submit remains one discovery call');
    check(document.getElementById('workspaceDiscoverySummary').textContent.includes('Workspace ws-a') && document.getElementById('workspaceDiscoverySummary').textContent.includes('path .') && document.getElementById('workspaceDiscoverySummary').textContent.includes('limits d4/e2000/c50/g100/b65536'), 'discovery summary shows stable scope path and effective limits');
    check(document.getElementById('workspaceDiscoveryResult').textContent.includes('Directory digest') && document.getElementById('workspaceDiscoveryResult').textContent.includes('Existing Environment: Environment A (env-a)'), 'discovery renders compact digest and explicit matching Environment identity');
    const discoveryDialog=document.getElementById('workspaceDiscoveryDialog');
    check(discoveryDialog.scrollWidth<=discoveryDialog.clientWidth+1, 'discovery dialog has no horizontal overflow with long candidate names');
    for(const row of document.querySelectorAll('#workspaceDiscoveryResult .discovery-candidate')) check(row.scrollWidth<=row.clientWidth+1, 'discovery candidate row has no horizontal overflow');
    const callsBeforeLocalCandidateFilter=window.__fakeADM.calls.length;
    const loadedFilter=document.getElementById('workspaceDiscoveryFilter'); loadedFilter.value='P2'; loadedFilter.dispatchEvent(new Event('input',{bubbles:true})); await sleep();
    check(document.getElementById('workspaceDiscoveryResult').textContent.includes('apps/p2') && !document.getElementById('workspaceDiscoveryResult').textContent.includes('Docs markerless'), 'loaded candidate filtering is local');
    check(window.__fakeADM.calls.length===callsBeforeLocalCandidateFilter, 'loaded candidate filtering makes no adapter calls');
    loadedFilter.value=''; loadedFilter.dispatchEvent(new Event('input',{bubbles:true}));
    document.getElementById('workspaceDiscoveryQuery').value='changed'; document.getElementById('workspaceDiscoveryQuery').dispatchEvent(new Event('input',{bubbles:true})); await sleep();
    check(document.getElementById('workspaceDiscoverySummary').textContent.includes('需要重新扫描') && window.__fakeADM.calls.length===callsBeforeLocalCandidateFilter, 'editing discovery request marks loaded result stale without scanning');
    document.getElementById('workspaceDiscoveryQuery').value=''; document.getElementById('workspaceDiscoveryQuery').dispatchEvent(new Event('input',{bubbles:true}));
    const managementBeforeRootHandoff=document.getElementById('managementEnvironment').value;
    const createBeforeRootHandoff=window.__fakeADM.calls.filter(c=>c.name==='CreateEnvironment').length;
    document.querySelector('#workspaceDiscoveryResult button[data-action="use-discovery-root"][data-root="apps/p2"]').click();
    await waitFor(() => document.getElementById('environmentDialog').open, 'Use root opens existing Create Environment dialog');
    check(document.getElementById('environmentWorkspace').value==='ws-a' && document.getElementById('environmentRoot').value==='apps/p2', 'Use root preserves exact stable Workspace ID and relative root');
    check(document.getElementById('managementEnvironment').value===managementBeforeRootHandoff && window.__fakeADM.calls.filter(c=>c.name==='CreateEnvironment').length===createBeforeRootHandoff, 'Use root does not create or retarget Management Environment');
    document.getElementById('environmentName').value='from-discovery'; document.getElementById('environmentForm').requestSubmit();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='CreateEnvironment').length===createBeforeRootHandoff+1, 'explicit Environment create after root handoff');
    await waitFor(() => document.getElementById('statusPanel').dataset.kind==='error', 'failed discovery-root Environment create status');
    check(document.getElementById('environmentDialog').open && document.getElementById('environmentRoot').value==='apps/p2' && document.getElementById('environmentName').value==='from-discovery', 'failed Environment create preserves root handoff form');
    check(document.getElementById('managementEnvironment').value===managementBeforeRootHandoff, 'failed Environment create does not retarget current Environment');
    const createCall=window.__fakeADM.calls.filter(c=>c.name==='CreateEnvironment').at(-1);
    check(createCall.args[0].workspace_id==='ws-a' && /apps[\\/]p2$/i.test(createCall.args[0].root), 'explicit create resolves relative discovery root inside selected Workspace before Core validation');
    document.getElementById('environmentDialog').querySelector('[data-dialog-close]').click(); await sleep();

    discoverA.click(); await waitFor(() => document.getElementById('workspaceDiscoveryDialog').open, 'discovery reopens after root handoff');
    const noMatchBefore=window.__fakeADM.calls.filter(c=>c.name==='DiscoverWorkspace').length;
    document.getElementById('workspaceDiscoveryQuery').value='nomatch'; document.getElementById('workspaceDiscoveryForm').requestSubmit();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='DiscoverWorkspace').length===noMatchBefore+1 && document.getElementById('workspaceDiscoveryResult').textContent.includes('discovery query 没有候选匹配'), 'explicit discovery no-match state');
    document.getElementById('workspaceDiscoveryQuery').value=''; document.getElementById('workspaceDiscoveryPath').value='empty'; document.getElementById('workspaceDiscoveryForm').requestSubmit();
    await waitFor(() => document.getElementById('workspaceDiscoveryResult').textContent.includes('没有发现项目候选'), 'explicit discovery empty state');
    document.getElementById('workspaceDiscoveryPath').value=''; document.getElementById('workspaceDiscoveryMaxEntries').value='1'; document.getElementById('workspaceDiscoveryForm').requestSubmit();
    await waitFor(() => document.getElementById('workspaceDiscoverySummary').textContent.includes('Partial') && document.getElementById('workspaceDiscoverySummary').textContent.includes('stop entry_limit'), 'explicit discovery partial state exposes reason');
    window.__fakeADM.state.failDiscovery=true; document.getElementById('workspaceDiscoveryMaxEntries').value=''; document.getElementById('workspaceDiscoveryForm').requestSubmit();
    await waitFor(() => document.getElementById('workspaceDiscoveryResult').textContent.includes('fixture discovery failure'), 'explicit discovery failure state');
    window.__fakeADM.state.failDiscovery=false;

    document.getElementById('workspaceDiscoveryDialog').querySelector('[data-dialog-close]').click(); await sleep();
    discoverA.click(); await waitFor(() => document.getElementById('workspaceDiscoveryDialog').open, 'discovery opens for stale response race');
    window.__fakeADM.state.delayDiscovery=true;
    const staleDiscoveryBefore=window.__fakeADM.calls.filter(c=>c.name==='DiscoverWorkspace').length;
    document.getElementById('workspaceDiscoveryForm').requestSubmit();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='DiscoverWorkspace').length===staleDiscoveryBefore+1, 'delayed Workspace A discovery starts');
    document.getElementById('workspaceDiscoveryDialog').querySelector('[data-dialog-close]').click(); await sleep();
    window.__fakeADM.state.delayDiscovery=false;
    const discoverB=document.querySelector('#workspaceList button[data-action="discover-workspace"][data-id="ws-b"]'); discoverB.click();
    await waitFor(() => document.getElementById('workspaceDiscoveryDialog').open && document.getElementById('workspaceDiscoveryTitle').textContent.includes('Workspace B'), 'Workspace B discovery reopens');
    document.getElementById('workspaceDiscoveryForm').requestSubmit();
    await waitFor(() => document.getElementById('workspaceDiscoveryResult').textContent.includes('service-b'), 'Workspace B discovery result renders');
    await sleep(240);
    const workspaceBCandidates=[...document.querySelectorAll('#workspaceDiscoveryResult .discovery-candidate')].map(row=>row.textContent);
    check(document.getElementById('workspaceDiscoveryTitle').textContent.includes('Workspace B') && workspaceBCandidates.some(text=>text.includes('service-b')) && workspaceBCandidates.every(text=>!text.includes('apps/p2')), 'late Workspace A discovery cannot overwrite reopened Workspace B dialog');
    document.getElementById('workspaceDiscoveryDialog').querySelector('[data-dialog-close]').click(); await sleep();
    check(document.activeElement===discoverB, 'closing discovery restores Workspace row action focus');

    const workspaceOpener = document.querySelector('[data-management-page="workspaces"] [data-dialog-open="workspaceDialog"]');
    workspaceOpener.focus(); workspaceOpener.click(); await sleep();
    const workspaceDialog=document.getElementById('workspaceDialog');
    check(workspaceDialog.open, 'workspace editor opens');
    document.getElementById('workspacePath').value='C:\\draft\\keep-me';
    location.hash='#/mcp'; await sleep(60);
    check(visibleRoute()==='workspaces' && location.hash==='#/workspaces', 'open editor guards route change');
    check(document.getElementById('workspacePath').value==='C:\\draft\\keep-me', 'guarded navigation retains draft');
    document.getElementById('workspaceForm').requestSubmit();
    await waitFor(() => window.__fakeADM.calls.some(c=>c.name==='AddWorkspace'), 'failed Workspace save call');
    await waitFor(() => document.getElementById('statusPanel').dataset.kind==='error', 'failed Workspace save status');
    check(workspaceDialog.open, 'failed Workspace save keeps editor open');
    check(document.getElementById('workspacePath').value==='C:\\draft\\keep-me', 'failed Workspace save retains draft');
    workspaceDialog.querySelector('[data-dialog-close]').click(); await sleep();
    check(document.activeElement===workspaceOpener, 'closing editor restores visible opener focus');

    await clickRoute('environments');
    let detailButton=document.querySelector('#environmentList button[data-action="inspect-environment"][data-id="env-a"]');
    const detailInspectBefore=window.__fakeADM.calls.filter(c=>c.name==='InspectEnvironment' && c.args[0]==='env-a').length;
    const detailMemoryBefore=window.__fakeADM.calls.filter(c=>c.name==='ListEnvironmentMemory').length;
    const detailDigestBefore=window.__fakeADM.calls.filter(c=>c.name==='EnvironmentTreeDigest').length;
    detailButton.focus(); detailButton.click();
    await waitFor(() => !document.getElementById('environmentDetailPanel').hidden, 'Environment detail open');
    check(window.__fakeADM.calls.filter(c=>c.name==='InspectEnvironment' && c.args[0]==='env-a').length===detailInspectBefore+1, 'Environment detail reuses one scoped inspection instead of duplicate reads');
    check(window.__fakeADM.calls.filter(c=>c.name==='EnvironmentTreeDigest').length===detailDigestBefore && document.getElementById('environmentTreeDigestResult').textContent.includes('尚未读取'), 'opening Environment detail does not read directory summary');
    check(document.getElementById('environmentDetail').textContent.includes('Identity') && document.getElementById('environmentDetail').textContent.includes('Runtime authority') && document.getElementById('environmentDetail').textContent.includes('Capability issues'), 'Environment detail groups identity authority and capability facts');
    check(document.getElementById('environmentDetail').textContent.includes('artifact_missing'), 'Environment detail exposes capability reason without probing');
    document.getElementById('environmentTreeDigestButton').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='EnvironmentTreeDigest').length===detailDigestBefore+1 && document.getElementById('environmentTreeDigestResult').textContent.includes('Environment env-a'), 'explicit Environment directory summary');
    check(document.getElementById('environmentTreeDigestResult').textContent.includes('src') && document.getElementById('environmentTreeDigestResult').textContent.includes('limits d4/e2000/g100/b65536'), 'Environment directory summary shows bounded digest and effective limits');
    window.__fakeADM.state.delayTreeDigest=true;
    const staleDigestBefore=window.__fakeADM.calls.filter(c=>c.name==='EnvironmentTreeDigest').length;
    document.getElementById('environmentTreeDigestButton').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='EnvironmentTreeDigest').length===staleDigestBefore+1, 'delayed Environment digest starts');
    document.getElementById('closeEnvironmentDetail').click(); await sleep();
    window.__fakeADM.state.delayTreeDigest=false;
    detailButton.click(); await waitFor(() => !document.getElementById('environmentDetailPanel').hidden, 'Environment detail reopens during stale digest');
    await sleep(240);
    check(document.getElementById('environmentTreeDigestResult').textContent.includes('尚未读取') && !document.getElementById('environmentTreeDigestResult').textContent.includes('src'), 'late digest from closed detail cannot populate reopened detail');
    const diagnosticTab=document.querySelector('#environmentDetailSubviewTabs button[data-environment-detail-subview="diagnostics"]');
    const diagnosticsCallCountBefore=window.__fakeADM.calls.length;
    diagnosticTab.click(); await sleep();
    check(!document.getElementById('environmentDiagnostics').hidden && document.getElementById('environmentDetail').hidden, 'Environment Diagnostics subview is local to detail modal');
    check(document.getElementById('environmentDiagnostics').textContent.includes('Existing InspectEnvironment payload only') && document.getElementById('environmentDiagnostics').textContent.includes('fixture capability report') && document.getElementById('environmentDiagnostics').textContent.includes('artifact_missing'), 'Diagnostics renders returned fact source reason and state');
    check(window.__fakeADM.calls.length===diagnosticsCallCountBefore, 'Diagnostics subview does not call probe verifier reconnect or Memory APIs');
    document.querySelector('#environmentDetailSubviewTabs button[data-environment-detail-subview="summary"]').click(); await sleep();
    check(!document.getElementById('environmentDetail').hidden && document.getElementById('environmentDiagnostics').hidden, 'Environment Summary subview restores summary without adapter calls');
    check(window.__fakeADM.calls.length===diagnosticsCallCountBefore, 'Summary/Diagnostics switching stays local');
    check(window.__fakeADM.calls.filter(c=>c.name==='ListEnvironmentMemory').length===detailMemoryBefore, 'opening Environment detail does not read private Memory values');
    location.hash='#/skills'; await sleep(60);
    check(visibleRoute()==='environments' && location.hash==='#/environments', 'Environment detail guards route change');
    document.getElementById('closeEnvironmentDetail').click(); await sleep();
    check(document.activeElement===detailButton, 'closing Environment detail restores opener focus');
    const currentEnvironmentRow=document.querySelector('#environmentList .managed-item.current-context');
    check(currentEnvironmentRow?.textContent.includes('Environment A') && currentEnvironmentRow?.textContent.includes('当前管理环境'), 'Environment list marks explicit current Management Environment');
    check(currentEnvironmentRow?.textContent.includes('Workspace A') && currentEnvironmentRow?.textContent.includes('MCP 1') && currentEnvironmentRow?.textContent.includes('Skills 2'), 'Environment rows join Workspace identity and selection counts');
    const workspaceBindingBefore=window.__fakeADM.calls.filter(c=>c.name==='EnvironmentWorkspaceOptions' && c.args[0]==='env-a').length;
    document.querySelector('#environmentList button[data-action="workspace-environment"][data-id="env-a"]').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='EnvironmentWorkspaceOptions' && c.args[0]==='env-a').length===workspaceBindingBefore+1 && document.getElementById('environmentWorkspaceDialog').open, 'Environment Workspace rebind preview opens explicitly');
    check(document.getElementById('environmentWorkspaceSummary').textContent.includes('Workspace A') && document.getElementById('environmentWorkspaceTarget').value==='ws-a', 'Workspace rebind preview reports current/recommended binding');
    check(document.getElementById('environmentWorkspaceSaveButton').disabled, 'Workspace rebind save stays disabled when recommended binding already matches current Workspace');
    document.querySelector('#environmentWorkspaceDialog [data-dialog-close]').click(); await sleep();
    const recommendationsBefore=window.__fakeADM.calls.filter(c=>c.name==='EnvironmentWorkspaceRecommendations').length;
    document.getElementById('environmentWorkspaceRecommendationsButton').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='EnvironmentWorkspaceRecommendations').length===recommendationsBefore+1 && document.getElementById('environmentWorkspaceRecommendationsDialog').open, 'Workspace recommendation batch preview opens explicitly');
    check(document.getElementById('environmentWorkspaceRecommendationsList').textContent.includes('Environment Recommendation') && document.getElementById('environmentWorkspaceRecommendationsList').textContent.includes('Workspace Recommendation Target'), 'Workspace recommendation preview shows actionable current to recommended binding');
    check(!document.getElementById('environmentWorkspaceRecommendationsApplyButton').disabled, 'Workspace recommendation batch apply enables only with actionable drift');
    document.getElementById('environmentWorkspaceRecommendationsApplyButton').click();
    await waitFor(() => window.__fakeADM.calls.some(c=>c.name==='SetEnvironmentWorkspace' && c.args[0]==='env-recommendation' && c.args[1]==='ws-recommendation-target') && document.getElementById('environmentWorkspaceRecommendationsResult').textContent.includes('已调整 1 个'), 'Workspace recommendation batch applies through existing per-Environment rebind validation');
    check(document.getElementById('environmentWorkspaceRecommendationsList').textContent.includes('所有普通 Environment 已绑定到最具体的可用 Workspace') && document.getElementById('environmentWorkspaceRecommendationsApplyButton').disabled, 'Workspace recommendation batch refreshes to zero drift after apply');
    document.querySelector('#environmentWorkspaceRecommendationsDialog [data-dialog-close]').click(); await sleep();
    check(document.querySelectorAll('#environmentList [data-environment-id]').length===2, 'isolated Workspace recommendation fixture does not mutate the base Environment list');
    detailButton=document.querySelector('#environmentList button[data-action="inspect-environment"][data-id="env-a"]');
    const diagnoseButton=document.querySelector('#environmentList button[data-action="diagnose-environment"][data-id="env-a"]');
    const diagnoseBefore=window.__fakeADM.calls.filter(c=>c.name==='InspectEnvironment' && c.args[0]==='env-a').length;
    const diagnoseCallCountBefore=window.__fakeADM.calls.length;
    diagnoseButton.click(); await waitFor(() => visibleRoute()==='diagnostics' && location.hash==='#/diagnostics', 'Environment diagnose action opens standalone Diagnostics route');
    await waitFor(() => document.getElementById('diagnosticsPageContent').textContent.includes('artifact_missing'), 'standalone Diagnostics route renders returned facts');
    check(document.getElementById('environmentDetailPanel').hidden, 'standalone Diagnostics route does not open Environment detail modal');
    check(window.__fakeADM.calls.filter(c=>c.name==='InspectEnvironment' && c.args[0]==='env-a').length===diagnoseBefore+1, 'Diagnose route reuses InspectEnvironment only');
    check(!window.__fakeADM.calls.slice(diagnoseCallCountBefore).some(c=>['ProbeMCPHealth','ListGlobalMemory','ListEnvironmentMemory','RunVerifier'].includes(c.name)), 'standalone Diagnostics route does not probe, run verifier, or read Memory values');
    detailButton.click(); await waitFor(() => !document.getElementById('environmentDetailPanel').hidden, 'Environment detail reopen for shortcut');
    document.querySelector('#environmentDetailRoutes button[data-detail-route="skills"]').click(); await sleep();
    check(document.getElementById('environmentDetailPanel').hidden && visibleRoute()==='skills', 'Environment detail shortcut closes modal then navigates');
    check(document.getElementById('managementEnvironment').value==='env-a', 'Environment detail shortcut preserves explicit Management Environment');
    check(window.__fakeADM.calls.filter(c=>c.name==='ListEnvironmentMemory').length===detailMemoryBefore, 'Environment detail shortcut does not implicitly load Memory');
    await clickRoute('environments');
    detailButton.click(); await waitFor(() => !document.getElementById('environmentDetailPanel').hidden, 'Environment detail reopen for Memory shortcut');
    document.querySelector('#environmentDetailRoutes button[data-detail-route="memory"]').click(); await sleep();
    check(document.getElementById('environmentDetailPanel').hidden && visibleRoute()==='memory', 'Environment detail Memory shortcut opens Memory page');
    check(document.getElementById('managementEnvironment').value==='env-a' && !document.getElementById('loadEnvironmentMemory').disabled, 'Memory page uses current Management Environment scope');
    check(window.__fakeADM.calls.filter(c=>c.name==='ListEnvironmentMemory').length===detailMemoryBefore, 'Memory route does not auto-read Environment-private values');
    document.getElementById('loadEnvironmentMemory').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='ListEnvironmentMemory' && c.args[0]==='env-a').length>detailMemoryBefore, 'explicit Environment-private Memory read');
    check(document.getElementById('environmentMemoryList').textContent.includes('env-a-private-visible'), 'Environment-private Memory renders only after explicit load');
    await clickRoute('environments');
    detailButton.click(); await waitFor(() => !document.getElementById('environmentDetailPanel').hidden, 'Environment detail can reopen after Memory load');
    document.getElementById('closeEnvironmentDetail').click(); await sleep();
    await clickRoute('memory');
    check(document.getElementById('environmentMemoryList').textContent.includes('env-a-private-visible'), 'closing Environment detail does not clear loaded Memory page values');

    await clickRoute('mcp');
    check(!document.getElementById('managementContextPanel').hidden, 'MCP route shows Management Context because Environment selection is relevant');
    check(document.getElementById('managementWorkspace').value==='ws-a' && document.getElementById('managementEnvironment').value==='env-a', 'Management Context keeps Workspace then Environment selection');
    const workspaceMCPBefore=window.__fakeADM.calls.filter(c=>c.name==='SetWorkspaceMCP' && c.args[0]==='ws-a' && c.args[1]==='mcp-a').length;
    document.querySelector('#mcpList input[data-action="workspace-mcp"][data-id="mcp-a"]').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='SetWorkspaceMCP' && c.args[0]==='ws-a' && c.args[1]==='mcp-a' && c.args[2]===true).length>workspaceMCPBefore, 'Workspace MCP inheritance enable call');
    document.querySelector('#mcpList input[data-action="environment-mcp"][data-id="mcp-a"]').click();
    await waitFor(() => window.__fakeADM.snapshotA.environments.find(env=>env.environment_id==='env-a').explicit_mcp_ids.length===0, 'Environment explicit MCP can be removed while Workspace inheritance remains');
    check(window.__fakeADM.snapshotA.environments.find(env=>env.environment_id==='env-a').enabled_mcp_ids.includes('mcp-a') && document.querySelector('#mcpList input[data-action="workspace-mcp"][data-id="mcp-a"]').checked && !document.querySelector('#mcpList input[data-action="environment-mcp"][data-id="mcp-a"]').checked, 'Workspace MCP inheritance keeps MCP effectively enabled without copying explicit Environment state');
    document.querySelector('#mcpList input[data-action="environment-mcp"][data-id="mcp-a"]').click(); await waitFor(() => window.__fakeADM.snapshotA.environments.find(env=>env.environment_id==='env-a').explicit_mcp_ids.includes('mcp-a'), 'restore explicit MCP selection');
    document.querySelector('#mcpList input[data-action="workspace-mcp"][data-id="mcp-a"]').click(); await waitFor(() => !window.__fakeADM.snapshotA.workspaces.find(workspace=>workspace.workspace_id==='ws-a').enabled_mcp_ids.includes('mcp-a'), 'restore Workspace MCP inheritance state');
    const mcpFilter=document.getElementById('mcpFilter');
    const mcpStateFilter=document.getElementById('mcpStateFilter');
    mcpFilter.value='/MCP [AB]/i'; mcpFilter.dispatchEvent(new Event('input',{bubbles:true}));
    mcpStateFilter.value='unselected'; mcpStateFilter.dispatchEvent(new Event('change',{bubbles:true})); await sleep();
    check(document.getElementById('mcpVisibleCount').textContent==='1' && document.getElementById('mcpList').textContent.includes('MCP B') && !document.getElementById('mcpList').textContent.includes('MCP A'), 'MCP regex search and current-env-unselected filter combine locally');
    const setMCPDefaultBefore=window.__fakeADM.calls.filter(c=>c.name==='SetMCPDefault').length;
    document.getElementById('mcpSetVisibleDefaultButton').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='SetMCPDefault').length>setMCPDefaultBefore, 'MCP visible default batch call');
    check(window.__fakeADM.calls.some(c=>c.name==='SetMCPDefault' && c.args[0]==='mcp-b' && c.args[1]===true) && !window.__fakeADM.calls.some(c=>c.name==='SetMCPDefault' && c.args[0]==='mcp-a'), 'MCP default batch uses current filtered result only');
    const setMCPEnvBefore=window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentMCP').length;
    document.getElementById('mcpEnableVisibleButton').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentMCP').length>setMCPEnvBefore, 'MCP visible Environment batch call');
    check(window.__fakeADM.calls.some(c=>c.name==='SetEnvironmentMCP' && c.args[0]==='env-a' && c.args[1]==='mcp-b' && c.args[2]===true), 'MCP Environment batch captures current Environment and filtered ID');
    mcpFilter.value=''; mcpFilter.dispatchEvent(new Event('input',{bubbles:true})); mcpStateFilter.value='all'; mcpStateFilter.dispatchEvent(new Event('change',{bubbles:true})); await sleep();

    document.querySelector('#mcpList button[data-action="copy-mcp-config"][data-id="mcp-b"]').click();
    await waitFor(() => String(window.__copiedText || '').includes('mcpServers'), 'MCP generic config copied to clipboard');
    const copiedMCPConfig=JSON.parse(window.__copiedText);
    check(copiedMCPConfig.mcpServers['MCP B'].url==='http://127.0.0.1:9901/mcp' && String(copiedMCPConfig.mcpServers['MCP B'].headers.Authorization).includes('MCP_HUB_TOKEN') && !('command' in copiedMCPConfig.mcpServers['MCP B']), 'MCP export contains only effective HTTP connection fields and secret references');

    const legacyMCPUpdateBefore=window.__fakeADM.calls.filter(c=>c.name==='UpdateMCP').length;
    document.querySelector('#mcpList button[data-action="edit-mcp"][data-id="mcp-b"]').click();
    await waitFor(() => document.getElementById('mcpEditorFlow').open, 'legacy MCP editor opens');
    check(document.getElementById('mcpAuthMode').value==='headers', 'legacy HTTP MCP with header refs opens as Secret-backed headers');
    check(document.getElementById('mcpReferencePairs').value.includes('MCP_HUB_TOKEN'), 'legacy MCP header reference remains visible during edit');
    check(getComputedStyle(document.getElementById('mcpExecutableField')).display==='none' && getComputedStyle(document.getElementById('mcpArgsField')).display==='none' && getComputedStyle(document.getElementById('mcpEndpointField')).display!=='none', 'HTTP transport hides stdio-only fields at the CSS level');
    check(getComputedStyle(document.getElementById('mcpRefsField')).display!=='none' && getComputedStyle(document.getElementById('mcpEnvRefsField')).display==='none', 'HTTP Secret-backed headers shows only Header references');
    const mcpAuthMode=document.getElementById('mcpAuthMode');
    mcpAuthMode.value='none'; mcpAuthMode.dispatchEvent(new Event('change',{bubbles:true}));
    check(getComputedStyle(document.getElementById('mcpRefsField')).display==='none', 'HTTP Auth None hides Header references');
    mcpAuthMode.value='headers'; mcpAuthMode.dispatchEvent(new Event('change',{bubbles:true}));
    const mcpTransport=document.getElementById('mcpTransport');
    mcpTransport.value='stdio'; mcpTransport.dispatchEvent(new Event('change',{bubbles:true}));
    check(getComputedStyle(document.getElementById('mcpEndpointField')).display==='none' && getComputedStyle(document.getElementById('mcpAuthField')).display==='none' && getComputedStyle(document.getElementById('mcpExecutableField')).display!=='none' && getComputedStyle(document.getElementById('mcpArgsField')).display!=='none' && getComputedStyle(document.getElementById('mcpEnvRefsField')).display!=='none', 'stdio transport shows only stdio connection fields and Environment references');
    mcpTransport.value='streamable-http'; mcpTransport.dispatchEvent(new Event('change',{bubbles:true}));
    check(mcpAuthMode.value==='headers' && getComputedStyle(document.getElementById('mcpRefsField')).display!=='none' && getComputedStyle(document.getElementById('mcpEnvRefsField')).display==='none', 'switching back to HTTP restores the valid HTTP reference draft');
    const mcpHealthEnabled=document.getElementById('mcpHealthEnabled');
    mcpHealthEnabled.checked=false; mcpHealthEnabled.dispatchEvent(new Event('change',{bubbles:true}));
    check(getComputedStyle(document.getElementById('mcpProbeTimeoutField')).display==='none' && getComputedStyle(document.getElementById('mcpCheckIntervalField')).display==='none', 'disabled health checks hide health-only timing fields');
    mcpHealthEnabled.checked=true; mcpHealthEnabled.dispatchEvent(new Event('change',{bubbles:true}));
    const mcpAutoReconnect=document.getElementById('mcpAutoReconnect');
    mcpAutoReconnect.checked=true; mcpAutoReconnect.dispatchEvent(new Event('change',{bubbles:true}));
    check(getComputedStyle(document.getElementById('mcpReconnectIntervalField')).display!=='none', 'auto reconnect reveals reconnect interval');
    mcpAutoReconnect.checked=false; mcpAutoReconnect.dispatchEvent(new Event('change',{bubbles:true}));
    document.getElementById('mcpForm').requestSubmit();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='UpdateMCP').length>legacyMCPUpdateBefore, 'legacy MCP update call');
    const legacyMCPUpdate=window.__fakeADM.calls.filter(c=>c.name==='UpdateMCP').at(-1);
    check(legacyMCPUpdate.args[1].auth_mode==='headers' && legacyMCPUpdate.args[1].header_refs.Authorization==='Bearer \${MCP_HUB_TOKEN}', 'legacy MCP save reconciles auth mode with header refs');
    await waitFor(() => !document.getElementById('mcpEditorFlow').open, 'legacy MCP editor closes after valid save');

    const globalProbeSelection=document.getElementById('managementEnvironment');
    globalProbeSelection.value=''; globalProbeSelection.dispatchEvent(new Event('change',{bubbles:true})); await sleep();
    check(!document.querySelector('#mcpList button[data-action="probe-mcp"][data-id="mcp-a"]').disabled, 'global MCP probe enabled without Environment');
    const mcpProbeBefore=window.__fakeADM.calls.filter(c=>c.name==='ProbeMCPHealth').length;
    document.querySelector('#mcpList button[data-action="probe-mcp"][data-id="mcp-a"]').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='ProbeMCPHealth').length>mcpProbeBefore, 'explicit MCP probe call');
    await waitFor(() => document.getElementById('mcpList').textContent.includes('fixture healthy'), 'MCP explicit probe renders observation');
    check(window.__fakeADM.calls.some(c=>c.name==='ProbeMCPHealth' && c.args.length===1 && c.args[0]==='mcp-a'), 'MCP probe uses global definition identity only');

    globalProbeSelection.value='env-b'; globalProbeSelection.dispatchEvent(new Event('change',{bubbles:true})); await sleep();
    check(!document.querySelector('#mcpList button[data-action="probe-mcp"][data-id="mcp-a"]').disabled, 'global MCP probe enabled when Environment has not selected MCP');
    window.__fakeADM.state.delayMCPProbe=true;
    const delayedProbeBefore=window.__fakeADM.calls.filter(c=>c.name==='ProbeMCPHealth' && c.args.length===1 && c.args[0]==='mcp-a').length;
    document.querySelector('#mcpList button[data-action="probe-mcp"][data-id="mcp-a"]').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='ProbeMCPHealth' && c.args.length===1 && c.args[0]==='mcp-a').length>delayedProbeBefore, 'delayed MCP probe started');
    const mcpProbeEnvSelect=document.getElementById('managementEnvironment');
    mcpProbeEnvSelect.value='env-b'; mcpProbeEnvSelect.dispatchEvent(new Event('change',{bubbles:true}));
    await waitFor(() => document.getElementById('mcpList').textContent.includes('MCP A'), 'MCP list rerenders after Environment switch');
    await sleep(240);
    check(document.getElementById('mcpList').textContent.includes('fixture healthy'), 'global probe result survives Environment change');
    window.__fakeADM.state.delayMCPProbe=false;
    mcpProbeEnvSelect.value='env-a'; mcpProbeEnvSelect.dispatchEvent(new Event('change',{bubbles:true}));
    await waitFor(() => document.getElementById('mcpList').textContent.includes('fixture healthy'), 'global probe observation is shared across Environments');

    await sleep(240); // Let the preceding Environment-switch fixture finish its status update.
    window.__fakeADM.state.mcpProbeError=true;
    document.querySelector('#mcpList button[data-action="probe-mcp"][data-id="mcp-a"]').click();
    await waitFor(() => document.querySelector('#mcpList .resource-row[data-id="mcp-a"]').textContent.includes('配置引用尚未解析'), 'unresolved reference probe result');
    const errorRow=document.querySelector('#mcpList .resource-row[data-id="mcp-a"]');
    check([...errorRow.querySelectorAll('.resource-badge')].some(b=>b.textContent==='配置 · 引用未解析' && b.dataset.state==='unavailable'), 'unresolved reference configuration badge is unavailable');
    check([...errorRow.querySelectorAll('.resource-badge')].some(b=>b.textContent==='全局探测 · 受阻' && b.dataset.state==='error'), 'error kind overrides configured probe badge');
    check(!errorRow.textContent.includes('已配置') && document.getElementById('statusPanel').textContent.includes('MCP 全局探测失败') && document.getElementById('statusPanel').textContent.includes('处理建议'), 'badge and toast agree with actionable unresolved reference diagnostic');
    mcpStateFilter.value='issues'; mcpStateFilter.dispatchEvent(new Event('change',{bubbles:true})); await sleep();
    check(document.querySelector('#mcpList .resource-row[data-id="mcp-a"]'), 'blocked probe appears in issue filter');
    mcpStateFilter.value='all'; mcpStateFilter.dispatchEvent(new Event('change',{bubbles:true})); await sleep();
    window.__fakeADM.state.mcpProbeError='connection';
    document.querySelector('#mcpList button[data-action="probe-mcp"][data-id="mcp-a"]').click();
    await waitFor(() => document.querySelector('#mcpList .resource-row[data-id="mcp-a"]').textContent.includes('处理建议：检查 Endpoint、Transport、TLS/代理'), 'connection failure renders actionable recovery hint');
    check(document.getElementById('statusPanel').textContent.includes('MCP 全局探测失败') && document.getElementById('statusPanel').textContent.includes('处理建议'), 'connection failure toast explains next action');
    window.__fakeADM.state.mcpProbeError=false;
    document.querySelector('#mcpList button[data-action="probe-mcp"][data-id="mcp-a"]').click();
    await waitFor(() => document.querySelector('#mcpList .resource-row[data-id="mcp-a"]').textContent.includes('全局探测 · 健康'), 'probe recovery returns healthy badge');
    check(!document.querySelector('#mcpList .resource-row[data-id="mcp-a"]').textContent.includes('引用未解析'), 'successful reprobe clears stale configuration failure');

    const mcpImportButton=document.querySelector('[data-dialog-open="mcpImportDialog"]');
    mcpImportButton.click(); await waitFor(() => document.getElementById('mcpImportDialog').open, 'MCP import dialog opens');
    const importContent=document.getElementById('mcpImportContent');
    importContent.value='{ "mcpServers": { "imported": { "url": "http://127.0.0.1:9902/mcp", "headers": { "Authorization": "secret-token" } } } }';
    const previewBefore=window.__fakeADM.calls.filter(c=>c.name==='PreviewMCPImport').length;
    document.getElementById('mcpImportForm').requestSubmit();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='PreviewMCPImport').length>previewBefore, 'MCP import preview call');
    await waitFor(() => !document.getElementById('mcpImportApplyButton').disabled, 'valid MCP import preview enables apply');
    check(document.getElementById('mcpImportPreview').textContent.includes('ADM_IMPORTED_TOKEN'), 'MCP import preview keeps reference requirements visible');
    importContent.value=importContent.value.replace('9902','9903'); importContent.dispatchEvent(new Event('input',{bubbles:true})); await sleep();
    check(document.getElementById('mcpImportApplyButton').disabled && document.getElementById('mcpImportPreview').textContent.includes('请重新预览'), 'MCP import input changes invalidate preview before apply');
    document.getElementById('mcpImportForm').requestSubmit();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='PreviewMCPImport').length>previewBefore+1, 'MCP import preview can be regenerated');
    await waitFor(() => !document.getElementById('mcpImportApplyButton').disabled, 'regenerated MCP import preview enables apply');
    const applyBefore=window.__fakeADM.calls.filter(c=>c.name==='ApplyMCPImport').length;
    document.getElementById('mcpImportApplyButton').click(); document.getElementById('mcpImportApplyButton').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='ApplyMCPImport').length>applyBefore, 'MCP import apply call');
    await waitFor(() => document.getElementById('mcpListTotalCount').textContent==='3', 'MCP import apply refreshes authoritative catalog');
    check(window.__fakeADM.calls.filter(c=>c.name==='ApplyMCPImport').length===applyBefore+1, 'MCP import apply is guarded against double submit');

    const staleEnv=window.__fakeADM.snapshotA.environments.find(env=>env.environment_id==='env-b');
    const staleWorkspace=window.__fakeADM.snapshotA.workspaces.find(workspace=>workspace.workspace_id==='ws-b');
    staleEnv.explicit_mcp_ids=[...(staleEnv.explicit_mcp_ids||[]),'mcp-missing']; staleWorkspace.enabled_mcp_ids=[...(staleWorkspace.enabled_mcp_ids||[]),'workspace-mcp-missing'];
    staleEnv.explicit_skill_ids=[...(staleEnv.explicit_skill_ids||[]),'skill-missing']; staleWorkspace.enabled_skill_ids=[...(staleWorkspace.enabled_skill_ids||[]),'workspace-skill-missing']; window.__fakeADM.recomputeEnvironment(staleEnv);
    document.getElementById('refreshButton').click();
    await waitFor(() => document.getElementById('mcpClearMissingButton').textContent.includes('(2)'), 'MCP page detects missing Environment selection');
    const missingMCPCallBefore=window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentMCP' && c.args[1]==='mcp-missing').length;
    const missingWorkspaceMCPCallBefore=window.__fakeADM.calls.filter(c=>c.name==='SetWorkspaceMCP' && c.args[1]==='workspace-mcp-missing').length;
    document.getElementById('mcpClearMissingButton').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentMCP' && c.args[1]==='mcp-missing').length>missingMCPCallBefore && window.__fakeADM.calls.filter(c=>c.name==='SetWorkspaceMCP' && c.args[1]==='workspace-mcp-missing').length>missingWorkspaceMCPCallBefore, 'MCP missing Workspace and Environment selection cleanup calls');
    await waitFor(() => document.getElementById('mcpClearMissingButton').disabled, 'MCP missing selection cleanup refreshes authoritative state');
    check(!staleEnv.enabled_mcp_ids.includes('mcp-missing'), 'MCP missing selection is removed from Environment');
    check(!staleWorkspace.enabled_mcp_ids.includes('workspace-mcp-missing'), 'MCP missing selection is removed from Workspace');

    await clickRoute('skills');
    await waitFor(() => document.getElementById('skillClearMissingButton').textContent.includes('(2)'), 'Skill page detects missing Environment selection');
    const missingSkillCallBefore=window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentSkill' && c.args[1]==='skill-missing').length;
    const missingWorkspaceSkillCallBefore=window.__fakeADM.calls.filter(c=>c.name==='SetWorkspaceSkill' && c.args[1]==='workspace-skill-missing').length;
    document.getElementById('skillClearMissingButton').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentSkill' && c.args[1]==='skill-missing').length>missingSkillCallBefore && window.__fakeADM.calls.filter(c=>c.name==='SetWorkspaceSkill' && c.args[1]==='workspace-skill-missing').length>missingWorkspaceSkillCallBefore, 'Skill missing Workspace and Environment selection cleanup calls');
    await waitFor(() => document.getElementById('skillClearMissingButton').disabled, 'Skill missing selection cleanup refreshes authoritative state');
    check(!staleEnv.enabled_skill_ids.includes('skill-missing'), 'Skill missing selection is removed from Environment');
    check(!staleWorkspace.enabled_skill_ids.includes('workspace-skill-missing'), 'Skill missing selection is removed from Workspace');

    await clickRoute('mcp');
    const removeMCPBefore=window.__fakeADM.calls.filter(c=>c.name==='RemoveMCP' && c.args[0]==='mcp-b').length;
    document.querySelector('#mcpList button[data-action="remove-mcp"][data-id="mcp-b"]').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='RemoveMCP' && c.args[0]==='mcp-b').length>removeMCPBefore, 'MCP delete call');
    await waitFor(() => !document.querySelector('#mcpList .resource-row[data-id="mcp-b"]'), 'deleted MCP disappears from catalog');
    check(!window.__fakeADM.snapshotA.environments.find(env=>env.environment_id==='env-a').enabled_mcp_ids.includes('mcp-b'), 'MCP delete also clears Environment selections');
    check(window.__fakeADM.confirmations.some(message=>message.includes('清除这个 MCP ID')), 'MCP delete confirmation explains selection cleanup');

    const bulkMCPSelect=document.querySelector('input[data-action="select-mcp"][data-id="mcp-a"]');
    bulkMCPSelect.click(); await sleep();
    check(document.getElementById('mcpSelectedCount').textContent==='1', 'MCP stable ID can be selected for cross-Environment assignment');

    await clickRoute('skills');
    check(!document.getElementById('skillsPanel').hidden && document.getElementById('skillSourcesPanel').hidden, 'Skill route defaults to Skills subview');
    const workspaceSkillBefore=window.__fakeADM.calls.filter(c=>c.name==='SetWorkspaceSkill' && c.args[0]==='ws-a' && c.args[1]==='skill-a').length;
    document.querySelector('#skillList input[data-action="workspace-skill"][data-id="skill-a"]').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='SetWorkspaceSkill' && c.args[0]==='ws-a' && c.args[1]==='skill-a' && c.args[2]===true).length>workspaceSkillBefore, 'Workspace Skill inheritance enable call');
    document.querySelector('#skillList input[data-action="environment-skill"][data-id="skill-a"]').click();
    await waitFor(() => !window.__fakeADM.snapshotA.environments.find(env=>env.environment_id==='env-a').explicit_skill_ids.includes('skill-a'), 'Environment explicit Skill can be removed while Workspace inheritance remains');
    check(window.__fakeADM.snapshotA.environments.find(env=>env.environment_id==='env-a').enabled_skill_ids.includes('skill-a') && document.getElementById('skillList').textContent.includes('已启用（Workspace 继承）'), 'Workspace Skill inheritance keeps Skill effectively enabled');
    document.querySelector('#skillList input[data-action="environment-skill"][data-id="skill-a"]').click(); await waitFor(() => window.__fakeADM.snapshotA.environments.find(env=>env.environment_id==='env-a').explicit_skill_ids.includes('skill-a'), 'restore explicit Skill selection');
    document.querySelector('#skillList input[data-action="workspace-skill"][data-id="skill-a"]').click(); await waitFor(() => !window.__fakeADM.snapshotA.workspaces.find(workspace=>workspace.workspace_id==='ws-a').enabled_skill_ids.includes('skill-a'), 'restore Workspace Skill inheritance state');
    const bulkSkillSelect=document.querySelector('input[data-action="select-skill"][data-id="skill-a"]');
    bulkSkillSelect.click(); await sleep();
    check(document.getElementById('skillSelectedCount').textContent==='1' && !document.getElementById('skillAssignEnvironmentsButton').disabled, 'MCP and Skill selections combine into one Environment assignment action');
    document.getElementById('skillAssignEnvironmentsButton').click();
    await waitFor(() => document.getElementById('capabilityAssignmentDialog').open, 'bulk Environment assignment dialog opens from selected resources');
    document.getElementById('capabilityAssignmentSelectAllButton').click(); await sleep();
    check(document.getElementById('capabilityAssignmentSummary').textContent.includes('MCP 1') && document.getElementById('capabilityAssignmentSummary').textContent.includes('Skill 1') && document.getElementById('capabilityAssignmentSummary').textContent.includes('Environment 2'), 'bulk dialog summarizes resource and Environment selection counts');
    const bulkEnvAText=document.querySelector('#capabilityAssignmentEnvironmentList input[data-id="env-a"]').closest('label').textContent;
    const bulkEnvBText=document.querySelector('#capabilityAssignmentEnvironmentList input[data-id="env-b"]').closest('label').textContent;
    check(bulkEnvAText.includes('MCP 1/1') && bulkEnvAText.includes('Skill 1/1') && bulkEnvAText.includes('显式 1') && bulkEnvBText.includes('MCP 0/1') && bulkEnvBText.includes('Skill 0/1'), 'bulk dialog exposes effective and explicit per-Environment state before apply');
    const bulkMCPBefore=window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentMCP').length;
    const bulkSkillBefore=window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentSkill').length;
    window.__fakeADM.state.failBulkSkillAssignment=true;
    document.getElementById('capabilityAssignmentEnableButton').click();
    await waitFor(() => document.getElementById('capabilityAssignmentResult').textContent.includes('失败 1'), 'bulk Environment assignment reports partial failure');
    const bulkMCPCalls=window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentMCP').slice(bulkMCPBefore);
    const bulkSkillCalls=window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentSkill').slice(bulkSkillBefore);
    check(bulkMCPCalls.length===1 && bulkMCPCalls[0].args[0]==='env-b' && bulkMCPCalls[0].args[1]==='mcp-a' && bulkMCPCalls[0].args[2]===true, 'bulk assignment skips already-enabled MCP pair and mutates only the needed Environment');
    check(bulkSkillCalls.length===1 && bulkSkillCalls[0].args[0]==='env-b' && bulkSkillCalls[0].args[1]==='skill-a' && bulkSkillCalls[0].args[2]===true, 'bulk assignment attempts only the needed Skill pair');
    check(document.getElementById('capabilityAssignmentResult').textContent.includes('组合 4') && document.getElementById('capabilityAssignmentResult').textContent.includes('已修改 1') && document.getElementById('capabilityAssignmentResult').textContent.includes('已是目标状态 2'), 'bulk result distinguishes changed, unchanged, and failed pairs');
    window.__fakeADM.state.failBulkSkillAssignment=false;
    window.__fakeADM.snapshotA.environments.find(env=>env.environment_id==='env-b').explicit_mcp_ids=[]; window.__fakeADM.recomputeEnvironment(window.__fakeADM.snapshotA.environments.find(env=>env.environment_id==='env-b'));
    document.querySelector('#capabilityAssignmentDialog [data-dialog-close]').click();
    await waitFor(() => !document.getElementById('capabilityAssignmentDialog').open, 'bulk Environment assignment dialog closes cleanly');
    document.getElementById('skillClearSelectionButton').click(); await sleep();
    check(document.getElementById('skillSelectedCount').textContent==='0', 'bulk assignment Skill selection can be cleared before unrelated destructive actions');
    document.querySelector('[data-skill-subview="sources"]').click(); await sleep();
    check(document.getElementById('skillsPanel').hidden && !document.getElementById('skillSourcesPanel').hidden, 'Skill Sources subview switches locally');
    check(document.getElementById('skillSourceVisibleCount').textContent==='1' && document.getElementById('skillSourceListTotalCount').textContent==='1', 'Skill Sources subview shows visible and total counts');
    const sourceSearch=document.getElementById('skillSourceFilterInput'); sourceSearch.value='/source-a|fixtures/'; sourceSearch.dispatchEvent(new Event('input',{bubbles:true})); await sleep();
    check(document.getElementById('skillSourceVisibleCount').textContent==='1', 'Skill source regex filter is local to source rows');
    const sourceEditBefore=window.__fakeADM.calls.filter(c=>c.name==='UpdateSkillSource').length;
    document.querySelector('#skillSourceList button[data-action="edit-skill-source"]').click();
    await waitFor(() => document.getElementById('skillSourceDialog').open, 'Skill source editor opens from row');
    check(document.getElementById('skillSourceRoot').value.includes('fixtures'), 'Skill source editor pre-fills existing root');
    document.getElementById('skillSupportRoots').value='C:\\fixtures\\support';
    document.getElementById('skillSourceForm').requestSubmit();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='UpdateSkillSource').length>sourceEditBefore, 'Skill source edit uses update API');
    check(!window.__fakeADM.calls.some(c=>c.name==='RefreshSkillSource'), 'Skill source edit does not implicitly refresh source');
    document.querySelector('[data-skill-subview="skills"]').click(); await sleep();
    check(!document.getElementById('skillsPanel').hidden && document.getElementById('skillSourcesPanel').hidden, 'Skill subview returns to installed Skills locally');
    const sourceFilterSelect=document.getElementById('skillSourceFilter'); sourceFilterSelect.value='source-a'; sourceFilterSelect.dispatchEvent(new Event('change',{bubbles:true})); await sleep();
    check(document.getElementById('skillVisibleCount').textContent==='3' && document.getElementById('skillList').textContent.includes('Source：skills · source-a') && !document.getElementById('skillList').textContent.includes('Legacy metadata'), 'Skill Source filter isolates source-managed rows and separate Source fact');
    sourceFilterSelect.value=''; sourceFilterSelect.dispatchEvent(new Event('change',{bubbles:true})); await sleep();

    const skillFilter=document.getElementById('skillFilter');
    const skillStateFilter=document.getElementById('skillStateFilter');
    skillFilter.value='/Idle|Legacy Skill 2/'; skillFilter.dispatchEvent(new Event('input',{bubbles:true}));
    skillStateFilter.value='unselected'; skillStateFilter.dispatchEvent(new Event('change',{bubbles:true})); await sleep();
    check(document.getElementById('skillVisibleCount').textContent==='2' && document.getElementById('skillList').textContent.includes('Idle Skill') && document.getElementById('skillList').textContent.includes('Legacy Skill 2'), 'Skill regex search and current-env-unselected filter combine locally');
    const setSkillDefaultBefore=window.__fakeADM.calls.filter(c=>c.name==='SetSkillDefault').length;
    document.getElementById('skillSetVisibleDefaultButton').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='SetSkillDefault').length>=setSkillDefaultBefore+2, 'Skill visible default batch calls');
    check(window.__fakeADM.calls.some(c=>c.name==='SetSkillDefault' && c.args[0]==='skill-idle' && c.args[1]===true) && window.__fakeADM.calls.some(c=>c.name==='SetSkillDefault' && c.args[0]==='skill-legacy-2' && c.args[1]===true) && !window.__fakeADM.calls.some(c=>c.name==='SetSkillDefault' && c.args[0]==='skill-a'), 'Skill default batch uses current filtered results only');
    const setSkillEnvBefore=window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentSkill').length;
    document.getElementById('skillEnableVisibleButton').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentSkill').length>=setSkillEnvBefore+2, 'Skill visible Environment batch calls');
    check(window.__fakeADM.calls.some(c=>c.name==='SetEnvironmentSkill' && c.args[0]==='env-a' && c.args[1]==='skill-idle' && c.args[2]===true) && window.__fakeADM.calls.some(c=>c.name==='SetEnvironmentSkill' && c.args[0]==='env-a' && c.args[1]==='skill-legacy-2' && c.args[2]===true), 'Skill Environment batch captures current Environment and filtered IDs');
    skillFilter.value=''; skillFilter.dispatchEvent(new Event('input',{bubbles:true})); skillStateFilter.value='all'; skillStateFilter.dispatchEvent(new Event('change',{bubbles:true})); await sleep();

    const noEnvSelect=document.getElementById('managementEnvironment'); noEnvSelect.value=''; noEnvSelect.dispatchEvent(new Event('change',{bubbles:true}));
    await waitFor(() => !document.getElementById('skillProbeAllButton').disabled, 'global Skill probe does not require Environment');
    check(document.getElementById('skillClearUnavailableButton').disabled, 'automatic availability load does not authorize one-click cleanup');
    check(document.getElementById('skillBulkHint').textContent.includes('全局 Skill catalog') && document.getElementById('skillBulkHint').textContent.includes('不依赖当前 Environment'), 'Skill bulk hint explains global catalog scope');
    const probeCallsBefore=window.__fakeADM.calls.filter(c=>c.name==='ListSkillAvailability').length;
    const envSkillCallsBeforeProbe=window.__fakeADM.calls.filter(c=>c.name==='ListEnvironmentSkills').length;
    document.getElementById('skillProbeAllButton').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='ListSkillAvailability').length>probeCallsBefore, 'explicit global Skill availability call');
    check(window.__fakeADM.calls.filter(c=>c.name==='ListEnvironmentSkills').length===envSkillCallsBeforeProbe, 'global Skill bulk check does not call Environment availability');
    await waitFor(() => !document.getElementById('skillClearUnavailableButton').disabled, 'fresh explicit global probe enables cleanup');
    check(document.getElementById('skillClearUnavailableButton').textContent.includes('(3)'), 'cleanup includes all global structural failures');
    check(document.getElementById('skillBulkHint').textContent.includes('可用 2') && document.getElementById('skillBulkHint').textContent.includes('不可用 3'), 'global probe counts healthy idle Skill as available and broken placeholders as unavailable');

    const selectLegacy=document.querySelector('input[data-action="select-skill"][data-id="skill-legacy"]');
    const selectLegacy2=document.querySelector('input[data-action="select-skill"][data-id="skill-legacy-2"]');
    selectLegacy.click(); selectLegacy2.click(); await sleep();
    check(document.getElementById('skillSelectedCount').textContent==='2', 'two Skills can be selected for bulk delete');
    document.getElementById('skillDeleteSelectedButton').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='RemoveSkill' && (c.args[0]==='skill-legacy' || c.args[0]==='skill-legacy-2')).length===2, 'bulk delete removes each selected stable Skill ID');
    await waitFor(() => document.getElementById('skillListTotalCount').textContent==='3', 'bulk delete refreshes authoritative Skill snapshot');
    check(!window.__fakeADM.snapshotA.environments.find(env=>env.environment_id==='env-a').enabled_skill_ids.includes('skill-legacy-2'), 'Skill delete also clears Environment selections');
    check(window.__fakeADM.confirmations.some(message=>message.includes('Legacy: 2') && message.includes('catalog metadata')), 'bulk delete confirmation explains metadata-only impact');
    check(document.getElementById('skillClearUnavailableButton').disabled, 'catalog refresh invalidates prior cleanup authorization');

    const secondProbeCalls=window.__fakeADM.calls.filter(c=>c.name==='ListSkillAvailability').length;
    document.getElementById('skillProbeAllButton').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='ListSkillAvailability').length>secondProbeCalls, 'second explicit global Skill availability call');
    await waitFor(() => !document.getElementById('skillClearUnavailableButton').disabled, 'fresh probe re-enables cleanup after catalog change');
    document.getElementById('skillClearUnavailableButton').click();
    await waitFor(() => window.__fakeADM.calls.some(c=>c.name==='RemoveSkill' && c.args[0]==='skill-broken'), 'one-click cleanup removes explicit-probe unavailable Skill');
    await waitFor(() => document.getElementById('skillListTotalCount').textContent==='2', 'one-click cleanup refreshes Skill catalog');
    check(!window.__fakeADM.snapshotA.environments.find(env=>env.environment_id==='env-a').enabled_skill_ids.includes('skill-broken'), 'unavailable Skill delete clears its Environment selection');
    check(document.getElementById('skillList').textContent.includes('Skill A') && document.getElementById('skillList').textContent.includes('Idle Skill'), 'one-click cleanup preserves available Skills');
    check(window.__fakeADM.confirmations.some(message=>message.includes('Source-managed: 1') && message.includes('刷新 Source')), 'cleanup confirmation warns source-managed Skill may return');

    window.__fakeADM.state.failSkillSources=true;
    document.getElementById('refreshButton').click();
    await waitFor(() => document.getElementById('skillSourceList').textContent.includes('不可用'), 'Skill source local failure');
    check(document.getElementById('workspaceList').textContent.includes('Workspace A'), 'auxiliary failure preserves base snapshot');
    check(document.getElementById('dashboardDataState').dataset.state==='success', 'auxiliary failure does not demote base snapshot');
    window.__fakeADM.state.failSkillSources=false;

    window.__fakeADM.state.failVerifiers=true;
    const envSelect=document.getElementById('managementEnvironment'); envSelect.value='env-a'; envSelect.dispatchEvent(new Event('change',{bubbles:true}));
    await waitFor(() => document.getElementById('verifierList').textContent.includes('不可用'), 'Verifier local failure');
    check(document.getElementById('processList').textContent.includes('proc-a'), 'Process list survives verifier failure');
    check(document.getElementById('runList').textContent.includes('run-a'), 'Run list survives verifier failure');
    window.__fakeADM.state.failVerifiers=false;

    await clickRoute('runtime');
    const runtimeCallsBeforeTabs=window.__fakeADM.calls.length;
    check(!document.querySelector('[data-runtime-subview-panel="verifiers"]').hidden && document.querySelector('[data-runtime-subview-panel="processes"]').hidden, 'Runtime starts on local Verifiers subview');
    document.querySelector('#runtimeSubviewTabs [data-runtime-subview="processes"]').click(); await sleep();
    check(document.querySelector('[data-runtime-subview-panel="verifiers"]').hidden && !document.querySelector('[data-runtime-subview-panel="processes"]').hidden, 'Runtime switches to Processes subview locally');
    check(window.__fakeADM.calls.length===runtimeCallsBeforeTabs, 'Runtime subview switch makes no adapter calls');
    const processLogCalls=window.__fakeADM.calls.filter(c=>c.name==='GetProcessLogs').length;
    document.querySelector('#processList button[data-action="process-logs"][data-id="proc-a"]').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='GetProcessLogs').length>processLogCalls, 'explicit Process logs call');
    await waitFor(() => document.getElementById('runtimeOutput').textContent.includes('PROCESS_A_LOG'), 'Process logs render bounded output');
    check(document.getElementById('runtimeOutputMeta').textContent.includes('current-owner observation') && document.getElementById('runtimeOutputMeta').textContent.includes('processes proc-a') && document.getElementById('runtimeOutputMeta').textContent.includes('stdout truncated'), 'Process output meta identifies resource and truncation');
    const runtimeCallsBeforeRunTab=window.__fakeADM.calls.length;
    document.querySelector('#runtimeSubviewTabs [data-runtime-subview="runs"]').click(); await sleep();
    check(window.__fakeADM.calls.length===runtimeCallsBeforeRunTab && document.getElementById('runtimeOutput').textContent==='尚无输出', 'Runtime tab change clears output without backend reads');
    document.querySelector('#runList button[data-action="run-output"][data-id="run-a"]').click(); await sleep();
    check(document.getElementById('runtimeOutput').textContent.includes('RUN_A_OUTPUT') && document.getElementById('runtimeOutputMeta').textContent.includes('runs run-a'), 'Run output uses existing owner-local list result');
    document.querySelector('#runtimeSubviewTabs [data-runtime-subview="processes"]').click(); await sleep();
    document.querySelector('#processList button[data-action="process-logs"][data-id="proc-a"]').click(); await waitFor(() => document.getElementById('runtimeOutput').textContent.includes('PROCESS_A_LOG'), 'Process output rebound before disappearance');
    window.__fakeADM.state.hideProcess=true; document.getElementById('runtimeRefreshButton').click();
    await waitFor(() => document.getElementById('processList').textContent.includes('没有 process'), 'authoritative Process disappearance refresh');
    check(document.getElementById('runtimeOutput').textContent==='尚无输出' && document.getElementById('runtimeOutputMeta').textContent.includes('资源已不在最新'), 'successful list refresh clears output for disappeared resource');
    window.__fakeADM.state.hideProcess=false; document.getElementById('runtimeRefreshButton').click(); await waitFor(() => document.getElementById('processList').textContent.includes('proc-a'), 'Process restored for stale-log test');
    window.__fakeADM.state.delayProcessLogs=true;
    const delayedLogCalls=window.__fakeADM.calls.filter(c=>c.name==='GetProcessLogs' && c.args[0]==='env-a').length;
    document.querySelector('#processList button[data-action="process-logs"][data-id="proc-a"]').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='GetProcessLogs' && c.args[0]==='env-a').length>delayedLogCalls, 'delayed Environment A process log started');
    const runtimeWorkspaceSelect=document.getElementById('managementWorkspace'); runtimeWorkspaceSelect.value='ws-b'; runtimeWorkspaceSelect.dispatchEvent(new Event('change',{bubbles:true}));
    await waitFor(() => document.getElementById('managementEnvironment').value==='env-b' && document.getElementById('processList').textContent.includes('proc-b'), 'Environment B Runtime list wins process-log race');
    await sleep(240);
    check(!document.getElementById('runtimeOutput').textContent.includes('PROCESS_A_LOG') && document.getElementById('runtimeOutputMeta').textContent.includes('Workspace 已切换'), 'late Environment A process log cannot populate Environment B output');
    window.__fakeADM.state.delayProcessLogs=false;

    check(window.__fakeADM.calls.filter(c=>c.name==='ProbeMCPHealth').length===5, 'only the five explicit MCP probes were executed');
    const forbiddenAutoCalls=['RefreshSkillSource','RunVerifier','StopProcess','CancelRun','WriteGlobalMemory','WriteEnvironmentMemory'];
    check(!window.__fakeADM.calls.some(c=>forbiddenAutoCalls.includes(c.name)), 'navigation/refresh makes no implicit mutation call');

    await clickRoute('environments');
    window.__fakeADM.state.delayEnvironmentAInspection=true;
    const delayedAButton=document.querySelector('#environmentList button[data-action="inspect-environment"][data-id="env-a"]');
    const fastBButton=document.querySelector('#environmentList button[data-action="inspect-environment"][data-id="env-b"]');
    const delayedACalls=window.__fakeADM.calls.filter(c=>c.name==='InspectEnvironment' && c.args[0]==='env-a').length;
    delayedAButton.click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='InspectEnvironment' && c.args[0]==='env-a').length>delayedACalls, 'delayed Environment A inspection started');
    fastBButton.click();
    await waitFor(() => !document.getElementById('environmentDetailPanel').hidden && document.getElementById('environmentDetailTitle').textContent==='Environment B', 'Environment B detail wins A to B race');
    await sleep(240);
    check(document.getElementById('environmentDetailTitle').textContent==='Environment B' && document.getElementById('environmentDetail').textContent.includes('env-b'), 'late Environment A response cannot overwrite Environment B detail');
    check(document.getElementById('managementEnvironment').value==='env-b', 'A to B detail race leaves explicit Management Environment on B');
    window.__fakeADM.state.delayEnvironmentAInspection=false;
    document.getElementById('closeEnvironmentDetail').click(); await sleep();

    await clickRoute('environments');
    const managementBeforeInvalidFilter=document.getElementById('managementEnvironment').value;
    const environmentWorkspaceFilter=document.getElementById('environmentWorkspaceFilter'); environmentWorkspaceFilter.value='ws-a'; environmentWorkspaceFilter.dispatchEvent(new Event('change',{bubbles:true}));
    const removedWorkspaceA=window.__fakeADM.snapshotA.workspaces.splice(window.__fakeADM.snapshotA.workspaces.findIndex(workspace=>workspace.workspace_id==='ws-a'),1)[0];
    document.getElementById('refreshButton').click();
    await waitFor(() => !document.getElementById('environmentFilterHint').hidden, 'removed Workspace filter invalid hint');
    check(document.getElementById('environmentWorkspaceFilter').value==='ws-a' && document.getElementById('environmentFilterHint').textContent.includes('ws-a'), 'removed Workspace filter stays visibly invalid');
    check(document.getElementById('managementEnvironment').value===managementBeforeInvalidFilter, 'invalid Workspace filter never retargets Management Environment');
    window.__fakeADM.snapshotA.workspaces.unshift(removedWorkspaceA); document.getElementById('refreshButton').click();
    await waitFor(() => [...document.getElementById('managementWorkspace').options].some(option=>option.value==='ws-a'), 'removed Workspace fixture restored for later acceptance cases');

    for (const [route, selector] of [['settings', '#launchAtLogin'], ['gateway', '#gatewayRefreshButton']]) {
      await clickRoute(route);
      const control=document.querySelector(selector);
      const body=control.closest('.subpanel-body');
      const card=body.closest('.subpanel');
      const heading=card.querySelector('.subpanel-heading').getBoundingClientRect();
      const note=body.querySelector('.subpanel-note').getBoundingClientRect();
      const rect=control.getBoundingClientRect();
      const cardRect=card.getBoundingClientRect();
      check(rect.left-cardRect.left>=14 && rect.top-heading.bottom>=14, route+' controls inset from card and heading');
      check(cardRect.bottom-note.bottom>=14 && note.left-cardRect.left>=14, route+' note inset from card edges');
      const preceding=body.querySelector('.gateway-actions, .desktop-toggle').getBoundingClientRect();
      check(note.top-preceding.bottom>=10, route+' guidance separated from controls');
    }
    await clickRoute('mcp');
    const catalog=document.getElementById('mcpList');
    check(catalog.querySelectorAll('.resource-row').length>0, 'MCP geometry fixture contains rows');
    for (const row of catalog.querySelectorAll('.resource-row')) {
      const content=row.querySelector('.resource-main').getBoundingClientRect();
      const actions=row.querySelector('.resource-actions').getBoundingClientRect();
      if (catalog.clientWidth<=860) check(actions.top-content.bottom>=10, 'narrow MCP actions stack below readable content');
      check(row.scrollWidth<=row.clientWidth+1, 'MCP row has no horizontal overflow');
    }

    window.__fakeADM.state.delayMCPProbe=true;
    const pendingGlobalCalls=window.__fakeADM.calls.filter(c=>c.name==='ProbeMCPHealth').length;
    document.querySelector('#mcpList button[data-action="probe-mcp"][data-id="mcp-a"]').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='ProbeMCPHealth').length>pendingGlobalCalls, 'global probe before definition edit');
    const changedDefinition=window.__fakeADM.snapshotA.mcps.find(mcp=>mcp.id==='mcp-a');
    changedDefinition.endpoint='http://127.0.0.1:9988/mcp';
    document.getElementById('refreshButton').click(); await sleep(240);
    check(!document.querySelector('#mcpList .resource-row[data-id="mcp-a"]').textContent.includes('fixture healthy'), 'changed definition rejects stale global probe result');
    const switchProbeCalls=window.__fakeADM.calls.filter(c=>c.name==='ProbeMCPHealth').length;
    document.querySelector('#mcpList button[data-action="probe-mcp"][data-id="mcp-a"]').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='ProbeMCPHealth').length>switchProbeCalls, 'global probe before connection switch');
    await clickRoute('workspaces');
    const preSwitchDiscover=document.querySelector('#workspaceList button[data-action="discover-workspace"][data-id="ws-b"]'); preSwitchDiscover.click();
    await waitFor(() => document.getElementById('workspaceDiscoveryDialog').open, 'discovery opens before profile switch');
    window.__fakeADM.state.delayDiscovery=true;
    const preSwitchDiscoveryCalls=window.__fakeADM.calls.filter(c=>c.name==='DiscoverWorkspace').length;
    document.getElementById('workspaceDiscoveryForm').requestSubmit();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='DiscoverWorkspace').length===preSwitchDiscoveryCalls+1, 'delayed discovery starts before profile switch');
    const profileSelect=document.getElementById('connectionSelect'); profileSelect.value='profile-b'; profileSelect.dispatchEvent(new Event('change',{bubbles:true}));
    await waitFor(() => document.getElementById('workspaceList').textContent.includes('Workspace Profile B'), 'profile B snapshot');
    window.__fakeADM.state.delayDiscovery=false;
    check(!document.getElementById('workspaceDiscoveryDialog').open, 'profile switch closes old discovery scope after pending request drains');
    check(!document.getElementById('workspaceList').textContent.includes('Workspace A'), 'profile switch clears old snapshot');
    check(!document.getElementById('globalMemoryList').textContent.includes('visible-after-explicit-load'), 'profile switch clears loaded Memory values');

    await sleep(240);
    check(!document.getElementById('mcpList').textContent.includes('fixture healthy'), 'old connection global probe cannot populate new connection');
    check(document.getElementById('environmentList').textContent.includes('Retention 未知') && !document.getElementById('environmentList').textContent.includes('Durable'), 'missing retention is rendered unknown rather than durable certainty');

    profileSelect.value='profile-a'; profileSelect.dispatchEvent(new Event('change',{bubbles:true}));
    await waitFor(() => document.getElementById('environmentList').textContent.includes('Environment A'), 'profile A restored for temporary lifecycle acceptance');
    const lifecycleWorkspaceSelect=document.getElementById('managementWorkspace'); lifecycleWorkspaceSelect.value='ws-a'; lifecycleWorkspaceSelect.dispatchEvent(new Event('change',{bubbles:true}));
    await waitFor(() => document.getElementById('managementEnvironment').value==='env-a', 'temporary lifecycle context switches to Workspace A then Environment A');
    const environmentList=document.getElementById('environmentList');
    check(environmentList.textContent.includes('Temporary · 已过期') && environmentList.textContent.includes('Owner lifecycle-owner-a') && environmentList.textContent.includes('Session session-a') && environmentList.textContent.includes('Run run-provenance-a'), 'temporary Environment list renders expiry and lifecycle provenance');
    check(!environmentList.textContent.includes('env-a-private-visible'), 'temporary Environment list does not expose private Memory values');
    const tempDetail=document.querySelector('#environmentList button[data-action="inspect-environment"][data-id="env-a"]'); tempDetail.click();
    await waitFor(() => !document.getElementById('environmentDetailPanel').hidden && document.getElementById('environmentDetail').textContent.includes('Lifecycle retention'), 'temporary Environment detail lifecycle group');
    check(document.getElementById('environmentDetail').textContent.includes('lifecycle-owner-a') && document.getElementById('environmentDetail').textContent.includes('session-a') && document.getElementById('environmentDetail').textContent.includes('run-provenance-a'), 'temporary Environment detail renders provenance');
    check(!document.getElementById('environmentDetail').textContent.includes('env-a-private-visible'), 'temporary Environment detail does not expose private Memory values');
    document.getElementById('closeEnvironmentDetail').click();

    const promoteBefore=window.__fakeADM.calls.filter(c=>c.name==='PromoteTemporaryEnvironment').length;
    document.querySelector('#environmentList button[data-action="promote-temporary-environment"][data-id="env-a"]').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='PromoteTemporaryEnvironment').length===promoteBefore+1 && document.querySelector('#environmentList [data-environment-id="env-a"]').textContent.includes('Durable'), 'temporary promote through Admin MCP');
    const promoteCall=window.__fakeADM.calls.filter(c=>c.name==='PromoteTemporaryEnvironment').at(-1);
    check(promoteCall.args[0]==='env-a' && promoteCall.args[1]==='lifecycle-owner-a' && document.querySelector('#environmentList [data-environment-id="env-a"]').textContent.includes('env-a'), 'promote preserves stable Environment identity and uses persisted owner');

    const fixtureEnv=window.__fakeADM.snapshotA.environments.find(item=>item.environment_id==='env-a');
    fixtureEnv.retention={persistence:'temporary', creator_surface:'agent', owner_id:'lifecycle-owner-a', session_id:'session-a', run_id:'run-provenance-a', created_at:'2026-09-12T00:00:00Z', expires_at:'2026-09-12T01:00:00Z'}; fixtureEnv.writer={owner:'writer-a'};
    document.getElementById('refreshButton').click(); await waitFor(() => document.querySelector('#environmentList [data-environment-id="env-a"]').textContent.includes('Temporary'), 'temporary fixture restored after promote acceptance');
    const tempPreviewBefore=window.__fakeADM.calls.filter(c=>c.name==='CleanupTemporaryEnvironment' && c.args[2]===false).length;
    document.querySelector('#environmentList button[data-action="preview-temporary-environment-cleanup"][data-id="env-a"]').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='CleanupTemporaryEnvironment' && c.args[2]===false).length===tempPreviewBefore+1, 'temporary cleanup blocked preview');
    check(document.getElementById('statusPanel').textContent.includes('active_writer') && !document.querySelector('#environmentList button[data-action="execute-temporary-environment-cleanup"][data-id="env-a"]'), 'cleanup preview is non-destructive and exposes active blocker before confirm');
    fixtureEnv.writer=null; document.getElementById('refreshButton').click(); await sleep(120);
    const eligiblePreviewBefore=window.__fakeADM.calls.filter(c=>c.name==='CleanupTemporaryEnvironment' && c.args[2]===false).length;
    document.querySelector('#environmentList button[data-action="preview-temporary-environment-cleanup"][data-id="env-a"]').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='CleanupTemporaryEnvironment' && c.args[2]===false).length===eligiblePreviewBefore+1 && document.querySelector('#environmentList button[data-action="execute-temporary-environment-cleanup"][data-id="env-a"]'), 'eligible cleanup preview unlocks explicit confirm action');
    check(!document.querySelector('[data-action*="force"]'), 'temporary lifecycle UI exposes no force cleanup control');
    const cleanupBefore=window.__fakeADM.calls.filter(c=>c.name==='CleanupTemporaryEnvironment' && c.args[2]===true).length;
    document.querySelector('#environmentList button[data-action="execute-temporary-environment-cleanup"][data-id="env-a"]').click();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='CleanupTemporaryEnvironment' && c.args[2]===true).length===cleanupBefore+1 && !document.querySelector('#environmentList [data-environment-id="env-a"]'), 'confirmed targeted temporary cleanup removes selected Environment only');
    check(document.querySelector('#environmentList [data-environment-id="env-b"]') && window.__fakeADM.confirmations.at(-1).includes('没有 force 路径'), 'cleanup leaves unrelated Environment and requires explicit non-force confirmation');
    check(document.getElementById('statusPanel').textContent.includes('项目目录未删除'), 'ordinary cleanup result does not imply project directory deletion');

    window.__fakeADM.state.failEnvironmentCreate=false;
    window.__fakeADM.snapshotA.workspaces.push({workspace_id:'ws-b-project', name:'Workspace B Project', path:'C:\\fixtures\\plain-non-git-workspace-b\\project', enabled_mcp_ids:[], enabled_skill_ids:[]});
    const createSuccessBefore=window.__fakeADM.calls.filter(c=>c.name==='CreateEnvironment').length;
    const workspaceSetBeforeCreate=window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentWorkspace').length;
    document.getElementById('createEnvironmentButton').click(); await waitFor(() => document.getElementById('environmentDialog').open, 'create Environment dialog opens for drift-prevention acceptance');
    document.getElementById('environmentWorkspace').value='ws-b'; document.getElementById('environmentRoot').value='project/service'; document.getElementById('environmentName').value='Created Drift'; document.getElementById('environmentForm').requestSubmit();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='CreateEnvironment').length===createSuccessBefore+1 && document.getElementById('environmentWorkspaceDialog').open, 'new broad-Workspace Environment triggers explicit recommendation after create');
    check(document.getElementById('environmentWorkspaceTarget').value==='ws-b-project' && document.getElementById('environmentWorkspaceSummary').textContent.includes('Workspace B Project'), 'post-create recommendation selects the most specific containing Workspace');
    check(document.getElementById('environmentWorkspaceReason').textContent.includes('刚创建的 Environment') && window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentWorkspace').length===workspaceSetBeforeCreate, 'post-create recommendation is advisory and never silently rebinds');
    check(document.querySelector('#environmentList [data-environment-id="env-created-drift"]')?.textContent.includes('Workspace B'), 'new Environment remains on explicitly selected broad Workspace until confirmation');
    document.querySelector('#environmentWorkspaceDialog [data-dialog-close]').click(); await sleep();

    window.__fakeADM.snapshotA.environments.push({environment_id:'env-existing-workspace-drift', workspace_id:'ws-b', name:'Existing Workspace Drift', root:'C:\\fixtures\\plain-non-git-workspace-b\\newscope\\service', state:'ready', writer:null, enabled_mcp_ids:[], explicit_mcp_ids:[], inherited_mcp_ids:[], enabled_skill_ids:[], explicit_skill_ids:[], inherited_skill_ids:[], private_memory_count:0, retention:{persistence:'durable', creator_surface:'fixture', created_at:'2026-09-16T00:00:00Z'}});
    window.__fakeADM.state.failWorkspaceAdd=false;
    const addWorkspaceBefore=window.__fakeADM.calls.filter(c=>c.name==='AddWorkspace').length;
    const workspaceSetBeforeAdd=window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentWorkspace').length;
    document.querySelector('[data-dialog-open="workspaceDialog"]').click(); await waitFor(() => document.getElementById('workspaceDialog').open, 'add Workspace dialog opens for existing Environment drift acceptance');
    document.getElementById('workspacePath').value='C:\\fixtures\\plain-non-git-workspace-b\\newscope'; document.getElementById('workspaceName').value='Added Workspace'; document.getElementById('workspaceForm').requestSubmit();
    await waitFor(() => window.__fakeADM.calls.filter(c=>c.name==='AddWorkspace').length===addWorkspaceBefore+1 && document.getElementById('environmentWorkspaceRecommendationsDialog').open, 'adding more-specific Workspace opens recommendation preview for existing Environment');
    check(document.getElementById('environmentWorkspaceRecommendationsList').textContent.includes('Existing Workspace Drift') && document.getElementById('environmentWorkspaceRecommendationsList').textContent.includes('Added Workspace'), 'Workspace-add preview is filtered to drift caused by the newly registered Workspace');
    check(document.getElementById('environmentWorkspaceRecommendationsResult').textContent.includes('刚添加的 Workspace') && window.__fakeADM.calls.filter(c=>c.name==='SetEnvironmentWorkspace').length===workspaceSetBeforeAdd, 'Workspace-add recommendation is advisory and never silently rebinds existing Environment');
    document.querySelector('#environmentWorkspaceRecommendationsDialog [data-dialog-close]').click(); await sleep();

    const pageWidth=Math.max(document.documentElement.scrollWidth, document.body.scrollWidth);
    check(pageWidth<=window.innerWidth+2, 'no whole-window horizontal overflow');
    check(document.querySelectorAll('[data-management-page][hidden] :focus').length===0, 'hidden pages do not retain focus');
    result.ok=true;
  } catch(error) {
    result.ok=false; result.failures.push(error?.stack || error?.message || String(error));
    result.diagnostics = {
      dashboardState: document.getElementById('dashboardDataState')?.dataset.state || '',
      dashboardText: document.getElementById('dashboardDataState')?.textContent || '',
      statusText: document.getElementById('statusPanel')?.textContent || '',
      statusKind: document.getElementById('statusPanel')?.dataset.kind || '',
      calls: window.__fakeADM?.calls || [],
      browserErrors: window.__fakeADM?.browserErrors || [],
      navigationType: typeof window.ADMNavigation,
      dashboardType: typeof window.ADMDashboard,
      initializeProfilesType: typeof window.initializeConnectionProfiles,
      appElementsType: typeof window.elements,
      baseURI: document.baseURI,
    };
  } finally {
    const encoded=btoa(unescape(encodeURIComponent(JSON.stringify(result))));
    document.documentElement.setAttribute('data-browser-smoke', encoded);
  }
});
</script>`;

function htmlForFixture() {
  const absoluteAssets = productionHTML.replace(/(src|href)="\.\/([^"]+)"/g, (_match, attribute, relative) => {
    return `${attribute}="${pathToFileURL(path.join(frontend, relative)).href}"`;
  });
  return absoluteAssets
    .replace('<body>', `<body>\n${fakeBridge}`)
    .replace('</body>', `${runner}\n</body>`);
}

function runBrowser(width, height, scale = 1) {
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'adm-desktop-browser-'));
  const fixture = path.join(temp, 'index.html');
  fs.writeFileSync(fixture, htmlForFixture());
  const profile = path.join(temp, 'profile');
  const args = [
    '--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check', '--allow-file-access-from-files',
    `--user-data-dir=${profile}`, `--window-size=${width},${height}`, `--force-device-scale-factor=${scale}`,
    '--virtual-time-budget=45000', '--dump-dom', pathToFileURL(fixture).href,
  ];
  const execution = spawnSync(browser, args, {encoding:'utf8', timeout:100000, maxBuffer:20*1024*1024});
  try {
    if (execution.error) throw execution.error;
    if (execution.status !== 0) throw new Error(`browser exit ${execution.status}: ${execution.stderr}`);
    const match = execution.stdout.match(/data-browser-smoke="([^"]+)"/);
    if (!match) throw new Error(`browser smoke result missing; status=${execution.status} signal=${execution.signal} error=${execution.error?.message || ''} stderr=${execution.stderr.slice(-2000)}\nstdoutLen=${execution.stdout.length} stdoutTail=${execution.stdout.slice(-4000)}`);
    const result = JSON.parse(Buffer.from(match[1], 'base64').toString('utf8'));
    if (!result.ok) throw new Error(`browser smoke failed ${width}x${height}@${scale}: ${result.failures.join('\n')}\nDiagnostics: ${JSON.stringify(result.diagnostics || {})}`);
    return result;
  } finally {
    fs.rmSync(temp, {recursive:true, force:true});
  }
}

const scenarios = [
  [1120, 760, 1],
  [820, 560, 1],
  [1120, 760, 1.25],
];
for (const [width,height,scale] of scenarios) {
  const result = runBrowser(width,height,scale);
  console.log(`PASS browser ${width}x${height} scale=${scale} checks=${result.checks.length}`);
}
