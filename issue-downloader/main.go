package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Issue struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
	HTMLURL string `json:"html_url"`
}

type Config struct {
	RepoURL string // GitHub 仓库地址，如 https://github.com/owner/repo
	Token   string // GitHub Personal Access Token（Settings → Developer settings → Tokens）
	OutDir  string // 本地输出目录
}

func main() {
	// 默认启动 GUI；加 cli 参数走命令行模式
	if len(os.Args) > 1 && os.Args[1] == "cli" {
		cfg := Config{
			RepoURL: os.Getenv("GITHUB_REPO_URL"),
			Token:   os.Getenv("GITHUB_TOKEN"),
			OutDir:  getEnv("ISSUE_OUT_DIR", "./issues"),
		}
		runCLI(cfg)
		return
	}
	startGUI()
}

func runCLI(cfg Config) {
	if cfg.RepoURL == "" {
		fmt.Println("错误：请设置 GITHUB_REPO_URL 环境变量")
		fmt.Println("示例：export GITHUB_REPO_URL=https://github.com/owner/repo")
		os.Exit(1)
	}
	if cfg.Token == "" {
		fmt.Println("错误：请设置 GITHUB_TOKEN 环境变量")
		fmt.Println("获取方式：GitHub → Settings → Developer settings → Personal access tokens → Generate new token (classic)")
		os.Exit(1)
	}

	// 解析 owner/repo
	owner, repo := parseRepo(cfg.RepoURL)
	if owner == "" || repo == "" {
		fmt.Println("错误：无法解析仓库地址，格式应为 https://github.com/owner/repo")
		os.Exit(1)
	}

	// 创建输出目录
	os.MkdirAll(cfg.OutDir, 0755)

	// 拉取所有 Issues
	issues, err := fetchIssues(owner, repo, cfg.Token)
	if err != nil {
		fmt.Printf("错误：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("共找到 %d 个 Issue\n", len(issues))
	if len(issues) == 0 {
		fmt.Println("没有可下载的 Issue")
		return
	}

	// 写入本地文件
	for _, issue := range issues {
		saveIssue(cfg.OutDir, issue)
		fmt.Printf("  #%d %s [%s]\n", issue.Number, issue.Title, issue.State)
	}

	fmt.Printf("\n已保存到 %s\n", cfg.OutDir)
}

func parseRepo(url string) (string, string) {
	url = strings.TrimRight(url, "/")
	url = strings.TrimSuffix(url, "/issues")
	url = strings.TrimRight(url, ".git")
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return "", ""
	}
	return parts[len(parts)-2], parts[len(parts)-1]
}

func fetchIssues(owner, repo, token string) ([]Issue, error) {
	var allIssues []Issue
	page := 1

	for {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues?state=all&page=%d&per_page=100&sort=created&direction=desc",
			owner, repo, page)

		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "issue-downloader")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return allIssues, fmt.Errorf("请求失败 (page %d): %w", page, err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			msg := string(body)
			if len(msg) > 200 {
				msg = msg[:200]
			}
			return allIssues, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, msg)
		}

		var issues []Issue
		json.Unmarshal(body, &issues)
		if len(issues) == 0 {
			break
		}

		allIssues = append(allIssues, issues...)
		page++
	}
	return allIssues, nil
}

func saveIssue(outDir string, issue Issue) {
	filename := fmt.Sprintf("#%d-%s.md", issue.Number, sanitizeFilename(issue.Title))

	// 构建 Markdown 内容
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# #%d %s\n\n", issue.Number, issue.Title))
	sb.WriteString(fmt.Sprintf("> 状态：%s | 创建：%s | 更新：%s\n\n",
		issue.State, issue.CreatedAt, issue.UpdatedAt))

	if len(issue.Labels) > 0 {
		sb.WriteString("标签：")
		for i, l := range issue.Labels {
			if i > 0 {
				sb.WriteString("、")
			}
			sb.WriteString("`" + l.Name + "`")
		}
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf("原文链接：%s\n\n---\n\n", issue.HTMLURL))
	sb.WriteString(issue.Body)

	os.WriteFile(filepath.Join(outDir, filename), []byte(sb.String()), 0644)
}

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "*", "")
	name = strings.ReplaceAll(name, "?", "")
	name = strings.ReplaceAll(name, "\"", "")
	name = strings.ReplaceAll(name, "<", "")
	name = strings.ReplaceAll(name, ">", "")
	name = strings.ReplaceAll(name, "|", "-")
	if len(name) > 60 {
		name = name[:60]
	}
	return name
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
