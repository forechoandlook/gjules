# Jules REST API Reference

> Base URL: `https://jules.googleapis.com/v1alpha`
> 
> Authentication: Pass API key in `x-goog-api-key` header
> 
> Docs: https://jules.google/docs/api/reference/

---

## Table of Contents

- [Authentication](#authentication)
- [Sessions](#sessions)
  - [Create a Session](#create-a-session)
  - [List Sessions](#list-sessions)
  - [Get a Session](#get-a-session)
  - [Delete a Session](#delete-a-session)
  - [Send a Message](#send-a-message)
  - [Approve a Plan](#approve-a-plan)
- [Activities](#activities)
  - [List Activities](#list-activities)
  - [Get an Activity](#get-an-activity)
- [Sources](#sources)
  - [List Sources](#list-sources)
  - [Get a Source](#get-a-source)
- [Key Types](#key-types)
  - [SessionState](#sessionstate)
  - [AutomationMode](#automationmode)
  - [Activity Event Types](#activity-event-types)

---

## Authentication

All requests require an API key passed via the `x-goog-api-key` header.

```bash
curl -H "x-goog-api-key: $JULES_API_KEY" \
  https://jules.googleapis.com/v1alpha/sessions
```

Generate your API key at [jules.google.com/settings](https://jules.google.com/settings).

---

## Sessions

### Create a Session

```
POST /v1alpha/sessions
```

Creates a new coding session.

**Request Body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `prompt` | string | Yes | Task description for Jules to execute |
| `title` | string | No | Optional title (auto-generated if omitted) |
| `sourceContext` | SourceContext | No | Source repository and branch context (optional for repoless) |
| `requirePlanApproval` | boolean | No | If true, plans require explicit approval (default: false) |
| `automationMode` | string | No | Use `AUTO_CREATE_PR` to auto-create pull requests |

**Example:**

```bash
curl -X POST \
  -H "x-goog-api-key: $JULES_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Add comprehensive unit tests for the authentication module",
    "title": "Add auth tests",
    "sourceContext": {
      "source": "sources/github-myorg-myrepo",
      "githubRepoContext": {
        "startingBranch": "main"
      }
    },
    "requirePlanApproval": true
  }' \
  https://jules.googleapis.com/v1alpha/sessions
```

**Response:**

```json
{
  "name": "sessions/1234567",
  "id": "abc123",
  "prompt": "Add comprehensive unit tests for the authentication module",
  "title": "Add auth tests",
  "state": "QUEUED",
  "url": "https://jules.google.com/session/abc123",
  "createTime": "2024-01-15T10:30:00Z",
  "updateTime": "2024-01-15T10:30:00Z"
}
```

---

### List Sessions

```
GET /v1alpha/sessions
```

Lists all sessions for the authenticated user.

**Query Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `pageSize` | integer | Number of sessions to return (1-100). Defaults to 30 |
| `pageToken` | string | Page token from previous response |

**Example:**

```bash
curl -H "x-goog-api-key: $JULES_API_KEY" \
  "https://jules.googleapis.com/v1alpha/sessions?pageSize=10"
```

**Response:**

```json
{
  "sessions": [
    {
      "name": "sessions/1234567",
      "id": "abc123",
      "title": "Add auth tests",
      "state": "COMPLETED",
      "createTime": "2024-01-15T10:30:00Z",
      "updateTime": "2024-01-15T11:45:00Z"
    }
  ],
  "nextPageToken": "eyJvZmZzZXQiOjEwfQ=="
}
```

---

### Get a Session

```
GET /v1alpha/sessions/{sessionId}
```

Retrieves a single session by ID.

**Path Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Resource name. Format: `sessions/{session}` |

**Example:**

```bash
curl -H "x-goog-api-key: $JULES_API_KEY" \
  https://jules.googleapis.com/v1alpha/sessions/1234567
```

**Response:**

```json
{
  "name": "sessions/1234567",
  "id": "abc123",
  "prompt": "Add comprehensive unit tests for the authentication module",
  "title": "Add auth tests",
  "state": "COMPLETED",
  "url": "https://jules.google.com/session/abc123",
  "createTime": "2024-01-15T10:30:00Z",
  "updateTime": "2024-01-15T11:45:00Z",
  "outputs": [
    {
      "pullRequest": {
        "url": "https://github.com/myorg/myrepo/pull/42",
        "title": "Add auth tests",
        "description": "Added unit tests for authentication module"
      }
    }
  ]
}
```

---

### Delete a Session

```
DELETE /v1alpha/sessions/{sessionId}
```

Deletes a session.

**Path Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Resource name. Format: `sessions/{session}` |

**Example:**

```bash
curl -X DELETE \
  -H "x-goog-api-key: $JULES_API_KEY" \
  https://jules.googleapis.com/v1alpha/sessions/1234567
```

**Response:** Empty response on success.

---

### Send a Message

```
POST /v1alpha/sessions/{sessionId}:sendMessage
```

Sends a message from the user to an active session (feedback, answers, instructions).

**Path Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `session` | string | Yes | Resource name. Format: `sessions/{session}` |

**Request Body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `prompt` | string | Yes | The message to send |

**Example:**

```bash
curl -X POST \
  -H "x-goog-api-key: $JULES_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Please also add integration tests for the login flow"
  }' \
  https://jules.googleapis.com/v1alpha/sessions/1234567:sendMessage
```

**Response:** Empty `SendMessageResponse` on success.

---

### Approve a Plan

```
POST /v1alpha/sessions/{sessionId}:approvePlan
```

Approves a pending plan (only needed when `requirePlanApproval` was set to `true`).

**Path Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `session` | string | Yes | Resource name. Format: `sessions/{session}` |

**Example:**

```bash
curl -X POST \
  -H "x-goog-api-key: $JULES_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{}' \
  https://jules.googleapis.com/v1alpha/sessions/1234567:approvePlan
```

**Response:** Empty `ApprovePlanResponse` on success.

---

## Activities

### List Activities

```
GET /v1alpha/sessions/{sessionId}/activities
```

Lists all activities for a session. Use to monitor progress, retrieve messages, and access artifacts.

**Path Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `parent` | string | Yes | Parent session. Format: `sessions/{session}` |

**Query Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `pageSize` | integer | Number of activities to return (1-100). Defaults to 50 |
| `pageToken` | string | Page token from previous response |

**Example:**

```bash
curl -H "x-goog-api-key: $JULES_API_KEY" \
  "https://jules.googleapis.com/v1alpha/sessions/1234567/activities?pageSize=20"
```

**Response:**

```json
{
  "activities": [
    {
      "name": "sessions/1234567/activities/act1",
      "id": "act1",
      "originator": "system",
      "description": "Session started",
      "createTime": "2024-01-15T10:30:00Z"
    },
    {
      "name": "sessions/1234567/activities/act2",
      "id": "act2",
      "originator": "agent",
      "description": "Plan generated",
      "planGenerated": {
        "plan": {
          "id": "plan1",
          "steps": [
            { "id": "step1", "index": 0, "title": "Analyze existing code", "description": "Review the authentication module structure" },
            { "id": "step2", "index": 1, "title": "Write unit tests", "description": "Create comprehensive test coverage" }
          ],
          "createTime": "2024-01-15T10:31:00Z"
        }
      },
      "createTime": "2024-01-15T10:31:00Z"
    }
  ],
  "nextPageToken": "eyJvZmZzZXQiOjIwfQ=="
}
```

---

### Get an Activity

```
GET /v1alpha/sessions/{sessionId}/activities/{activityId}
```

Retrieves a single activity by ID.

**Path Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Resource name. Format: `sessions/{session}/activities/{activity}` |

**Example:**

```bash
curl -H "x-goog-api-key: $JULES_API_KEY" \
  https://jules.googleapis.com/v1alpha/sessions/1234567/activities/act2
```

**Response:**

```json
{
  "name": "sessions/1234567/activities/act2",
  "id": "act2",
  "originator": "agent",
  "description": "Code changes ready",
  "createTime": "2024-01-15T11:00:00Z",
  "artifacts": [
    {
      "changeSet": {
        "source": "sources/github-myorg-myrepo",
        "gitPatch": {
          "baseCommitId": "a1b2c3d4",
          "unidiffPatch": "diff --git a/tests/auth.test.js...",
          "suggestedCommitMessage": "Add unit tests for authentication module"
        }
      }
    }
  ]
}
```

---

## Sources

### List Sources

```
GET /v1alpha/sources
```

Lists all connected sources (repositories). Sources are created via the web interface, not the API.

**Query Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `pageSize` | integer | Number of sources to return (1-100). Defaults to 30 |
| `pageToken` | string | Page token from previous response |
| `filter` | string | Filter expression (AIP-160). Example: `name=sources/source1 OR name=sources/source2` |

**Example:**

```bash
curl -H "x-goog-api-key: $JULES_API_KEY" \
  "https://jules.googleapis.com/v1alpha/sources?pageSize=10"
```

**Response:**

```json
{
  "sources": [
    {
      "name": "sources/github-myorg-myrepo",
      "id": "github-myorg-myrepo",
      "githubRepo": {
        "owner": "myorg",
        "repo": "myrepo",
        "isPrivate": false,
        "defaultBranch": { "displayName": "main" },
        "branches": [
          { "displayName": "main" },
          { "displayName": "develop" },
          { "displayName": "feature/auth" }
        ]
      }
    }
  ],
  "nextPageToken": "eyJvZmZzZXQiOjEwfQ=="
}
```

---

### Get a Source

```
GET /v1alpha/sources/{sourceId}
```

Retrieves a single source by ID.

**Path Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Resource name. Format: `sources/{source}` |

**Example:**

```bash
curl -H "x-goog-api-key: $JULES_API_KEY" \
  https://jules.googleapis.com/v1alpha/sources/github-myorg-myrepo
```

**Response:**

```json
{
  "name": "sources/github-myorg-myrepo",
  "id": "github-myorg-myrepo",
  "githubRepo": {
    "owner": "myorg",
    "repo": "myrepo",
    "isPrivate": false,
    "defaultBranch": { "displayName": "main" },
    "branches": [
      { "displayName": "main" },
      { "displayName": "develop" },
      { "displayName": "feature/auth" },
      { "displayName": "feature/tests" }
    ]
  }
}
```

---

## Key Types

### SessionState

| Value | Description |
|---|---|
| `STATE_UNSPECIFIED` | State is unspecified |
| `QUEUED` | Session is waiting to be processed |
| `PLANNING` | Jules is creating a plan |
| `AWAITING_PLAN_APPROVAL` | Plan is ready for user approval |
| `AWAITING_USER_FEEDBACK` | Jules needs user input |
| `IN_PROGRESS` | Jules is actively working |
| `PAUSED` | Session is paused |
| `FAILED` | Session failed |
| `COMPLETED` | Session completed successfully |

### AutomationMode

| Value | Description |
|---|---|
| `AUTOMATION_MODE_UNSPECIFIED` | No automation (default) |
| `AUTO_CREATE_PR` | Automatically create a pull request when code changes are ready |

### Activity Event Types

Each activity has exactly one of these fields populated:

| Type | Field | Description |
|---|---|---|
| Plan Generated | `planGenerated` | Jules created a plan |
| Plan Approved | `planApproved` | A plan was approved |
| User Messaged | `userMessaged` | A message from the user |
| Agent Messaged | `agentMessaged` | A message from Jules |
| Progress Updated | `progressUpdated` | A status update during execution |
| Session Completed | `sessionCompleted` | The session finished successfully |
| Session Failed | `sessionFailed` | The session encountered an error |

### Artifact Types

| Type | Description |
|---|---|
| `changeSet` | Code changes (Git patch with base commit, unidiff, suggested commit message) |
| `bashOutput` | Command output (command, output, exit code) |
| `media` | Media file (MIME type, base64-encoded data) |

---

## Error Handling

| Status | Description |
|---|---|
| 200 | Success |
| 400 | Bad request — invalid parameters |
| 401 | Unauthorized — invalid or missing token |
| 403 | Forbidden — insufficient permissions |
| 404 | Not found — resource doesn't exist |
| 429 | Rate limited — too many requests |
| 500 | Server error |

Error response format:

```json
{
  "error": {
    "code": 400,
    "message": "Invalid session ID format",
    "status": "INVALID_ARGUMENT"
  }
}
```

---

## Resource Naming Convention

Resources follow Google API hierarchical naming:

- Sessions: `sessions/{sessionId}`
- Activities: `sessions/{sessionId}/activities/{activityId}`
- Sources: `sources/{sourceId}`
