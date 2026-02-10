# Ultra Fast Telegram Approval Bot 2026

A high-performance Telegram join request auto-approval bot built with Go and MongoDB. Optimized for extreme speed, concurrency, and minimal latency.

## Features

- **Instant Approval**: Join requests approved in <100ms.
- **Worker Pool**: Handles high burst traffic concurrently.
- **Webhook Only**: Uses Telegram Bot API webhooks for real-time updates.
- **Broadcast System**: Admin can broadcast messages to all users.
- **Production Ready**: Graceful shutdown and resilient to failures.

## Environment Variables

- `BOT_TOKEN`: Your Telegram Bot Token.
- `MONGODB_URI`: MongoDB connection string.
- `ADMIN_ID`: Your Telegram User ID for broadcast access.
- `PORT`: (Optional) Server port, defaults to 8080.

## Deployment on Render

1. Create a new Web Service on Render.
2. Connect your repository.
3. Select **Go** runtime.
4. Set the build command: `go build -o telegram-approval-bot`
5. Set the start command: `./telegram-approval-bot`
6. Add the environment variables listed above.
7. After deployment, set your webhook URL:
   `https://api.telegram.org/bot<TOKEN>/setWebhook?url=<RENDER_URL>/webhook/<TOKEN>`

## 2026 Optimization Notes

- Uses native `net/http` to avoid framework overhead.
- HTTP client connection pooling.
- Async database writes.
- Non-blocking webhook handler.
