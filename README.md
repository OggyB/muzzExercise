# Backend Candidate Exercise – Explore Service

This repository contains my solution for the **Backend Candidate Exercise**.  
The assignment was to design and implement a backend service for a simplified dating app scenario.

The core challenge is to handle **decisions** (likes or passes) between users and expose APIs that allow:
- Recording new decisions or updating existing ones.
- Querying who liked a given user.
- Counting total likes efficiently.
- Distinguishing between mutual and non-mutual likes.

The exercise explicitly emphasizes:
- **Scalability**: some users may have given or received hundreds of thousands of decisions.
- **Correctness**: overwrite behavior, pass handling, mutual likes, and pagination must be consistent.
- **Simplicity (KISS)**: avoid unnecessary over-engineering while still demonstrating good architecture and code quality.

This README explains the design choices, schema, API endpoints, and how to run and test the solution.


## Design Overview

This service is built around a simple idea: every "decision" is an action from one user (the actor) towards another (the recipient). A decision can be either a **like** or a **pass**, and each `(actor, recipient)` pair always has at most one row in the database (the latest decision overwrites the old one).

The gRPC service exposes four endpoints that map directly to common dating app flows:

- **PutDecision**: record or update a decision. If the same actor decides again, their choice is overwritten (no duplicates).
- **ListLikedYou**: show all users who have liked a given recipient. Users that the recipient has explicitly passed will never show up here.
- **ListNewLikedYou**: like above, but filters out *mutual* likes. This is essentially the "new likes" list, highlighting only people the recipient hasn’t liked back.
- **CountLikedYou**: return a count of how many people liked the recipient. This is cached in Redis for performance.

### Key principles we followed
- **Overwrite semantics**: if a user changes their mind, the new decision replaces the old one.
- **Excluding passes**: if you passed on someone, they won’t resurface in your “Liked You” feed, even if they like you later.
- **Cursor-based pagination**: to support large datasets, we use cursor tokens instead of offset/limit. This avoids expensive scans and makes pagination stable.
- **Cache-first counts**: counting likes is hot-path data, so it’s served from Redis when possible, with a fallback to MySQL.
- **Scalability**: we designed indexes to make queries efficient even for users with hundreds of thousands of decisions.

The overall approach balances correctness and clarity, while keeping the codebase simple and easy to reason about.

---

## Decision Schema

The core of the system is the **Decision** model, which records whether one user (the actor) has liked or passed another user (the recipient).  
Each `(actor_id, recipient_id)` pair is unique — if a user changes their mind, the decision is simply overwritten.

### Tables

#### `users`
A minimal user table to support decision-making.

```go
type User struct {
ID           uint64 `gorm:"primaryKey;autoIncrement"`
Username     string `gorm:"uniqueIndex;size:64;not null"`
Email        string `gorm:"uniqueIndex;size:128;not null"`
PasswordHash string `gorm:"size:255;not null"`
Active       bool   `gorm:"default:true"`
LastLoginAt  time.Time
Gender       string    `gorm:"size:16;not null"`
CreatedAt    time.Time `gorm:"autoCreateTime"`
UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}
```
#### `decisions`
Captures one user’s decision about another.

```go
type Decision struct {
ActorID     uint64    `gorm:"primaryKey;index:idx_actor_recipient_liked,priority:1"`
RecipientID uint64    `gorm:"primaryKey;index:idx_recipient_liked_updated_actor,priority:1;index:idx_actor_recipient_liked,priority:2"`
Liked       bool      `gorm:"not null;type:tinyint(1);index:idx_recipient_liked_updated_actor,priority:2;index:idx_actor_recipient_liked,priority:3"`
CreatedAt   time.Time `gorm:"autoCreateTime"`
UpdatedAt   time.Time `gorm:"autoUpdateTime;index:idx_recipient_liked_updated_actor,priority:3,sort:desc"`
}
```


### Indexing strategy
- Primary key `(actor_id, recipient_id)` Ensures there is only one row per pair, making overwrites trivial.
- `idx_recipient_liked_updated_actor (recipient_id, liked, updated_at DESC, actor_id)` Supports efficient “who liked me” queries with pagination.
- `idx_actor_recipient_liked (actor_id, recipient_id, liked)` Used for quick mutual-like checks.

### Why this works
This schema is small but covers the key access patterns:
- Inserting/updating decisions (`PutDecision `)
- Fetching who liked you (`ListLikedYou` / `ListNewLikedYou`)
- Counting likes (`CountLikedYou`)

With the right indexes, queries remain performant even if a user has hundreds of thousands of decisions.

## gRPC API
The Explore service exposes four RPCs. All endpoints are defined in `explore-service.proto`.

