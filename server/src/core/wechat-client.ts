import { EventEmitter } from 'events';
import http from 'http';
import https from 'https';

export interface WeChatConfig {
  apiUrl: string;   // e.g. http://wechat:3001
  token: string;    // LOGIN_API_TOKEN
}

export class WeChatClient extends EventEmitter {
  private apiUrl: string;
  private token: string;
  public connected = false;
  public loginUser: { name: string; id: string } | null = null;
  private healthTimer: ReturnType<typeof setInterval> | null = null;

  constructor(config: WeChatConfig) {
    super();
    this.apiUrl = config.apiUrl || 'http://wechat:3001';
    this.token = config.token || '';
  }

  start() {
    this.checkHealth();
    this.healthTimer = setInterval(() => this.checkHealth(), 10000);
  }

  stop() {
    if (this.healthTimer) {
      clearInterval(this.healthTimer);
      this.healthTimer = null;
    }
  }

  private async checkHealth() {
    try {
      const res = await this.request('GET', `/healthz?token=${this.token}`);
      const wasConnected = this.connected;
      if (typeof res === 'string') {
        this.connected = res.trim() === 'healthy';
      } else if (res && typeof res === 'object') {
        this.connected = res.success === true;
      } else {
        this.connected = false;
      }
      if (this.connected && !wasConnected) {
        this.emit('connect');
      } else if (!this.connected && wasConnected) {
        this.emit('disconnect');
      }
    } catch {
      if (this.connected) {
        this.connected = false;
        this.emit('disconnect');
      }
    }
  }

  async getLoginStatus(): Promise<any> {
    try {
      const res = await this.request('GET', `/login?token=${this.token}`);
      if (res && typeof res === 'object') {
        if (res.success && res.message) {
          // Already logged in: {"success":true,"message":"Contact<Name>is already login"}
          const match = res.message.match(/Contact<(.+?)>is already login/);
          if (match) {
            this.loginUser = { name: match[1], id: '' };
            this.connected = true;
            return { loggedIn: true, name: match[1] };
          }
        }
        return { loggedIn: false, raw: res };
      }
      // If response is HTML (QR code page), user is not logged in
      return { loggedIn: false, needScan: true };
    } catch (e) {
      return { loggedIn: false, error: String(e) };
    }
  }

  getLoginUrl(): string {
    return `${this.apiUrl}/login?token=${this.token}`;
  }

  async sendMessage(to: string, content: string, isRoom = false): Promise<any> {
    const body: any = {
      to,
      isRoom,
      data: { type: 'text', content },
    };
    return this.request('POST', `/webhook/msg/v2?token=${this.token}`, body);
  }

  async sendFile(to: string, fileUrl: string, isRoom = false): Promise<any> {
    const body: any = {
      to,
      isRoom,
      data: { type: 'fileUrl', content: fileUrl },
    };
    return this.request('POST', `/webhook/msg/v2?token=${this.token}`, body);
  }

  // Handle incoming message from webhook callback
  handleCallback(formData: any): void {
    const type = formData.type || 'unknown';
    const content = formData.content || '';
    const isMentioned = formData.isMentioned === '1';
    const isMsgFromSelf = formData.isMsgFromSelf === '1';
    const isSystemEvent = type.startsWith('system_event_');

    let source: any = {};
    try {
      if (formData.source && typeof formData.source === 'string') {
        source = JSON.parse(formData.source);
      } else if (formData.source && typeof formData.source === 'object') {
        source = formData.source;
      }
    } catch {}

    if (isSystemEvent) {
      if (type === 'system_event_login') {
        this.connected = true;
        this.emit('login', { name: source?.from?.payload?.name || '' });
      } else if (type === 'system_event_logout') {
        this.connected = false;
        this.emit('disconnect');
      }
      this.emit('system', { type, content, source });
      return;
    }

    if (isMsgFromSelf) return;

    const fromName = source?.from?.payload?.name || source?.from?.name || '';
    const fromId = source?.from?.payload?.id || source?.from?.id || '';
    const roomName = source?.room || '';
    const isGroup = !!roomName;

    const event = {
      type,
      content,
      fromName,
      fromId,
      roomName,
      isGroup,
      isMentioned,
      source,
      timestamp: Date.now(),
    };

    this.emit('message', event);
  }

  private request(method: string, path: string, body?: any): Promise<any> {
    return new Promise((resolve, reject) => {
      const url = new URL(path, this.apiUrl);
      const isHttps = url.protocol === 'https:';
      const data = body ? JSON.stringify(body) : '';
      const headers: Record<string, string> = {};
      if (body) {
        headers['Content-Type'] = 'application/json';
        headers['Content-Length'] = Buffer.byteLength(data).toString();
      }

      const mod = isHttps ? https : http;
      const req = mod.request({
        hostname: url.hostname,
        port: url.port,
        path: url.pathname + url.search,
        method,
        headers,
      }, (res) => {
        let chunks = '';
        res.on('data', (c: Buffer) => chunks += c.toString());
        res.on('end', () => {
          try { resolve(JSON.parse(chunks)); }
          catch { resolve(chunks); }
        });
      });
      req.on('error', reject);
      req.setTimeout(10000, () => { req.destroy(); reject(new Error('timeout')); });
      if (data) req.write(data);
      req.end();
    });
  }
}
