# ChatIM Client

This is the client application for ChatIM, built with Vue 3, TypeScript, Vite, and Element Plus.

## Project Setup

### Install Dependencies

```sh
npm install
```

### Compile and Hot-Reload for Development

```sh
npm run dev
```

### Type-Check, Compile and Minify for Production

```sh
npm run build
```

## Features

- **Authentication**: Login and Register.
- **Chat**: Real-time private and group chat using WebSocket.
- **Contacts**: Manage friends and friend requests.
- **Groups**: Create and join groups.

## Configuration

The API base URL is configured in `vite.config.ts` via proxy:

```typescript
proxy: {
  '/api': {
    target: 'http://localhost:8080',
    changeOrigin: true,
  },
  '/ws': {
    target: 'ws://localhost:8080',
    ws: true,
  }
}
```

Ensure your backend server is running on port 8080.
