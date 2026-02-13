import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { Radio, Wifi, WifiOff, Settings2, ChevronDown, RefreshCw, QrCode, Key, Zap } from 'lucide-react';

type ChannelDef = {
  id: string;
  label: string;
  description: string;
  configFields: { key: string; label: string; type: 'text' | 'password' | 'toggle' | 'number' | 'select'; options?: string[]; placeholder?: string; help?: string }[];
  loginMethods?: ('qrcode' | 'quick' | 'password')[];
};

const CHANNEL_DEFS: ChannelDef[] = [
  {
    id: 'qq', label: 'QQ (NapCat)', description: 'QQ个人号，通过NapCat协议接入',
    loginMethods: ['qrcode', 'quick', 'password'],
    configFields: [
      { key: 'wsUrl', label: 'WebSocket 地址', type: 'text', placeholder: 'ws://127.0.0.1:3001' },
      { key: 'wakeProbability', label: '唤醒概率 (%)', type: 'number', help: '群聊中Bot回复的概率，0-100' },
      { key: 'minSendIntervalMs', label: '最小发送间隔 (ms)', type: 'number', help: '同一目标的最小发送间隔' },
      { key: 'wakeTrigger', label: '唤醒触发词', type: 'text', help: '逗号分隔，如: 念念,@bot' },
      { key: 'pokeReply', label: '戳一戳回复', type: 'toggle', help: '收到戳一戳时自动回复' },
      { key: 'pokeReplyText', label: '戳一戳回复内容', type: 'text', placeholder: '别戳我啦~' },
      { key: 'errorDedupMinutes', label: '错误去重时间 (分钟)', type: 'number', help: '同类API错误在此时间内只发送一次' },
      { key: 'autoApproveGroup', label: '自动同意加群', type: 'toggle' },
      { key: 'autoApproveFriend', label: '自动同意好友', type: 'toggle' },
    ],
  },
  {
    id: 'telegram', label: 'Telegram', description: 'Telegram Bot API',
    configFields: [
      { key: 'token', label: 'Bot Token', type: 'password', placeholder: '123456:ABC-DEF...' },
      { key: 'webhookUrl', label: 'Webhook URL (可选)', type: 'text', placeholder: 'https://your-domain.com/webhook' },
      { key: 'allowedChatIds', label: '允许的Chat ID', type: 'text', help: '逗号分隔，留空允许所有' },
    ],
  },
  {
    id: 'discord', label: 'Discord', description: 'Discord Bot API',
    configFields: [
      { key: 'token', label: 'Bot Token', type: 'password' },
      { key: 'applicationId', label: 'Application ID', type: 'text' },
      { key: 'guildIds', label: 'Guild IDs', type: 'text', help: '逗号分隔' },
    ],
  },
  {
    id: 'wechat', label: '微信', description: '微信个人号 (wechatbot-webhook)',
    loginMethods: ['qrcode'],
    configFields: [
      { key: 'webhookUrl', label: 'Webhook URL', type: 'text', placeholder: 'http://openclaw-wechat:3001' },
      { key: 'token', label: 'API Token', type: 'password' },
    ],
  },
  {
    id: 'whatsapp', label: 'WhatsApp', description: 'WhatsApp Web QR扫码',
    loginMethods: ['qrcode'],
    configFields: [],
  },
  {
    id: 'slack', label: 'Slack', description: 'Slack Socket Mode',
    configFields: [
      { key: 'appToken', label: 'App Token', type: 'password', placeholder: 'xapp-...' },
      { key: 'botToken', label: 'Bot Token', type: 'password', placeholder: 'xoxb-...' },
    ],
  },
  {
    id: 'signal', label: 'Signal', description: 'Signal (signal-cli REST)',
    configFields: [
      { key: 'apiUrl', label: 'REST API URL', type: 'text', placeholder: 'http://signal-cli:8080' },
      { key: 'phoneNumber', label: '手机号', type: 'text', placeholder: '+86...' },
    ],
  },
  {
    id: 'googlechat', label: 'Google Chat', description: 'Google Workspace Chat',
    configFields: [
      { key: 'serviceAccountKey', label: '服务账号 JSON', type: 'text', help: '粘贴服务账号密钥JSON' },
      { key: 'webhookUrl', label: 'Webhook URL', type: 'text' },
    ],
  },
];

