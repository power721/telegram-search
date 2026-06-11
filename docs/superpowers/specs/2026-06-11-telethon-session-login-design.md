# Telethon Session Login Design

## Overview
Add third login method: accept Telethon StringSession string, convert to gotd session, authenticate.

## Motivation
Users with existing Telethon sessions can migrate without re-authenticating via phone/QR.

## API

### Endpoint
**POST `/api/telegram/login/telethon-session`**

Request:
```json
{
  "session_string": "1BVtsOLABu7W..."
}
```

Response (success):
```json
{
  "status": "online",
  "account": {...},
  "metadata_sync": {...}
}
```

Response (error):
```json
{
  "error": {
    "code": "bad_request",
    "message": "Invalid session string"
  }
}
```

## Implementation

### 1. Client Interface
Add method to `telegram.Client`:
```go
LoginWithTelethonSession(ctx context.Context, sessionString string, sessionPath string) (Profile, error)
```

### 2. GotdClient Implementation
```go
func (g *GotdClient) LoginWithTelethonSession(ctx, sessionString, sessionPath) (Profile, error) {
  1. Parse: data, err := session.TelethonSession(sessionString)
  2. Save: session.NewStorageMemory().StoreSession(ctx, data)
  3. Write to file using session.Loader
  4. Connect and fetch Self() to get profile
  5. Return Profile
}
```

### 3. API Handler
Add `telethonSessionLogin` handler:
```go
func (h handlers) telethonSessionLogin(c *gin.Context) {
  1. Parse request: session_string
  2. Validate: not empty, trimmed
  3. Create account record (phone: extract or placeholder)
  4. Call telegram.LoginWithTelethonSession()
  5. Update account with profile
  6. Respond with account + metadata_sync
}
```

### 4. Router
Add route in `internal/api/router.go`:
```go
telegramAPI.POST("/login/telethon-session", h.telethonSessionLogin)
```

## Conversion Details
Use gotd's built-in conversion: `session.TelethonSession(stringSession) -> session.Data`
- Handles base64 decoding
- Validates structure
- Returns error for malformed input

## Error Cases
- Empty/whitespace string → 400 "session_string is required"
- Invalid format → 400 "Invalid session string format"
- Expired/revoked session → 401 "Session expired or invalid"
- Network errors → 500

## Phone Number Handling
Telethon sessions may not contain phone. Fallback:
1. Try extracting from session metadata
2. Use placeholder: `"tg:{user_id}"` (same as QR login)
3. Phone can be updated later if needed

## Testing Strategy
- Unit test: session.TelethonSession() conversion
- Integration test: end-to-end login flow
- Error cases: invalid string, expired session

## UI Consideration
Frontend adds third option on login page:
- Radio/tab: "Phone", "QR Code", "Telethon Session"
- Input field for session string
- Calls new endpoint

## Rollout
1. Backend: Add endpoint + implementation
2. Test manually with curl
3. Frontend: Add UI option
4. Documentation: Update user guide
