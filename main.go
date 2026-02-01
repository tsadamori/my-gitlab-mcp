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

	// ファイル作成/更新
	s.AddTool(
		mcp.NewTool("create_or_update_file",
			mcp.WithDescription("Create or update a file in a GitLab repository"),
			mcp.WithString("project_id",
				mcp.Required(),
				mcp.Description("Project ID or path"),
			),
			mcp.WithString("file_path",
				mcp.Required(),
				mcp.Description("Path to the file in the repository"),
			),
			mcp.WithString("branch",
				mcp.Required(),
				mcp.Description("Branch name to commit to"),
			),
			mcp.WithString("content",
				mcp.Required(),
				mcp.Description("File content"),
			),
			mcp.WithString("commit_message",
				mcp.Required(),
				mcp.Description("Commit message"),
			),
			mcp.WithString("author_email",
				mcp.Description("Author email for the commit"),
			),
			mcp.WithString("author_name",
				mcp.Description("Author name for the commit"),
			),
		),
		handleCreateOrUpdateFile,
	)

	// ファイル削除
	s.AddTool(
		mcp.NewTool("delete_file",
			mcp.WithDescription("Delete a file from a GitLab repository"),
			mcp.WithString("project_id",
				mcp.Required(),
				mcp.Description("Project ID or path"),
			),
			mcp.WithString("file_path",
				mcp.Required(),
				mcp.Description("Path to the file to delete"),
			),
			mcp.WithString("branch",
				mcp.Required(),
				mcp.Description("Branch name to commit to"),
			),
			mcp.WithString("commit_message",
				mcp.Required(),
				mcp.Description("Commit message"),
			),
			mcp.WithString("author_email",
				mcp.Description("Author email for the commit"),
			),
			mcp.WithString("author_name",
				mcp.Description("Author name for the commit"),
			),
		),
		handleDeleteFile,
	)

	// ブランチ作成
	s.AddTool(
		mcp.NewTool("create_branch",
			mcp.WithDescription("Create a new branch in a GitLab repository"),
			mcp.WithString("project_id",
				mcp.Required(),
				mcp.Description("Project ID or path"),
			),
			mcp.WithString("branch",
				mcp.Required(),
				mcp.Description("Name of the new branch"),
			),
			mcp.WithString("ref",
				mcp.Required(),
				mcp.Description("Branch name or commit SHA to create branch from"),
			),
		),
		handleCreateBranch,
	)

	// ブランチ一覧取得
	s.AddTool(
		mcp.NewTool("list_branches",
			mcp.WithDescription("List branches in a GitLab repository"),
			mcp.WithString("project_id",
				mcp.Required(),
				mcp.Description("Project ID or path"),
			),
			mcp.WithString("search",
				mcp.Description("Search branches by name"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("Number of branches per page (default: 20)"),
			),
		),
		handleListBranches,
	)

	// 複数ファイルを一度にPush
	s.AddTool(
		mcp.NewTool("push_files",
			mcp.WithDescription("Push multiple files to a GitLab repository in a single commit"),
			mcp.WithString("project_id",
				mcp.Required(),
				mcp.Description("Project ID or path"),
			),
			mcp.WithString("branch",
				mcp.Required(),
				mcp.Description("Branch to push to"),
			),
			mcp.WithString("commit_message",
				mcp.Required(),
				mcp.Description("Commit message"),
			),
			mcp.WithArray("files",
				mcp.Required(),
				mcp.Description("Array of file objects with 'path' and 'content' fields"),
			),
			mcp.WithString("author_email",
				mcp.Description("Author email for the commit"),
			),
			mcp.WithString("author_name",
				mcp.Description("Author name for the commit"),
			),
		),
		handlePushFiles,
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

func handleCreateOrUpdateFile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	projectID, ok := args["project_id"].(string)
	if !ok || projectID == "" {
		return mcp.NewToolResultError("project_id is required"), nil
	}

	filePath, ok := args["file_path"].(string)
	if !ok || filePath == "" {
		return mcp.NewToolResultError("file_path is required"), nil
	}

	branch, ok := args["branch"].(string)
	if !ok || branch == "" {
		return mcp.NewToolResultError("branch is required"), nil
	}

	content, ok := args["content"].(string)
	if !ok {
		return mcp.NewToolResultError("content is required"), nil
	}

	commitMessage, ok := args["commit_message"].(string)
	if !ok || commitMessage == "" {
		return mcp.NewToolResultError("commit_message is required"), nil
	}

	// ファイルが存在するかチェック
	_, resp, err := gitlabClient.RepositoryFiles.GetFile(projectID, filePath, &gitlab.GetFileOptions{
		Ref: gitlab.Ptr(branch),
	})

	fileExists := err == nil && resp.StatusCode == 200

	if fileExists {
		// ファイル更新
		opts := &gitlab.UpdateFileOptions{
			Branch:        gitlab.Ptr(branch),
			Content:       gitlab.Ptr(content),
			CommitMessage: gitlab.Ptr(commitMessage),
		}

		if authorEmail := getString(args, "author_email", ""); authorEmail != "" {
			opts.AuthorEmail = gitlab.Ptr(authorEmail)
		}
		if authorName := getString(args, "author_name", ""); authorName != "" {
			opts.AuthorName = gitlab.Ptr(authorName)
		}

		fileResp, _, err := gitlabClient.RepositoryFiles.UpdateFile(projectID, filePath, opts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to update file: %v", err)), nil
		}

		result := map[string]interface{}{
			"action":    "updated",
			"file_path": fileResp.FilePath,
			"branch":    fileResp.Branch,
		}
		return jsonResult(result)
	} else {
		// ファイル作成
		opts := &gitlab.CreateFileOptions{
			Branch:        gitlab.Ptr(branch),
			Content:       gitlab.Ptr(content),
			CommitMessage: gitlab.Ptr(commitMessage),
		}

		if authorEmail := getString(args, "author_email", ""); authorEmail != "" {
			opts.AuthorEmail = gitlab.Ptr(authorEmail)
		}
		if authorName := getString(args, "author_name", ""); authorName != "" {
			opts.AuthorName = gitlab.Ptr(authorName)
		}

		fileResp, _, err := gitlabClient.RepositoryFiles.CreateFile(projectID, filePath, opts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create file: %v", err)), nil
		}

		result := map[string]interface{}{
			"action":    "created",
			"file_path": fileResp.FilePath,
			"branch":    fileResp.Branch,
		}
		return jsonResult(result)
	}
}

func handleDeleteFile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	projectID, ok := args["project_id"].(string)
	if !ok || projectID == "" {
		return mcp.NewToolResultError("project_id is required"), nil
	}

	filePath, ok := args["file_path"].(string)
	if !ok || filePath == "" {
		return mcp.NewToolResultError("file_path is required"), nil
	}

	branch, ok := args["branch"].(string)
	if !ok || branch == "" {
		return mcp.NewToolResultError("branch is required"), nil
	}

	commitMessage, ok := args["commit_message"].(string)
	if !ok || commitMessage == "" {
		return mcp.NewToolResultError("commit_message is required"), nil
	}

	opts := &gitlab.DeleteFileOptions{
		Branch:        gitlab.Ptr(branch),
		CommitMessage: gitlab.Ptr(commitMessage),
	}

	if authorEmail := getString(args, "author_email", ""); authorEmail != "" {
		opts.AuthorEmail = gitlab.Ptr(authorEmail)
	}
	if authorName := getString(args, "author_name", ""); authorName != "" {
		opts.AuthorName = gitlab.Ptr(authorName)
	}

	_, err := gitlabClient.RepositoryFiles.DeleteFile(projectID, filePath, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete file: %v", err)), nil
	}

	result := map[string]interface{}{
		"action":    "deleted",
		"file_path": filePath,
		"branch":    branch,
	}
	return jsonResult(result)
}