### Proto definition
```protobuf
service ExploreService {
  rpc ListLikedYou(ListLikedYouRequest) returns (ListLikedYouResponse);
  rpc ListNewLikedYou(ListLikedYouRequest) returns (ListLikedYouResponse);
  rpc CountLikedYou(CountLikedYouRequest) returns (CountLikedYouResponse);
  rpc PutDecision(PutDecisionRequest) returns (PutDecisionResponse);
}
```

### Endpoints

#### 1. `PutDecision`
Record or update a decision from one user (actor) to another (recipient).

- Overwrites existing decision if present.
- Returns whether this decision created a **mutual like**.

**Request**
```json
{
  "actor_user_id": "1",
  "recipient_user_id": "12",
  "liked_recipient": true
}
```

**Response**
```json
{
  "mutual_likes": true
}
```

#### 2. `CountLikedYou`
Return the number of users who liked the given recipient.
This endpoint is cache-first: served from Redis if available, falling back to MySQL otherwise.

**Request**
```json
{
  "recipient_user_id": "12"
}

```

**Response**
```json
{
  "count": 5
}
```

#### 3. `ListLikedYou`
List all users who liked the given recipient.
Supports cursor-based pagination.

**Request**
```json
{
  "recipient_user_id": "12",
  "pagination_token": ""
}
```

**Response**
```json
{
  "likers": [
    {
      "actor_id": "7",
      "unix_timestamp": "1758473078000"
    },
    {
      "actor_id": "9",
      "unix_timestamp": "1758473077000"
    }
  ],
  "next_pagination_token": "eyJhY3Rvcl9pZCI6OSwidXBkYXRlZF91bml4IjoxNzU4NDczMDc3fQ=="
}
```


#### 4. `ListNewLikedYou`
List users who liked the recipient but have not been liked back (non-mutual).
Also excludes users the recipient explicitly passed.

**Request**
```json
{
  "recipient_user_id": "12"
}

```

**Response**
```json
{
  "likers": [
    {
      "actor_id": "3",
      "unix_timestamp": "1758473077856"
    },
    {
      "actor_id": "2",
      "unix_timestamp": "1758473077850"
    }
  ],
  "next_pagination_token": "eyJhY3Rvcl9pZCI6OSwidXBkYXRlZF91bml4IjoxNzU4NDczMDc3fQ=="
}
```

### Example Usage with grpcurl

**PutDecision**
```bash
grpcurl -plaintext \
  -d '{"actor_user_id":"1","recipient_user_id":"12","liked_recipient":true}' \
  localhost:50051 explore.ExploreService/PutDecision
 ```

**CountLikedYou**
```bash
grpcurl -plaintext \
  -d '{"recipient_user_id":"12"}' \
  localhost:50051 explore.ExploreService/CountLikedYou
```

**ListLikedYou**
```bash
grpcurl -plaintext \
  -d '{"recipient_user_id":"12"}' \
  localhost:50051 explore.ExploreService/ListLikedYou
```

**ListNewLikedYou**
```bash
grpcurl -plaintext \
  -d '{"recipient_user_id":"12"}' \
  localhost:50051 explore.ExploreService/ListNewLikedYou
```


## Design Decisions & Assumptions

- **Overwrite behavior**  
  An existing decision (`like` or `pass`) can always be overwritten by the same actor.  
  Example: A first `likes` B, then `passes` → final state = `pass`.

- **Permanent pass rule**  
  If A passes B once, and later B likes A, B will **not** appear again in A’s "new liked you" list.  
  This prevents unwanted resurfacing of previously rejected users.

- **Indexing & Scaling**
  - Composite PK `(actor_id, recipient_id)` ensures O(1) overwrite without duplicates.
  - Index `(recipient_id, liked, updated_at DESC, actor_id)` optimizes "who liked me" queries.
  - Redis counters avoid full table scans for heavy users.

- **KISS principle**  
  Code and schema are kept simple while covering real-world scale (hundreds of thousands of decisions per user).


## Design Decisions & Assumptions

1. **Overwrite behavior**  
   A decision is always overwritten if the same `(actor_id, recipient_id)` pair already exists.  
   Example: If user A first likes B, then later passes B, the final state is `pass`.

2. **Permanent pass rule**  
   If a recipient explicitly passes on a user, that user will not reappear in their "Liked You" list, even if the other side likes them later.  
   This prevents unwanted resurfacing of previously rejected profiles.

3. **Mutual likes detection**  
   When `PutDecision` records a like, the service checks if the recipient has also liked the actor.  
   If so, `mutual_likes = true` is returned. Passes are ignored in mutual checks.

4. **Pagination consistency**  
   Cursor-based pagination is used instead of `OFFSET`, which avoids skipping or duplicating results when the dataset grows.  
   Tokens encode both `updated_at` (with millisecond precision) and `actor_id` to maintain a stable order.

5. **Cache-first counters**  
   `CountLikedYou` relies on Redis counters (`INCR`/`DECR`) to avoid expensive DB scans for heavy users.  
   TTL is refreshed whenever a key is accessed, so active users remain in cache while inactive ones expire naturally.

