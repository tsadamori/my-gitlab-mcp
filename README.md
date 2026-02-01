# my-gitlab-mcp

GitLab MCP Server written in Go.

## Features

| Tool | Description |
|------|-------------|
| `list_projects` | List GitLab projects accessible to the user |
| `get_project` | Get details of a specific project |
| `list_issues` | List issues in a project |
| `create_issue` | Create a new issue |
| `list_merge_requests` | List merge requests in a project |
| `create_merge_request` | Create a new merge request |
| `get_file` | Get contents of a file from a repository |
| `create_or_update_file` | Create or update a file in a repository |
| `delete_file` | Delete a file from a repository |
| `create_branch` | Create a new branch |
| `list_branches` | List branches in a repository |
| `push_files` | Push multiple files in a single commit |

## Installation

### Prerequisites

- Go 1.21+

### Build

```bash
git clone https://github.com/tsadamori/my-gitlab-mcp.git
cd my-gitlab-mcp
go build -o gitlab-mcp
```

## Configuration

### 1. Create GitLab Personal Access Token

1. Go to GitLab → Settings → Access Tokens
2. Create a token with scopes: `api`, `read_api`, `read_repository`, `write_repository`

### 2. Add to Claude Code

Edit `~/.claude.json`:

```json
{
  "mcpServers": {
    "gitlab": {
      "type": "stdio",
      "command": "/path/to/gitlab-mcp",
      "args": [],
      "env": {
        "GITLAB_TOKEN": "glpat-xxxxxxxxxxxx",
        "GITLAB_URL": "https://gitlab.com"
      }
    }
  }
}
```

For self-hosted GitLab, change `GITLAB_URL` to your instance URL.

### 3. Restart Claude Code

```bash
exit
claude
```

## Usage Examples

### List projects
```
List my GitLab projects
```

### Create an issue
```
Create an issue in project "mygroup/myproject" with title "Bug fix needed"
```

### Create a merge request
```
Create a merge request from branch "feature" to "main" in project "mygroup/myproject"
```

### Push multiple files
```
Push files src/app.ts and README.md to project "mygroup/myproject" on branch "main"
```

### Create a branch
```
Create a branch "feature/new-feature" from "main" in project "mygroup/myproject"
```

## License

MIT
