import { useEffect, useMemo, useState } from 'react';
import { api } from '../lib/api';
import { Plus, RefreshCw, Save, Trash2, ArrowUp, ArrowDown, Route, Bot, Settings } from 'lucide-react';

interface AgentItem {
  id: string;
  workspace?: string;
  agentDir?: string;
  model?: any;
  tools?: any;
  sandbox?: any;
  default?: boolean;
  sessions?: number;
  lastActive?: number;
}

interface AgentFormState {
  id: string;
  workspace: string;
  agentDir: string;
  isDefault: boolean;
  modelText: string;
  toolsText: string;
  sandboxText: string;
}

interface BindingDraft {
  name: string;
  agent: string;
  enabled: boolean;
  matchText: string;
}

interface PreviewResult {
  agent?: string;
  matchedBy?: string;
  trace?: string[];
}

export default function Agents() {
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');

  const [defaultAgent, setDefaultAgent] = useState('main');
  const [agents, setAgents] = useState<AgentItem[]>([]);
  const [bindings, setBindings] = useState<BindingDraft[]>([]);

  const [editingId, setEditingId] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState<AgentFormState>({
    id: '',
    workspace: '',
    agentDir: '',
    isDefault: false,
    modelText: '',
    toolsText: '',
    sandboxText: '',
  });

  const [previewMeta, setPreviewMeta] = useState<Record<string, string>>({
    channel: '',
    sender: '',
    peer: '',
    parentPeer: '',
    guildId: '',
    teamId: '',
    accountId: '',
    groupId: '',
    roles: '',
  });
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewResult, setPreviewResult] = useState<PreviewResult | null>(null);

  const agentOptions = useMemo(() => {
    return agents.map(a => a.id).filter(Boolean);
  }, [agents]);

  const loadData = async () => {
    setLoading(true);
    try {
      const res = await api.getAgentsConfig();
      if (res.ok) {
        const data = res.agents || {};
        const list: AgentItem[] = data.list || [];
        const incomingBindings = (data.bindings || []) as any[];
        setDefaultAgent(data.default || 'main');
        setAgents(list);
        setBindings(incomingBindings.map((b: any) => ({
          name: String(b?.name || ''),
          agent: String(b?.agentId || b?.agent || data.default || 'main'),
          enabled: b?.enabled !== false,
          matchText: JSON.stringify(b?.match || {}, null, 2),
        })));
      } else {
        setAgents([]);
        setBindings([]);
      }
    } catch {
      setAgents([]);
      setBindings([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const openCreate = () => {
    setEditingId(null);
    setForm({
      id: '',
      workspace: '',
      agentDir: '',
      isDefault: false,
      modelText: '',
      toolsText: '',
      sandboxText: '',
    });
    setShowForm(true);
  };

  const openEdit = (agent: AgentItem) => {
    setEditingId(agent.id);
    setForm({
      id: agent.id,
      workspace: agent.workspace || '',
      agentDir: agent.agentDir || '',
      isDefault: !!agent.default,
      modelText: agent.model ? JSON.stringify(agent.model, null, 2) : '',
      toolsText: agent.tools ? JSON.stringify(agent.tools, null, 2) : '',
      sandboxText: agent.sandbox ? JSON.stringify(agent.sandbox, null, 2) : '',
    });
    setShowForm(true);
  };

  const parseJSONText = (raw: string, fieldName: string) => {
    const text = raw.trim();
    if (!text) return undefined;
    try {
      return JSON.parse(text);
    } catch (err) {
      throw new Error(`${fieldName} JSON 格式错误: ${String(err)}`);
    }
  };

  const saveAgent = async () => {
    const id = form.id.trim();
    if (!id) {
      setMsg('Agent ID 不能为空');
      return;
    }
    let modelObj: any;
    let toolsObj: any;
    let sandboxObj: any;
    try {
      modelObj = parseJSONText(form.modelText, 'model');
      toolsObj = parseJSONText(form.toolsText, 'tools');
      sandboxObj = parseJSONText(form.sandboxText, 'sandbox');
    } catch (err) {
      setMsg(String(err));
      return;
    }

    const payload: any = {
      id,
      workspace: form.workspace.trim() || undefined,
      agentDir: form.agentDir.trim() || undefined,
      default: form.isDefault,
    };
    if (modelObj !== undefined) payload.model = modelObj;
    if (toolsObj !== undefined) payload.tools = toolsObj;
    if (sandboxObj !== undefined) payload.sandbox = sandboxObj;

    setSaving(true);
    try {
      if (editingId) {
        await api.updateAgent(editingId, payload);
      } else {
        await api.createAgent(payload);
      }
      setMsg('Agent 保存成功');
      setShowForm(false);
      await loadData();
    } catch (err) {
      setMsg('保存失败: ' + String(err));
    } finally {
      setSaving(false);
      setTimeout(() => setMsg(''), 4000);
    }
  };

  const deleteAgent = async (agent: AgentItem) => {
    if (!window.confirm(`确认删除 Agent "${agent.id}"？`)) return;
    const preserveSessions = window.confirm('是否保留该 Agent 的 sessions 文件？\n确定=保留，取消=删除');
    try {
      await api.deleteAgent(agent.id, preserveSessions);
      setMsg('删除成功');
      await loadData();
    } catch (err) {
      setMsg('删除失败: ' + String(err));
    } finally {
      setTimeout(() => setMsg(''), 4000);
    }
  };

  const saveBindings = async () => {
    const parsed: any[] = [];
    for (let i = 0; i < bindings.length; i++) {
      const row = bindings[i];
      if (!row.agent.trim()) {
        setMsg(`第 ${i + 1} 条 binding 缺少 agent`);
        return;
      }
      let matchObj: any;
      try {
        matchObj = JSON.parse(row.matchText || '{}');
      } catch (err) {
        setMsg(`第 ${i + 1} 条 binding 的 match JSON 错误: ${String(err)}`);
        return;
      }
      parsed.push({
        name: row.name.trim() || undefined,
        agentId: row.agent.trim(),
        enabled: row.enabled,
        match: matchObj,
      });
    }

    setSaving(true);
    try {
      await api.updateBindings(parsed);
      setMsg('Bindings 保存成功');
      await loadData();
    } catch (err) {
      setMsg('Bindings 保存失败: ' + String(err));
    } finally {
      setSaving(false);
      setTimeout(() => setMsg(''), 4000);
    }
  };

  const addBinding = () => {
    setBindings(prev => [...prev, {
      name: '',
      agent: defaultAgent || agentOptions[0] || 'main',
      enabled: true,
      matchText: '{\n  "channel": "qq"\n}',
    }]);
  };

  const removeBinding = (idx: number) => {
    setBindings(prev => prev.filter((_, i) => i !== idx));
  };

  const moveBinding = (idx: number, delta: number) => {
    const to = idx + delta;
    if (to < 0 || to >= bindings.length) return;
    setBindings(prev => {
      const arr = [...prev];
      const [item] = arr.splice(idx, 1);
      arr.splice(to, 0, item);
      return arr;
    });
  };

  const runPreview = async () => {
    const meta: Record<string, any> = {};
    Object.entries(previewMeta).forEach(([k, v]) => {
      if (!v.trim()) return;
      if (k === 'roles') {
        const roles = v.split(',').map(x => x.trim()).filter(Boolean);
        if (roles.length > 0) meta[k] = roles;
        return;
      }
      meta[k] = v.trim();
    });

    setPreviewLoading(true);
    try {
      const r = await api.previewRoute(meta);
      if (r.ok) {
        setPreviewResult(r.result || {});
      } else {
        setPreviewResult({ trace: [r.error || '预览失败'] });
      }
    } catch (err) {
      setPreviewResult({ trace: [String(err)] });
    } finally {
      setPreviewLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="py-16 text-center text-gray-400 text-sm">
        <RefreshCw size={18} className="animate-spin inline mr-2" />
        加载中...
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-bold text-gray-900 dark:text-white">Agents</h2>
          <p className="text-sm text-gray-500 mt-1">管理 OpenClaw 多智能体、bindings 路由规则和命中预览</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={loadData} className="flex items-center gap-2 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors shadow-sm">
            <RefreshCw size={14} /> 刷新
          </button>
          <button onClick={openCreate} className="flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 shadow-sm shadow-violet-200 dark:shadow-none transition-all">
            <Plus size={14} /> 新建 Agent
          </button>
        </div>
      </div>

      {msg && (
        <div className={`px-4 py-3 rounded-lg text-sm ${msg.includes('失败') || msg.includes('错误') ? 'bg-red-50 dark:bg-red-900/20 text-red-600' : 'bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600'}`}>
          {msg}
        </div>
      )}

      <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-100 dark:border-gray-700/50 shadow-sm">
        <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-700/50 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <Bot size={15} className="text-violet-500" />
            Agent 列表
          </h3>
          <span className="text-xs text-gray-500">默认: {defaultAgent}</span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-xs text-gray-500 border-b border-gray-100 dark:border-gray-700/50">
                <th className="px-4 py-2">ID</th>
                <th className="px-4 py-2">Workspace</th>
                <th className="px-4 py-2">AgentDir</th>
                <th className="px-4 py-2">会话数</th>
                <th className="px-4 py-2">最后活跃</th>
                <th className="px-4 py-2">操作</th>
              </tr>
            </thead>
            <tbody>
              {agents.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-6 text-center text-gray-400 text-xs">暂无 Agent</td>
                </tr>
              ) : agents.map(agent => (
                <tr key={agent.id} className="border-b border-gray-50 dark:border-gray-700/30">
                  <td className="px-4 py-3">
                    <span className="font-mono text-xs">{agent.id}</span>
                    {agent.default && <span className="ml-2 text-[10px] px-1.5 py-0.5 rounded bg-violet-100 text-violet-700">DEFAULT</span>}
                  </td>
                  <td className="px-4 py-3 text-xs text-gray-600 dark:text-gray-300">{agent.workspace || '-'}</td>
                  <td className="px-4 py-3 text-xs text-gray-600 dark:text-gray-300">{agent.agentDir || '-'}</td>
                  <td className="px-4 py-3 text-xs">{agent.sessions ?? 0}</td>
                  <td className="px-4 py-3 text-xs text-gray-500">{agent.lastActive ? new Date(agent.lastActive).toLocaleString('zh-CN') : '-'}</td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <button onClick={() => openEdit(agent)} className="px-2 py-1 text-xs rounded bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600">编辑</button>
                      <button onClick={() => deleteAgent(agent)} className="px-2 py-1 text-xs rounded bg-red-50 text-red-600 hover:bg-red-100">删除</button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-100 dark:border-gray-700/50 shadow-sm">
        <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-700/50 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <Settings size={15} className="text-violet-500" />
            Bindings（按顺序匹配）
          </h3>
          <div className="flex items-center gap-2">
            <button onClick={addBinding} className="px-2 py-1 text-xs rounded bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600">新增规则</button>
            <button onClick={saveBindings} disabled={saving} className="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50">
              <Save size={12} /> 保存 Bindings
            </button>
          </div>
        </div>
        <div className="p-4 space-y-3">
          {bindings.length === 0 && (
            <div className="text-xs text-gray-400">暂无 bindings，消息将落到默认 Agent。</div>
          )}
          {bindings.map((row, idx) => (
            <div key={idx} className="border border-gray-100 dark:border-gray-700 rounded-lg p-3 space-y-2">
              <div className="flex items-center gap-2">
                <input
                  value={row.name}
                  onChange={e => setBindings(prev => prev.map((x, i) => i === idx ? { ...x, name: e.target.value } : x))}
                  placeholder="规则名（可选）"
                  className="px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                />
                <select
                  value={row.agent}
                  onChange={e => setBindings(prev => prev.map((x, i) => i === idx ? { ...x, agent: e.target.value } : x))}
                  className="px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                >
                  {(agentOptions.length ? agentOptions : ['main']).map(id => (
                    <option key={id} value={id}>{id}</option>
                  ))}
                </select>
                <label className="text-xs text-gray-600 flex items-center gap-1">
                  <input
                    type="checkbox"
                    checked={row.enabled}
                    onChange={e => setBindings(prev => prev.map((x, i) => i === idx ? { ...x, enabled: e.target.checked } : x))}
                  />
                  启用
                </label>
                <button onClick={() => moveBinding(idx, -1)} className="p-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700" title="上移"><ArrowUp size={13} /></button>
                <button onClick={() => moveBinding(idx, 1)} className="p-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700" title="下移"><ArrowDown size={13} /></button>
                <button onClick={() => removeBinding(idx)} className="p-1 rounded text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20" title="删除"><Trash2 size={13} /></button>
              </div>
              <textarea
                value={row.matchText}
                onChange={e => setBindings(prev => prev.map((x, i) => i === idx ? { ...x, matchText: e.target.value } : x))}
                rows={4}
                className="w-full font-mono text-xs px-2 py-2 border border-gray-200 dark:border-gray-700 rounded bg-gray-50 dark:bg-gray-900"
              />
            </div>
          ))}
        </div>
      </div>

      <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-100 dark:border-gray-700/50 shadow-sm">
        <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-700/50 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <Route size={15} className="text-violet-500" />
            路由预览
          </h3>
          <button onClick={runPreview} disabled={previewLoading} className="px-3 py-1.5 text-xs rounded bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50">
            {previewLoading ? '预览中...' : '执行预览'}
          </button>
        </div>
        <div className="p-4 grid grid-cols-1 md:grid-cols-3 gap-3">
          {Object.keys(previewMeta).map(key => (
            <div key={key}>
              <label className="text-xs text-gray-500">{key}</label>
              <input
                value={previewMeta[key] || ''}
                onChange={e => setPreviewMeta(prev => ({ ...prev, [key]: e.target.value }))}
                className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
              />
            </div>
          ))}
        </div>
        {previewResult && (
          <div className="px-4 pb-4">
            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-100 dark:border-gray-700 p-3 text-xs space-y-2">
              <div><span className="text-gray-500">命中 Agent:</span> <span className="font-mono text-violet-600">{previewResult.agent || '-'}</span></div>
              <div><span className="text-gray-500">匹配来源:</span> <span className="font-mono">{previewResult.matchedBy || '-'}</span></div>
              <div>
                <div className="text-gray-500 mb-1">Trace:</div>
                <ul className="list-disc pl-4 space-y-1">
                  {(previewResult.trace || []).map((line, i) => (
                    <li key={i} className="font-mono">{line}</li>
                  ))}
                </ul>
              </div>
            </div>
          </div>
        )}
      </div>

      {showForm && (
        <div className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center p-4">
          <div className="w-full max-w-3xl max-h-[90vh] overflow-y-auto rounded-xl bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 shadow-xl">
            <div className="px-5 py-4 border-b border-gray-100 dark:border-gray-700 flex items-center justify-between">
              <h3 className="font-semibold text-gray-900 dark:text-white">{editingId ? `编辑 Agent: ${editingId}` : '新建 Agent'}</h3>
              <button onClick={() => setShowForm(false)} className="text-xs px-2 py-1 rounded bg-gray-100 dark:bg-gray-700">关闭</button>
            </div>
            <div className="p-5 space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                <div>
                  <label className="text-xs text-gray-500">ID</label>
                  <input
                    value={form.id}
                    disabled={!!editingId}
                    onChange={e => setForm(prev => ({ ...prev, id: e.target.value }))}
                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900 disabled:opacity-60"
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-500">Workspace</label>
                  <input
                    value={form.workspace}
                    onChange={e => setForm(prev => ({ ...prev, workspace: e.target.value }))}
                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-500">AgentDir</label>
                  <input
                    value={form.agentDir}
                    onChange={e => setForm(prev => ({ ...prev, agentDir: e.target.value }))}
                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                  />
                </div>
              </div>
              <label className="text-xs text-gray-600 flex items-center gap-2">
                <input type="checkbox" checked={form.isDefault} onChange={e => setForm(prev => ({ ...prev, isDefault: e.target.checked }))} />
                设为默认 Agent
              </label>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                <div>
                  <label className="text-xs text-gray-500">model (JSON)</label>
                  <textarea
                    rows={8}
                    value={form.modelText}
                    onChange={e => setForm(prev => ({ ...prev, modelText: e.target.value }))}
                    className="w-full mt-1 px-2 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded bg-gray-50 dark:bg-gray-900"
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-500">tools (JSON)</label>
                  <textarea
                    rows={8}
                    value={form.toolsText}
                    onChange={e => setForm(prev => ({ ...prev, toolsText: e.target.value }))}
                    className="w-full mt-1 px-2 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded bg-gray-50 dark:bg-gray-900"
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-500">sandbox (JSON)</label>
                  <textarea
                    rows={8}
                    value={form.sandboxText}
                    onChange={e => setForm(prev => ({ ...prev, sandboxText: e.target.value }))}
                    className="w-full mt-1 px-2 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded bg-gray-50 dark:bg-gray-900"
                  />
                </div>
              </div>
            </div>
            <div className="px-5 py-4 border-t border-gray-100 dark:border-gray-700 flex items-center justify-end gap-2">
              <button onClick={() => setShowForm(false)} className="px-4 py-2 text-xs rounded bg-gray-100 dark:bg-gray-700">取消</button>
              <button onClick={saveAgent} disabled={saving} className="px-4 py-2 text-xs rounded bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50">
                {saving ? '保存中...' : '保存'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
