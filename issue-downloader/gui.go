package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

//go:embed static/index.html
var guiHTML string

func startGUI() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(guiHTML))
	})

	http.HandleFunc("/api/list", handleList)
	http.HandleFunc("/api/fetch", handleFetch)

	// 配置读取：优先本地 config.json，其次 ~/.issue-downloader.json
	home, _ := os.UserHomeDir()
	userCfgFile := filepath.Join(home, ".issue-downloader.json")

	loadConfig := func() []byte {
		// 先找当前目录下的 config.json
		if data, err := os.ReadFile("config.json"); err == nil {
			return data
		}
		// 再找用户目录下的配置文件
		if data, err := os.ReadFile(userCfgFile); err == nil {
			return data
		}
		return []byte("{}")
	}

	http.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			os.WriteFile("config.json", body, 0600)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadConfig())
	})

	port := "8877"
	url := "http://localhost:" + port
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(url)
	}()

	fmt.Printf("GUI 已启动：%s\n", url)
	fmt.Println("按 Ctrl+C 退出")
	http.ListenAndServe(":"+port, nil)
}

func handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	var req struct {
		RepoURL string `json:"repo_url"`
		Token   string `json:"token"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	owner, repo := parseRepo(req.RepoURL)
	issues, err := fetchIssues(owner, repo, req.Token)
	if err != nil {
		writeJSON(w, map[string]interface{}{"error": err.Error()})
		return
	}

	type issueBrief struct {
		Number int      `json:"number"`
		Title  string   `json:"title"`
		State  string   `json:"state"`
		Labels []string `json:"labels"`
	}
	var list []issueBrief
	for _, iss := range issues {
		var labels []string
		for _, l := range iss.Labels {
			labels = append(labels, l.Name)
		}
		list = append(list, issueBrief{
			Number: iss.Number,
			Title:  iss.Title,
			State:  iss.State,
			Labels: labels,
		})
	}
	writeJSON(w, map[string]interface{}{"issues": list})
}

func handleFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	var req struct {
		RepoURL      string `json:"repo_url"`
		Token        string `json:"token"`
		OutDir       string `json:"out_dir"`
		IssueNumbers []int  `json:"issue_numbers"` // 指定下载的编号，为空则下载全部
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.RepoURL == "" || req.Token == "" {
		writeJSON(w, map[string]interface{}{"error": "仓库地址和 Token 不能为空"})
		return
	}

	owner, repo := parseRepo(req.RepoURL)
	if owner == "" || repo == "" {
		writeJSON(w, map[string]interface{}{"error": "仓库地址格式错误，应为 https://github.com/owner/repo"})
		return
	}

	if req.OutDir == "" {
		req.OutDir = "./issues"
	}
	os.MkdirAll(req.OutDir, 0755)

	// 构造选中集合，为空表示全选
	wanted := make(map[int]bool)
	for _, n := range req.IssueNumbers {
		wanted[n] = true
	}
	selectAll := len(req.IssueNumbers) == 0

	issues, err := fetchIssues(owner, repo, req.Token)
	if err != nil {
		writeJSON(w, map[string]interface{}{"error": err.Error()})
		return
	}

	count := 0
	for _, issue := range issues {
		if selectAll || wanted[issue.Number] {
			saveIssue(req.OutDir, issue)
			count++
		}
	}

	writeJSON(w, map[string]interface{}{
		"ok":     true,
		"count":  count,
		"outDir": req.OutDir,
	})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	cmd.Start()
}

