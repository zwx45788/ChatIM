const http = require('http');
const https = require('https');
const fs = require('fs');
const path = require('path');
const { URL } = require('url');

const PORT = 8082;
const API_BACKEND = 'http://localhost:8081';

const MIME_TYPES = {
  '.html': 'text/html',
  '.js': 'text/javascript',
  '.mjs': 'text/javascript',
  '.css': 'text/css',
  '.json': 'application/json',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.gif': 'image/gif',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
};

function forwardApiRequest(req, res) {
  const backendUrl = `${API_BACKEND}${req.url}`;
  console.log(`Forwarding ${req.method} ${req.url} to ${backendUrl}`);
  
  const url = new URL(backendUrl);
  const options = {
    hostname: url.hostname,
    port: url.port,
    path: url.pathname + url.search,
    method: req.method,
    headers: req.headers
  };
  
  // 移除 host 头以避免冲突
  delete options.headers.host;
  
  const apiReq = http.request(options, (apiRes) => {
    res.writeHead(apiRes.statusCode, apiRes.headers);
    apiRes.pipe(res);
  });
  
  apiReq.on('error', (error) => {
    console.error('API request error:', error);
    res.writeHead(502, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ code: -1, message: 'Backend service unavailable' }));
  });
  
  req.pipe(apiReq);
}

http.createServer((req, res) => {
  console.log(`${req.method} ${req.url}`);
  
  // 如果是 API 请求，转发到后端
  if (req.url.startsWith('/api/')) {
    forwardApiRequest(req, res);
    return;
  }
  
  // Handle query parameters by removing them for file lookup
  const urlPath = req.url.split('?')[0];
  
  let filePath = '.' + urlPath;
  if (filePath === './') {
    filePath = './index.html';
  }

  const extname = path.extname(filePath);
  let contentType = MIME_TYPES[extname] || 'application/octet-stream';

  fs.readFile(filePath, (error, content) => {
    if (error) {
      if(error.code == 'ENOENT') {
        // SPA fallback: for routes without file extension, serve index.html
        const isAsset = path.extname(urlPath) !== '';
        if (!isAsset) {
          fs.readFile('./index.html', (err2, content2) => {
            if (err2) {
              res.writeHead(500);
              res.end('Error loading index.html');
            } else {
              res.writeHead(200, { 'Content-Type': 'text/html' });
              res.end(content2, 'utf-8');
            }
          });
        } else {
          res.writeHead(404);
          res.end('404 Not Found');
        }
      } else {
        res.writeHead(500);
        res.end('Sorry, check with the site admin for error: '+error.code+' ..\n');
      }
    } else {
      res.writeHead(200, { 'Content-Type': contentType });
      res.end(content, 'utf-8');
    }
  });
}).listen(PORT);

console.log(`Server running at http://127.0.0.1:${PORT}/`);
console.log(`API requests will be sent to http://localhost:8081`);