func handleCreateBranch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	projectID, ok := args["project_id"].(string)
	if !ok || projectID == "" {
		return mcp.NewToolResultError("project_id is required"), nil
	}

	branchName, ok := args["branch"].(string)
	if !ok || branchName == "" {
		return mcp.NewToolResultError("branch is required"), nil
	}

	ref, ok := args["ref"].(string)
	if !ok || ref == "" {
		return mcp.NewToolResultError("ref is required"), nil
	}

	opts := &gitlab.CreateBranchOptions{
		Branch: gitlab.Ptr(branchName),
		Ref:    gitlab.Ptr(ref),
	}

	branch, _, err := gitlabClient.Branches.CreateBranch(projectID, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create branch: %v", err)), nil
	}

	result := map[string]interface{}{
		"name":      branch.Name,
		"commit":    branch.Commit.ID,
		"protected": branch.Protected,
		"web_url":   branch.WebURL,
	}
	return jsonResult(result)
}

func handleListBranches(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	projectID, ok := args["project_id"].(string)
	if !ok || projectID == "" {
		return mcp.NewToolResultError("project_id is required"), nil
	}

	perPage := getInt(args, "per_page", 20)
	search := getString(args, "search", "")

	opts := &gitlab.ListBranchesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: perPage,
		},
	}
	if search != "" {
		opts.Search = gitlab.Ptr(search)
	}

	branches, _, err := gitlabClient.Branches.ListBranches(projectID, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list branches: %v", err)), nil
	}

	result := make([]map[string]interface{}, len(branches))
	for i, b := range branches {
		result[i] = map[string]interface{}{
			"name":      b.Name,
			"commit":    b.Commit.ID,
			"protected": b.Protected,
			"default":   b.Default,
			"web_url":   b.WebURL,
		}
	}

	return jsonResult(result)
}

