import { Router } from 'express';
import jwt from 'jsonwebtoken';
import http from 'http';
import crypto from 'crypto';
import { AdminConfig } from '../core/admin-config.js';
import { OneBotClient } from '../core/onebot-client.js';
import { OpenClawConfig } from '../core/openclaw-config.js';
import { WeChatClient } from '../core/wechat-client.js';
import { getPendingRequests } from '../core/event-router.js';
import { JWT_SECRET } from '../core/ws-manager.js';

export function createRoutes(adminConfig: AdminConfig, onebotClient: OneBotClient, openclawConfig: OpenClawConfig, wechatClient: WeChatClient) {
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
      },
      admin: {
        uptime: process.uptime(),
        memoryMB: Math.round(process.memoryUsage().rss / 1024 / 1024),
      },
    });
  });

  // === OpenClaw Config ===
  router.get('/openclaw/config', auth, (_req, res) => {
    const config = openclawConfig.read();
    res.json({ ok: true, config: config || {} });
  });

  router.put('/openclaw/config', auth, (req, res) => {
    try {
      openclawConfig.write(req.body.config);
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
    const webuiToken = adminCfg.get().server.token || 'openclaw-qq-admin';
    const hash = crypto.createHash('sha256').update(webuiToken + '.napcat').digest('hex');
    try {
      const res = await napcatProxy('POST', '/api/auth/login', { hash });
      if (res.code === 0 && res.data?.Credential) {
        napcatCredential = res.data.Credential;
      }
    } catch {}
    return napcatCredential;
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

  // QQ Login Status
  router.post('/napcat/login-status', auth, async (_req, res) => {
    try {
      const cred = await napcatAuth(adminConfig);
      const r = await napcatProxy('POST', '/api/QQLogin/CheckLoginStatus', {}, cred);
      res.json({ ok: true, ...r });
    } catch (err) { res.json({ ok: false, error: String(err) }); }
  });

  // Get QR Code
  router.post('/napcat/qrcode', auth, async (_req, res) => {
    try {
      const cred = await napcatAuth(adminConfig);
      const r = await napcatProxy('POST', '/api/QQLogin/GetQQLoginQrcode', {}, cred);
      res.json({ ok: true, ...r });
    } catch (err) { res.json({ ok: false, error: String(err) }); }
  });

  // Refresh QR Code
  router.post('/napcat/qrcode/refresh', auth, async (_req, res) => {
    try {
      const cred = await napcatAuth(adminConfig);
      const r = await napcatProxy('POST', '/api/QQLogin/RefreshQRcode', {}, cred);
      res.json({ ok: true, ...r });
    } catch (err) { res.json({ ok: false, error: String(err) }); }
  });

  // Quick Login List
  router.get('/napcat/quick-login-list', auth, async (_req, res) => {
    try {
      const cred = await napcatAuth(adminConfig);
      const r = await napcatProxy('POST', '/api/QQLogin/GetQuickLoginQQ', {}, cred);
      res.json({ ok: true, ...r });
    } catch (err) { res.json({ ok: false, error: String(err) }); }
  });

  // Quick Login
  router.post('/napcat/quick-login', auth, async (req, res) => {
    try {
      const cred = await napcatAuth(adminConfig);
      const r = await napcatProxy('POST', '/api/QQLogin/SetQuickLogin', { uin: req.body.uin }, cred);
      res.json({ ok: true, ...r });
    } catch (err) { res.json({ ok: false, error: String(err) }); }
  });

  // Password Login (server computes MD5 since browsers don't support it)
  router.post('/napcat/password-login', auth, async (req, res) => {
    try {
      const cred = await napcatAuth(adminConfig);
      const pwd = req.body.password || req.body.passwordMd5 || '';
      const passwordMd5 = crypto.createHash('md5').update(pwd).digest('hex');
      const r = await napcatProxy('POST', '/api/QQLogin/PasswordLogin', { uin: req.body.uin, passwordMd5 }, cred);
      res.json({ ok: true, ...r });
    } catch (err) { res.json({ ok: false, error: String(err) }); }
  });

  // Get QQ Login Info
  router.get('/napcat/login-info', auth, async (_req, res) => {
    try {
      const cred = await napcatAuth(adminConfig);
      const r = await napcatProxy('POST', '/api/QQLogin/GetQQLoginInfo', {}, cred);
      res.json({ ok: true, ...r });
    } catch (err) { res.json({ ok: false, error: String(err) }); }
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

  return router;
}