6. **Excluding passes everywhere**  
   Both `ListLikedYou` and `ListNewLikedYou` queries filter out users that the recipient has passed.  
   This keeps the UX aligned with real dating apps where "passes" are final.

7. **Scaling assumptions**
  - Some users may accumulate hundreds of thousands of decisions over years.
  - Composite indexes and cache-first counts ensure queries remain performant at scale.
  - The schema and queries are designed to handle millions of rows without major rewrites.

8. **KISS principle**  
   The solution avoids unnecessary complexity.  
   Features like snapshot-based pagination, sharding, or event-driven cache invalidation are left as future improvements, not required for the exercise.

## Future Improvements

- **Snapshot tokens**: Add an `as_of` timestamp into pagination tokens for stronger consistency if underlying data changes mid-pagination.
- **Event-driven cache updates**: Instead of relying solely on TTL, use a pub/sub system (e.g. NATS or Redis streams) to invalidate and refresh counters in real time.
- **Archival strategy**: For users with millions of decisions, older rows could be moved to cold storage or partitioned by time.
- **Authentication & authorization**: Ensure requests are tied to the authenticated user to prevent data leaks.
- **Metrics & observability**: Expose Prometheus metrics (query latency, cache hit/miss, mutual match rates).
- **Sharding & partitioning**: Horizontally partition the `decisions` table for extremely high-volume users.
- **API Gateway**: Provide REST/GraphQL endpoints on top of gRPC for easier client consumption.


## Setup & Running

### Prerequisites
- Go 1.25+
- Docker & Docker Compose
- Make (optional, for convenience)

### Clone the repo
```bash
git clone https://github.com/oggyb/muzz-exercise.git
cd muzz-exercise
```

### Environment Variables

The service is configured via a `.env` file.  
Below are the available variables and their purpose:

| Variable          | Description                                             | Example / Default   |
|-------------------|---------------------------------------------------------|---------------------|
| `APP_ENV`         | Application environment (`development`, `production`)   | `development`       |
| `LOG_LEVEL`       | Logging level (`debug`, `info`, `warn`, `error`)        | `debug`             |
| `LOG_FORMAT`      | Log format (`text` or `json`)                           | `text`              |
| `LOG_COMPONENT`   | Component name for structured logging                   | `grpc_server`       |
| `LOG_SOURCE`      | Show source file/line in logs (`1` = enabled)           | `1`                 |
| `DB_HOST`         | MySQL host                                              | `db`                |
| `DB_PORT`         | MySQL port                                              | `3306`              |
| `DB_USER`         | MySQL username                                          | `root`              |
| `DB_PASSWORD`     | MySQL password                                          | `root`              |
| `DB_NAME`         | MySQL database name                                     | `muzz`              |
| `REDIS_ADDR`      | Redis address                                           | `redis:6379`        |
| `REDIS_PASSWORD`  | Redis password (leave empty if none)                    | *(empty)*           |
| `REDIS_DB`        | Redis DB index (integer)                                | `0`                 |
| `GRPC_HOST`       | Host to bind the gRPC server                            | `0.0.0.0`           |
| `GRPC_PORT`       | Port for the gRPC server                                | `50051`             |

Example `.env` file:

```env
# App
APP_ENV=development

# Logger
LOG_LEVEL=debug
LOG_FORMAT=text
LOG_COMPONENT=grpc_server
LOG_SOURCE=1

# MySQL
DB_HOST=db
DB_PORT=3306
DB_USER=root
DB_PASSWORD=root
DB_NAME=muzz

# Redis
REDIS_ADDR=redis:6379
REDIS_PASSWORD=
REDIS_DB=0

# gRPC
GRPC_HOST=0.0.0.0
GRPC_PORT=50051
```

### Run with Docker Compose
```bash
docker-compose up --build
```
This will start:
- MySQL (with `decisions` and `users` tables)
- Redis
- The gRPC service (`ExploreService`) on `localhost:50051`

## Conclusion

Thanks for taking the time to review this project!  
The Explore Service was built with a focus on being **clear, correct, and scalable**, while still keeping things **simple enough to follow**.

- Decisions are easy to manage: no duplicates, always overwrite the latest choice.
- The queries are backed by proper indexes and cursor-based pagination, so even large datasets behave nicely.
- Redis helps keep counts fast for active users without overwhelming the database.
- The rules (like excluding passes or filtering out mutual likes) are handled consistently at the query layer.

This isn’t a full production-ready dating app service, but it captures the **core mechanics** you’d expect in the real world and shows how it could scale as usage grows.

If I had more time, I’d love to expand on things like authentication, metrics, or even a REST/GraphQL gateway. But for now, I hope this strikes the right balance between being practical, thoughtful, and straightforward.

---
