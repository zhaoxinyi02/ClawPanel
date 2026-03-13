#!/bin/bash
set -euo pipefail

GITHUB_REPO=${GITHUB_REPO:-zhaoxinyi02/ClawPanel}
GITEE_OWNER=${GITEE_OWNER:-zxy000006}
GITEE_REPO=${GITEE_REPO:-ClawPanel}
GITEE_TOKEN=${GITEE_TOKEN:-07f76b72d1515262bb10fdc93ba431c9}
SYNC_ROOT=${SYNC_ROOT:-/tmp/clawpanel-gitee-sync}

RED='\033[31m'
GREEN='\033[32m'
YELLOW='\033[33m'
BLUE='\033[34m'
MAGENTA='\033[35m'
CYAN='\033[36m'
BOLD='\033[1m'
NC='\033[0m'

log(){ echo -e "${GREEN}[Gitee Sync]${NC} $1"; }
info(){ echo -e "${CYAN}[Gitee Sync]${NC} $1"; }
warn(){ echo -e "${YELLOW}[Gitee Sync]${NC} $1"; }
err(){ echo -e "${RED}[Gitee Sync]${NC} $1" >&2; exit 1; }
step(){ echo -e "${MAGENTA}[$1/$2]${NC} ${BOLD}$3${NC}"; }

print_banner() {
  echo ""
  echo -e "${BLUE}===============================================================${NC}"
  echo -e "${BLUE}                                                               ${NC}"
  echo -e "${BLUE}   ██████╗ ██╗████████╗███████╗███████╗     ███████╗██╗   ██╗ ${NC}"
  echo -e "${BLUE}  ██╔════╝ ██║╚══██╔══╝██╔════╝██╔════╝     ██╔════╝╚██╗ ██╔╝ ${NC}"
  echo -e "${BLUE}  ██║  ███╗██║   ██║   █████╗  █████╗       ███████╗ ╚████╔╝  ${NC}"
  echo -e "${BLUE}  ██║   ██║██║   ██║   ██╔══╝  ██╔══╝       ╚════██║  ╚██╔╝   ${NC}"
  echo -e "${BLUE}  ╚██████╔╝██║   ██║   ███████╗███████╗     ███████║   ██║    ${NC}"
  echo -e "${BLUE}   ╚═════╝ ╚═╝   ╚═╝   ╚══════╝╚══════╝     ╚══════╝   ╚═╝    ${NC}"
  echo -e "${BLUE}                                                               ${NC}"
  echo -e "${BLUE}   GitHub -> Gitee Release 手动同步工具                        ${NC}"
  echo -e "${BLUE}   GitHub: ${GITHUB_REPO}                                      ${NC}"
  echo -e "${BLUE}   Gitee:  ${GITEE_OWNER}/${GITEE_REPO}                        ${NC}"
  echo -e "${BLUE}                                                               ${NC}"
  echo -e "${BLUE}===============================================================${NC}"
  echo ""
}

command -v python3 >/dev/null 2>&1 || err "缺少 python3"
command -v curl >/dev/null 2>&1 || err "缺少 curl"

print_banner

mkdir -p "$SYNC_ROOT"

python3 - <<'PY' "$GITHUB_REPO" "$GITEE_OWNER" "$GITEE_REPO" "$GITEE_TOKEN" "$SYNC_ROOT"
import json
import os
import pathlib
import subprocess
import sys
import urllib.parse
import urllib.request

github_repo, gitee_owner, gitee_repo, gitee_token, sync_root = sys.argv[1:6]
sync_root = pathlib.Path(sync_root)
sync_root.mkdir(parents=True, exist_ok=True)

def gh_api(url):
    with urllib.request.urlopen(url) as resp:
        return json.load(resp)

def gitee_api(path, method='GET', data=None):
    base = f'https://gitee.com/api/v5/repos/{gitee_owner}/{gitee_repo}{path}'
    sep = '&' if '?' in base else '?'
    url = f'{base}{sep}access_token={gitee_token}'
    if data is not None:
        data = urllib.parse.urlencode(data).encode()
    req = urllib.request.Request(url, data=data, method=method)
    with urllib.request.urlopen(req) as resp:
        body = resp.read().decode() or 'null'
        return json.loads(body)

def version_key(tag):
    tag = tag.split('-', 1)[1]
    tag = tag.lstrip('v')
    out = []
    for part in tag.replace('-', '.').split('.'):
        if part.isdigit():
            out.append((0, int(part)))
        else:
            out.append((1, part))
    return out

gh_releases = gh_api(f'https://api.github.com/repos/{github_repo}/releases?per_page=50')
gitee_releases = gitee_api('/releases?per_page=100')

gitee_tags = {r.get('tag_name'): r for r in gitee_releases}

def gitee_asset_names(release):
    names = set()
    for asset in release.get('assets') or []:
        name = (asset or {}).get('name')
        if name:
            names.add(name)
    return names

for prefix in ('pro-v', 'lite-v'):
    gh_candidates = [r for r in gh_releases if r.get('tag_name', '').startswith(prefix)]
    if not gh_candidates:
        print(f'[Gitee Sync] WARN no GitHub release found for prefix {prefix}')
        continue
    latest = sorted(gh_candidates, key=lambda r: version_key(r['tag_name']), reverse=True)[0]
    tag = latest['tag_name']
    title = latest['name'] or tag
    print(f'[Gitee Sync] latest GitHub {prefix}: {tag}')
    if tag in gitee_tags:
        release = gitee_tags[tag]
        print(f'[Gitee Sync] {tag} already exists on Gitee, checking assets ...')
    else:
        print(f'[Gitee Sync] creating Gitee release {tag} ...')
        release = gitee_api('/releases', method='POST', data={
            'tag_name': tag,
            'target_commitish': tag,
            'name': title,
            'body': latest.get('body') or f'Auto synced from GitHub release {tag}',
            'prerelease': 'false',
        })

    release_id = release['id']
    existing_assets = gitee_asset_names(release)
    release_dir = sync_root / tag
    release_dir.mkdir(parents=True, exist_ok=True)

    for asset in latest.get('assets', []):
        name = asset['name']
        if name in existing_assets:
            print(f'[Gitee Sync] asset already exists on Gitee, skip: {name}')
            continue
        url = asset['browser_download_url']
        target = release_dir / name
        if not target.exists():
            print(f'[Gitee Sync] downloading {name} from GitHub ...')
            subprocess.check_call([
                'curl', '--http1.1', '--progress-bar', '--retry', '3', '--retry-delay', '2',
                '--connect-timeout', '15', '--max-time', '7200',
                '-fL', url, '-o', str(target),
            ])
        else:
            print(f'[Gitee Sync] reuse local cached asset: {name}')
        size_mb = target.stat().st_size / 1024 / 1024
        print(f'[Gitee Sync] uploading {name} to Gitee ... ({size_mb:.1f} MB)')
        try:
            subprocess.check_call([
                'curl', '--http1.1', '--progress-bar', '-fL', '-X', 'POST', '--retry', '3', '--retry-delay', '2',
                '--connect-timeout', '15', '--max-time', '7200', '--speed-time', '30', '--speed-limit', '10240',
                '-H', 'Expect:',
                '-F', f'name={name}',
                '-F', f'file=@{target}',
                f'https://gitee.com/api/v5/repos/{gitee_owner}/{gitee_repo}/releases/{release_id}/attach_files?access_token={gitee_token}',
            ])
            existing_assets.add(name)
        except subprocess.CalledProcessError:
            print(f'[Gitee Sync] WARN upload failed for {name}, continue')

print('[Gitee Sync] done')
PY
