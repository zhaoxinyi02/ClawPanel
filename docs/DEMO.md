# ClawPanel Demo Site Deployment Guide

ClawPanel Demo 是一个纯静态的演示站点，内含假数据，无需后端服务。适合展示 ClawPanel 管理面板的功能和界面。

## 构建 Demo 站点

```bash
cd web
npm install
npx vite build --outDir dist-demo --mode demo
# 生成的静态文件在 web/dist-demo/ 目录
```

## 部署方式

### 方式一：Nginx（推荐）

1. 将 `web/dist-demo/` 目录上传到服务器：

```bash
scp -r web/dist-demo/* user@your-server:/var/www/clawpanel-demo/
```

2. 配置 Nginx：

```nginx
server {
    listen 80;
    server_name demo.yourdomain.com;  # 替换为你的域名

    root /var/www/clawpanel-demo;
    index index.html;

    # SPA 路由支持 — 所有路径都返回 index.html
    location / {
        try_files $uri $uri/ /index.html;
    }

    # 静态资源缓存
    location /assets/ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # 可选：启用 gzip
    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml;
}
```

3. 重载 Nginx：

```bash
sudo nginx -t && sudo systemctl reload nginx
```

### 方式二：Docker + Nginx

1. 创建 Dockerfile：

```dockerfile
FROM nginx:alpine
COPY web/dist-demo/ /usr/share/nginx/html/
# SPA fallback
RUN echo 'server { listen 80; root /usr/share/nginx/html; index index.html; location / { try_files $uri $uri/ /index.html; } }' > /etc/nginx/conf.d/default.conf
EXPOSE 80
```

2. 构建并运行：

```bash
docker build -t clawpanel-demo .
docker run -d --name clawpanel-demo -p 8080:80 clawpanel-demo
```

访问 `http://your-server:8080` 即可。

### 方式三：Node.js serve

```bash
npm install -g serve
serve -s web/dist-demo -l 3000
```

### 方式四：Caddy

```
demo.yourdomain.com {
    root * /var/www/clawpanel-demo
    file_server
    try_files {path} /index.html
}
```

### 方式五：Netlify / Vercel / Cloudflare Pages

直接上传 `web/dist-demo/` 目录即可。`_redirects` 文件已包含 SPA 路由规则。

## Demo 登录

Demo 站点的登录密码可以输入**任意内容**，会自动通过验证。

## 语言切换

Demo 站点支持中英文切换，点击左侧边栏底部的 **English / 中文（简体）** 按钮即可切换。语言偏好会自动保存到浏览器 localStorage。

## 注意事项

- Demo 站点所有数据均为假数据，操作不会影响任何真实系统
- 所有 API 调用均在前端模拟，无需后端服务
- 日志流会每 8 秒自动生成一条模拟消息
- 适合用于产品演示、截图、文档编写等用途
