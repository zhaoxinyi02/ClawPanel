/** localStorage key for onboarding completion state */
export const ONBOARDING_SEEN_KEY = 'help.onboarding.seen';
export const ONBOARDING_VERSION_KEY = 'help.onboarding.version';
export const ONBOARDING_SESSION_KEY_PREFIX = 'help.onboarding.session-shown';
export const DEFAULT_ONBOARDING_VERSION = 'v1';
export const DEFAULT_ONBOARDING_FORCE_TOKEN = 'base';

export interface HelpOnboardingPolicy {
  version: string;
  enabled_at?: string | null;
  force_token?: string | null;
  force_updated_at?: string | null;
}

export interface HelpFaqItem {
  question: string;
  answer: string;
}

export interface HelpInstallCommand {
  platform: 'linux' | 'windows';
  label: string;
  command: string;
  language: string;
}

const README_PATHS = ['/README.md', '/src/README.md'];
const FAQ_SECTION_PATTERN = /^#{1,6}\s+.*(?:常见问题|FAQ)/i;

function isMarkdownTableDivider(line: string): boolean {
  return /^\|?(\s*:?-{3,}:?\s*\|)+\s*$/.test(line.trim());
}

export async function fetchHelpMarkdown(): Promise<string> {
  for (const path of README_PATHS) {
    const res = await fetch(path);
    if (res.ok) {
      return res.text();
    }
  }

  throw new Error('Failed to fetch help markdown');
}

export function extractFaqPreview(markdown: string, limit = 5): HelpFaqItem[] {
  const lines = markdown.split('\n');
  const faqSectionStart = lines.findIndex(line => FAQ_SECTION_PATTERN.test(line.trim()));

  if (faqSectionStart === -1) return [];

  let tableStart = -1;
  for (let i = faqSectionStart + 1; i < lines.length; i += 1) {
    const line = lines[i].trim();
    if (!line) continue;
    if (/^#{1,6}\s+/.test(line)) break;
    if (line.startsWith('|') && i + 1 < lines.length && isMarkdownTableDivider(lines[i + 1])) {
      tableStart = i;
      break;
    }
  }

  if (tableStart === -1) return [];

  const items: HelpFaqItem[] = [];
  for (let i = tableStart + 2; i < lines.length; i += 1) {
    const line = lines[i].trim();
    if (!line.startsWith('|') || line === '|') break;

    const cells = line
      .split('|')
      .map(cell => cell.trim())
      .filter(Boolean);

    if (cells.length >= 2) {
      items.push({ question: cells[0], answer: cells[1] });
    }

    if (items.length >= limit) break;
  }

  return items;
}

function extractCodeFence(markdown: string, startIndex: number): { language: string; code: string } | null {
  const fenceMatch = markdown.slice(startIndex).match(/^```([\w-]*)\n([\s\S]*?)\n```/m);
  if (!fenceMatch) return null;

  return {
    language: fenceMatch[1] || 'text',
    code: fenceMatch[2].trim(),
  };
}

export function extractQuickInstallCommands(markdown: string): HelpInstallCommand[] {
  const commands: HelpInstallCommand[] = [];
  const quickStartIndex = markdown.search(/###\s+方式一[:：]\s*一键安装/i);
  const source = quickStartIndex >= 0 ? markdown.slice(quickStartIndex) : markdown;

  const linuxMatch = source.match(/\*\*Linux\s*\/\s*macOS\*\*[\s\S]*?```([\w-]*)\n([\s\S]*?)\n```/i);
  if (linuxMatch) {
    commands.push({
      platform: 'linux',
      label: 'Linux / macOS',
      language: linuxMatch[1] || 'bash',
      command: linuxMatch[2].trim(),
    });
  }

  const windowsMatch = source.match(/\*\*Windows[^*]*\*\*[\s\S]*?```([\w-]*)\n([\s\S]*?)\n```/i);
  if (windowsMatch) {
    commands.push({
      platform: 'windows',
      label: 'Windows',
      language: windowsMatch[1] || 'powershell',
      command: windowsMatch[2].trim(),
    });
  }

  return commands;
}

export function getOnboardingVersion(): string {
  return localStorage.getItem(ONBOARDING_VERSION_KEY) || DEFAULT_ONBOARDING_VERSION;
}

export function getOnboardingCacheKey(version = getOnboardingVersion(), forceToken = DEFAULT_ONBOARDING_FORCE_TOKEN): string {
  return `${version}@@${forceToken || DEFAULT_ONBOARDING_FORCE_TOKEN}`;
}

export function getOnboardingSessionKey(version = getOnboardingVersion(), forceToken = DEFAULT_ONBOARDING_FORCE_TOKEN): string {
  return `${ONBOARDING_SESSION_KEY_PREFIX}.${getOnboardingCacheKey(version, forceToken)}`;
}

export function hasOnboardingSessionShown(version = getOnboardingVersion(), forceToken = DEFAULT_ONBOARDING_FORCE_TOKEN): boolean {
  return sessionStorage.getItem(getOnboardingSessionKey(version, forceToken)) === 'true';
}

export function markOnboardingSessionShown(version = getOnboardingVersion(), forceToken = DEFAULT_ONBOARDING_FORCE_TOKEN): void {
  sessionStorage.setItem(getOnboardingSessionKey(version, forceToken), 'true');
}

export function getOnboardingSeen(version = getOnboardingVersion(), forceToken = DEFAULT_ONBOARDING_FORCE_TOKEN): boolean {
  const raw = localStorage.getItem(ONBOARDING_SEEN_KEY);
  if (!raw) return false;

  if (raw === 'true') {
    return version === DEFAULT_ONBOARDING_VERSION && forceToken === DEFAULT_ONBOARDING_FORCE_TOKEN;
  }

  try {
    const seenMap = JSON.parse(raw) as Record<string, boolean>;
    const cacheKey = getOnboardingCacheKey(version, forceToken);
    return seenMap[cacheKey] === true || seenMap[version] === true;
  } catch {
    return false;
  }
}

export function setOnboardingSeen(version = getOnboardingVersion(), forceToken = DEFAULT_ONBOARDING_FORCE_TOKEN): void {
  const raw = localStorage.getItem(ONBOARDING_SEEN_KEY);
  let seenMap: Record<string, boolean> = {};

  if (raw && raw !== 'true') {
    try {
      seenMap = JSON.parse(raw) as Record<string, boolean>;
    } catch {
      seenMap = {};
    }
  } else if (raw === 'true') {
    seenMap[getOnboardingCacheKey(DEFAULT_ONBOARDING_VERSION, DEFAULT_ONBOARDING_FORCE_TOKEN)] = true;
  }

  seenMap[getOnboardingCacheKey(version, forceToken)] = true;
  localStorage.setItem(ONBOARDING_SEEN_KEY, JSON.stringify(seenMap));
  localStorage.setItem(ONBOARDING_VERSION_KEY, version);
}

export function normalizeOnboardingPolicy(input?: Partial<HelpOnboardingPolicy> | null): HelpOnboardingPolicy {
  return {
    version: input?.version || DEFAULT_ONBOARDING_VERSION,
    enabled_at: input?.enabled_at || null,
    force_token: input?.force_token || DEFAULT_ONBOARDING_FORCE_TOKEN,
    force_updated_at: input?.force_updated_at || null,
  };
}

export async function copyTextWithFallback(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {}

  try {
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.setAttribute('readonly', 'true');
    textarea.style.position = 'fixed';
    textarea.style.top = '-9999px';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    const success = document.execCommand('copy');
    document.body.removeChild(textarea);
    return success;
  } catch {
    return false;
  }
}
