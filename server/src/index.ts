import express from 'express';
import http from 'http';
import path from 'path';
import multer from 'multer';
import { fileURLToPath } from 'url';
import { AdminConfig } from './core/admin-config.js';
import { OneBotClient } from './core/onebot-client.js';
import { OpenClawConfig } from './core/openclaw-config.js';
import { WsManager } from './core/ws-manager.js';
import { WeChatClient } from './core/wechat-client.js';
import { createEventRouter } from './core/event-router.js';
import { createRoutes } from './routes/index.js';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

async function main() {
  console.log('[System] OpenClaw QQ & WeChat Manager 启动中...');

  // Load configs
  const adminConfig = new AdminConfig();
  const cfg = adminConfig.get();
  const openclawConfig = new OpenClawConfig(cfg.openclaw.configPath || undefined);

  // Auto-setup OpenClaw config
  if (cfg.openclaw.autoSetup) {
    openclawConfig.autoSetup(cfg.napcat.wsUrl, cfg.napcat.accessToken, cfg.qq.ownerQQ);
  }

  // Create OneBot client (connects to NapCat in the same container)
  const onebotClient = new OneBotClient(cfg.napcat.wsUrl, cfg.napcat.accessToken);

  // Create WeChat client
  const wechatClient = new WeChatClient({
    apiUrl: cfg.wechat.apiUrl,
    token: cfg.wechat.token,
  });

  // Create event router for QQ features
  const eventRouter = createEventRouter(onebotClient, cfg.qq);

  // Express app
  const app = express();
  app.use(express.json());
  app.use(express.urlencoded({ extended: true }));

  // Multer for multipart/form-data (wechatbot-webhook callback)
  const upload = multer({ storage: multer.memoryStorage() });

  // === WeChat message callback (from wechatbot-webhook container) ===
  // This endpoint receives messages WITHOUT auth (called by wechat container internally)
  app.post('/api/wechat/callback', upload.any(), (req, res) => {
    try {
      const formData = req.body || {};
      // If files are present (images, etc.), note them
      if (req.files && Array.isArray(req.files) && req.files.length > 0) {
        formData._files = (req.files as Express.Multer.File[]).map(f => ({
          fieldname: f.fieldname,
          originalname: f.originalname,
          mimetype: f.mimetype,
          size: f.size,
        }));
      }

      console.log(`[WeChat] 收到消息: type=${formData.type}, content=${(formData.content || '').slice(0, 50)}`);
      wechatClient.handleCallback(formData);
      res.json({ success: true });
    } catch (err) {
      console.error('[WeChat] 回调处理失败:', err);
      res.status(500).json({ success: false, error: String(err) });
    }
  });

  // API routes
  const routes = createRoutes(adminConfig, onebotClient, openclawConfig, wechatClient);
  app.use('/api', routes);

  // Serve frontend
  const webDistPath = path.resolve(__dirname, '../../web/dist');
  app.use(express.static(webDistPath));
  app.get('*', (_req, res) => {
    res.sendFile(path.join(webDistPath, 'index.html'));
  });

  // HTTP + WebSocket server
  const server = http.createServer(app);
  const wsManager = new WsManager(server);

  // === NapCat (QQ) ===
  onebotClient.connect();

  onebotClient.on('connect', () => {
    console.log('[NapCat] 已连接 OneBot11 WebSocket');
    wsManager.broadcast('napcat-status', { connected: true, selfId: onebotClient.selfId, nickname: onebotClient.nickname });
  });

  onebotClient.on('login', (info: any) => {
    console.log(`[NapCat] Bot 登录: ${info.nickname}(${info.selfId})`);
    wsManager.broadcast('napcat-status', { connected: true, ...info });
  });

  onebotClient.on('disconnect', () => {
    console.log('[NapCat] 连接断开，5秒后重连...');
    wsManager.broadcast('napcat-status', { connected: false });
  });

  onebotClient.on('event', (event: any) => {
    eventRouter(event);
    wsManager.broadcast('event', event);
  });

  // === WeChat ===
  wechatClient.start();

  wechatClient.on('connect', () => {
    console.log('[WeChat] 微信已连接');
    wsManager.broadcast('wechat-status', { connected: true, name: wechatClient.loginUser?.name || '' });
  });

  wechatClient.on('disconnect', () => {
    console.log('[WeChat] 微信已断开');
    wsManager.broadcast('wechat-status', { connected: false });
  });

  wechatClient.on('login', (info: any) => {
    console.log(`[WeChat] 微信登录: ${info.name}`);
    wsManager.broadcast('wechat-status', { connected: true, name: info.name });
  });

  wechatClient.on('message', async (event: any) => {
    console.log(`[WeChat] 消息: ${event.fromName}: ${(event.content || '').slice(0, 50)}`);
    wsManager.broadcast('wechat-event', event);

    // Auto-reply via OpenClaw if enabled
    const wcfg = adminConfig.get().wechat;
    if (wcfg.enabled && wcfg.autoReply && event.type === 'text' && event.content) {
      // Only reply to private messages (not group) unless mentioned
      if (!event.isGroup || event.isMentioned) {
        try {
          // Forward to OpenClaw via the QQ plugin's OneBot interface
          // For now, we use a simple echo or the user can configure OpenClaw's wechat channel
          // The bridge: receive wechat msg -> call OpenClaw API -> send reply back
          // This will be handled by the wechat-bridge module
        } catch (err) {
          console.error('[WeChat] 自动回复失败:', err);
        }
      }
    }
  });

  wechatClient.on('system', (event: any) => {
    console.log(`[WeChat] 系统事件: ${event.type}`);
    wsManager.broadcast('wechat-event', event);
  });

  // Start server
  server.listen(cfg.server.port, cfg.server.host, () => {
    console.log(`[System] 管理后台已启动: http://${cfg.server.host}:${cfg.server.port}`);
    console.log(`[System] NapCat WebSocket: ${cfg.napcat.wsUrl}`);
    console.log(`[System] WeChat API: ${cfg.wechat.apiUrl}`);
    console.log(`[System] OpenClaw 配置: ${cfg.openclaw.configPath}`);
  });
}

main().catch(err => {
  console.error('[System] 启动失败:', err);
  process.exit(1);
});
