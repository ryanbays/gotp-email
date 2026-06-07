# gotp-email
Email receiving server for handling otps

# gotp-email API Reference

## Authentication

All endpoints except `/health` require an API key passed in the request header.

```
X-API-Key: <your-api-key>
```

Requests with a missing or incorrect key return `401 Unauthorized`. If the server has no API key configured, all protected requests return `500 Internal Server Error`.

---

## Endpoints

### Health Check

```
GET /health
```

Returns the server status. No authentication required.

**Response `200`**
```json
{ "status": "ok" }
```

---

### Submit Inbound Email

```
POST /inbound-email
```

Accepts an inbound email, caches it, and attempts to extract an OTP from the body.

**Request body**
```json
{
  "from": "noreply@example.com",
  "to": "alice@yourdomain.com",
  "raw": "<html>Your OTP is <b>123456</b></html>"
}
```

| Field | Type   | Description                        |
|-------|--------|------------------------------------|
| from  | string | Sender email address               |
| to    | string | Recipient address — the local part becomes the inbox ID |
| raw   | string | Raw HTML body of the email         |

**Response `200` — OTP found**
```json
{
  "status": "ok",
  "inbox": "alice",
  "otp": "123456"
}
```

**Response `200` — No OTP found**
```json
{
  "status": "ok",
  "message": "no otp found"
}
```

**Response `400`**
```json
{ "error": "bad request" }
```

> The inbox ID is derived from the local part of the `to` address (everything before `@`), lowercased and trimmed. For example, `alice@yourdomain.com` → `alice`.

---

### Get Latest OTP

```
GET /otp/:inbox
```

Returns the most recently extracted OTP for the given inbox. OTPs expire after **1 hour**.

| Parameter | Description          |
|-----------|----------------------|
| inbox     | The inbox ID to look up |

**Response `200` — OTP found**
```json
{
  "otp": "123456",
  "from": "noreply@example.com",
  "timestamp": 1749340800
}
```

| Field     | Type   | Description                            |
|-----------|--------|----------------------------------------|
| otp       | string | The extracted OTP code                 |
| from      | string | Sender address the OTP arrived from    |
| timestamp | int64  | Unix timestamp of when it was received |

**Response `200` — No OTP**
```json
{ "otp": null }
```

---

### Get OTP History

```
GET /otp/:inbox/history
```

Returns up to the last 11 OTPs received for the given inbox, most recent first.

| Parameter | Description          |
|-----------|----------------------|
| inbox     | The inbox ID to look up |

**Response `200`**
```json
{
  "history": [
    {
      "otp": "123456",
      "from": "noreply@example.com",
      "timestamp": 1749340800
    },
    {
      "otp": "654321",
      "from": "noreply@example.com",
      "timestamp": 1749337200
    }
  ]
}
```

`history` is `null` if no OTPs have been received for the inbox.

---

### Get Cached Email

```
GET /email-cache/:inbox
```

Returns the raw cached email body for the given inbox as stored on disk.

| Parameter | Description          |
|-----------|----------------------|
| inbox     | The inbox ID to look up |

**Response `200`**

```json
{
  "timestamp": 1749340800,
  "email": {
    "from": "noreply@example.com",
    "to": "alice@yourdomain.com",
    "raw": "<html>...</html>"
  }
}
```

**Response `200` — No cache**
```json
{ "email": null }
```

---

## Running the Server

```sh
./gotp-email [flags]
```

| Flag      | Default                          | Description               |
|-----------|----------------------------------|---------------------------|
| -config   | /etc/gotp-email/gotp.conf        | Path to the config file   |
| -rules    | /etc/gotp-email/rules.json       | Path to the rules file    |

Log level is controlled via the `LOG_LEVEL` environment variable. Accepted values: `trace`, `debug`, `info`, `warn`, `error`, `fatal`. Defaults to `debug`.

```sh
LOG_LEVEL=info ./gotp-email -config ./gotp.conf -rules ./rules.json
```
