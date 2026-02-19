import { Router } from 'express';
import jwt from 'jsonwebtoken';
import http from 'http';
import crypto from 'crypto';
import multer from 'multer';
import { AdminConfig } from '../core/admin-config.js';
import { OneBotClient } from '../core/onebot-client.js';
import { OpenClawConfig } from '../core/openclaw-config.js';
import { WeChatClient } from '../core/wechat-client.js';
import { WorkspaceManager } from '../core/workspace-manager.js';
import { getPendingRequests } from '../core/event-router.js';
import { JWT_SECRET } from '../core/ws-manager.js';
import { EventLog } from '../core/event-log.js';
import fs from 'fs';
import path from 'path';

export function createRoutes(adminConfig: AdminConfig, onebotClient: OneBotClient, openclawConfig: OpenClawConfig, wechatClient: WeChatClient, workspaceManager?: WorkspaceManager, eventLog?: EventLog) {
  const router = Router();

  // Auth middleware
  const auth = (req: any, res: any, next: any) => {
    const token = req.headers.authorization?.replace('Bearer ', '');
    if (!token) return res.status(401).json({ ok: false, error: 'No token' });
    try { jwt.verify(token, JWT_SECRET); next(); } catch { res.status(401).json({ ok: false, error: 'Invalid token' }); }
  };

  // === Auth ===
  router.post('/auth/login', (req, res) => {
    const { token } = req.body;
    const cfg = adminConfig.get();
    if (token === cfg.server.token) {
      const jwtToken = jwt.sign({ role: 'admin' }, JWT_SECRET, { expiresIn: '7d' });
      res.json({ ok: true, token: jwtToken });
    } else {
      res.status(401).json({ ok: false, error: '密码错误' });
    }
  });

  // Change admin password
  router.post('/auth/change-password', auth, (req, res) => {
    const { oldPassword, newPassword } = req.body;
    if (!oldPassword || !newPassword) return res.status(400).json({ ok: false, error: 'Missing fields' });
    if (newPassword.length < 4) return res.status(400).json({ ok: false, error: 'Password too short' });
    const cfg = adminConfig.get();
    if (oldPassword !== cfg.server.token) return res.status(401).json({ ok: false, error: 'Wrong current password' });
    adminConfig.update({ server: { ...cfg.server, token: newPassword } });
    if (eventLog) eventLog.addSystemEvent('管理后台密码已修改');
    res.json({ ok: true });
  });

  // === Status ===
  router.get('/status', auth, async (_req, res) => {
    const cfg = adminConfig.get();
    let groupCount = 0, friendCount = 0;
    try {
      const groups = await onebotClient.callApi('get_group_list');
      groupCount = groups?.data?.length || 0;
    } catch {}
    try {
      const friends = await onebotClient.callApi('get_friend_list');
      friendCount = friends?.data?.length || 0;
    } catch {}

    const ocConfig = openclawConfig.read();
    let wechatInfo: any = { connected: wechatClient.connected };
    try {
      const wl = await wechatClient.getLoginStatus();
      wechatInfo.loggedIn = wl.loggedIn || false;
      wechatInfo.name = wl.name || wechatClient.loginUser?.name || '';
    } catch { wechatInfo.loggedIn = false; }

    // Build list of ALL enabled channels from openclaw.json (channels + plugins.entries)
    const channelLabels: Record<string, string> = {
      qq: 'QQ (NapCat)', wechat: '微信', whatsapp: 'WhatsApp', telegram: 'Telegram',
      discord: 'Discord', irc: 'IRC', slack: 'Slack', signal: 'Signal',
      googlechat: 'Google Chat', bluebubbles: 'BlueBubbles', webchat: 'WebChat',
      feishu: '飞书 / Lark', qqbot: 'QQ 官方机器人', dingtalk: '钉钉',
      wecom: '企业微信', msteams: 'Microsoft Teams', mattermost: 'Mattermost',
      line: 'LINE', matrix: 'Matrix', twitch: 'Twitch',
    };
    const enabledChannels: { id: string; label: string; type: 'builtin' | 'plugin' }[] = [];
    const channels = ocConfig?.channels || {};
    const plugins = ocConfig?.plugins?.entries || {};
    for (const [id, conf] of Object.entries(channels)) {
      if ((conf as any)?.enabled) {
        enabledChannels.push({ id, label: channelLabels[id] || id, type: 'builtin' });
      }
    }
    for (const [id, conf] of Object.entries(plugins)) {
      if ((conf as any)?.enabled && !enabledChannels.find(c => c.id === id)) {
        enabledChannels.push({ id, label: channelLabels[id] || id, type: 'plugin' });
      }
    }

    res.json({
      ok: true,
      napcat: {
        connected: onebotClient.connected,
        selfId: onebotClient.selfId,
        nickname: onebotClient.nickname,
        groupCount, friendCount,
      },
      wechat: wechatInfo,
      openclaw: {
        configured: openclawConfig.exists(),
        qqPluginEnabled: !!ocConfig?.plugins?.entries?.qq?.enabled,
        qqChannelEnabled: !!ocConfig?.channels?.qq?.enabled,
        currentModel: ocConfig?.agents?.defaults?.model?.primary || '',
        enabledChannels,
      },
      admin: {
        uptime: process.uptime(),
        memoryMB: Math.round(process.memoryUsage().rss / 1024 / 1024),
      },
    });
  });

  // === OpenClaw Config ===
  router.get('/openclaw/config', auth, (_req, res) => {
    const config = openclawConfig.read() || {};
    // Strip keys that shouldn't be edited via WebUI
    delete config.tools;
    delete config.session;
    if (config.cron?.jobs) delete config.cron.jobs;
    if (config.cron && Object.keys(config.cron).length === 0) delete config.cron;
    res.json({ ok: true, config });
  });

  router.put('/openclaw/config', auth, (req, res) => {
    try {
      const cfg = req.body.config || {};
      // Strip keys not recognized by OpenClaw 2026.2.14+
      delete cfg.tools;
      delete cfg.session;
      // cron.jobs belong in cron/jobs.json, not in openclaw.json
      if (cfg.cron?.jobs) delete cfg.cron.jobs;
      if (cfg.cron && Object.keys(cfg.cron).length === 0) delete cfg.cron;
      openclawConfig.write(cfg);
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  router.get('/openclaw/models', auth, (_req, res) => {
    res.json({ ok: true, ...openclawConfig.getModels() });
  });

  router.put('/openclaw/models', auth, (req, res) => {
    try {
      openclawConfig.updateModels(req.body);
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  router.get('/openclaw/channels', auth, (_req, res) => {
    res.json({ ok: true, ...openclawConfig.getChannels() });
  });

  router.put('/openclaw/channels/:id', auth, (req, res) => {
    try {
      openclawConfig.updateChannel(req.params.id, req.body);
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  router.put('/openclaw/plugins/:id', auth, (req, res) => {
    try {
      openclawConfig.updatePlugin(req.params.id, req.body);
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // Toggle channel enabled/disabled — handles full lifecycle:
  // 1. Update channels + plugins.entries config
  // 2. Log system event
  // 3. If QQ disabled: disconnect OneBot + clear NapCat session → force re-login next time
  // 4. Signal host to restart gateway so config takes effect
  const OPENCLAW_DIR_FOR_SIGNAL = process.env['OPENCLAW_CONFIG'] ? path.dirname(process.env['OPENCLAW_CONFIG']) : '/root/.openclaw';

  router.post('/openclaw/toggle-channel', auth, async (req, res) => {
    try {
      const { channelId, enabled } = req.body;
      if (!channelId) return res.status(400).json({ ok: false, error: 'channelId required' });

      const channelNames: Record<string, string> = { qq: 'QQ (NapCat)', wechat: '微信', feishu: '飞书', qqbot: 'QQ Bot', dingtalk: '钉钉', wecom: '企业微信' };
      const label = channelNames[channelId] || channelId;

      // 1. Update config
      const existingChannel = openclawConfig.read()?.channels?.[channelId] || {};
      openclawConfig.updateChannel(channelId, { ...existingChannel, enabled: !!enabled });
      openclawConfig.updatePlugin(channelId, { enabled: !!enabled });

      // 2. Log system event
      if (eventLog) {
        eventLog.addSystemEvent(`${label} 通道已${enabled ? '启用' : '禁用'}`);
      }

      // 3. If QQ channel is being disabled, disconnect OneBot and clear ALL QQ session data
      if (channelId === 'qq' && !enabled) {
        if (eventLog) eventLog.addSystemEvent('正在退出 QQ 登录...');
        try { onebotClient.disconnect(); } catch {}
        const { execSync: execSyncCh } = await import('child_process');
        // Delete QQ login database + account session + NapCat cache
        try { execSyncCh('rm -rf /app/.config/QQ/nt_qq /app/.config/QQ/nt_qq_* /app/.config/QQ/NapCat/data /app/.config/QQ/NapCat/quickLoginCache /app/.config/QQ/NapCat/temp 2>/dev/null', { timeout: 5000 }); } catch {}
      }

      // 4. If QQ channel is being enabled, reconnect OneBot
      if (channelId === 'qq' && enabled) {
        if (eventLog) eventLog.addSystemEvent('正在重新连接 QQ...');
        try { onebotClient.connect(); } catch {}
      }

      // 5. Signal host to restart gateway
      if (eventLog) eventLog.addSystemEvent('正在重启 OpenClaw 网关...');
      const restartSignal = path.join(OPENCLAW_DIR_FOR_SIGNAL, 'restart-gateway-signal.json');
      const restartResult = path.join(OPENCLAW_DIR_FOR_SIGNAL, 'restart-gateway-result.json');
      // Remove old result before requesting restart
      try { if (fs.existsSync(restartResult)) fs.unlinkSync(restartResult); } catch {}
      fs.writeFileSync(restartSignal, JSON.stringify({ requestedAt: new Date().toISOString(), reason: `${label} ${enabled ? 'enabled' : 'disabled'}` }));

      // Poll for restart result (up to 30s) and log success/failure
      if (eventLog) {
        (async () => {
          for (let i = 0; i < 15; i++) {
            await new Promise(r => setTimeout(r, 2000));
            try {
              if (fs.existsSync(restartResult)) {
                const result = JSON.parse(fs.readFileSync(restartResult, 'utf-8'));
                if (result.status === 'success') {
                  eventLog.addSystemEvent('OpenClaw 网关重启成功');
                } else if (result.status === 'failed') {
                  eventLog.addSystemEvent(`OpenClaw 网关重启失败: ${result.error || '未知错误'}`);
                }
                return;
              }
            } catch {}
          }
          // Timeout — gateway may have restarted without writing result
          eventLog.addSystemEvent('OpenClaw 网关重启状态未知（超时）');
        })();
      }

      // 6. If QQ disabled, restart container QQ process after a delay (forces NapCat to show login screen)
      if (channelId === 'qq' && !enabled) {
        const { exec: exec2 } = await import('child_process');
        setTimeout(() => {
          exec2('kill -9 1', { timeout: 5000 }, () => {});
        }, 1500);
      }

      res.json({ ok: true, message: `${label} 通道已${enabled ? '启用' : '禁用'}` });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // === Admin Config (QQ features) ===
  router.get('/admin/config', auth, (_req, res) => {
    res.json({ ok: true, config: adminConfig.get() });
  });

  router.put('/admin/config', auth, (req, res) => {
    try {
      adminConfig.update(req.body);
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  router.put('/admin/config/:section', auth, (req, res) => {
    try {
      adminConfig.updateSection(req.params.section, req.body);
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // === Bot operations ===
  router.get('/bot/groups', auth, async (_req, res) => {
    try {
      const result = await onebotClient.getGroupList();
      res.json({ ok: true, groups: result.data || [] });
    } catch (err) {
      res.json({ ok: false, groups: [], error: String(err) });
    }
  });

  router.get('/bot/friends', auth, async (_req, res) => {
    try {
      const result = await onebotClient.getFriendList();
      res.json({ ok: true, friends: result.data || [] });
    } catch (err) {
      res.json({ ok: false, friends: [], error: String(err) });
    }
  });

  router.post('/bot/send', auth, async (req, res) => {
    try {
      const { type, id, message } = req.body;
      if (type === 'group') await onebotClient.sendGroupMsg(id, message);
      else await onebotClient.sendPrivateMsg(id, message);
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  router.post('/bot/reconnect', auth, (_req, res) => {
    onebotClient.disconnect();
    setTimeout(() => onebotClient.connect(), 1000);
    res.json({ ok: true });
  });

  // === Requests (approval) ===
  router.get('/requests', auth, (_req, res) => {
    const requests = Array.from(getPendingRequests().values());
    res.json({ ok: true, requests });
  });

  router.post('/requests/:flag/approve', auth, async (req, res) => {
    const { flag } = req.params;
    const pending = getPendingRequests().get(flag);
    if (!pending) return res.status(404).json({ ok: false, error: 'Request not found' });
    try {
      if (pending.type === 'group') {
        await onebotClient.setGroupAddRequest(flag, pending.subType || 'add', true);
      } else {
        await onebotClient.setFriendAddRequest(flag, true);
      }
      getPendingRequests().delete(flag);
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  router.post('/requests/:flag/reject', auth, async (req, res) => {
    const { flag } = req.params;
    const pending = getPendingRequests().get(flag);
    if (!pending) return res.status(404).json({ ok: false, error: 'Request not found' });
    try {
      if (pending.type === 'group') {
        await onebotClient.setGroupAddRequest(flag, pending.subType || 'add', false, req.body.reason || '');
      } else {
        await onebotClient.setFriendAddRequest(flag, false);
      }
      getPendingRequests().delete(flag);
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // === NapCat WebUI Proxy (QQ Login) ===
  const NAPCAT_WEBUI = 'http://127.0.0.1:6099';
  let napcatCredential = '';

  async function napcatAuth(adminCfg: AdminConfig): Promise<string> {
    if (napcatCredential) return napcatCredential;
    const cfg = adminCfg.get();
    const webuiToken = cfg.napcat.webuiToken || process.env['WEBUI_TOKEN'] || 'openclaw-qq-admin';
    const hash = crypto.createHash('sha256').update(webuiToken + '.napcat').digest('hex');
    try {
      const res = await napcatProxy('POST', '/api/auth/login', { hash });
      if (res.code === 0 && res.data?.Credential) {
        napcatCredential = res.data.Credential;
      } else {
        console.error('[NapCat Auth] Login failed:', JSON.stringify(res));
      }
    } catch (err) {
      console.error('[NapCat Auth] Error:', err);
    }
    return napcatCredential;
  }

  function invalidateNapcatCredential() {
    napcatCredential = '';
  }

  function napcatProxy(method: string, path: string, body?: any, credential?: string): Promise<any> {
    return new Promise((resolve, reject) => {
      const data = body ? JSON.stringify(body) : '';
      const headers: Record<string, string> = { 'Content-Type': 'application/json' };
      if (credential) headers['Authorization'] = `Bearer ${credential}`;
      if (data) headers['Content-Length'] = Buffer.byteLength(data).toString();
      const url = new URL(path, NAPCAT_WEBUI);
      const req = http.request({ hostname: url.hostname, port: url.port, path: url.pathname, method, headers }, (res) => {
        let chunks = '';
        res.on('data', (c: Buffer) => chunks += c.toString());
        res.on('end', () => { try { resolve(JSON.parse(chunks)); } catch { resolve({ raw: chunks }); } });
      });
      req.on('error', reject);
      if (data) req.write(data);
      req.end();
    });
  }

  // Helper: call NapCat API with auto-retry on auth failure
  async function napcatApiCall(method: string, path: string, body?: any): Promise<any> {
    let cred = await napcatAuth(adminConfig);
    const r = await napcatProxy(method, path, body, cred);
    // If unauthorized, invalidate credential and retry once
    if (r.code === -1 && r.message?.toLowerCase().includes('unauthorized')) {
      invalidateNapcatCredential();
      cred = await napcatAuth(adminConfig);
      return napcatProxy(method, path, body, cred);
    }
    return r;
  }

  // QQ Login Status
  router.post('/napcat/login-status', auth, async (_req, res) => {
    try {
      const r = await napcatApiCall('POST', '/api/QQLogin/CheckLoginStatus');
      res.json({ ok: true, ...r });
    } catch (err) { res.json({ ok: false, error: String(err) }); }
  });

  // Helper: convert QR URL to base64 data URI using qrcode library
  const qrUrlToBase64 = async (data: any): Promise<any> => {
    if (data?.data?.qrcode && typeof data.data.qrcode === 'string' && data.data.qrcode.startsWith('http')) {
      try {
        const QRCode = await import('qrcode');
        const dataUrl = await QRCode.toDataURL(data.data.qrcode, { width: 256, margin: 2 });
        data.data.qrcode = dataUrl;
      } catch {}
    }
    return data;
  };

  // Get QR Code
  router.post('/napcat/qrcode', auth, async (_req, res) => {
    try {
      const r = await napcatApiCall('POST', '/api/QQLogin/GetQQLoginQrcode');
      const result = await qrUrlToBase64(r);
      res.json({ ok: true, ...result });
    } catch (err) { res.json({ ok: false, error: String(err) }); }
  });

  // Refresh QR Code
  router.post('/napcat/qrcode/refresh', auth, async (_req, res) => {
    try {
      const r = await napcatApiCall('POST', '/api/QQLogin/RefreshQRcode');
      const result = await qrUrlToBase64(r);
      res.json({ ok: true, ...result });
    } catch (err) { res.json({ ok: false, error: String(err) }); }
  });

  // Quick Login List
  router.get('/napcat/quick-login-list', auth, async (_req, res) => {
    try {
      const r = await napcatApiCall('POST', '/api/QQLogin/GetQuickLoginQQ');
      res.json({ ok: true, ...r });
    } catch (err) { res.json({ ok: false, error: String(err) }); }
  });

  // Quick Login
  router.post('/napcat/quick-login', auth, async (req, res) => {
    try {
      const r = await napcatApiCall('POST', '/api/QQLogin/SetQuickLogin', { uin: req.body.uin });
      res.json({ ok: true, ...r });
    } catch (err) { res.json({ ok: false, error: String(err) }); }
  });

  // Password Login (server computes MD5 since browsers don't support it)
  router.post('/napcat/password-login', auth, async (req, res) => {
    try {
      const pwd = req.body.password || req.body.passwordMd5 || '';
      const passwordMd5 = crypto.createHash('md5').update(pwd).digest('hex');
      const r = await napcatApiCall('POST', '/api/QQLogin/PasswordLogin', { uin: req.body.uin, passwordMd5 });
      res.json({ ok: true, ...r });
    } catch (err) { res.json({ ok: false, error: String(err) }); }
  });

  // Get QQ Login Info
  router.get('/napcat/login-info', auth, async (_req, res) => {
    try {
      const r = await napcatApiCall('POST', '/api/QQLogin/GetQQLoginInfo');
      res.json({ ok: true, ...r });
    } catch (err) { res.json({ ok: false, error: String(err) }); }
  });

  // QQ Logout — delete ALL QQ session/login data and restart container to force re-login
  router.post('/napcat/logout', auth, async (_req, res) => {
    try {
      // Disconnect our OneBot WebSocket client
      try { onebotClient.disconnect(); } catch {}
      const { execSync } = await import('child_process');
      // Delete QQ login database + account session + NapCat cache
      // login.db is the key file that allows QQ auto-login
      try { execSync('rm -rf /app/.config/QQ/nt_qq /app/.config/QQ/nt_qq_* /app/.config/QQ/NapCat/data /app/.config/QQ/NapCat/quickLoginCache /app/.config/QQ/NapCat/temp 2>/dev/null', { timeout: 5000 }); } catch {}
      if (eventLog) eventLog.addSystemEvent('QQ 已退出登录，正在重启容器...');
      res.json({ ok: true, message: 'QQ 已退出登录，容器正在重启...' });
      // Restart the container after a short delay (kill PID 1)
      const { exec } = await import('child_process');
      setTimeout(() => {
        exec('kill -9 1', { timeout: 5000 }, () => {});
      }, 1000);
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // === WeChat API ===

  // WeChat status (health + login)
  router.get('/wechat/status', auth, async (_req, res) => {
    try {
      const loginRes = await wechatClient.getLoginStatus();
      res.json({
        ok: true,
        connected: wechatClient.connected,
        loggedIn: loginRes.loggedIn || false,
        name: loginRes.name || wechatClient.loginUser?.name || '',
      });
    } catch (err) {
      res.json({ ok: false, connected: false, loggedIn: false, error: String(err) });
    }
  });

  // WeChat login URL (returns the URL for QR code scanning)
  router.get('/wechat/login-url', auth, (_req, res) => {
    const cfg = adminConfig.get().wechat;
    // The login page is served by wechatbot-webhook container
    // From the browser, user accesses it via the mapped port 3002
    const externalUrl = `http://${_req.hostname}:3002/login?token=${cfg.token}`;
    const internalUrl = wechatClient.getLoginUrl();
    res.json({ ok: true, externalUrl, internalUrl });
  });

  // WeChat send message
  router.post('/wechat/send', auth, async (req, res) => {
    try {
      const { to, content, isRoom } = req.body;
      const r = await wechatClient.sendMessage(to, content, isRoom || false);
      res.json({ ok: true, result: r });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // WeChat send file
  router.post('/wechat/send-file', auth, async (req, res) => {
    try {
      const { to, fileUrl, isRoom } = req.body;
      const r = await wechatClient.sendFile(to, fileUrl, isRoom || false);
      res.json({ ok: true, result: r });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // WeChat config
  router.get('/wechat/config', auth, (_req, res) => {
    const cfg = adminConfig.get().wechat;
    res.json({ ok: true, config: cfg });
  });

  router.put('/wechat/config', auth, (req, res) => {
    try {
      adminConfig.updateSection('wechat', { ...adminConfig.get().wechat, ...req.body });
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // === Workspace API ===
  if (workspaceManager) {
    const wsUpload = multer({ storage: multer.memoryStorage(), limits: { fileSize: 100 * 1024 * 1024 } });

    // List files
    router.get('/workspace/files', auth, (req: any, res: any) => {
      try {
        const subPath = (req.query.path as string) || '';
        const result = workspaceManager.listFiles(subPath);
        res.json({ ok: true, ...result });
      } catch (err) {
        res.status(400).json({ ok: false, error: String(err) });
      }
    });

    // Get workspace stats
    router.get('/workspace/stats', auth, (_req: any, res: any) => {
      try {
        const stats = workspaceManager.getStats();
        res.json({ ok: true, ...stats });
      } catch (err) {
        res.status(500).json({ ok: false, error: String(err) });
      }
    });

    // Get workspace config
    router.get('/workspace/config', auth, (_req: any, res: any) => {
      res.json({ ok: true, config: workspaceManager.getConfig() });
    });

    // Update workspace config
    router.put('/workspace/config', auth, (req: any, res: any) => {
      try {
        const config = workspaceManager.updateConfig(req.body);
        res.json({ ok: true, config });
      } catch (err) {
        res.status(500).json({ ok: false, error: String(err) });
      }
    });

    // Upload file(s)
    router.post('/workspace/upload', auth, wsUpload.array('files', 20), (req: any, res: any) => {
      try {
        const subPath = (req.body.path as string) || '';
        const files = req.files as Express.Multer.File[];
        if (!files || files.length === 0) {
          return res.status(400).json({ ok: false, error: 'No files provided' });
        }
        const uploaded = files.map((f: Express.Multer.File) =>
          workspaceManager.saveUploadedFile(f.originalname, f.buffer, subPath)
        );
        res.json({ ok: true, files: uploaded });
      } catch (err) {
        res.status(500).json({ ok: false, error: String(err) });
      }
    });

    // Create directory
    router.post('/workspace/mkdir', auth, (req: any, res: any) => {
      try {
        const { name, path: subPath } = req.body;
        if (!name) return res.status(400).json({ ok: false, error: 'Directory name required' });
        workspaceManager.createDirectory(name, subPath || '');
        res.json({ ok: true });
      } catch (err) {
        res.status(500).json({ ok: false, error: String(err) });
      }
    });

    // Delete file(s)
    router.post('/workspace/delete', auth, (req: any, res: any) => {
      try {
        const { paths } = req.body;
        if (!paths || !Array.isArray(paths) || paths.length === 0) {
          return res.status(400).json({ ok: false, error: 'No paths provided' });
        }
        const result = workspaceManager.deleteMultiple(paths);
        res.json({ ok: true, ...result });
      } catch (err) {
        res.status(500).json({ ok: false, error: String(err) });
      }
    });

    // Download file (supports token via query param for direct browser downloads)
    router.get('/workspace/download', (req: any, res: any) => {
      try {
        const token = req.headers.authorization?.replace('Bearer ', '') || req.query.token;
        if (!token) return res.status(401).json({ ok: false, error: 'No token' });
        try { jwt.verify(token, JWT_SECRET); } catch { return res.status(401).json({ ok: false, error: 'Invalid token' }); }
        const filePath = req.query.path as string;
        if (!filePath) return res.status(400).json({ ok: false, error: 'Path required' });
        const { fullPath, stat } = workspaceManager.getFileContent(filePath);
        res.setHeader('Content-Length', stat.size);
        res.setHeader('Content-Disposition', `attachment; filename="${encodeURIComponent(filePath.split('/').pop() || 'file')}"`);
        const fs = require('fs');
        fs.createReadStream(fullPath).pipe(res);
      } catch (err) {
        res.status(400).json({ ok: false, error: String(err) });
      }
    });

    // Manual clean expired files
    router.post('/workspace/clean', auth, (_req: any, res: any) => {
      try {
        const result = workspaceManager.cleanExpiredFiles();
        res.json({ ok: true, ...result });
      } catch (err) {
        res.status(500).json({ ok: false, error: String(err) });
      }
    });

    // File notes/remarks
    router.get('/workspace/notes', auth, (_req: any, res: any) => {
      try {
        const notes = workspaceManager.getNotes();
        res.json({ ok: true, notes });
      } catch (err) {
        res.json({ ok: true, notes: {} });
      }
    });

    router.put('/workspace/notes', auth, (req: any, res: any) => {
      try {
        const { path: filePath, note } = req.body;
        if (!filePath) return res.status(400).json({ ok: false, error: 'Path required' });
        workspaceManager.setNote(filePath, note || '');
        res.json({ ok: true });
      } catch (err) {
        res.status(500).json({ ok: false, error: String(err) });
      }
    });

    // File preview (text files, images served inline)
    router.get('/workspace/preview', (req: any, res: any) => {
      try {
        const token = req.headers.authorization?.replace('Bearer ', '') || req.query.token;
        if (!token) return res.status(401).json({ ok: false, error: 'No token' });
        try { jwt.verify(token, JWT_SECRET); } catch { return res.status(401).json({ ok: false, error: 'Invalid token' }); }
        const filePath = req.query.path as string;
        if (!filePath) return res.status(400).json({ ok: false, error: 'Path required' });
        const { fullPath, stat } = workspaceManager.getFileContent(filePath);
        const ext = path.extname(fullPath).toLowerCase();
        const IMG_EXTS = ['.jpg','.jpeg','.png','.gif','.webp','.bmp','.svg','.ico'];
        const TXT_EXTS = ['.txt','.md','.log','.json','.jsonl','.js','.ts','.py','.sh','.yaml','.yml','.xml','.html','.css','.csv','.ini','.conf','.toml','.env'];
        if (IMG_EXTS.includes(ext)) {
          const mimeMap: Record<string,string> = {'.jpg':'image/jpeg','.jpeg':'image/jpeg','.png':'image/png','.gif':'image/gif','.webp':'image/webp','.bmp':'image/bmp','.svg':'image/svg+xml','.ico':'image/x-icon'};
          res.setHeader('Content-Type', mimeMap[ext] || 'application/octet-stream');
          res.setHeader('Content-Length', stat.size);
          fs.createReadStream(fullPath).pipe(res);
        } else if (TXT_EXTS.includes(ext)) {
          const maxSize = 512 * 1024; // 512KB max for text preview
          if (stat.size > maxSize) {
            return res.json({ ok: true, type: 'text', content: fs.readFileSync(fullPath, 'utf-8').slice(0, maxSize) + '\n\n... (文件过大，已截断)', truncated: true });
          }
          res.json({ ok: true, type: 'text', content: fs.readFileSync(fullPath, 'utf-8'), truncated: false });
        } else {
          res.json({ ok: false, error: '不支持预览此文件类型', ext });
        }
      } catch (err) {
        res.status(400).json({ ok: false, error: String(err) });
      }
    });
  }

  // === System Info API ===
  const OPENCLAW_DIR = process.env['OPENCLAW_CONFIG'] ? path.dirname(process.env['OPENCLAW_CONFIG']) : '/root/.openclaw';

  // System environment detection
  router.get('/system/env', auth, async (_req, res) => {
    const { execSync } = await import('child_process');
    const os = await import('os');
    const info: any = { os: {}, software: {} };

    // Check for host-env.json (generated by host script, mounted via .openclaw volume)
    let hostEnv: any = null;
    try {
      const hostEnvPath = path.join(OPENCLAW_DIR, 'host-env.json');
      if (fs.existsSync(hostEnvPath)) {
        hostEnv = JSON.parse(fs.readFileSync(hostEnvPath, 'utf-8'));
      }
    } catch {}

    // OS info — prefer host-env.json for distro/release
    try { info.os.platform = process.platform; } catch {}
    try { info.os.arch = process.arch; } catch {}
    try { info.os.hostname = os.hostname(); } catch {}
    try { info.os.totalMemMB = Math.round(os.totalmem() / 1024 / 1024); } catch {}
    try { info.os.freeMemMB = Math.round(os.freemem() / 1024 / 1024); } catch {}
    try { info.os.cpus = os.cpus().length; } catch {}
    try { info.os.cpuModel = os.cpus()[0]?.model || ''; } catch {}
    try { info.os.uptime = Math.round(os.uptime()); } catch {}
    try { info.os.userInfo = os.userInfo().username; } catch {}
    info.os.distro = hostEnv?.os?.distro || '';
    info.os.release = hostEnv?.os?.release || '';
    if (!info.os.distro) {
      try { info.os.distro = execSync('cat /etc/os-release 2>/dev/null | grep "^PRETTY_NAME=" | cut -d= -f2 | tr -d \'"\'', { timeout: 5000 }).toString().trim() || ''; } catch {}
    }
    if (!info.os.release) {
      try { info.os.release = execSync('uname -r 2>/dev/null', { timeout: 5000 }).toString().trim(); } catch { info.os.release = 'unknown'; }
    }
    try { info.os.loadAvg = os.loadavg().map((v: number) => v.toFixed(2)).join(', '); } catch {}

    // Software detection — prefer host-env.json, fallback to local detection
    const execOpt = { timeout: 10000, encoding: 'utf-8' as const, shell: '/bin/bash' as const };
    try { info.software.node = process.version; } catch {}
    try { info.software.npm = execSync('npm --version 2>/dev/null', execOpt).toString().trim() || 'not installed'; } catch { info.software.npm = 'not installed'; }

    const hostSw = hostEnv?.software || {};
    info.software.docker = hostSw.docker || (() => { try { return execSync('docker --version 2>/dev/null', execOpt).toString().trim(); } catch { return 'not installed'; } })();
    info.software.git = hostSw.git || (() => { try { return execSync('git --version 2>/dev/null', execOpt).toString().trim(); } catch { return 'not installed'; } })();
    info.software.bun = hostSw.bun || (() => { try { return execSync('bun --version 2>/dev/null', execOpt).toString().trim(); } catch { return 'not installed'; } })();
    info.software.python = hostSw.python || (() => { try { return execSync('python3 --version 2>/dev/null', execOpt).toString().trim(); } catch { return 'not installed'; } })();

    // OpenClaw detection
    let ocVer = hostSw.openclaw || '';
    if (!ocVer || ocVer === 'not found') {
      // Check if openclaw config exists (proof it's installed and running)
      const ocConfigPath = path.join(OPENCLAW_DIR, 'openclaw.json');
      if (fs.existsSync(ocConfigPath)) {
        const ocCfg = openclawConfig.read();
        ocVer = ocCfg?.meta?.lastTouchedVersion ? `v${ocCfg.meta.lastTouchedVersion} (config)` : 'installed (config found)';
      }
    }
    info.software.openclaw = ocVer || 'not found';

    res.json({ ok: true, ...info });
  });

  // OpenClaw version info
  router.get('/system/version', auth, (_req, res) => {
    try {
      const ocConfig = openclawConfig.read();
      const currentVersion = ocConfig?.meta?.lastTouchedVersion || 'unknown';
      let updateInfo: any = {};
      const updateCheckPath = path.join(OPENCLAW_DIR, 'update-check.json');
      try {
        if (fs.existsSync(updateCheckPath)) {
          updateInfo = JSON.parse(fs.readFileSync(updateCheckPath, 'utf-8'));
        }
      } catch {}
      res.json({
        ok: true,
        currentVersion,
        latestVersion: updateInfo.lastNotifiedVersion || '',
        lastCheckedAt: updateInfo.lastCheckedAt || '',
        updateAvailable: updateInfo.lastNotifiedVersion && updateInfo.lastNotifiedVersion !== currentVersion,
      });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // Backup config
  router.post('/system/backup', auth, (_req, res) => {
    try {
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
      const backupDir = path.join(OPENCLAW_DIR, 'backups');
      fs.mkdirSync(backupDir, { recursive: true });
      const configSrc = path.join(OPENCLAW_DIR, 'openclaw.json');
      if (fs.existsSync(configSrc)) {
        fs.copyFileSync(configSrc, path.join(backupDir, `openclaw-${timestamp}.json`));
      }
      // Also backup cron jobs
      const cronSrc = path.join(OPENCLAW_DIR, 'cron', 'jobs.json');
      if (fs.existsSync(cronSrc)) {
        fs.copyFileSync(cronSrc, path.join(backupDir, `cron-jobs-${timestamp}.json`));
      }
      res.json({ ok: true, backupId: timestamp });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // List backups
  router.get('/system/backups', auth, (_req, res) => {
    try {
      const backupDir = path.join(OPENCLAW_DIR, 'backups');
      if (!fs.existsSync(backupDir)) return res.json({ ok: true, backups: [] });
      const files = fs.readdirSync(backupDir).filter(f => f.endsWith('.json')).sort().reverse();
      const backups = files.map(f => ({
        name: f,
        path: path.join(backupDir, f),
        size: fs.statSync(path.join(backupDir, f)).size,
        time: fs.statSync(path.join(backupDir, f)).mtime.toISOString(),
      }));
      res.json({ ok: true, backups });
    } catch (err) {
      res.json({ ok: true, backups: [] });
    }
  });

  // Restore backup
  router.post('/system/restore', auth, (req, res) => {
    try {
      const { backupName } = req.body;
      const backupDir = path.join(OPENCLAW_DIR, 'backups');
      const backupPath = path.join(backupDir, backupName);
      if (!fs.existsSync(backupPath)) return res.status(404).json({ ok: false, error: 'Backup not found' });
      // Auto-backup current before restore
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
      const configPath = path.join(OPENCLAW_DIR, 'openclaw.json');
      if (fs.existsSync(configPath)) {
        fs.copyFileSync(configPath, path.join(backupDir, `pre-restore-${timestamp}.json`));
      }
      if (backupName.startsWith('openclaw-')) {
        fs.copyFileSync(backupPath, configPath);
      } else if (backupName.startsWith('cron-jobs-')) {
        const cronPath = path.join(OPENCLAW_DIR, 'cron', 'jobs.json');
        fs.copyFileSync(backupPath, cronPath);
      }
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // ClawHub sync — fetch skill catalog from clawhub.ai (or use local cache)
  router.post('/system/clawhub-sync', auth, async (_req, res) => {
    const cachePath = path.join(OPENCLAW_DIR, 'clawhub-cache.json');
    try {
      // Try to fetch from ClawHub
      const https = await import('https');
      const fetchUrl = (url: string): Promise<string> => new Promise((resolve, reject) => {
        https.get(url, { timeout: 15000 }, (resp: any) => {
          let data = '';
          resp.on('data', (c: string) => data += c);
          resp.on('end', () => resolve(data));
        }).on('error', reject);
      });
      // Fetch the skills page and extract data
      const html = await fetchUrl('https://clawhub.ai/skills?sort=downloads');
      // Parse skill entries from HTML (basic extraction)
      const skills: any[] = [];
      const regex = /href="\/skills\/([^"]+)"[^>]*>[\s\S]*?<h[23][^>]*>([^<]+)<\/h[23]>[\s\S]*?<p[^>]*>([^<]*)<\/p>/gi;
      let match;
      while ((match = regex.exec(html)) !== null) {
        skills.push({ id: match[1].trim(), name: match[2].trim(), description: match[3].trim() });
      }
      if (skills.length > 0) {
        fs.writeFileSync(cachePath, JSON.stringify({ skills, syncedAt: new Date().toISOString() }), 'utf-8');
        return res.json({ ok: true, skills, source: 'remote' });
      }
      // If parsing failed, try cached
      if (fs.existsSync(cachePath)) {
        const cached = JSON.parse(fs.readFileSync(cachePath, 'utf-8'));
        return res.json({ ok: true, skills: cached.skills || [], source: 'cache', syncedAt: cached.syncedAt });
      }
      res.json({ ok: true, skills: [], source: 'empty' });
    } catch (err) {
      // Fallback to cache
      try {
        if (fs.existsSync(cachePath)) {
          const cached = JSON.parse(fs.readFileSync(cachePath, 'utf-8'));
          return res.json({ ok: true, skills: cached.skills || [], source: 'cache', syncedAt: cached.syncedAt });
        }
      } catch {}
      res.json({ ok: true, skills: [], source: 'error', error: String(err) });
    }
  });

  // Scan installed skills/extensions — returns { skills, plugins } separately
  router.get('/system/skills', auth, async (_req, res) => {
    try {
      const skills: any[] = [];
      const plugins: any[] = [];
      const seen = new Set<string>();

      // Helper to parse SKILL.md frontmatter
      const parseSkillMd = (mdPath: string) => {
        try {
          const content = fs.readFileSync(mdPath, 'utf-8');
          const match = content.match(/^---\n([\s\S]*?)\n---/);
          if (!match) return null;
          const frontmatter = match[1];
          const nameMatch = frontmatter.match(/^name:\s*(.+)$/m);
          const descMatch = frontmatter.match(/^description:\s*["']?(.+?)["']?$/m);
          const metaMatch = frontmatter.match(/^metadata:\s*(\{[\s\S]*?\})\s*$/m);
          let metadata = null;
          if (metaMatch) {
            try { metadata = JSON.parse(metaMatch[1].replace(/\n/g, '')); } catch {}
          }
          return {
            name: nameMatch?.[1]?.trim(),
            description: descMatch?.[1]?.trim(),
            metadata
          };
        } catch { return null; }
      };

      const ocConfig = openclawConfig.read() || {};
      const pluginEntries = ocConfig?.plugins?.entries || {};
      const pluginInstalls = ocConfig?.plugins?.installs || {};
      const pluginSeen = new Set<string>();

      // --- PLUGINS: scan extensions directories ---
      const extDir = path.join(OPENCLAW_DIR, 'extensions');
      if (fs.existsSync(extDir)) {
        for (const name of fs.readdirSync(extDir)) {
          const extPath = path.join(extDir, name);
          if (!fs.statSync(extPath).isDirectory()) continue;
          let pkgInfo: any = {};
          try { pkgInfo = JSON.parse(fs.readFileSync(path.join(extPath, 'package.json'), 'utf-8')); } catch {}
          let pluginJson: any = {};
          try { pluginJson = JSON.parse(fs.readFileSync(path.join(extPath, 'openclaw.plugin.json'), 'utf-8')); } catch {}
          pluginSeen.add(name);
          plugins.push({
            id: name,
            name: pluginJson.name || pkgInfo.name || name,
            description: pluginJson.description || pkgInfo.description || '',
            version: pluginInstalls[name]?.version || pkgInfo.version || '',
            enabled: pluginEntries[name]?.enabled !== false,
            source: 'installed',
            installedAt: pluginInstalls[name]?.installedAt || '',
            path: extPath,
          });
        }
      }

      // Also scan config/extensions (development extensions)
      const configExtDir = path.join(path.dirname(OPENCLAW_DIR), 'openclaw', 'config', 'extensions');
      if (fs.existsSync(configExtDir)) {
        for (const name of fs.readdirSync(configExtDir)) {
          if (pluginSeen.has(name)) continue;
          const extPath = path.join(configExtDir, name);
          if (!fs.statSync(extPath).isDirectory()) continue;
          let pkgInfo: any = {};
          try { pkgInfo = JSON.parse(fs.readFileSync(path.join(extPath, 'package.json'), 'utf-8')); } catch {}
          let pluginJson: any = {};
          try { pluginJson = JSON.parse(fs.readFileSync(path.join(extPath, 'openclaw.plugin.json'), 'utf-8')); } catch {}
          pluginSeen.add(name);
          plugins.push({
            id: name,
            name: pluginJson.name || pkgInfo.name || name,
            description: pluginJson.description || pkgInfo.description || '',
            version: pluginInstalls[name]?.version || pkgInfo.version || '',
            enabled: pluginEntries[name]?.enabled !== false,
            source: 'config-ext',
            path: extPath,
          });
        }
      }

      // Add entries from plugins.entries that aren't already scanned (e.g. NapCat QQ)
      for (const [id, entry] of Object.entries(pluginEntries)) {
        if (pluginSeen.has(id)) continue;
        pluginSeen.add(id);
        plugins.push({
          id,
          name: id,
          description: '',
          version: pluginInstalls[id]?.version || '',
          enabled: (entry as any)?.enabled !== false,
          source: 'config',
          installedAt: pluginInstalls[id]?.installedAt || '',
        });
      }

      // --- SKILLS: scan skill directories ---

      // Helper to scan a skills directory
      const scanSkillDir = (dir: string, source: string, requireSkillMd = false) => {
        if (!fs.existsSync(dir)) return;
        for (const name of fs.readdirSync(dir)) {
          if (seen.has(name)) continue;
          const skillPath = path.join(dir, name);
          if (!fs.statSync(skillPath).isDirectory()) continue;
          const skillMdPath = path.join(skillPath, 'SKILL.md');
          if (requireSkillMd && !fs.existsSync(skillMdPath)) continue;
          let pkgInfo: any = {};
          try { pkgInfo = JSON.parse(fs.readFileSync(path.join(skillPath, 'package.json'), 'utf-8')); } catch {}
          let skillDesc = pkgInfo.description || '';
          let skillName = pkgInfo.name || name;
          let metadata = null;
          let requires = undefined;
          if (fs.existsSync(skillMdPath)) {
            const skillInfo = parseSkillMd(skillMdPath);
            if (skillInfo) {
              skillName = skillInfo.name || skillName;
              skillDesc = skillInfo.description || skillDesc;
              metadata = skillInfo.metadata;
              requires = skillInfo.metadata?.openclaw?.requires;
            }
          }
          if (!skillDesc) {
            try {
              const md = fs.readFileSync(skillMdPath, 'utf-8');
              const firstLine = md.split('\n').find((l: string) => l.trim() && !l.startsWith('#') && !l.startsWith('---'));
              if (firstLine) skillDesc = firstLine.trim().slice(0, 200);
            } catch {}
          }
          seen.add(name);
          skills.push({
            id: name,
            name: skillName,
            description: skillDesc,
            version: pkgInfo.version || '',
            enabled: true,
            source,
            path: skillPath,
            metadata,
            requires,
          });
        }
      };

      // 1. OPENCLAW_DIR/skills (user-installed skills)
      scanSkillDir(path.join(OPENCLAW_DIR, 'skills'), 'skill');

      // 2. Workspace work/skills
      const workDir = process.env['OPENCLAW_WORK'] || path.join(path.dirname(OPENCLAW_DIR), 'openclaw', 'work');
      scanSkillDir(path.join(workDir, 'skills'), 'workspace');

      // 3. Built-in app skills — use OPENCLAW_APP env var, fallback to common paths
      const appDir = process.env['OPENCLAW_APP'] || '';
      const appSkillsCandidates = [
        appDir ? path.join(appDir, 'skills') : '',
        path.join(path.dirname(OPENCLAW_DIR), 'openclaw', 'app', 'skills'),
        path.join(path.dirname(OPENCLAW_DIR), 'app', 'skills'),
        '/home/zhaoxinyi/openclaw/app/skills',
      ].filter(Boolean);
      for (const candidate of appSkillsCandidates) {
        if (fs.existsSync(candidate)) {
          scanSkillDir(candidate, 'app-skill', true);
          break;
        }
      }

      // 4. Workspace work/scripts (utility scripts as skills)
      const workScriptsDir = path.join(workDir, 'scripts');
      if (fs.existsSync(workScriptsDir)) {
        for (const name of fs.readdirSync(workScriptsDir)) {
          const scriptPath = path.join(workScriptsDir, name);
          if (fs.statSync(scriptPath).isDirectory()) continue;
          if (!name.endsWith('.js') && !name.endsWith('.ts')) continue;
          const scriptId = 'script-' + name.replace(/\.(js|ts)$/, '');
          if (seen.has(scriptId)) continue;
          seen.add(scriptId);
          const friendlyName = name.replace(/\.(js|ts)$/, '').replace(/[-_]/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
          skills.push({
            id: scriptId,
            name: friendlyName,
            description: `工作区脚本: ${name}`,
            version: '',
            enabled: true,
            source: 'script',
            path: scriptPath,
          });
        }
      }

      res.json({ ok: true, skills, plugins });
    } catch (err) {
      res.json({ ok: true, skills: [], plugins: [] });
    }
  });

  // Read cron jobs directly from file
  router.get('/system/cron', auth, (_req, res) => {
    try {
      const cronPath = path.join(OPENCLAW_DIR, 'cron', 'jobs.json');
      if (!fs.existsSync(cronPath)) return res.json({ ok: true, jobs: [] });
      const data = JSON.parse(fs.readFileSync(cronPath, 'utf-8'));
      res.json({ ok: true, jobs: data.jobs || [] });
    } catch (err) {
      res.json({ ok: true, jobs: [] });
    }
  });

  // Write cron jobs directly to file (for toggle/delete/create)
  router.put('/system/cron', auth, (req, res) => {
    try {
      const cronPath = path.join(OPENCLAW_DIR, 'cron', 'jobs.json');
      const cronDir = path.dirname(cronPath);
      if (!fs.existsSync(cronDir)) fs.mkdirSync(cronDir, { recursive: true });
      const jobs = req.body.jobs || [];
      // Preserve existing file structure (version field etc.)
      let existing: any = { version: 1 };
      try {
        if (fs.existsSync(cronPath)) {
          existing = JSON.parse(fs.readFileSync(cronPath, 'utf-8'));
        }
      } catch {}
      existing.jobs = jobs;
      fs.writeFileSync(cronPath, JSON.stringify(existing, null, 2));
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // Read identity MD files from workspace (AGENTS.md, BOOTSTRAP.md, etc.)
  router.get('/system/identity-docs', auth, (_req, res) => {
    try {
      const docs: any[] = [];
      const workDir = process.env['OPENCLAW_WORK'] || path.join(path.dirname(OPENCLAW_DIR), 'openclaw', 'work');
      const IDENTITY_FILES = ['AGENTS.md', 'BOOTSTRAP.md', 'HEARTBEAT.md', 'IDENTITY.md', 'SOUL.md', 'TOOLS.md', 'USER.md'];
      for (const name of IDENTITY_FILES) {
        const full = path.join(workDir, name);
        if (fs.existsSync(full)) {
          const stat = fs.statSync(full);
          docs.push({
            name,
            path: full,
            content: fs.readFileSync(full, 'utf-8'),
            size: stat.size,
            modified: stat.mtime.toISOString(),
          });
        } else {
          docs.push({ name, path: full, content: '', size: 0, exists: false });
        }
      }
      res.json({ ok: true, docs, workDir });
    } catch (err) {
      res.json({ ok: true, docs: [] });
    }
  });

  // Save identity MD file
  router.put('/system/identity-docs', auth, (req, res) => {
    try {
      const { path: docPath, content } = req.body;
      if (!docPath || !docPath.endsWith('.md')) return res.status(400).json({ ok: false, error: 'Invalid path' });
      const workDir = process.env['OPENCLAW_WORK'] || path.join(path.dirname(OPENCLAW_DIR), 'openclaw', 'work');
      const resolved = path.resolve(docPath);
      // Security: ensure path is within work dir or openclaw dir
      if (!resolved.startsWith(path.resolve(workDir)) && !resolved.startsWith(path.resolve(OPENCLAW_DIR))) {
        return res.status(403).json({ ok: false, error: 'Path outside allowed directories' });
      }
      fs.writeFileSync(resolved, content, 'utf-8');
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // Check for updates manually
  router.post('/system/check-update', auth, async (_req, res) => {
    try {
      const { execSync } = await import('child_process');
      const os = await import('os');
      const execOpt = { timeout: 30000, encoding: 'utf-8' as const, env: { ...process.env, PATH: (process.env.PATH || '') + ':/usr/local/bin:/usr/bin:/bin:/snap/bin:/home/' + (os.userInfo().username || '') + '/.local/bin' }, shell: '/bin/bash' as const };
      let latestVersion = '';
      let updateAvailable = false;
      const ocConfig = openclawConfig.read();
      const currentVersion = ocConfig?.meta?.lastTouchedVersion || 'unknown';

      // Try openclaw update --check or similar
      try {
        const out = execSync('openclaw update --check 2>/dev/null || openclaw version --check 2>/dev/null || echo ""', execOpt).toString().trim();
        if (out) latestVersion = out;
      } catch {}

      // If that didn't work, try npm/github
      if (!latestVersion) {
        try {
          const out = execSync('npm view openclaw version 2>/dev/null || echo ""', execOpt).toString().trim();
          if (out && out !== '') latestVersion = out;
        } catch {}
      }

      if (latestVersion && latestVersion !== currentVersion) {
        updateAvailable = true;
      }

      // Save check result
      const updateCheckPath = path.join(OPENCLAW_DIR, 'update-check.json');
      const checkData = {
        lastCheckedAt: new Date().toISOString(),
        lastNotifiedVersion: latestVersion || currentVersion,
        updateAvailable,
      };
      fs.writeFileSync(updateCheckPath, JSON.stringify(checkData, null, 2), 'utf-8');

      res.json({ ok: true, currentVersion, latestVersion: latestVersion || currentVersion, updateAvailable, checkedAt: checkData.lastCheckedAt });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // Perform OpenClaw update — uses file-based signaling to host
  // Container writes signal file to mounted .openclaw volume, host watcher script picks it up
  const UPDATE_SIGNAL = path.join(OPENCLAW_DIR, 'update-signal.json');
  const UPDATE_RESULT = path.join(OPENCLAW_DIR, 'update-result.json');
  const UPDATE_LOG = path.join(OPENCLAW_DIR, 'update-log.txt');

  router.post('/system/do-update', auth, async (_req, res) => {
    // Check if already running
    try {
      if (fs.existsSync(UPDATE_RESULT)) {
        const result = JSON.parse(fs.readFileSync(UPDATE_RESULT, 'utf-8'));
        if (result.status === 'running') {
          return res.json({ ok: false, error: '更新正在进行中' });
        }
      }
    } catch {}

    // Write signal file for host watcher
    try {
      fs.writeFileSync(UPDATE_SIGNAL, JSON.stringify({ requestedAt: new Date().toISOString() }));
      fs.writeFileSync(UPDATE_RESULT, JSON.stringify({ status: 'running', log: ['等待宿主机执行更新...'], startedAt: new Date().toISOString() }));
      res.json({ ok: true, message: '更新请求已发送' });
    } catch (err) {
      res.status(500).json({ ok: false, error: '无法发送更新请求: ' + String(err) });
    }
  });

  // Get update progress — reads result file written by host watcher
  router.get('/system/update-status', auth, (_req, res) => {
    try {
      if (fs.existsSync(UPDATE_RESULT)) {
        const result = JSON.parse(fs.readFileSync(UPDATE_RESULT, 'utf-8'));
        // Also try to read live log
        let log = result.log || [];
        if (fs.existsSync(UPDATE_LOG)) {
          try {
            const logContent = fs.readFileSync(UPDATE_LOG, 'utf-8').trim();
            if (logContent) log = logContent.split('\n');
          } catch {}
        }

        // If update succeeded, send QQ notification (once)
        if (result.status === 'success' && !result.notified) {
          try {
            const qqConf = openclawConfig.read()?.channels?.qq;
            const ownerQQ = qqConf?.ownerQQ || adminConfig.get().qq?.ownerQQ;
            if (ownerQQ && onebotClient.connected) {
              const ocCfg = openclawConfig.read();
              const newVer = ocCfg?.meta?.lastTouchedVersion || 'unknown';
              onebotClient.sendPrivateMsg(ownerQQ, [{ type: 'text', data: { text: `🔄 OpenClaw 更新完成！\n新版本: ${newVer}\n时间: ${new Date().toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' })}` } }]);
              log.push(`📨 已向主人QQ(${ownerQQ})发送更新通知`);
            }
          } catch {}
          result.notified = true;
          fs.writeFileSync(UPDATE_RESULT, JSON.stringify({ ...result, notified: true }));
        }

        res.json({ ok: true, status: result.status, log, startedAt: result.startedAt, finishedAt: result.finishedAt });
      } else {
        res.json({ ok: true, status: 'idle', log: [] });
      }
    } catch (err) {
      res.json({ ok: true, status: 'idle', log: [] });
    }
  });

  // Restart OpenClaw gateway — writes signal file for host watcher
  const RESTART_SIGNAL = path.join(OPENCLAW_DIR, 'restart-gateway-signal.json');
  const RESTART_RESULT = path.join(OPENCLAW_DIR, 'restart-gateway-result.json');

  router.post('/system/restart-gateway', auth, async (_req, res) => {
    try {
      fs.writeFileSync(RESTART_SIGNAL, JSON.stringify({ requestedAt: new Date().toISOString() }));
      res.json({ ok: true, message: '网关重启请求已发送' });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  router.get('/system/restart-gateway-status', auth, (_req, res) => {
    try {
      if (fs.existsSync(RESTART_RESULT)) {
        const result = JSON.parse(fs.readFileSync(RESTART_RESULT, 'utf-8'));
        res.json({ ok: true, ...result });
      } else {
        res.json({ ok: true, status: 'idle' });
      }
    } catch {
      res.json({ ok: true, status: 'idle' });
    }
  });

  // Get admin login token (for display in system config)
  router.get('/system/admin-token', auth, (_req, res) => {
    try {
      const cfg = adminConfig.get();
      res.json({ ok: true, token: cfg.server?.token || '' });
    } catch (err) {
      res.json({ ok: false, token: '' });
    }
  });

  // Get/set sudo password for update operations
  router.get('/system/sudo-password', auth, (_req, res) => {
    try {
      const cfg = adminConfig.get();
      const hasPwd = Boolean(cfg.system?.sudoPassword);
      res.json({ ok: true, configured: hasPwd });
    } catch (err) {
      res.json({ ok: false, configured: false });
    }
  });

  router.put('/system/sudo-password', auth, (req, res) => {
    try {
      const { password } = req.body;
      adminConfig.update({ system: { sudoPassword: password || '' } });
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // Read MD docs from openclaw dir (agent profile, etc)
  router.get('/system/docs', auth, (_req, res) => {
    try {
      const docs: any[] = [];
      // Check agents dir for profile docs
      const agentsDir = path.join(OPENCLAW_DIR, 'agents');
      if (fs.existsSync(agentsDir)) {
        const scanDir = (dir: string, prefix: string) => {
          for (const name of fs.readdirSync(dir)) {
            const full = path.join(dir, name);
            const stat = fs.statSync(full);
            if (stat.isDirectory()) {
              scanDir(full, prefix + name + '/');
            } else if (name.endsWith('.md')) {
              docs.push({
                name: prefix + name,
                path: full,
                content: fs.readFileSync(full, 'utf-8'),
                size: stat.size,
              });
            }
          }
        };
        scanDir(agentsDir, 'agents/');
      }
      // Check root-level md files
      for (const name of fs.readdirSync(OPENCLAW_DIR)) {
        if (name.endsWith('.md')) {
          const full = path.join(OPENCLAW_DIR, name);
          docs.push({
            name,
            path: full,
            content: fs.readFileSync(full, 'utf-8'),
            size: fs.statSync(full).size,
          });
        }
      }
      res.json({ ok: true, docs });
    } catch (err) {
      res.json({ ok: true, docs: [] });
    }
  });

  // Save MD doc
  router.put('/system/docs', auth, (req, res) => {
    try {
      const { path: docPath, content } = req.body;
      if (!docPath || !docPath.endsWith('.md')) return res.status(400).json({ ok: false, error: 'Invalid path' });
      // Security: ensure path is within OPENCLAW_DIR
      const resolved = path.resolve(docPath);
      if (!resolved.startsWith(path.resolve(OPENCLAW_DIR))) {
        return res.status(403).json({ ok: false, error: 'Path outside openclaw dir' });
      }
      fs.writeFileSync(resolved, content, 'utf-8');
      res.json({ ok: true });
    } catch (err) {
      res.status(500).json({ ok: false, error: String(err) });
    }
  });

  // === Event Log API ===
  if (eventLog) {
    router.get('/events', auth, (req: any, res: any) => {
      const limit = parseInt(req.query.limit) || 100;
      const offset = parseInt(req.query.offset) || 0;
      const source = req.query.source as string || undefined;
      const search = req.query.search as string || undefined;
      const result = eventLog.getEntries({ limit, offset, source, search });
      res.json({ ok: true, ...result });
    });

    router.post('/events/clear', auth, (_req: any, res: any) => {
      eventLog.clear();
      res.json({ ok: true });
    });

    // Allow external services (e.g. OpenClaw QQ plugin) to post log entries
    router.post('/events/log', (req: any, res: any) => {
      const { source, type, summary, detail } = req.body || {};
      if (!summary) return res.status(400).json({ ok: false, error: 'summary required' });
      const entry = eventLog.add({
        time: Date.now(),
        source: source || 'openclaw',
        type: type || 'openclaw.action',
        summary,
        detail,
      });
      res.json({ ok: true, id: entry.id });
    });
  }

  return router;
}