export default function Channels() {
  const [status, setStatus] = useState<any>(null);
  const [selectedChannel, setSelectedChannel] = useState('qq');
  const [configs, setConfigs] = useState<Record<string, any>>({});
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');

  useEffect(() => {
    api.getStatus().then(r => { if (r.ok) setStatus(r); });
    api.getAdminConfig().then(r => { if (r.ok) setConfigs(r.config || {}); });
  }, []);

  const activeChannels = [
    { id: 'qq', connected: status?.napcat?.connected, label: status?.napcat?.connected ? `QQ: ${status.napcat.nickname || ''}` : 'QQ 未连接' },
    { id: 'wechat', connected: status?.wechat?.loggedIn, label: status?.wechat?.loggedIn ? `微信: ${status.wechat.name || ''}` : '微信 未连接' },
  ];

  const currentDef = CHANNEL_DEFS.find(c => c.id === selectedChannel);

  const handleSave = async () => {
    if (!currentDef) return;
    setSaving(true);
    setMsg('');
    try {
      await api.updateAdminConfig(configs);
      setMsg('保存成功');
      setTimeout(() => setMsg(''), 2000);
    } catch (err) {
      setMsg('保存失败: ' + String(err));
    } finally {
      setSaving(false);
    }
  };

  const updateField = (channelId: string, key: string, value: any) => {
    setConfigs(prev => ({
      ...prev,
      [channelId]: { ...(prev[channelId] || {}), [key]: value },
    }));
  };

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-bold">通道管理</h2>
        <p className="text-xs text-gray-500 mt-0.5">配置和管理所有消息通道</p>
      </div>

      {/* Active channels */}
      <div className="flex gap-2 flex-wrap">
        {activeChannels.map(ch => (
          <button key={ch.id} onClick={() => setSelectedChannel(ch.id)}
            className={`flex items-center gap-2 px-3 py-2 rounded-lg text-xs border transition-colors ${
              selectedChannel === ch.id
                ? 'border-violet-300 dark:border-violet-700 bg-violet-50 dark:bg-violet-950/50'
                : 'border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800'
            }`}>
            {ch.connected ? <Wifi size={14} className="text-emerald-500" /> : <WifiOff size={14} className="text-red-400" />}
            <span>{ch.label}</span>
          </button>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-4">
        {/* Channel selector */}
        <div className="card p-3 space-y-1">
          <h3 className="text-xs font-semibold text-gray-500 mb-2 px-1">所有通道</h3>
          {CHANNEL_DEFS.map(ch => (
            <button key={ch.id} onClick={() => setSelectedChannel(ch.id)}
              className={`w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-left text-sm transition-colors ${
                selectedChannel === ch.id
                  ? 'bg-violet-50 dark:bg-violet-950/50 text-violet-700 dark:text-violet-300 font-medium'
                  : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-800'
              }`}>
              <Radio size={14} />
              <div className="min-w-0">
                <div className="text-xs font-medium truncate">{ch.label}</div>
                <div className="text-[10px] text-gray-400 truncate">{ch.description}</div>
              </div>
            </button>
          ))}
        </div>

        {/* Channel config */}
        <div className="lg:col-span-3 card p-4 space-y-4">
          {currentDef && (
            <>
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="font-semibold text-sm">{currentDef.label} 配置</h3>
                  <p className="text-[11px] text-gray-500 mt-0.5">{currentDef.description}</p>
                </div>
                {currentDef.loginMethods && currentDef.loginMethods.length > 0 && (
                  <div className="flex gap-1.5">
                    {currentDef.loginMethods.includes('qrcode') && (
                      <button className="flex items-center gap-1 px-2.5 py-1.5 text-[11px] rounded-lg bg-blue-50 dark:bg-blue-950 text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900">
                        <QrCode size={12} />扫码登录
                      </button>
                    )}
                    {currentDef.loginMethods.includes('quick') && (
                      <button className="flex items-center gap-1 px-2.5 py-1.5 text-[11px] rounded-lg bg-emerald-50 dark:bg-emerald-950 text-emerald-600 dark:text-emerald-400 hover:bg-emerald-100 dark:hover:bg-emerald-900">
                        <Zap size={12} />快速登录
                      </button>
                    )}
                    {currentDef.loginMethods.includes('password') && (
                      <button className="flex items-center gap-1 px-2.5 py-1.5 text-[11px] rounded-lg bg-amber-50 dark:bg-amber-950 text-amber-600 dark:text-amber-400 hover:bg-amber-100 dark:hover:bg-amber-900">
                        <Key size={12} />账密登录
                      </button>
                    )}
                  </div>
                )}
              </div>

              <div className="space-y-3">
                {currentDef.configFields.map(field => (
                  <div key={field.key}>
                    <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                      {field.label}
                      {field.help && <span className="text-gray-400 font-normal ml-1">— {field.help}</span>}
                    </label>
                    {field.type === 'toggle' ? (
                      <button
                        onClick={() => updateField(currentDef.id, field.key, !configs[currentDef.id]?.[field.key])}
                        className={`relative w-10 h-5 rounded-full transition-colors ${configs[currentDef.id]?.[field.key] ? 'bg-violet-500' : 'bg-gray-300 dark:bg-gray-600'}`}>
                        <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${configs[currentDef.id]?.[field.key] ? 'translate-x-5' : ''}`} />
                      </button>
                    ) : (
                      <input
                        type={field.type === 'password' ? 'password' : field.type === 'number' ? 'number' : 'text'}
                        value={configs[currentDef.id]?.[field.key] || ''}
                        onChange={e => updateField(currentDef.id, field.key, field.type === 'number' ? Number(e.target.value) : e.target.value)}
                        placeholder={field.placeholder}
                        className="w-full px-3 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-transparent"
                      />
                    )}
                  </div>
                ))}
              </div>

              <div className="flex items-center gap-3 pt-2">
                <button onClick={handleSave} disabled={saving}
                  className="px-4 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50">
                  {saving ? '保存中...' : '保存配置'}
                </button>
                {msg && <span className={`text-xs ${msg.includes('失败') ? 'text-red-500' : 'text-emerald-500'}`}>{msg}</span>}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
