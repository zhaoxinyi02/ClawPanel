# Twitter/X Thread — ClawPanel v4.3.0

## Tweet 1 (Main)

🐾 ClawPanel v4.3.0 is here!

The open-source AI assistant management panel just got a major upgrade:

⚡ Skills & Plugins separated into tabs
📺 Real-time update logs with live terminal output
🔐 Change admin password from UI
🌍 English + Chinese i18n
💻 Native installers for Linux/macOS/Windows

🔗 GitHub: github.com/zhaoxinyi02/ClawPanel
🎮 Live demo: demo.zhaoxinyi.xyz

🧵 Thread with details ↓

📸 **Image 1**: Dashboard screenshot (img/dashboard.png)

---

## Tweet 2

⚡ Skills Center overhaul:

Before: skills & plugins mixed together, only 13 items showing
After: 3 separate tabs — Skills (52+ built-in) · Plugins · ClawHub Store

Fixed a Docker path scanning bug that was hiding 50+ built-in AI skills!

📸 **Image 2**: Skills page screenshot (img/skills.png)

---

## Tweet 3

📺 Finally — real-time update progress!

Clicking "Update Now" used to show... nothing. Just "updating..."

Now you get:
• Live terminal-style log panel (dark theme, colored lines)
• Auto-scroll as new output arrives
• Elapsed time counter
• ✅ Success / ❌ Failed status at a glance
• "Force Update" button even when no update detected

📸 **Image 3**: Version management page with update log panel visible (take a new screenshot: img/config-version-update.png)

---

## Tweet 4

🔧 More in v4.3.0:

• 🔐 Change admin password (Settings → General)
• 🌍 i18n: English + 简体中文, one-click switch
• 💻 Native install scripts (no Docker needed!)
  - Linux: curl one-liner
  - macOS: brew-style install
  - Windows: PowerShell one-liner
• 🛡️ update-watcher.sh — host-side daemon for container updates

📸 **Image 4**: System config page showing change password section (take a new screenshot: img/config-general.png)

---

## Tweet 5

Built with:
⚛️ React + Vite + TailwindCSS
🟦 TypeScript + Express backend
🐳 Docker Compose orchestration
🤖 Supports 20+ channels: QQ, WeChat, Telegram, Discord, Slack, Feishu, DingTalk...

If you're running OpenClaw and want a beautiful management UI — give it a try!

⭐ github.com/zhaoxinyi02/ClawPanel

📸 **Image 5**: Channel management page (img/channels.png)

---

# 📸 Images to prepare:

1. **img/dashboard.png** — Main dashboard (already exists)
2. **img/skills.png** — Skills center with tabs (already exists, but should retake to show the new 3-tab layout with 65 skills)
3. **img/config-version-update.png** — NEW: Version management page showing the real-time update log panel (trigger an update or mock one to capture the terminal log UI)
4. **img/config-general.png** — NEW: General config tab showing the change password section
5. **img/channels.png** — Channel management (already exists)

## Optional bonus images:
6. **img/i18n-switch.png** — Sidebar showing language switch button
7. **img/skills-plugins-tab.png** — Close-up of the plugins tab
