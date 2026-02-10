import * as http from "http";
import * as fs from "fs";
import * as path from "path";

let server: http.Server | null = null;

// 允许文件服务器访问的目录白名单
const ALLOWED_DIRS = [
  "/root/openclaw/work",
  "/root/.openclaw",
  "/tmp",
];

function isPathAllowed(realPath: string): boolean {
  return ALLOWED_DIRS.some(dir => realPath.startsWith(dir));
}

const MIME_TYPES: Record<string, string> = {
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".png": "image/png",
  ".gif": "image/gif",
  ".webp": "image/webp",
  ".bmp": "image/bmp",
  ".svg": "image/svg+xml",
  ".ico": "image/x-icon",
  ".txt": "text/plain",
  ".html": "text/html",
  ".css": "text/css",
  ".js": "application/javascript",
  ".json": "application/json",
  ".pdf": "application/pdf",
  ".zip": "application/zip",
  ".doc": "application/msword",
  ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  ".xls": "application/vnd.ms-excel",
  ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  ".ppt": "application/vnd.ms-powerpoint",
  ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
  ".mp3": "audio/mpeg",
  ".wav": "audio/wav",
  ".ogg": "audio/ogg",
  ".mp4": "video/mp4",
  ".avi": "video/x-msvideo",
  ".mov": "video/quicktime",
  ".mkv": "video/x-matroska",
  ".amr": "audio/amr",
  ".silk": "audio/silk",
};

export function startFileServer(port: number = 18790): void {
  if (server) {
    console.log("[QQ FileServer] Already running");
    return;
  }

  server = http.createServer((req, res) => {
    try {
      // URL 格式: /file?path=/root/openclaw/work/xxx.png
      // 或旧格式: /相对路径（兼容，默认基于 /root/openclaw/work）
      const url = new URL(req.url || "/", "http://localhost");
      
      let filePath: string;
      if (url.pathname === "/file" && url.searchParams.has("path")) {
        // 新格式：绝对路径
        filePath = url.searchParams.get("path")!;
      } else {
        // 旧格式：相对于 /root/openclaw/work
        const decodedUrl = decodeURIComponent(url.pathname);
        filePath = path.join("/root/openclaw/work", decodedUrl);
      }
      
      console.log("[QQ FileServer] Request: " + filePath);

      if (!fs.existsSync(filePath)) {
        console.log("[QQ FileServer] File not found: " + filePath);
        res.writeHead(404);
        res.end("Not Found");
        return;
      }

      const realPath = fs.realpathSync(filePath);
      if (!isPathAllowed(realPath)) {
        console.log("[QQ FileServer] Forbidden path: " + realPath);
        res.writeHead(403);
        res.end("Forbidden");
        return;
      }

      if (fs.statSync(realPath).isFile()) {
        const stat = fs.statSync(realPath);
        const ext = path.extname(realPath).toLowerCase();
        const contentType = MIME_TYPES[ext] || "application/octet-stream";
        
        res.writeHead(200, {
          "Content-Type": contentType,
          "Content-Length": stat.size,
          "Access-Control-Allow-Origin": "*",
        });
        
        fs.createReadStream(realPath).pipe(res);
      } else {
        res.writeHead(404);
        res.end("Not Found");
      }
    } catch (err) {
      console.error("[QQ FileServer] Error:", err);
      res.writeHead(500);
      res.end("Error");
    }
  });

  server.listen(port, "0.0.0.0", () => {
    console.log("[QQ FileServer] Started on port " + port + ", serving: " + ALLOWED_DIRS.join(", "));
  });
}

export function stopFileServer(): void {
  if (server) {
    server.close();
    server = null;
    console.log("[QQ FileServer] Stopped");
  }
}
