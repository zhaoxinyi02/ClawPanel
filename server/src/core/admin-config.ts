import fs from 'fs';
import path from 'path';

export interface AdminConfigData {
  server: { port: number; host: string; token: string };
  openclaw: { configPath: string; autoSetup: boolean };
  napcat: { wsUrl: string; accessToken: string; webuiPort: number };
  wechat: { apiUrl: string; token: string; enabled: boolean; autoReply: boolean };
  qq: {
    ownerQQ: number;
    antiRecall: { enabled: boolean };
    poke: { enabled: boolean; replies: string[] };
    welcome: { enabled: boolean; template: string; delayMs: number };
    autoApprove: {
      friend: { enabled: boolean; pattern: string };
      group: { enabled: boolean; pattern: string; rules: Array<{ groupId: number; autoApprovePattern?: string; welcomeMessage?: string }> };
    };
    notifications: Record<string, boolean>;
  };
}

const DEFAULTS: AdminConfigData = {
  server: { port: 6199, host: '0.0.0.0', token: 'openclaw-qq-admin' },
  openclaw: { configPath: '/root/.openclaw/openclaw.json', autoSetup: true },
  napcat: { wsUrl: 'ws://127.0.0.1:3001', accessToken: '', webuiPort: 6099 },
  wechat: {
    apiUrl: process.env['WECHAT_API_URL'] || 'http://wechat:3001',
    token: process.env['WECHAT_TOKEN'] || 'openclaw-wechat',
    enabled: true,
    autoReply: true,
  },
  qq: {
    ownerQQ: 0,
    antiRecall: { enabled: true },
    poke: { enabled: true, replies: ['别戳了！', '再戳就坏了！', '讨厌~', '哼！', '🐾'] },
    welcome: { enabled: true, template: '欢迎 {nickname} 加入本群！', delayMs: 1500 },
    autoApprove: {
      friend: { enabled: false, pattern: '' },
      group: { enabled: false, pattern: '', rules: [] },
    },
    notifications: { memberChange: true, adminChange: true, banNotice: true, antiRecall: true, pokeReply: true, honorNotice: true },
  },
};

export class AdminConfig {
  private data: AdminConfigData;
  private filePath: string;

  constructor(filePath?: string) {
    this.filePath = filePath || process.env['ADMIN_CONFIG_PATH'] || path.resolve('data/admin-config.json');
    this.data = this.load();
  }

  private load(): AdminConfigData {
    try {
      if (fs.existsSync(this.filePath)) {
        const raw = JSON.parse(fs.readFileSync(this.filePath, 'utf-8'));
        return this.merge(DEFAULTS, raw);
      }
    } catch {}
    fs.mkdirSync(path.dirname(this.filePath), { recursive: true });
    fs.writeFileSync(this.filePath, JSON.stringify(DEFAULTS, null, 2));
    return { ...DEFAULTS };
  }

  private merge(defaults: any, source: any): any {
    const result = { ...defaults };
    for (const key of Object.keys(source)) {
      if (source[key] && typeof source[key] === 'object' && !Array.isArray(source[key]) && defaults[key] && typeof defaults[key] === 'object') {
        result[key] = this.merge(defaults[key], source[key]);
      } else {
        result[key] = source[key];
      }
    }
    return result;
  }

  get(): AdminConfigData { return this.data; }

  update(partial: any) {
    this.data = this.merge(this.data, partial);
    fs.writeFileSync(this.filePath, JSON.stringify(this.data, null, 2));
  }

  updateSection(section: string, value: any) {
    (this.data as any)[section] = value;
    fs.writeFileSync(this.filePath, JSON.stringify(this.data, null, 2));
  }
}
