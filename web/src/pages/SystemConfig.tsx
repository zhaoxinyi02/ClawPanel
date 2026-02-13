import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import {
  Settings, Save, RefreshCw, ChevronDown, ChevronRight,
  Brain, MessageSquare, Shield, Globe, Cpu, Terminal, Webhook,
  Users, Volume2, Eye, Palette, Key,
} from 'lucide-react';

type ConfigGroup = {
  id: string;
  label: string;
  icon: any;
  fields: ConfigField[];
};

type ConfigField = {
  path: string;
  label: string;
  type: 'text' | 'password' | 'number' | 'toggle' | 'textarea' | 'select';
  options?: string[];
  placeholder?: string;
  help?: string;
};

const CONFIG_GROUPS: ConfigGroup[] = [
  {
    id: 'models', label: '模型配置', icon: Brain,
    fields: [
      { path: 'agents.defaults.model.primary', label: '主模型', type: 'text', placeholder: 'anthropic/claude-sonnet-4-5', help: '格式: provider/model-name' },
      { path: 'agents.defaults.model.contextTokens', label: '上下文Token数', type: 'number', placeholder: '200000' },
      { path: 'agents.defaults.model.maxTokens', label: '最大输出Token', type: 'number', placeholder: '8192' },
    ],
  },
  {
    id: 'identity', label: '身份设置', icon: Users,
    fields: [
      { path: 'ui.assistant.name', label: '助手名称', type: 'text', placeholder: 'OpenClaw' },
      { path: 'ui.assistant.avatar', label: '助手头像', type: 'text', placeholder: 'emoji或URL', help: '支持emoji、短文本或图片URL' },
      { path: 'ui.seamColor', label: '主题色', type: 'text', placeholder: '#7c3aed', help: 'HEX颜色值' },
    ],
  },
  {
    id: 'messages', label: '消息配置', icon: MessageSquare,
    fields: [
      { path: 'messages.systemPrompt', label: '系统提示词', type: 'textarea', placeholder: '你是一个有帮助的AI助手...', help: '定义AI的角色和行为' },
      { path: 'messages.maxHistoryMessages', label: '最大历史消息数', type: 'number', placeholder: '50' },
      { path: 'messages.maxMessageLength', label: '最大消息长度', type: 'number', placeholder: '4000' },
    ],
  },
  {
    id: 'tools', label: '工具配置', icon: Terminal,
    fields: [
      { path: 'tools.mediaUnderstanding.enabled', label: '媒体理解', type: 'toggle', help: '启用图片/音频/视频理解' },
      { path: 'tools.webSearch.enabled', label: '网页搜索', type: 'toggle' },
      { path: 'tools.webSearch.provider', label: '搜索引擎', type: 'select', options: ['google', 'bing', 'duckduckgo', 'brave'] },
    ],
  },
  {
    id: 'gateway', label: '网关配置', icon: Globe,
    fields: [
      { path: 'gateway.port', label: '端口', type: 'number', placeholder: '18789' },
      { path: 'gateway.auth.mode', label: '认证模式', type: 'select', options: ['token', 'password'] },
      { path: 'gateway.auth.token', label: '认证Token', type: 'password' },
    ],
  },
  {
    id: 'hooks', label: 'Hooks', icon: Webhook,
    fields: [
      { path: 'hooks.enabled', label: '启用Hooks', type: 'toggle' },
      { path: 'hooks.basePath', label: '基础路径', type: 'text', placeholder: '/hooks' },
      { path: 'hooks.secret', label: 'Webhook密钥', type: 'password' },
    ],
  },
  {
    id: 'session', label: '会话配置', icon: Eye,
    fields: [
      { path: 'session.compaction.enabled', label: '自动压缩', type: 'toggle', help: '自动压缩过长的会话历史' },
      { path: 'session.compaction.threshold', label: '压缩阈值(tokens)', type: 'number', placeholder: '100000' },
      { path: 'session.pruning.enabled', label: '自动修剪', type: 'toggle' },
    ],
  },
  {
    id: 'browser', label: '浏览器', icon: Globe,
    fields: [
      { path: 'browser.enabled', label: '启用浏览器', type: 'toggle', help: '允许AI使用浏览器' },
      { path: 'browser.headless', label: '无头模式', type: 'toggle' },
    ],
  },
  {
    id: 'auth', label: '认证密钥', icon: Key,
    fields: [
      { path: 'env.vars.ANTHROPIC_API_KEY', label: 'Anthropic API Key', type: 'password' },
      { path: 'env.vars.OPENAI_API_KEY', label: 'OpenAI API Key', type: 'password' },
      { path: 'env.vars.GOOGLE_API_KEY', label: 'Google API Key', type: 'password' },
    ],
  },
];

