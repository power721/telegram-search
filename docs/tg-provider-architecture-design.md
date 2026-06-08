# TG Provider Architecture Design Document
## For AList-TVBox Integration

Version: v1.0

# 1. Architecture

AList-TVBox (Spring Boot)
        |
        | HTTP localhost:9900
        v
+-------------------+
|   tg-provider     |
+-------------------+
| API Layer         |
| Service Layer     |
| Repository Layer  |
+-------------------+
        |
        v
+-------------------+
| SQLite + FTS5     |
+-------------------+
        |
        v
+-------------------+
| gotd Client Pool  |
+-------------------+
        |
        v
     Telegram

---

# 2. Lifecycle

Startup

Load Config
 -> Open SQLite
 -> Run Migration
 -> Load Accounts
 -> Restore Sessions
 -> Start Update Engine
 -> Start Scheduler
 -> Start API Server

Shutdown

Stop Scheduler
 -> Flush Queue
 -> Close Telegram Clients
 -> Close DB

---

# 3. AccountManager State Machine

NEW
 |
LOGIN_REQUIRED
 |
SYNCING
 |
ONLINE
 |
RECONNECTING
 |
ONLINE

Error:
ONLINE -> FLOOD_WAIT -> ONLINE

Error:
ONLINE -> DISCONNECTED -> RECONNECTING

---

# 4. Sync Flow

History Sync

Channel
  |
Get Last Message ID
  |
Fetch History Batch
  |
Store Messages
  |
Extract Links
  |
Update Cursor
  |
Next Batch

---

# 5. Update Flow

Telegram
  |
Updates Engine
  |
UpdateListener
  |
Message Processor
  |
SQLite
  |
FTS5

Events:

- New Message
- Edit Message
- Delete Message

---

# 6. Database DDL

telegram_accounts

CREATE TABLE telegram_accounts (
 id INTEGER PRIMARY KEY,
 phone TEXT,
 telegram_user_id INTEGER,
 first_name TEXT,
 last_name TEXT,
 username TEXT,
 status TEXT,
 created_at DATETIME,
 updated_at DATETIME
);

telegram_channels

CREATE TABLE telegram_channels (
 id INTEGER PRIMARY KEY,
 account_id INTEGER,
 telegram_channel_id INTEGER,
 access_hash INTEGER,
 title TEXT,
 username TEXT,
 type TEXT,
 last_message_id INTEGER,
 last_sync_time DATETIME
);

telegram_messages

CREATE TABLE telegram_messages (
 id INTEGER PRIMARY KEY,
 account_id INTEGER,
 channel_id INTEGER,
 telegram_message_id INTEGER,
 sender_id INTEGER,
 text TEXT,
 raw_json TEXT,
 date DATETIME,
 edit_date DATETIME,
 deleted INTEGER DEFAULT 0
);

telegram_links

CREATE TABLE telegram_links (
 id INTEGER PRIMARY KEY,
 message_id INTEGER,
 type TEXT,
 url TEXT,
 password TEXT,
 created_at DATETIME
);

---

# 7. FTS5

CREATE VIRTUAL TABLE telegram_messages_fts
USING fts5(text);

Trigger:

Insert -> update fts

Update -> update fts

Delete -> update fts

---

# 8. Repository Interfaces

AccountRepository

- Save
- Update
- Delete
- FindByID
- FindAll

ChannelRepository

- Save
- UpdateCursor
- FindAll

MessageRepository

- SaveBatch
- Search
- Latest

LinkRepository

- Save
- Search

---

# 9. Service Interfaces

AccountService

ChannelService

HistorySyncService

UpdateService

SearchService

LinkService

---

# 10. Scheduler

Jobs

AccountHealthCheck

ChannelSync

HistorySync

RetryQueue

Cleanup

Default:

Health Check: 1 min

Sync Check: 10 min

---

# 11. FloodWait Strategy

Catch FloodWait

Read Wait Seconds

Sleep

Retry

Exponential Backoff

1x
2x
4x
8x

Max:

30 min

---

# 12. SQLite Optimization

PRAGMA journal_mode=WAL;

PRAGMA synchronous=NORMAL;

PRAGMA temp_store=MEMORY;

PRAGMA cache_size=-200000;

Indexes

telegram_messages:

(channel_id,date)

(telegram_message_id)

telegram_links:

(type)

(message_id)

---

# 13. OpenAPI Core Models

SearchResult

{
  "account":"Main",
  "channel":"VIP",
  "date":"2026-01-01",
  "text":"message",
  "links":[]
}

StatusResponse

{
  "accounts":1,
  "channels":100,
  "messages":1000000,
  "links":50000
}

---

# 14. Supervisord

[program:tg-provider]

command=/opt/tg-provider/tg-provider

autostart=true

autorestart=true

stdout_logfile=/data/tg-provider/logs/stdout.log

stderr_logfile=/data/tg-provider/logs/stderr.log

---

# 15. Docker Integration

COPY tg-provider /opt/tg-provider/

Expose:

localhost only

9900

No public port mapping.

---

# 16. Spring Boot SDK Example

GET

http://127.0.0.1:9900/api/search?q=庆余年

Use RestTemplate or WebClient.

Do not access SQLite directly.

---

# 17. Future Roadmap

Phase 7

Media Metadata

Phase 8

Download Proxy

Phase 9

STRM Generator

Phase 10

Distributed Sync

Not part of MVP.
