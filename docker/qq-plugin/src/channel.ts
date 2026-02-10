import {
  type ChannelPlugin,
  type ChannelAccountSnapshot,
  buildChannelConfigSchema,
  DEFAULT_ACCOUNT_ID,
  normalizeAccountId,
  type ReplyPayload,
} from "openclaw/plugin-sdk";
import { OneBotClient } from "./client.js";
import { QQConfigSchema, type QQConfig } from "./config.js";
import { getQQRuntime } from "./runtime.js";
import type { OneBotMessage, OneBotMessageSegment } from "./types.js";
import { startFileServer, stopFileServer } from "./file-server.js";
import { createEventRouter } from "./events/index.js";
import { getMessageCache } from "./utils/message-cache.js";
import { parseApprovalCommand, getPendingRequests } from "./utils/pending-requests.js";
import * as fs from "fs";
import * as path from "path";

export type ResolvedQQAccount = ChannelAccountSnapshot & {
  config: QQConfig;
  client?: OneBotClient;
};

let lastActiveUser: { userId: number; isGroup: boolean; groupId?: number } | null = null;
const sessionToUserMap = new Map<string, { userId: number; isGroup: boolean; groupId?: number }>();

const FILE_SERVER_PORT = 18790;
// 使用宿主机的Docker网桥IP，让Docker容器中的NapCat可以访问
const FILE_SERVER_BASE_URL = "http://172.17.107.147:" + FILE_SERVER_PORT;

function normalizeTarget(raw: string): string {
  let target = raw.replace(/^(qq:)/i, "");
  
  if (target === "bot" && lastActiveUser) {
    if (lastActiveUser.isGroup && lastActiveUser.groupId) {
      return "group:" + lastActiveUser.groupId;
    }
    return String(lastActiveUser.userId);
  }
  
  if (!/^\d+$/.test(target) && !target.startsWith("group:")) {
    const mapping = sessionToUserMap.get("qq:" + target) || sessionToUserMap.get(target);
    if (mapping) {
      if (mapping.isGroup && mapping.groupId) {
        return "group:" + mapping.groupId;
      }
      return String(mapping.userId);
    }
    
    if (lastActiveUser) {
      if (lastActiveUser.isGroup && lastActiveUser.groupId) {
        return "group:" + lastActiveUser.groupId;
      }
      return String(lastActiveUser.userId);
    }
  }
  
  return target;
}

function looksLikeQQTargetId(raw: string, normalized: string): boolean {
  const trimmed = raw.trim();
  if (!trimmed) return false;
  if (/^qq:bot$/i.test(trimmed)) return lastActiveUser !== null;
  if (/^qq:\d+$/i.test(trimmed)) return true;
  if (/^group:\d+$/i.test(trimmed)) return true;
  if (/^\d{5,}$/.test(trimmed)) return true;
  if (sessionToUserMap.has(trimmed) || sessionToUserMap.has("qq:" + trimmed)) return true;
  if (lastActiveUser) return true;
  return false;
}

const clients = new Map<string, OneBotClient>();

export function getClientForAccount(accountId: string) {
  return clients.get(accountId);
}

// 文件服务器支持的目录白名单（需与 file-server.ts 中的 ALLOWED_DIRS 一致）
const SERVED_DIRS = [
  "/root/openclaw/work",
  "/root/.openclaw",
  "/tmp",
];

