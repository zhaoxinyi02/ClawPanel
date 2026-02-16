import { useEffect, useState, useCallback } from 'react';
import { api } from '../lib/api';
import { Radio, Wifi, WifiOff, QrCode, Key, Zap, UserCheck, Check, X, Power, Loader2, RefreshCw, LogOut, Sparkles } from 'lucide-react';
import { useI18n } from '../i18n';

type ChannelDef = {
  id: string; label: string; description: string; type: 'builtin' | 'plugin';
  configFields: { key: string; label: string; type: 'text' | 'password' | 'toggle' | 'number' | 'select'; options?: string[]; placeholder?: string; help?: string }[];
  loginMethods?: ('qrcode' | 'quick' | 'password')[];
};

const CHANNEL_DEFS: ChannelDef[] = [
  { id: 'qq', label: 'QQ (NapCat)', description: 'QQ个人号，NapCat OneBot11协议', type: 'builtin',
    loginMethods: ['qrcode', 'quick', 'password'],
    configFields: [
      { key: 'wsUrl', label: 'WebSocket 地址', type: 'text', placeholder: 'ws://127.0.0.1:3001' },
      { key: 'accessToken', label: 'Access Token', type: 'password' },
      { key: 'ownerQQ', label: '主人QQ号', type: 'number', help: '接收通知的QQ号' },
      { key: 'wakeProbability', label: '唤醒概率 (%)', type: 'number', help: '群聊中Bot回复的概率，0-100' },
      { key: 'minSendIntervalMs', label: '最小发送间隔 (ms)', type: 'number' },
      { key: 'wakeTrigger', label: '唤醒触发词', type: 'text', help: '逗号分隔' },
      { key: 'pokeReplyText', label: '戳一戳回复内容', type: 'text', placeholder: '别戳我啦~', help: '自定义戳一戳回复文本' },
      { key: 'autoApproveGroup', label: '自动同意加群', type: 'toggle' },
      { key: 'autoApproveFriend', label: '自动同意好友', type: 'toggle' },
      { key: 'notifications.antiRecall', label: '防撤回通知', type: 'toggle', help: '撤回消息时发送通知' },
      { key: 'notifications.memberChange', label: '成员变动通知', type: 'toggle', help: '群成员加入/退出通知' },
      { key: 'notifications.adminChange', label: '管理员变动通知', type: 'toggle', help: '管理员设置/取消通知' },
      { key: 'notifications.banNotice', label: '禁言通知', type: 'toggle', help: '禁言/解禁通知' },
      { key: 'notifications.pokeReply', label: '戳一戳回复', type: 'toggle', help: '收到戳一戳时自动回复' },
      { key: 'notifications.honorNotice', label: '荣誉通知', type: 'toggle', help: '群荣誉变动通知' },
      { key: 'notifications.fileUpload', label: '文件上传通知', type: 'toggle', help: '群文件上传通知' },
      { key: 'welcome.enabled', label: '入群欢迎', type: 'toggle', help: '新成员入群时发送欢迎消息' },
      { key: 'welcome.template', label: '欢迎模板', type: 'text', placeholder: '欢迎 {nickname} 加入本群！' },
    ] },
  { id: 'wechat', label: '微信', description: '微信个人号 (wechatbot-webhook)', type: 'builtin',
    loginMethods: ['qrcode'],
    configFields: [
      { key: 'webhookUrl', label: 'Webhook URL', type: 'text', placeholder: 'http://openclaw-wechat:3001' },
      { key: 'token', label: 'API Token', type: 'password' },
    ] },
  { id: 'whatsapp', label: 'WhatsApp', description: 'Baileys QR扫码配对', type: 'builtin', loginMethods: ['qrcode'],
    configFields: [{ key: 'dmPolicy', label: 'DM策略', type: 'select', options: ['pairing','open','allowlist'] }] },
  { id: 'telegram', label: 'Telegram', description: 'Bot API via grammŸ，支持群组', type: 'builtin',
    configFields: [
      { key: 'token', label: 'Bot Token', type: 'password', placeholder: '123456:ABC-DEF...' },
      { key: 'webhookUrl', label: 'Webhook URL', type: 'text' },
    ] },
  { id: 'discord', label: 'Discord', description: 'Discord Bot API + Gateway', type: 'builtin',
    configFields: [
      { key: 'token', label: 'Bot Token', type: 'password' },
      { key: 'applicationId', label: 'Application ID', type: 'text' },
      { key: 'guildIds', label: 'Guild IDs', type: 'text', help: '逗号分隔' },
    ] },
  { id: 'irc', label: 'IRC', description: '经典IRC服务器', type: 'builtin',
    configFields: [
      { key: 'server', label: '服务器', type: 'text', placeholder: 'irc.libera.chat' },
      { key: 'nick', label: '昵称', type: 'text' },
      { key: 'channels', label: '频道', type: 'text', help: '逗号分隔' },
    ] },
  { id: 'slack', label: 'Slack', description: 'Bolt SDK，工作区应用', type: 'builtin',
    configFields: [
      { key: 'appToken', label: 'App Token', type: 'password', placeholder: 'xapp-...' },
      { key: 'botToken', label: 'Bot Token', type: 'password', placeholder: 'xoxb-...' },
    ] },
  { id: 'signal', label: 'Signal', description: 'signal-cli REST API', type: 'builtin',
    configFields: [
      { key: 'apiUrl', label: 'REST API URL', type: 'text', placeholder: 'http://signal-cli:8080' },
      { key: 'phoneNumber', label: '手机号', type: 'text' },
    ] },
  { id: 'googlechat', label: 'Google Chat', description: 'Google Chat API Webhook', type: 'builtin',
    configFields: [
      { key: 'serviceAccountKey', label: '服务账号 JSON', type: 'text' },
      { key: 'webhookUrl', label: 'Webhook URL', type: 'text' },
    ] },
  { id: 'bluebubbles', label: 'BlueBubbles (iMessage)', description: 'macOS iMessage', type: 'builtin',
    configFields: [
      { key: 'serverUrl', label: '服务器URL', type: 'text' },
      { key: 'password', label: '密码', type: 'password' },
    ] },
  { id: 'webchat', label: 'WebChat', description: 'Gateway WebChat UI (内置)', type: 'builtin', configFields: [] },
  // Plugin channels
  { id: 'feishu', label: '飞书 / Lark', description: '飞书机器人 WebSocket (插件)', type: 'plugin',
    configFields: [
      { key: 'appId', label: 'App ID', type: 'text' },
      { key: 'appSecret', label: 'App Secret', type: 'password' },
    ] },
  { id: 'qqbot', label: 'QQ 官方机器人', description: 'QQ开放平台官方Bot API (插件)', type: 'plugin',
    configFields: [
      { key: 'appId', label: 'App ID', type: 'text' },
      { key: 'clientSecret', label: 'Client Secret', type: 'password' },
    ] },
  { id: 'dingtalk', label: '钉钉', description: '钉钉机器人 (插件)', type: 'plugin',
    configFields: [
      { key: 'appKey', label: 'App Key', type: 'text' },
      { key: 'appSecret', label: 'App Secret', type: 'password' },
      { key: 'robotCode', label: 'Robot Code', type: 'text' },
    ] },
  { id: 'wecom', label: '企业微信', description: '企业微信应用消息 (插件)', type: 'plugin',
    configFields: [
      { key: 'corpId', label: 'Corp ID', type: 'text' },
      { key: 'agentId', label: 'Agent ID', type: 'text' },
      { key: 'secret', label: 'Secret', type: 'password' },
    ] },
  { id: 'msteams', label: 'Microsoft Teams', description: 'Bot Framework (插件)', type: 'plugin',
    configFields: [
      { key: 'appId', label: 'App ID', type: 'text' },
      { key: 'appPassword', label: 'App Password', type: 'password' },
    ] },
  { id: 'mattermost', label: 'Mattermost', description: 'Bot API + WebSocket (插件)', type: 'plugin',
    configFields: [
      { key: 'url', label: '服务器URL', type: 'text' },
      { key: 'token', label: 'Bot Token', type: 'password' },
    ] },
  { id: 'line', label: 'LINE', description: 'LINE Messaging API (插件)', type: 'plugin',
    configFields: [
      { key: 'channelAccessToken', label: 'Channel Access Token', type: 'password' },
      { key: 'channelSecret', label: 'Channel Secret', type: 'password' },
    ] },
  { id: 'matrix', label: 'Matrix', description: 'Matrix 协议 (插件)', type: 'plugin',
    configFields: [
      { key: 'homeserverUrl', label: 'Homeserver URL', type: 'text' },
      { key: 'accessToken', label: 'Access Token', type: 'password' },
    ] },
  { id: 'twitch', label: 'Twitch', description: 'Twitch Chat via IRC (插件)', type: 'plugin',
    configFields: [
      { key: 'username', label: '用户名', type: 'text' },
      { key: 'oauthToken', label: 'OAuth Token', type: 'password' },
      { key: 'channels', label: '频道', type: 'text', help: '逗号分隔' },
    ] },
];