export default function SystemConfig() {
  const [config, setConfig] = useState<any>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');
  const [expandedGroup, setExpandedGroup] = useState<string>('models');

  useEffect(() => { loadConfig(); }, []);

  const loadConfig = async () => {
    setLoading(true);
    try {
      const r = await api.getOpenClawConfig();
      if (r.ok) setConfig(r.config || {});
    } catch {}
    finally { setLoading(false); }
  };

  const getNestedValue = (obj: any, path: string): any => {
    return path.split('.').reduce((o, k) => o?.[k], obj);
  };

  const setNestedValue = (obj: any, path: string, value: any): any => {
    const clone = JSON.parse(JSON.stringify(obj));
    const keys = path.split('.');
    let current = clone;
    for (let i = 0; i < keys.length - 1; i++) {
      if (!current[keys[i]]) current[keys[i]] = {};
      current = current[keys[i]];
    }
    current[keys[keys.length - 1]] = value;
    return clone;
  };

  const updateField = (path: string, value: any) => {
    setConfig((prev: any) => setNestedValue(prev, path, value));
  };

  const handleSave = async () => {
    setSaving(true);
    setMsg('');
    try {
      await api.updateOpenClawConfig(config);
      setMsg('配置已保存，部分配置需要重启 OpenClaw 生效');
      setTimeout(() => setMsg(''), 4000);
    } catch (err) {
      setMsg('保存失败: ' + String(err));
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <div className="text-center py-12 text-gray-400 text-xs">加载配置中...</div>;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold">系统配置</h2>
          <p className="text-xs text-gray-500 mt-0.5">OpenClaw 深度配置 — 模型、提示词、工具、网关等</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={loadConfig} className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800 text-gray-600 dark:text-gray-400">
            <RefreshCw size={13} />重新加载
          </button>
          <button onClick={handleSave} disabled={saving}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50">
            <Save size={13} />{saving ? '保存中...' : '保存配置'}
          </button>
        </div>
      </div>

      {msg && (
        <div className={`px-3 py-2 rounded-lg text-xs ${msg.includes('失败') ? 'bg-red-50 dark:bg-red-950 text-red-600' : 'bg-emerald-50 dark:bg-emerald-950 text-emerald-600'}`}>
          {msg}
        </div>
      )}

      {/* Config groups */}
      <div className="space-y-2">
        {CONFIG_GROUPS.map(group => {
          const Icon = group.icon;
          const isExpanded = expandedGroup === group.id;
          return (
            <div key={group.id} className="card overflow-hidden">
              <button
                onClick={() => setExpandedGroup(isExpanded ? '' : group.id)}
                className="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
              >
                {isExpanded ? <ChevronDown size={14} className="text-gray-400" /> : <ChevronRight size={14} className="text-gray-400" />}
                <Icon size={16} className="text-violet-500 shrink-0" />
                <span className="text-sm font-medium flex-1">{group.label}</span>
                <span className="text-[10px] text-gray-400">{group.fields.length} 项</span>
              </button>
              {isExpanded && (
                <div className="px-4 pb-4 pt-1 border-t border-gray-100 dark:border-gray-800 space-y-3">
                  {group.fields.map(field => (
                    <div key={field.path}>
                      <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                        {field.label}
                        {field.help && <span className="text-gray-400 font-normal ml-1">— {field.help}</span>}
                      </label>
                      {field.type === 'toggle' ? (
                        <button
                          onClick={() => updateField(field.path, !getNestedValue(config, field.path))}
                          className={`relative w-10 h-5 rounded-full transition-colors ${getNestedValue(config, field.path) ? 'bg-violet-500' : 'bg-gray-300 dark:bg-gray-600'}`}>
                          <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${getNestedValue(config, field.path) ? 'translate-x-5' : ''}`} />
                        </button>
                      ) : field.type === 'textarea' ? (
                        <textarea
                          value={getNestedValue(config, field.path) || ''}
                          onChange={e => updateField(field.path, e.target.value)}
                          placeholder={field.placeholder}
                          rows={4}
                          className="w-full px-3 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-transparent resize-none font-mono"
                        />
                      ) : field.type === 'select' ? (
                        <select
                          value={getNestedValue(config, field.path) || ''}
                          onChange={e => updateField(field.path, e.target.value)}
                          className="w-full px-3 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-transparent"
                        >
                          <option value="">选择...</option>
                          {field.options?.map(o => <option key={o} value={o}>{o}</option>)}
                        </select>
                      ) : (
                        <input
                          type={field.type === 'password' ? 'password' : field.type === 'number' ? 'number' : 'text'}
                          value={getNestedValue(config, field.path) || ''}
                          onChange={e => updateField(field.path, field.type === 'number' ? Number(e.target.value) : e.target.value)}
                          placeholder={field.placeholder}
                          className="w-full px-3 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-transparent"
                        />
                      )}
                      <p className="text-[10px] text-gray-400 mt-0.5 font-mono">{field.path}</p>
                    </div>
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Raw config viewer */}
      <details className="card">
        <summary className="px-4 py-3 text-xs font-medium text-gray-500 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800/50">
          查看原始配置 (JSON)
        </summary>
        <pre className="px-4 pb-4 text-[11px] text-gray-600 dark:text-gray-400 overflow-x-auto max-h-96 overflow-y-auto font-mono">
          {JSON.stringify(config, null, 2)}
        </pre>
      </details>
    </div>
  );
}
