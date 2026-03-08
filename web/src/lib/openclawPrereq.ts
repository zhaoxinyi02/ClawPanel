import { api } from './api';

export interface OpenClawInstallPrereqStatus {
  ok: boolean;
  requiresManualInstall: boolean;
  message: string;
}

function nodeMajor(version: string): number {
  const raw = String(version || '').trim().replace(/^v/i, '');
  const major = Number(raw.split('.')[0]);
  return Number.isFinite(major) ? major : -1;
}

export async function getOpenClawInstallPrerequisiteStatus(): Promise<OpenClawInstallPrereqStatus> {
  try {
    const r = await api.getSoftwareList();
    if (!r?.ok) return { ok: true, requiresManualInstall: false, message: '' };

    const platform = String(r.platform || '').toLowerCase();
    if (platform !== 'windows' && platform !== 'darwin') {
      return { ok: true, requiresManualInstall: false, message: '' };
    }

    const software = Array.isArray(r.software) ? r.software : [];
    const node = software.find((s: any) => s.id === 'nodejs');
    const git = software.find((s: any) => s.id === 'git');

    const missing: string[] = [];
    const nodeVersion = String(node?.version || '').trim();
    if (!node?.installed || !nodeVersion) {
      missing.push('Node.js (>=20)');
    } else if (nodeMajor(nodeVersion) < 20) {
      missing.push(`Node.js >=20 (当前 ${nodeVersion})`);
    }

    if (!git?.installed || !String(git?.version || '').trim()) {
      missing.push('Git');
    }

    if (missing.length === 0) {
      return { ok: true, requiresManualInstall: false, message: '' };
    }

    const platformLabel = platform === 'windows' ? 'Windows' : 'macOS';
    const joined = missing.join(' 和 ');
    return {
      ok: false,
      requiresManualInstall: true,
      message: `检测到 ${platformLabel} 平台缺少 ${joined}，请先自行安装 Node.js 及 Git 后再执行一键安装 OpenClaw`,
    };
  } catch {
    return { ok: true, requiresManualInstall: false, message: '' };
  }
}

export async function ensureOpenClawInstallPrerequisites(): Promise<boolean> {
  const status = await getOpenClawInstallPrerequisiteStatus();
  if (status.ok) return true;
  if (status.message) window.alert(status.message);
  return false;
}