func handlePushFiles(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	projectID, ok := args["project_id"].(string)
	if !ok || projectID == "" {
		return mcp.NewToolResultError("project_id is required"), nil
	}

	branch, ok := args["branch"].(string)
	if !ok || branch == "" {
		return mcp.NewToolResultError("branch is required"), nil
	}

	commitMessage, ok := args["commit_message"].(string)
	if !ok || commitMessage == "" {
		return mcp.NewToolResultError("commit_message is required"), nil
	}

	filesArg, ok := args["files"].([]interface{})
	if !ok || len(filesArg) == 0 {
		return mcp.NewToolResultError("files is required and must be a non-empty array"), nil
	}

	// CommitActionsを構築
	var actions []*gitlab.CommitActionOptions
	for _, f := range filesArg {
		fileMap, ok := f.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("each file must be an object with 'path' and 'content' fields"), nil
		}

		filePath, ok := fileMap["path"].(string)
		if !ok || filePath == "" {
			return mcp.NewToolResultError("each file must have a 'path' field"), nil
		}

		content, ok := fileMap["content"].(string)
		if !ok {
			return mcp.NewToolResultError("each file must have a 'content' field"), nil
		}

		// ファイルが存在するかチェックしてアクションを決定
		_, resp, err := gitlabClient.RepositoryFiles.GetFile(projectID, filePath, &gitlab.GetFileOptions{
			Ref: gitlab.Ptr(branch),
		})

		var action gitlab.FileActionValue
		if err == nil && resp.StatusCode == 200 {
			action = gitlab.FileUpdate
		} else {
			action = gitlab.FileCreate
		}

		actions = append(actions, &gitlab.CommitActionOptions{
			Action:   gitlab.Ptr(action),
			FilePath: gitlab.Ptr(filePath),
			Content:  gitlab.Ptr(content),
		})
	}

	// コミットオプションを構築
	opts := &gitlab.CreateCommitOptions{
		Branch:        gitlab.Ptr(branch),
		CommitMessage: gitlab.Ptr(commitMessage),
		Actions:       actions,
	}

	if authorEmail := getString(args, "author_email", ""); authorEmail != "" {
		opts.AuthorEmail = gitlab.Ptr(authorEmail)
	}
	if authorName := getString(args, "author_name", ""); authorName != "" {
		opts.AuthorName = gitlab.Ptr(authorName)
	}

	// コミットを作成
	commit, _, err := gitlabClient.Commits.CreateCommit(projectID, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to push files: %v", err)), nil
	}

	// プッシュされたファイルのパスを収集
	var pushedFiles []string
	for _, a := range actions {
		pushedFiles = append(pushedFiles, *a.FilePath)
	}

	result := map[string]interface{}{
		"commit_id":     commit.ID,
		"commit_sha":    commit.ShortID,
		"message":       commit.Message,
		"branch":        branch,
		"files_pushed":  pushedFiles,
		"files_count":   len(pushedFiles),
		"web_url":       commit.WebURL,
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
