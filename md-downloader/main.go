package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func main() {
	outDir := flag.String("o", ".", "输出目录")
	flag.Parse()

	urls := flag.Args()
	if len(urls) == 0 {
		printUsage()
		os.Exit(1)
	}

	token := os.Getenv("GITHUB_TOKEN")
	os.MkdirAll(*outDir, 0755)

	success, fail := 0, 0
	for _, rawURL := range urls {
		parsed, err := parseURL(rawURL)
		if err != nil {
			fmt.Printf("✗ %s\n", err)
			fail++
			continue
		}

		fmt.Printf("下载 %s/%s → ", parsed.Owner, parsed.Repo)

		var content []byte
		var localPath string

		switch parsed.Type {
		case "blob":
			fmt.Printf("%s/%s ... ", parsed.Branch, parsed.Path)
			content, err = fetchBlob(parsed.Owner, parsed.Repo, parsed.Branch, parsed.Path, token)
			localPath = filepath.Join(*outDir, parsed.Path)
		case "wiki":
			fmt.Printf("%s ... ", parsed.Page)
			content, err = fetchWiki(parsed.Owner, parsed.Repo, parsed.Page, token)
			localPath = filepath.Join(*outDir, parsed.Page+".md")
		}

		if err != nil {
			fmt.Printf("失败：%v\n", err)
			fail++
			continue
		}

		os.MkdirAll(filepath.Dir(localPath), 0755)
		if err := os.WriteFile(localPath, content, 0644); err != nil {
			fmt.Printf("写入失败：%v\n", err)
			fail++
			continue
		}

		fmt.Printf("✓ → %s\n", localPath)
		success++
	}

	fmt.Printf("\n完成：成功 %d，失败 %d，文件保存在 %s\n", success, fail, *outDir)
}

func printUsage() {
	fmt.Println("用法：md-downloader [-o 输出目录] <GitHub 文档 URL> [URL2] ...")
	fmt.Println()
	fmt.Println("支持的 URL 类型：")
	fmt.Println("  · 仓库文件：https://github.com/owner/repo/blob/branch/path/to/file.md")
	fmt.Println("  · Wiki 页面：https://github.com/owner/repo/wiki/页面标题")
	fmt.Println()
	fmt.Println("示例：")
	fmt.Println("  md-downloader https://github.com/owner/repo/blob/main/docs/readme.md")
	fmt.Println("  md-downloader https://github.com/owner/repo/wiki/产品定义")
	fmt.Println("  md-downloader -o ./my-docs url1 url2 url3")
	fmt.Println()
	fmt.Println("私有仓库请设置 GITHUB_TOKEN 环境变量")
}

// ParsedURL 解析结果
type ParsedURL struct {
	Type   string // "blob" 或 "wiki"
	Owner  string
	Repo   string
	Branch string // 仅 blob
	Path   string // 仅 blob
	Page   string // 仅 wiki
}

var (
	blobPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/blob/([^/]+)/(.+)$`)
	wikiPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/wiki/(.+)$`)
)

func parseURL(rawURL string) (*ParsedURL, error) {
	rawURL = strings.TrimSpace(rawURL)

	// 尝试 blob URL
	if m := blobPattern.FindStringSubmatch(rawURL); m != nil {
		return &ParsedURL{
			Type:   "blob",
			Owner:  m[1],
			Repo:   m[2],
			Branch: m[3],
			Path:   m[4],
		}, nil
	}

	// 尝试 wiki URL
	if m := wikiPattern.FindStringSubmatch(rawURL); m != nil {
		page, _ := url.QueryUnescape(m[3])
		return &ParsedURL{
			Type:  "wiki",
			Owner: m[1],
			Repo:  m[2],
			Page:  page,
		}, nil
	}

	return nil, fmt.Errorf("无法解析 URL：%s", rawURL)
}

// fetchBlob 通过 GitHub API 获取仓库文件内容
func fetchBlob(owner, repo, branch, path, token string) ([]byte, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		owner, repo, path, branch)

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "md-downloader")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("文件不存在（404）")
	}
	if resp.StatusCode == 403 && token == "" {
		return nil, fmt.Errorf("触发 API 限流，请设置 GITHUB_TOKEN 环境变量后重试")
	}
	if resp.StatusCode != 200 {
		msg := string(body)
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, fmt.Errorf("API 返回 %d：%s", resp.StatusCode, msg)
	}

	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败：%w", err)
	}

	if result.Encoding != "base64" {
		return nil, fmt.Errorf("不支持的编码格式：%s", result.Encoding)
	}

	return base64.StdEncoding.DecodeString(result.Content)
}

// fetchWiki 通过浅克隆 Wiki 仓库获取页面内容
// raw.githubusercontent.com 对中文路径支持不稳定，直接读 git 仓库最可靠
func fetchWiki(owner, repo, page, token string) ([]byte, error) {
	// 构造带 Token 的 Wiki 仓库地址
	wikiURL := fmt.Sprintf("https://github.com/%s/%s.wiki.git", owner, repo)
	if token != "" {
		wikiURL = fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.wiki.git",
			token, owner, repo)
	}

	// 缓存目录：~/.md-downloader/wiki-cache/{owner}/{repo}/
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".md-downloader", "wiki-cache", owner, repo)
	filename := page + ".md"
	filePath := filepath.Join(cacheDir, filename)

	// 如果缓存存在且是今天更新的，直接读
	if info, err := os.Stat(filePath); err == nil {
		if time.Since(info.ModTime()) < 24*time.Hour {
			return os.ReadFile(filePath)
		}
	}

	// 克隆或更新（绕过系统代理，避免被本地代理拦截）
	if _, err := os.Stat(filepath.Join(cacheDir, ".git")); err == nil {
		// 已有克隆，执行 pull
		cmd := exec.Command("git", "-c", "http.proxy=", "-c", "https.proxy=",
			"-C", cacheDir, "pull", "--ff-only")
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		cmd.Run() // 忽略 pull 错误（可能没有网络等），继续读本地文件
	} else {
		// 浅克隆
		os.MkdirAll(filepath.Dir(cacheDir), 0755)
		// 先清理可能的不完整克隆
		os.RemoveAll(cacheDir)
		cmd := exec.Command("git", "-c", "http.proxy=", "-c", "https.proxy=",
			"clone", "--depth", "1", "--single-branch",
			wikiURL, cacheDir)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			os.RemoveAll(cacheDir)
			msg := string(out)
			if strings.Contains(msg, "not found") || strings.Contains(msg, "404") {
				return nil, fmt.Errorf("Wiki 仓库不存在或未开启")
			}
			if strings.Contains(msg, "403") || strings.Contains(msg, "429") {
				return nil, fmt.Errorf("触发 GitHub 限流，请设置 GITHUB_TOKEN 环境变量后重试")
			}
			return nil, fmt.Errorf("克隆 Wiki 仓库失败：%s", strings.TrimSpace(msg))
		}
	}

	// 读取文件
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("Wiki 页面不存在（%s）", filename)
	}
	return content, nil
}
