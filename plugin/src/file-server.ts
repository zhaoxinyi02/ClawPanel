import * as http from "http";
import * as fs from "fs";
import * as path from "path";

let server: http.Server | null = null;

export function startFileServer(port: number = 18790): void {
  if (server) {
    console.log("[QQ FileServer] Already running");
    return;
  }

  server = http.createServer((req, res) => {
    try {
      const decodedUrl = decodeURIComponent(req.url || "/");
      const filePath = path.join("/root/openclaw/work", decodedUrl);
      
      console.log("[QQ FileServer] Request: " + decodedUrl);

      const realPath = fs.realpathSync(filePath);
      if (!realPath.startsWith("/root/openclaw/work")) {
        res.writeHead(403);
        res.end("Forbidden");
        return;
      }

      if (fs.existsSync(realPath) && fs.statSync(realPath).isFile()) {
        const stat = fs.statSync(realPath);
        const ext = path.extname(realPath).toLowerCase();
        
        const mimeTypes: Record<string, string> = {
          ".jpg": "image/jpeg",
          ".jpeg": "image/jpeg",
          ".png": "image/png",
          ".gif": "image/gif",
          ".webp": "image/webp",
          ".txt": "text/plain",
          ".pdf": "application/pdf",
          ".mp3": "audio/mpeg",
          ".mp4": "video/mp4",
        };
        
        const contentType = mimeTypes[ext] || "application/octet-stream";
        
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
    console.log("[QQ FileServer] Started on port " + port);
  });
}

export function stopFileServer(): void {
  if (server) {
    server.close();
    server = null;
    console.log("[QQ FileServer] Stopped");
  }
}