function convertLocalPathToUrl(filePath: string): string {
  if (filePath.startsWith("http://") || filePath.startsWith("https://")) {
    return filePath;
  }
  
  if (filePath.startsWith("base64://")) {
    return filePath;
  }
  
  // 检查是否是文件服务器可以提供的本地路径
  const isServedPath = SERVED_DIRS.some(dir => filePath.startsWith(dir + "/") || filePath === dir);
  
  if (isServedPath) {
    // 使用 /file?path= 端点，支持任意白名单目录下的文件
    const url = FILE_SERVER_BASE_URL + "/file?path=" + encodeURIComponent(filePath);
    console.log("[QQ] Converted local path to URL: " + filePath + " -> " + url);
    return url;
  }
  
  // 对于其他绝对路径，也尝试通过文件服务器提供（会被白名单检查拦截）
  if (filePath.startsWith("/") && fs.existsSync(filePath)) {
    const url = FILE_SERVER_BASE_URL + "/file?path=" + encodeURIComponent(filePath);
    console.log("[QQ] Attempting to serve local path via file server: " + filePath + " -> " + url);
    return url;
  }
  
  return filePath;
}

function detectMediaType(url: string): "image" | "audio" | "video" | "file" {
  const lowerUrl = url.toLowerCase();
  
  // 检查扩展名
  const ext = path.extname(url).toLowerCase();
  if ([".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp"].includes(ext)) {
    return "image";
  }
  if ([".mp3", ".wav", ".amr", ".silk", ".ogg"].includes(ext)) {
    return "audio";
  }
  if ([".mp4", ".avi", ".mov", ".mkv"].includes(ext)) {
    return "video";
  }
  
  // 检查URL路径中的关键词
  if (lowerUrl.includes("/image/") || lowerUrl.includes("/img/") || lowerUrl.includes("/photo/")) {
    return "image";
  }
  if (lowerUrl.includes("/audio/") || lowerUrl.includes("/sound/")) {
    return "audio";
  }
  if (lowerUrl.includes("/video/")) {
    return "video";
  }
  
  // 检查常见图片服务域名
  if (lowerUrl.includes("picsum.photos") || lowerUrl.includes("placeholder.com") || 
      lowerUrl.includes("lorempixel.com") || lowerUrl.includes("loremflickr.com") ||
      lowerUrl.includes("httpbin.org/image")) {
    return "image";
  }
  
  return "file";
}

function buildMessage(text?: string, files?: Array<{ url?: string; name?: string; mimeType?: string }>): OneBotMessage {
  const segments: OneBotMessageSegment[] = [];
  
  if (text) {
    segments.push({ type: "text", data: { text } });
  }
  
  if (files && files.length > 0) {
    for (const file of files) {
      if (!file.url) continue;
      
      const processedUrl = convertLocalPathToUrl(file.url);
      const mimeType = file.mimeType || "";
      
      if (mimeType.startsWith("image/")) {
        segments.push({ type: "image", data: { file: processedUrl } });
      } else if (mimeType.startsWith("audio/")) {
        segments.push({ type: "record", data: { file: processedUrl } });
      } else if (mimeType.startsWith("video/")) {
        segments.push({ type: "video", data: { file: processedUrl } });
      } else {
        segments.push({ type: "image", data: { file: processedUrl } });
      }
    }
  }
  
  return segments.length > 0 ? segments : [{ type: "text", data: { text: "" } }];
}

async function sendFileToTarget(
  client: OneBotClient, 
  to: string, 
  fileUrl: string, 
  fileName: string
): Promise<boolean> {
  const target = normalizeTarget(to);
  const processedUrl = convertLocalPathToUrl(fileUrl);
  
  console.log("[QQ] sendFileToTarget: to=" + to + ", file=" + processedUrl);
  
  if (target.startsWith("group:")) {
    const groupId = parseInt(target.replace("group:", ""), 10);
    if (isNaN(groupId)) return false;
    await client.uploadGroupFile(groupId, processedUrl, fileName);
    return true;
  } else if (/^\d+$/.test(target)) {
    const userId = parseInt(target, 10);
    await client.uploadPrivateFile(userId, processedUrl, fileName);
    return true;
  }
  
  return false;
}

