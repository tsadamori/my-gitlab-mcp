package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/xanzy/go-gitlab"
)

var gitlabClient *gitlab.Client

func main() {
	// GitLab クライアントの初期化
	token := os.Getenv("GITLAB_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "GITLAB_TOKEN environment variable is required")
		os.Exit(1)
	}

	baseURL := os.Getenv("GITLAB_URL")
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}

	var err error
	gitlabClient, err = gitlab.NewClient(token, gitlab.WithBaseURL(baseURL+"/api/v4"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
		os.Exit(1)
	}

	// MCP サーバーの作成
	s := server.NewMCPServer(
		"GitLab MCP Server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// ツールの登録
	registerTools(s)

	// サーバー起動
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func registerTools(s *server.MCPServer) {
	// プロジェクト一覧取得
	s.AddTool(
		mcp.NewTool("list_projects",
			mcp.WithDescription("List GitLab projects accessible to the user"),
			mcp.WithNumber("per_page",
				mcp.Description("Number of projects per page (default: 20, max: 100)"),
			),
			mcp.WithNumber("page",
				mcp.Description("Page number (default: 1)"),
			),
		),
		handleListProjects,
	)

	// プロジェクト詳細取得
	s.AddTool(
		mcp.NewTool("get_project",
			mcp.WithDescription("Get details of a specific GitLab project"),
			mcp.WithString("project_id",
				mcp.Required(),
				mcp.Description("Project ID or path (e.g., 'namespace/project-name' or '12345')"),
			),
		),
		handleGetProject,
	)

	// イシュー一覧取得
	s.AddTool(
		mcp.NewTool("list_issues",
			mcp.WithDescription("List issues in a GitLab project"),
			mcp.WithString("project_id",
				mcp.Required(),
				mcp.Description("Project ID or path"),
			),
			mcp.WithString("state",
				mcp.Description("Filter by state: opened, closed, all (default: opened)"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("Number of issues per page (default: 20)"),
			),
		),
		handleListIssues,
	)

	// イシュー作成
	s.AddTool(
		mcp.NewTool("create_issue",
			mcp.WithDescription("Create a new issue in a GitLab project"),
			mcp.WithString("project_id",
				mcp.Required(),
				mcp.Description("Project ID or path"),
			),
			mcp.WithString("title",
				mcp.Required(),
				mcp.Description("Issue title"),
			),
			mcp.WithString("description",
				mcp.Description("Issue description (supports Markdown)"),
			),
			mcp.WithString("labels",
				mcp.Description("Comma-separated list of labels"),
			),
		),
		handleCreateIssue,
	)

	// マージリクエスト一覧取得
	s.AddTool(
		mcp.NewTool("list_merge_requests",
			mcp.WithDescription("List merge requests in a GitLab project"),
			mcp.WithString("project_id",
				mcp.Required(),
				mcp.Description("Project ID or path"),
			),
			mcp.WithString("state",
				mcp.Description("Filter by state: opened, closed, merged, all (default: opened)"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("Number of merge requests per page (default: 20)"),
			),
		),
		handleListMergeRequests,
	)

	// マージリクエスト作成
	s.AddTool(
		mcp.NewTool("create_merge_request",
			mcp.WithDescription("Create a new merge request in a GitLab project"),
			mcp.WithString("project_id",
				mcp.Required(),
				mcp.Description("Project ID or path"),
			),
			mcp.WithString("source_branch",
				mcp.Required(),
				mcp.Description("Source branch name"),
			),
			mcp.WithString("target_branch",
				mcp.Required(),
				mcp.Description("Target branch name (e.g., main, master)"),
			),
			mcp.WithString("title",
				mcp.Required(),
				mcp.Description("Merge request title"),
			),
			mcp.WithString("description",
				mcp.Description("Merge request description (supports Markdown)"),
			),
			mcp.WithBoolean("remove_source_branch",
				mcp.Description("Remove source branch after merge (default: false)"),
			),
			mcp.WithBoolean("squash",
				mcp.Description("Squash commits on merge (default: false)"),
			),
			mcp.WithString("labels",
				mcp.Description("Comma-separated list of labels"),
			),
			mcp.WithString("assignee_ids",
				mcp.Description("Comma-separated list of assignee user IDs"),
			),
		),
		handleCreateMergeRequest,
	)

	// ファイル内容取得
	s.AddTool(
		mcp.NewTool("get_file",
			mcp.WithDescription("Get contents of a file from a GitLab repository"),
			mcp.WithString("project_id",
				mcp.Required(),
				mcp.Description("Project ID or path"),
			),
			mcp.WithString("file_path",
				mcp.Required(),
				mcp.Description("Path to the file in the repository"),
			),
			mcp.WithString("ref",
				mcp.Description("Branch, tag, or commit SHA (default: default branch)"),
			),
		),
		handleGetFile,
	)
}

// ツールハンドラー

func handleListProjects(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	perPage := getInt(args, "per_page", 20)
	page := getInt(args, "page", 1)

	opts := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: perPage,
			Page:    page,
		},
		Membership: gitlab.Ptr(true),
	}

	projects, _, err := gitlabClient.Projects.ListProjects(opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list projects: %v", err)), nil
	}

	result := make([]map[string]interface{}, len(projects))
	for i, p := range projects {
		result[i] = map[string]interface{}{
			"id":                  p.ID,
			"name":                p.Name,
			"path_with_namespace": p.PathWithNamespace,
			"description":         p.Description,
			"web_url":             p.WebURL,
			"default_branch":      p.DefaultBranch,
		}
	}

	return jsonResult(result)
}

func handleGetProject(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	projectID, ok := args["project_id"].(string)
	if !ok || projectID == "" {
		return mcp.NewToolResultError("project_id is required"), nil
	}

	project, _, err := gitlabClient.Projects.GetProject(projectID, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get project: %v", err)), nil
	}

	result := map[string]interface{}{
		"id":                  project.ID,
		"name":                project.Name,
		"path_with_namespace": project.PathWithNamespace,
		"description":         project.Description,
		"web_url":             project.WebURL,
		"default_branch":      project.DefaultBranch,
		"visibility":          project.Visibility,
		"created_at":          project.CreatedAt,
		"last_activity_at":    project.LastActivityAt,
		"open_issues_count":   project.OpenIssuesCount,
		"star_count":          project.StarCount,
		"forks_count":         project.ForksCount,
	}

	return jsonResult(result)
}

func handleListIssues(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	projectID, ok := args["project_id"].(string)
	if !ok || projectID == "" {
		return mcp.NewToolResultError("project_id is required"), nil
	}

	state := getString(args, "state", "opened")
	perPage := getInt(args, "per_page", 20)

	opts := &gitlab.ListProjectIssuesOptions{
		State: gitlab.Ptr(state),
		ListOptions: gitlab.ListOptions{
			PerPage: perPage,
		},
	}

	issues, _, err := gitlabClient.Issues.ListProjectIssues(projectID, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list issues: %v", err)), nil
	}

	result := make([]map[string]interface{}, len(issues))
	for i, issue := range issues {
		result[i] = map[string]interface{}{
			"iid":        issue.IID,
			"title":      issue.Title,
			"state":      issue.State,
			"author":     issue.Author.Username,
			"labels":     issue.Labels,
			"created_at": issue.CreatedAt,
			"web_url":    issue.WebURL,
		}
	}

	return jsonResult(result)
}

func handleCreateIssue(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	projectID, ok := args["project_id"].(string)
	if !ok || projectID == "" {
		return mcp.NewToolResultError("project_id is required"), nil
	}

	title, ok := args["title"].(string)
	if !ok || title == "" {
		return mcp.NewToolResultError("title is required"), nil
	}

	opts := &gitlab.CreateIssueOptions{
		Title: gitlab.Ptr(title),
	}

	if desc := getString(args, "description", ""); desc != "" {
		opts.Description = gitlab.Ptr(desc)
	}

	if labels := getString(args, "labels", ""); labels != "" {
		labelList := gitlab.LabelOptions(splitLabels(labels))
		opts.Labels = &labelList
	}

	issue, _, err := gitlabClient.Issues.CreateIssue(projectID, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create issue: %v", err)), nil
	}

	result := map[string]interface{}{
		"iid":     issue.IID,
		"title":   issue.Title,
		"web_url": issue.WebURL,
	}

	return jsonResult(result)
}

func handleListMergeRequests(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	projectID, ok := args["project_id"].(string)
	if !ok || projectID == "" {
		return mcp.NewToolResultError("project_id is required"), nil
	}

	state := getString(args, "state", "opened")
	perPage := getInt(args, "per_page", 20)

	opts := &gitlab.ListProjectMergeRequestsOptions{
		State: gitlab.Ptr(state),
		ListOptions: gitlab.ListOptions{
			PerPage: perPage,
		},
	}

	mrs, _, err := gitlabClient.MergeRequests.ListProjectMergeRequests(projectID, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list merge requests: %v", err)), nil
	}

	result := make([]map[string]interface{}, len(mrs))
	for i, mr := range mrs {
		result[i] = map[string]interface{}{
			"iid":           mr.IID,
			"title":         mr.Title,
			"state":         mr.State,
			"author":        mr.Author.Username,
			"source_branch": mr.SourceBranch,
			"target_branch": mr.TargetBranch,
			"created_at":    mr.CreatedAt,
			"web_url":       mr.WebURL,
		}
	}

	return jsonResult(result)
}

func handleCreateMergeRequest(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	projectID, ok := args["project_id"].(string)
	if !ok || projectID == "" {
		return mcp.NewToolResultError("project_id is required"), nil
	}

	sourceBranch, ok := args["source_branch"].(string)
	if !ok || sourceBranch == "" {
		return mcp.NewToolResultError("source_branch is required"), nil
	}

	targetBranch, ok := args["target_branch"].(string)
	if !ok || targetBranch == "" {
		return mcp.NewToolResultError("target_branch is required"), nil
	}

	title, ok := args["title"].(string)
	if !ok || title == "" {
		return mcp.NewToolResultError("title is required"), nil
	}

	opts := &gitlab.CreateMergeRequestOptions{
		SourceBranch: gitlab.Ptr(sourceBranch),
		TargetBranch: gitlab.Ptr(targetBranch),
		Title:        gitlab.Ptr(title),
	}

	if desc := getString(args, "description", ""); desc != "" {
		opts.Description = gitlab.Ptr(desc)
	}

	if remove, ok := args["remove_source_branch"].(bool); ok {
		opts.RemoveSourceBranch = gitlab.Ptr(remove)
	}

	if squash, ok := args["squash"].(bool); ok {
		opts.Squash = gitlab.Ptr(squash)
	}

	if labels := getString(args, "labels", ""); labels != "" {
		labelList := gitlab.LabelOptions(splitLabels(labels))
		opts.Labels = &labelList
	}

	if assigneeIDs := getString(args, "assignee_ids", ""); assigneeIDs != "" {
		ids := parseIntList(assigneeIDs)
		if len(ids) > 0 {
			opts.AssigneeIDs = &ids
		}
	}

	mr, _, err := gitlabClient.MergeRequests.CreateMergeRequest(projectID, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create merge request: %v", err)), nil
	}

	result := map[string]interface{}{
		"iid":           mr.IID,
		"title":         mr.Title,
		"state":         mr.State,
		"source_branch": mr.SourceBranch,
		"target_branch": mr.TargetBranch,
		"web_url":       mr.WebURL,
	}

	return jsonResult(result)
}

func handleGetFile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	projectID, ok := args["project_id"].(string)
	if !ok || projectID == "" {
		return mcp.NewToolResultError("project_id is required"), nil
	}

	filePath, ok := args["file_path"].(string)
	if !ok || filePath == "" {
		return mcp.NewToolResultError("file_path is required"), nil
	}

	ref := getString(args, "ref", "")

	opts := &gitlab.GetFileOptions{}
	if ref != "" {
		opts.Ref = gitlab.Ptr(ref)
	}

	file, _, err := gitlabClient.RepositoryFiles.GetFile(projectID, filePath, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get file: %v", err)), nil
	}

	var content string
	if file.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to decode file content: %v", err)), nil
		}
		content = string(decoded)
	} else {
		content = file.Content
	}

	result := map[string]interface{}{
		"file_name": file.FileName,
		"file_path": file.FilePath,
		"size":      file.Size,
		"ref":       file.Ref,
		"content":   content,
	}

	return jsonResult(result)
}

// ヘルパー関数

func jsonResult(data interface{}) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

func getString(args map[string]interface{}, key, defaultVal string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return defaultVal
}

func getInt(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	return defaultVal
}

func parseIntList(s string) []int {
	var result []int
	for _, part := range splitString(s, ",") {
		if trimmed := trimSpace(part); trimmed != "" {
			if id, err := strconv.Atoi(trimmed); err == nil {
				result = append(result, id)
			}
		}
	}
	return result
}

func splitLabels(labels string) []string {
	var result []string
	for _, l := range splitString(labels, ",") {
		if trimmed := trimSpace(l); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