// Determine channel status: 'enabled' (green), 'configured' (red/orange), 'unconfigured' (gray)
function getChannelStatus(ch: ChannelDef, ocConfig: any): 'enabled' | 'configured' | 'unconfigured' {
  const chConf = ocConfig?.channels?.[ch.id] || {};
  const pluginConf = ocConfig?.plugins?.entries?.[ch.id] || {};
  const isEnabled = chConf.enabled || pluginConf.enabled;
  // Check if any config field has a value
  const hasConfig = ch.configFields.some(f => {
    const v = chConf[f.key];
    return v !== undefined && v !== null && v !== '';
  });
  if (isEnabled) return 'enabled';
  if (hasConfig) return 'configured';
  return 'unconfigured';
}

function statusDot(s: 'enabled' | 'configured' | 'unconfigured') {
  if (s === 'enabled') return 'bg-emerald-500';
  if (s === 'configured') return 'bg-red-400';
  return 'bg-gray-300 dark:bg-gray-600';
}

// statusLabel is now inside the component to access i18n

export default function Channels() {
  const { t } = useI18n();

  const statusLabel = (s: 'enabled' | 'configured' | 'unconfigured') => {
    if (s === 'enabled') return t.channels.statusEnabled;
    if (s === 'configured') return t.channels.statusConfigured;
    return t.channels.statusUnconfigured;
  };

  const [status, setStatus] = useState<any>(null);
  const [selectedChannel, setSelectedChannel] = useState('qq');
  const [ocConfig, setOcConfig] = useState<any>({});
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');
  const [requests, setRequests] = useState<any[]>([]);
  // QQ Login state
  const [loginModal, setLoginModal] = useState<'qrcode' | 'quick' | 'password' | null>(null);
  const [qrImg, setQrImg] = useState('');
  const [qrLoading, setQrLoading] = useState(false);
  const [quickList, setQuickList] = useState<string[]>([]);
  const [loginUin, setLoginUin] = useState('');
  const [loginPwd, setLoginPwd] = useState('');
  const [loginLoading, setLoginLoading] = useState(false);
  const [loginMsg, setLoginMsg] = useState('');

  const reload = () => {
    api.getStatus().then(r => { if (r.ok) setStatus(r); });
    api.getOpenClawConfig().then(r => { if (r.ok) setOcConfig(r.config || {}); });
    api.getRequests().then(r => { if (r.ok) setRequests(r.requests || []); });
  };

  useEffect(() => { reload(); }, []);

  const ocChannels = ocConfig?.channels || {};
  const ocPlugins = ocConfig?.plugins?.entries || {};

  // Get the merged config for the current channel (supports nested keys like notifications.antiRecall)
  const getFieldValue = (channelId: string, key: string) => {
    const chConf = ocChannels[channelId] || {};
    return key.split('.').reduce((o: any, k: string) => o?.[k], chConf);
  };

  const isChannelEnabled = (channelId: string) => {
    return ocChannels[channelId]?.enabled || ocPlugins[channelId]?.enabled || false;
  };

  const currentDef = CHANNEL_DEFS.find(c => c.id === selectedChannel);

  const handleToggleEnabled = async (channelId: string) => {
    const newEnabled = !isChannelEnabled(channelId);
    try {
      const r = await api.toggleChannel(channelId, newEnabled);
      if (r.ok) {
        setMsg(r.message || (newEnabled ? t.channels.channelEnabled : t.channels.channelDisabled));
        if (channelId === 'qq' && !newEnabled) {
          setMsg(t.channels.qqClosing);
        }
      } else {
        setMsg(r.error || t.common.operationFailed);
      }
      reload();
      setTimeout(() => setMsg(''), 5000);
    } catch (err) { setMsg(t.common.operationFailed + ': ' + String(err)); setTimeout(() => setMsg(''), 3000); }
  };

  const handleSave = async () => {
    if (!currentDef) return;
    setSaving(true); setMsg('');
    try {
      // Collect values from form inputs
      const formEl = document.getElementById('channel-config-form') as HTMLFormElement;
      if (!formEl) return;
      const formData = new FormData(formEl);
      const chData: any = JSON.parse(JSON.stringify(ocChannels[currentDef.id] || {}));
      for (const f of currentDef.configFields) {
        if (f.type === 'toggle') continue; // toggles handled separately via handleToggleField
        const val = formData.get(f.key);
        if (val !== null && val !== '') {
          const parsed = f.type === 'number' ? Number(val) : val;
          // Support nested keys like welcome.template
          const keys = f.key.split('.');
          if (keys.length === 1) {
            chData[f.key] = parsed;
          } else {
            let cur = chData;
            for (let i = 0; i < keys.length - 1; i++) { if (!cur[keys[i]]) cur[keys[i]] = {}; cur = cur[keys[i]]; }
            cur[keys[keys.length - 1]] = parsed;
          }
        }
      }
      await api.updateChannel(currentDef.id, chData);
      if (currentDef.type === 'plugin') await api.updatePlugin(currentDef.id, { enabled: chData.enabled || false });
      setMsg(t.channels.saveSuccess);
      reload();
      setTimeout(() => setMsg(''), 2000);
    } catch (err) { setMsg(t.channels.saveFailed + ': ' + String(err)); }
    finally { setSaving(false); }
  };

  const handleToggleField = async (channelId: string, key: string) => {
    const chConf = JSON.parse(JSON.stringify(ocChannels[channelId] || {}));
    const keys = key.split('.');
    if (keys.length === 1) {
      chConf[key] = !chConf[key];
    } else {
      let cur = chConf;
      for (let i = 0; i < keys.length - 1; i++) { if (!cur[keys[i]]) cur[keys[i]] = {}; cur = cur[keys[i]]; }
      cur[keys[keys.length - 1]] = !cur[keys[keys.length - 1]];
    }
    try {
      await api.updateChannel(channelId, chConf);
      reload();
    } catch {}
  };

  // === QQ Login handlers ===
  const handleQRLogin = async () => {
    setLoginModal('qrcode'); setQrImg(''); setQrLoading(true); setLoginMsg('');
    try {
      const r = await api.napcatGetQRCode();
      if (r.ok && r.data?.qrcode) {
        setQrImg(r.data.qrcode);
      } else if (r.message?.includes('Logined') || r.data?.message?.includes('Logined')) {
        setLoginMsg('QQ 已登录，无需重复登录');
      } else {
        setLoginMsg(r.message || r.data?.message || r.error || '获取二维码失败');
      }
    } catch (err) { setLoginMsg('获取二维码失败: ' + String(err)); }
    finally { setQrLoading(false); }
  };

  const handleRefreshQR = async () => {
    setQrLoading(true); setLoginMsg('');
    try {
      const r = await api.napcatRefreshQRCode();
      if (r.ok && r.data?.qrcode) {
        setQrImg(r.data.qrcode);
      } else {
        setLoginMsg(r.data?.message || '刷新失败');
      }
    } catch { setLoginMsg('刷新失败'); }
    finally { setQrLoading(false); }
  };

  const handleQuickLoginOpen = async () => {
    setLoginModal('quick'); setQuickList([]); setLoginLoading(true); setLoginMsg('');
    try {
      const r = await api.napcatQuickLoginList();
      if (r.ok && r.data) {
        const list = Array.isArray(r.data) ? r.data : (r.data.QuickLoginList || r.data.quickLoginList || []);
        setQuickList(list.map((item: any) => typeof item === 'string' ? item : item.uin || String(item)));
        if (list.length === 0 && (r.message?.includes('Logined') || r.data?.message?.includes('Logined'))) {
          setLoginMsg('QQ 已登录，无需重复登录');
        }
      } else { setLoginMsg(r.message || r.error || '获取快速登录列表失败'); }
    } catch (err) { setLoginMsg('获取快速登录列表失败: ' + String(err)); }
    finally { setLoginLoading(false); }
  };

  const handleQuickLogin = async (uin: string) => {
    setLoginLoading(true); setLoginMsg('');
    try {
      const r = await api.napcatQuickLogin(uin);
      if (r.ok && (r.code === 0 || r.message?.includes('Logined'))) { setLoginMsg('登录成功！'); reload(); setTimeout(() => setLoginModal(null), 1500); }
      else { setLoginMsg(r.message || r.data?.message || r.error || '快速登录失败'); }
    } catch (err) { setLoginMsg('快速登录失败: ' + String(err)); }
    finally { setLoginLoading(false); }
  };

  const handlePasswordLoginOpen = () => {
    setLoginModal('password'); setLoginUin(''); setLoginPwd(''); setLoginMsg('');
  };

  const handlePasswordLogin = async () => {
    if (!loginUin || !loginPwd) { setLoginMsg('请输入QQ号和密码'); return; }
    setLoginLoading(true); setLoginMsg('');
    try {
      const r = await api.napcatPasswordLogin(loginUin, loginPwd);
      if (r.ok && (r.code === 0 || r.message?.includes('Logined'))) { setLoginMsg('登录成功！'); reload(); setTimeout(() => setLoginModal(null), 1500); }
      else { setLoginMsg(r.message || r.data?.message || r.error || '账密登录失败'); }
    } catch (err) { setLoginMsg('账密登录失败: ' + String(err)); }
    finally { setLoginLoading(false); }
  };

  const handleQQLogout = async () => {
    if (!confirm('确定要退出当前QQ登录？退出后需要重新扫码或快速登录。')) return;
    setLoginLoading(true); setLoginMsg('');
    try {
      const r = await api.napcatLogout();
      if (r.ok) {
        setMsg('QQ 已退出登录，容器正在重启...');
        setTimeout(() => { reload(); setMsg(''); }, 5000);
      } else {
        setMsg(r.error || '退出登录失败');
      }
    } catch (err) { setMsg('退出登录失败: ' + String(err)); }
    finally { setLoginLoading(false); setTimeout(() => setMsg(''), 5000); }
  };

  const handleApprove = async (flag: string) => {
    await api.approveRequest(flag);
    setRequests(prev => prev.filter(r => r.flag !== flag));
  };
  const handleReject = async (flag: string) => {
    await api.rejectRequest(flag);
    setRequests(prev => prev.filter(r => r.flag !== flag));
  };

  // Sort channels: enabled first, then configured, then unconfigured
  const sortedBuiltin = CHANNEL_DEFS.filter(c => c.type === 'builtin').sort((a, b) => {
    const order = { enabled: 0, configured: 1, unconfigured: 2 };
    return order[getChannelStatus(a, ocConfig)] - order[getChannelStatus(b, ocConfig)];
  });
  const sortedPlugin = CHANNEL_DEFS.filter(c => c.type === 'plugin').sort((a, b) => {
    const order = { enabled: 0, configured: 1, unconfigured: 2 };
    return order[getChannelStatus(a, ocConfig)] - order[getChannelStatus(b, ocConfig)];
  });

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-bold">{t.channels.title}</h2>
        <p className="text-xs text-gray-500 mt-0.5">{t.channels.subtitle} — <span className="inline-flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-emerald-500 inline-block" />{t.channels.statusEnabled}</span> <span className="inline-flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-red-400 inline-block" />{t.channels.statusConfigured}</span> <span className="inline-flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-gray-300 inline-block" />{t.channels.statusUnconfigured}</span></p>
      </div>

      {msg && (
        <div className={`px-3 py-2 rounded-lg text-xs ${msg.includes('失败') ? 'bg-red-50 dark:bg-red-950 text-red-600' : 'bg-emerald-50 dark:bg-emerald-950 text-emerald-600'}`}>
          {msg}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
        {/* Channel selector */}
        <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 flex flex-col max-h-[75vh] overflow-hidden">
          <div className="p-3 border-b border-gray-100 dark:border-gray-700/50 bg-gray-50/50 dark:bg-gray-800/50">
            <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider px-1">{t.channels.channelList}</h3>
          </div>
          <div className="flex-1 overflow-y-auto p-2 space-y-4">
            <div>
              <h3 className="text-[10px] font-semibold text-gray-400 mb-2 px-2 uppercase tracking-wide">{t.channels.builtinChannels}</h3>
              <div className="space-y-1">
                {sortedBuiltin.map(ch => {
                  const st = getChannelStatus(ch, ocConfig);
                  return (
                    <button key={ch.id} onClick={() => setSelectedChannel(ch.id)}
                      className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-left text-sm transition-all duration-200 group ${
                        selectedChannel === ch.id
                          ? 'bg-violet-50 dark:bg-violet-900/20 text-violet-700 dark:text-violet-300 shadow-sm ring-1 ring-violet-100 dark:ring-violet-800'
                          : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700/50'
                      }`}>
                      <div className={`p-1.5 rounded-md transition-colors ${selectedChannel === ch.id ? 'bg-violet-100 dark:bg-violet-900/40 text-violet-600' : 'bg-gray-100 dark:bg-gray-700 text-gray-500 group-hover:bg-white group-hover:shadow-sm'}`}>
                        <Radio size={14} />
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="text-xs font-semibold truncate">{ch.label}</div>
                        <div className="text-[10px] text-gray-400 truncate opacity-80">{ch.description}</div>
                      </div>
                      <span className={`w-2 h-2 rounded-full shrink-0 ${statusDot(st)} ring-2 ring-white dark:ring-gray-800`} title={statusLabel(st)} />
                    </button>
                  );
                })}
              </div>
            </div>
            
            {sortedPlugin.length > 0 && (
              <div>
                <h3 className="text-[10px] font-semibold text-gray-400 mb-2 px-2 uppercase tracking-wide">{t.channels.pluginChannels}</h3>
                <div className="space-y-1">
                  {sortedPlugin.map(ch => {
                    const st = getChannelStatus(ch, ocConfig);
                    return (
                      <button key={ch.id} onClick={() => setSelectedChannel(ch.id)}
                        className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-left text-sm transition-all duration-200 group ${
                          selectedChannel === ch.id
                            ? 'bg-violet-50 dark:bg-violet-900/20 text-violet-700 dark:text-violet-300 shadow-sm ring-1 ring-violet-100 dark:ring-violet-800'
                            : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700/50'
                        }`}>
                        <div className={`p-1.5 rounded-md transition-colors ${selectedChannel === ch.id ? 'bg-violet-100 dark:bg-violet-900/40 text-violet-600' : 'bg-gray-100 dark:bg-gray-700 text-gray-500 group-hover:bg-white group-hover:shadow-sm'}`}>
                          <Radio size={14} />
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="text-xs font-semibold truncate">{ch.label}</div>
                          <div className="text-[10px] text-gray-400 truncate opacity-80">{ch.description}</div>
                        </div>
                        <span className={`w-2 h-2 rounded-full shrink-0 ${statusDot(st)} ring-2 ring-white dark:ring-gray-800`} title={statusLabel(st)} />
                      </button>
                    );
                  })}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Channel config */}
        <div className="lg:col-span-3 space-y-6">
          {currentDef && (
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-6 space-y-6">
              <div className="flex items-center justify-between pb-4 border-b border-gray-100 dark:border-gray-800">
                <div className="flex items-center gap-4">
                  <div className={`p-2.5 rounded-xl ${isChannelEnabled(currentDef.id) ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600' : 'bg-gray-100 dark:bg-gray-800 text-gray-500'}`}>
                    <Power size={20} />
                  </div>
                  <div>
                    <div className="flex items-center gap-3">
                      <h3 className="font-bold text-base text-gray-900 dark:text-white">{currentDef.label} {t.channels.config}</h3>
                      <span className={`px-2 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wider ${
                        getChannelStatus(currentDef, ocConfig) === 'enabled' ? 'bg-emerald-100 dark:bg-emerald-900/40 text-emerald-700 dark:text-emerald-400' :
                        getChannelStatus(currentDef, ocConfig) === 'configured' ? 'bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-400' :
                        'bg-gray-100 dark:bg-gray-800 text-gray-500'
                      }`}>{statusLabel(getChannelStatus(currentDef, ocConfig))}</span>
                    </div>
                    <p className="text-xs text-gray-500 mt-1">{currentDef.description}</p>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  {/* Enable/Disable toggle switch */}
                  <div className="flex items-center gap-2 bg-gray-50 dark:bg-gray-900/50 px-3 py-1.5 rounded-lg border border-gray-100 dark:border-gray-800">
                    <span className={`text-[11px] font-medium ${isChannelEnabled(currentDef.id) ? 'text-gray-900 dark:text-gray-100' : 'text-gray-400'}`}>
                      {isChannelEnabled(currentDef.id) ? t.channels.enabledState : t.channels.disabledState}
                    </span>
                    <button onClick={() => handleToggleEnabled(currentDef.id)}
                      className={`relative w-9 h-5 rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-violet-500 ${isChannelEnabled(currentDef.id) ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'}`}>
                      <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow-sm transition-transform ${isChannelEnabled(currentDef.id) ? 'translate-x-4' : ''}`} />
                    </button>
                  </div>
                  
                  {currentDef.loginMethods && currentDef.loginMethods.length > 0 && (
                    <div className="flex items-center gap-2 border-l border-gray-200 dark:border-gray-700 pl-3 ml-1">
                      {currentDef.loginMethods.includes('qrcode') && (
                        <button onClick={handleQRLogin} className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors">
                          <QrCode size={14} />{t.channels.qrLogin}
                        </button>
                      )}
                      {currentDef.loginMethods.includes('quick') && (
                        <button onClick={handleQuickLoginOpen} className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400 hover:bg-emerald-100 dark:hover:bg-emerald-900/50 transition-colors">
                          <Zap size={14} />{t.channels.quickLogin}
                        </button>
                      )}
                      {currentDef.loginMethods.includes('password') && (
                        <button onClick={handlePasswordLoginOpen} className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-amber-50 dark:bg-amber-900/30 text-amber-600 dark:text-amber-400 hover:bg-amber-100 dark:hover:bg-amber-900/50 transition-colors">
                          <Key size={14} />{t.channels.passwordLogin}
                        </button>
                      )}
                      {currentDef.id === 'qq' && (
                        <button onClick={handleQQLogout} className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-400 hover:bg-red-100 dark:hover:bg-red-900/50 transition-colors">
                          <LogOut size={14} />{t.channels.logoutQQ}
                        </button>
                      )}
                    </div>
                  )}
                </div>
              </div>

              <form id="channel-config-form" className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-5" onSubmit={e => { e.preventDefault(); handleSave(); }}>
                {currentDef.configFields.map(field => {
                  const currentVal = getFieldValue(currentDef.id, field.key);
                  const isFullWidth = field.type === 'toggle' || field.key === 'webhookUrl' || field.key === 'token' || field.key === 'accessToken' || field.key === 'appSecret';
                  
                  return (
                    <div key={field.key} className={isFullWidth ? "md:col-span-2" : ""}>
                      <div className="flex items-center justify-between mb-1.5">
                        <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">
                          {field.label}
                        </label>
                        {field.help && <span className="text-[10px] text-gray-400">{field.help}</span>}
                      </div>
                      
                      {field.type === 'toggle' ? (
                        <div className="flex items-center gap-3 p-3 rounded-lg border border-gray-100 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-900/30">
                          <button type="button"
                            onClick={() => handleToggleField(currentDef.id, field.key)}
                            className={`relative w-9 h-5 rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-violet-500 ${currentVal ? 'bg-violet-600' : 'bg-gray-300 dark:bg-gray-600'}`}>
                            <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow-sm transition-transform ${currentVal ? 'translate-x-4' : ''}`} />
                          </button>
                          <span className={`text-xs ${currentVal ? 'text-violet-600 dark:text-violet-400 font-medium' : 'text-gray-500'}`}>
                            {currentVal ? t.channels.opened : t.channels.closed}
                          </span>
                        </div>
                      ) : (
                        <div className="relative">
                          <input
                            name={field.key}
                            type={field.type === 'password' ? 'password' : field.type === 'number' ? 'number' : 'text'}
                            defaultValue={currentVal ?? ''}
                            placeholder={field.placeholder || '未配置'}
                            className={`w-full px-3.5 py-2 text-sm border rounded-lg bg-white dark:bg-gray-900 transition-all focus:ring-2 focus:ring-violet-100 dark:focus:ring-violet-900/30 focus:border-violet-500 outline-none
                              ${(currentVal !== undefined && currentVal !== null && currentVal !== '') 
                                ? 'border-gray-300 dark:border-gray-700 text-gray-900 dark:text-gray-100' 
                                : 'border-gray-200 dark:border-gray-800 text-gray-400'}`}
                          />
                          {(currentVal !== undefined && currentVal !== null && currentVal !== '') && (
                            <div className="absolute right-3 top-1/2 -translate-y-1/2 text-emerald-500">
                              <Check size={14} strokeWidth={3} />
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  );
                })}
              </form>

              {currentDef.configFields.length === 0 && (
                <div className="py-12 flex flex-col items-center justify-center text-gray-400 border-2 border-dashed border-gray-100 dark:border-gray-800 rounded-xl">
                  <Sparkles size={32} className="mb-2 opacity-20" />
                  <p className="text-sm">{t.channels.noConfigNeeded}</p>
                </div>
              )}

              <div className="flex items-center justify-end pt-4 border-t border-gray-50 dark:border-gray-800">
                <button onClick={handleSave} disabled={saving}
                  className="flex items-center gap-2 px-5 py-2.5 text-sm font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 shadow-sm shadow-violet-200 dark:shadow-none transition-all hover:shadow-md hover:shadow-violet-200 dark:hover:shadow-none">
                  {saving ? <Loader2 size={16} className="animate-spin" /> : <Check size={16} />}
                  {saving ? t.channels.saving : t.channels.saveConfig}
                </button>
              </div>
            </div>
          )}

          {/* QQ Requests — only when QQ selected */}
          {selectedChannel === 'qq' && requests.length > 0 && (
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-5 space-y-4">
              <div className="flex items-center gap-3 pb-2 border-b border-gray-50 dark:border-gray-800">
                <div className="p-1.5 rounded-lg bg-violet-100 dark:bg-violet-900/30 text-violet-600">
                  <UserCheck size={18} />
                </div>
                <h3 className="font-bold text-sm text-gray-900 dark:text-white">{t.channels.pendingRequests}</h3>
                <span className="bg-red-100 text-red-600 text-xs font-bold px-2 py-0.5 rounded-full">{requests.length}</span>
              </div>
              <div className="space-y-2.5">
                {requests.map((r: any) => (
                  <div key={r.flag} className="flex items-center gap-4 px-4 py-3 rounded-xl bg-gray-50 dark:bg-gray-900/30 border border-gray-100 dark:border-gray-800 text-sm">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-0.5">
                        <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded ${r.type === 'group' ? 'bg-indigo-100 text-indigo-700' : 'bg-pink-100 text-pink-700'}`}>
                          {r.type === 'group' ? t.channels.groupRequest : t.channels.friendRequest}
                        </span>
                        <span className="font-mono text-gray-900 dark:text-gray-100 font-medium">{r.userId || r.groupId || ''}</span>
                      </div>
                      {r.comment && <div className="text-gray-500 text-xs truncate">{t.channels.comment}: "{r.comment}"</div>}
                    </div>
                    <div className="flex items-center gap-2">
                      <button onClick={() => handleApprove(r.flag)} className="p-2 rounded-lg bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600 hover:bg-emerald-100 dark:hover:bg-emerald-900/50 transition-colors" title={t.channels.approve}>
                        <Check size={16} />
                      </button>
                      <button onClick={() => handleReject(r.flag)} className="p-2 rounded-lg bg-red-50 dark:bg-red-900/30 text-red-500 hover:bg-red-100 dark:hover:bg-red-900/50 transition-colors" title={t.channels.reject}>
                        <X size={16} />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* QQ Login Modal */}
      {loginModal && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4" onClick={() => setLoginModal(null)}>
          <div className="bg-white dark:bg-gray-900 rounded-2xl shadow-2xl max-w-md w-full overflow-hidden transform transition-all border border-gray-100 dark:border-gray-800" onClick={e => e.stopPropagation()}>
            <div className="px-5 py-4 border-b border-gray-100 dark:border-gray-800 flex items-center justify-between bg-gray-50/50 dark:bg-gray-900">
              <h3 className="text-sm font-bold text-gray-900 dark:text-white flex items-center gap-2">
                {loginModal === 'qrcode' && <><QrCode size={18} className="text-blue-500" /> QQ {t.channels.qrLogin}</>}
                {loginModal === 'quick' && <><Zap size={18} className="text-emerald-500" /> QQ {t.channels.quickLogin}</>}
                {loginModal === 'password' && <><Key size={18} className="text-amber-500" /> QQ {t.channels.passwordLogin}</>}
              </h3>
              <button onClick={() => setLoginModal(null)} className="p-1 rounded-md text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors">
                <X size={18} />
              </button>
            </div>
            <div className="p-6 space-y-4">
              {loginMsg && (
                <div className={`px-4 py-3 rounded-xl text-sm font-medium flex items-center gap-2 ${loginMsg.includes(t.common.success) || loginMsg.includes('success') ? 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600' : 'bg-red-50 dark:bg-red-900/30 text-red-600'}`}>
                   {loginMsg.includes(t.common.success) || loginMsg.includes('success') ? <Check size={16} /> : <X size={16} />}
                  {loginMsg}
                </div>
              )}

              {/* QR Code Login */}
              {loginModal === 'qrcode' && (
                <div className="flex flex-col items-center gap-3">
                  {qrLoading ? (
                    <div className="w-48 h-48 flex items-center justify-center bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <Loader2 size={24} className="animate-spin text-gray-400" />
                    </div>
                  ) : qrImg ? (
                    <img src={qrImg.startsWith('data:') ? qrImg : `data:image/png;base64,${qrImg}`} alt="QR Code" className="w-48 h-48 rounded-lg border border-gray-200 dark:border-gray-700" />
                  ) : (
                    <div className="w-48 h-48 flex items-center justify-center bg-gray-50 dark:bg-gray-800 rounded-lg text-xs text-gray-400">
                      {t.channels.cannotLoadQR}
                    </div>
                  )}
                  <p className="text-xs text-gray-500">{t.channels.scanQR}</p>
                  <button onClick={handleRefreshQR} disabled={qrLoading}
                    className="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50">
                    <RefreshCw size={12} className={qrLoading ? 'animate-spin' : ''} />{t.channels.refreshQR}
                  </button>
                </div>
              )}

              {/* Quick Login */}
              {loginModal === 'quick' && (
                <div className="space-y-2">
                  {loginLoading ? (
                    <div className="flex items-center justify-center py-8"><Loader2 size={20} className="animate-spin text-gray-400" /></div>
                  ) : quickList.length === 0 ? (
                    <p className="text-xs text-gray-400 text-center py-8">{t.channels.noQuickAccounts}</p>
                  ) : (
                    <>
                      <p className="text-xs text-gray-500">{t.channels.selectQuickAccount}</p>
                      {quickList.map(uin => (
                        <button key={uin} onClick={() => handleQuickLogin(uin)}
                          className="w-full flex items-center gap-3 px-4 py-3 rounded-lg bg-gray-50 dark:bg-gray-800 hover:bg-emerald-50 dark:hover:bg-emerald-950 text-sm transition-colors">
                          <Zap size={14} className="text-emerald-500" />
                          <span className="font-mono">{uin}</span>
                        </button>
                      ))}
                    </>
                  )}
                </div>
              )}

              {/* Password Login */}
              {loginModal === 'password' && (
                <div className="space-y-3">
                  <div>
                    <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">{t.channels.qqNumberLabel}</label>
                    <input type="text" value={loginUin} onChange={e => setLoginUin(e.target.value)}
                      placeholder={t.channels.qqNumberPlaceholder} className="w-full px-3 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-transparent" />
                  </div>
                  <div>
                    <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">{t.channels.passwordLabel}</label>
                    <input type="password" value={loginPwd} onChange={e => setLoginPwd(e.target.value)}
                      placeholder={t.channels.passwordPlaceholder} className="w-full px-3 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-transparent" />
                  </div>
                  <button onClick={handlePasswordLogin} disabled={loginLoading || !loginUin || !loginPwd}
                    className="w-full flex items-center justify-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-amber-600 text-white hover:bg-amber-700 disabled:opacity-50">
                    {loginLoading ? <Loader2 size={12} className="animate-spin" /> : <Key size={12} />}
                    {loginLoading ? t.login.loggingIn : t.login.loginButton}
                  </button>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