async function sendToTarget(client: OneBotClient, to: string, message: OneBotMessage): Promise<boolean> {
  const target = normalizeTarget(to);
  
  if (target.startsWith("group:")) {
    const groupId = parseInt(target.replace("group:", ""), 10);
    if (isNaN(groupId)) return false;
    await client.sendGroupMsg(groupId, message);
    return true;
  } else if (/^\d+$/.test(target)) {
    const userId = parseInt(target, 10);
    await client.sendPrivateMsg(userId, message);
    return true;
  }
  
  return false;
}

export const qqChannel: ChannelPlugin<ResolvedQQAccount> = {
  id: "qq",
  meta: {
    id: "qq",
    label: "QQ (OneBot)",
    selectionLabel: "QQ",
    docsPath: "extensions/qq",
    blurb: "Connect to QQ via OneBot v11 (NapCat)",
  },
  capabilities: {
    chatTypes: ["direct", "group"],
    media: true,
  },
  configSchema: buildChannelConfigSchema(QQConfigSchema),
  config: {
    listAccountIds: (cfg) => {
      // @ts-ignore
      const qq = cfg.channels?.qq;
      if (!qq) return [];
      if (qq.accounts) return Object.keys(qq.accounts);
      return [DEFAULT_ACCOUNT_ID];
    },
    resolveAccount: (cfg, accountId) => {
      const id = accountId ?? DEFAULT_ACCOUNT_ID;
      // @ts-ignore
      const qq = cfg.channels?.qq;
      const accountConfig = id === DEFAULT_ACCOUNT_ID ? qq : qq?.accounts?.[id];
      
      return {
        accountId: id,
        name: accountConfig?.name ?? "QQ Default",
        enabled: true,
        configured: Boolean(accountConfig?.wsUrl),
        tokenSource: accountConfig?.accessToken ? "config" : "none",
        config: accountConfig || {},
      };
    },
    defaultAccountId: () => DEFAULT_ACCOUNT_ID,
    describeAccount: (acc) => ({
      accountId: acc.accountId,
      configured: acc.configured,
    }),
  },
  gateway: {
    startAccount: async (ctx) => {
      const { account, cfg } = ctx;
      const config = account.config;

      if (!config.wsUrl) {
        throw new Error("QQ: wsUrl is required");
      }

      startFileServer(FILE_SERVER_PORT);

      const client = new OneBotClient({
        wsUrl: config.wsUrl,
        accessToken: config.accessToken,
      });
      
      clients.set(account.accountId, client);

      client.on("connect", () => {
        console.log("[QQ] Connected account " + account.accountId);
        try {
          getQQRuntime().channel.activity.record({
            channel: "qq",
            accountId: account.accountId,
            direction: "inbound", 
          });
        } catch (err) {}
      });

      // 获取机器人自身QQ号
      let selfId = 0;
      client.on("connect", async () => {
        try {
          const loginInfo = await client.getLoginInfo();
          selfId = loginInfo.data?.user_id || 0;
          console.log("[QQ] Bot QQ ID: " + selfId);
        } catch (err) {
          console.error("[QQ] Failed to get login info:", err);
        }
      });

      // 通知主人的快捷方法
      const notifyOwner = async (text: string) => {
        const ownerQQ = config.ownerQQ;
        if (!ownerQQ) {
          console.log("[QQ] No ownerQQ configured, skipping notification: " + text.substring(0, 50));
          return;
        }
        try {
          await client.sendPrivateMsg(ownerQQ, [{ type: "text", data: { text } }]);
        } catch (err) {
          console.error("[QQ] Failed to notify owner:", err);
        }
      };

      // 创建事件路由器
      const eventRouter = createEventRouter({ client, config, selfId, notifyOwner });

      client.on("message", async (event) => {
        // 非 message 事件交给事件路由器处理
        if (event.post_type !== "message") {
          // 动态传入最新的 selfId
          await createEventRouter({ client, config, selfId, notifyOwner })(event);
          return;
        }

        // 缓存消息（用于防撤回）
        if (event.message_id && event.raw_message) {
          getMessageCache().set(event.message_id, {
            text: event.raw_message,
            userId: event.user_id || 0,
            groupId: event.group_id,
            time: event.time,
          });
        }

        const isGroup = event.message_type === "group";
        const userId = event.user_id;
        const groupId = event.group_id;
        let text = event.raw_message || "";
        
        // 如果消息为空但包含图片/文件等媒体，添加描述性文本避免OpenClaw发送"没收到文本"的提示
        if (!text && event.message && Array.isArray(event.message)) {
          const mediaTypes = event.message.map((seg: any) => seg.type).filter((t: string) => t !== "text");
          if (mediaTypes.length > 0) {
            const descriptions = mediaTypes.map((type: string) => {
              if (type === "image") return "[图片]";
              if (type === "file") return "[文件]";
              if (type === "video") return "[视频]";
              if (type === "record") return "[语音]";
              return `[${type}]`;
            });
            text = descriptions.join(" ");
          }
        }
        
        // 如果消息仍然为空（可能是系统消息或回执），忽略不处理
        if (!text || text.trim() === "") {
          console.log("[QQ] Ignoring empty message event");
          return;
        }

        // 检查是否是主人的审核回复指令
        if (!isGroup && userId && userId === config.ownerQQ && text) {
          const cmd = parseApprovalCommand(text);
          if (cmd) {
            const req = getPendingRequests().get(cmd.flag);
            if (req) {
              try {
                if (cmd.type === "group") {
                  await client.setGroupAddRequest(cmd.flag, req.subType || "add", cmd.action === "approve", cmd.reason || "");
                } else {
                  await client.setFriendAddRequest(cmd.flag, cmd.action === "approve", cmd.reason || "");
                }
                getPendingRequests().delete(cmd.flag);
                const actionText = cmd.action === "approve" ? "同意" : "拒绝";
                const typeText = cmd.type === "group" ? "入群" : "好友";
                await notifyOwner(`✅ 已${actionText}${typeText}请求`);
              } catch (err) {
                await notifyOwner(`❌ 处理请求失败: ${err}`);
              }
              return; // 审核指令不转发给 Agent
            }
          }
        }

        const fromId = isGroup ? "group:" + groupId : String(userId);
        const sessionKey = "qq:" + fromId;

        if (userId) {
          lastActiveUser = { userId, isGroup, groupId };
          sessionToUserMap.set(sessionKey, lastActiveUser);
          sessionToUserMap.set(String(userId), lastActiveUser);
          sessionToUserMap.set("bot", lastActiveUser);
          sessionToUserMap.set("qq:bot", lastActiveUser);
        }

        const runtime = getQQRuntime();

        const deliver = async (payload: ReplyPayload) => {
          try {
            const message = buildMessage(payload.text, payload.files);
            console.log("[QQ] Delivering: " + JSON.stringify(message).substring(0, 300));
            
            if (isGroup && groupId) {
              await client.sendGroupMsg(groupId, message);
            } else if (userId) {
              await client.sendPrivateMsg(userId, message);
            }
          } catch (err) {
            console.error("[QQ] Deliver error:", err);
          }
        };

        const { dispatcher, replyOptions } = runtime.channel.reply.createReplyDispatcherWithTyping({ deliver });

        const ctxPayload = runtime.channel.reply.finalizeInboundContext({
          Provider: "qq",
          Channel: "qq",
          From: fromId,
          To: fromId,
          Body: text,
          RawBody: text,
          SenderId: String(userId),
          SenderName: event.sender?.nickname || "Unknown",
          ConversationLabel: isGroup ? "QQ Group " + groupId : "QQ User " + userId,
          SessionKey: sessionKey,
          AccountId: account.accountId,
          ChatType: isGroup ? "group" : "direct",
          Timestamp: event.time * 1000,
          OriginatingChannel: "qq",
          OriginatingTo: fromId,
          CommandAuthorized: true 
        });
        
        await runtime.channel.session.recordInboundSession({
          storePath: runtime.channel.session.resolveStorePath(cfg.session?.store, { agentId: "default" }),
          sessionKey: ctxPayload.SessionKey!,
          ctx: ctxPayload,
          updateLastRoute: {
            sessionKey: ctxPayload.SessionKey!,
            channel: "qq",
            to: fromId,
            accountId: account.accountId,
          },
          onRecordError: (err) => console.error("QQ Session Error:", err)
        });

        await runtime.channel.reply.dispatchReplyFromConfig({
          ctx: ctxPayload,
          cfg,
          dispatcher,
          replyOptions,
        });
      });

      client.connect();
      
      return () => {
        client.disconnect();
        clients.delete(account.accountId);
        stopFileServer();
      };
    },
  },
  outbound: {
    sendText: async ({ to, text, accountId }) => {
      const client = getClientForAccount(accountId || DEFAULT_ACCOUNT_ID);
      if (!client) {
        return { channel: "qq", sent: false, error: "Client not connected" };
      }

      try {
        const message: OneBotMessage = [{ type: "text", data: { text } }];
        const success = await sendToTarget(client, to, message);
        return { channel: "qq", sent: success, error: success ? undefined : "Unknown target" };
      } catch (err) {
        console.error("[QQ] sendText error:", err);
        return { channel: "qq", sent: false, error: String(err) };
      }
    },
    sendMedia: async ({ to, text, mediaUrl, accountId }) => {
      const client = getClientForAccount(accountId || DEFAULT_ACCOUNT_ID);
      if (!client) {
        return { channel: "qq", sent: false, error: "Client not connected" };
      }

      try {
        console.log("[QQ] sendMedia: to=" + to + ", url=" + mediaUrl.substring(0, 100));
        
        // 拒绝data URL
        if (mediaUrl.startsWith("data:")) {
          console.error("[QQ] Rejected data URL. Use HTTP URL or save to /home/node/clawd/ instead.");
          return { 
            channel: "qq", 
            sent: false, 
            error: "Data URLs not supported. Please use HTTP URL or save file to /home/node/clawd/ directory and use file path." 
          };
        }
        
        const mediaType = detectMediaType(mediaUrl);
        console.log("[QQ] Detected media type: " + mediaType + " for " + mediaUrl);
        
        // 如果没有文本，使用空字符串而不是undefined，避免OpenClaw发送额外的提示消息
        const messageText = text || "";
        
        if (mediaType === "image" || mediaType === "audio" || mediaType === "video") {
          const processedUrl = convertLocalPathToUrl(mediaUrl);
          const message: OneBotMessage = [];
          if (text) {
            message.push({ type: "text", data: { text } });
          }
          
          if (mediaType === "image") {
            message.push({ type: "image", data: { file: processedUrl } });
          } else if (mediaType === "audio") {
            message.push({ type: "record", data: { file: processedUrl } });
          } else if (mediaType === "video") {
            message.push({ type: "video", data: { file: processedUrl } });
          }
          
          const success = await sendToTarget(client, to, message);
          return { channel: "qq", sent: success };
        }
        
        // 其他文件类型用文件上传接口
        const fileName = path.basename(mediaUrl);
        const success = await sendFileToTarget(client, to, mediaUrl, fileName);
        
        if (success && text) {
          await sendToTarget(client, to, [{ type: "text", data: { text } }]);
        }
        
        return { channel: "qq", sent: success };
      } catch (err) {
        console.error("[QQ] sendMedia error:", err);
        return { channel: "qq", sent: false, error: String(err) };
      }
    }
  },
  messaging: {
    normalizeTarget: normalizeTarget,
    targetResolver: {
      looksLikeId: looksLikeQQTargetId,
      hint: "<QQ号> 或 group:<群号>",
    },
  },
  setup: {
    resolveAccountId: ({ accountId }) => normalizeAccountId(accountId),
  }
};
