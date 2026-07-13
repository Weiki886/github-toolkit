# Issue Downloader

从任意的 GitHub 仓库拉取 Issue 列表，将 Issue 下载到本地保存为 Markdown 文件。

## 功能

- **Web GUI**：浏览器操作，可视化选择要下载的 Issue，支持全选/单选
- **CLI 模式**：命令行一键下载，适合脚本和自动化
- **选择性导出**：可以只下载部分 Issue，也可以一次下载全部
- **配置记忆**：GUI 模式下自动读取本地配置文件，下次打开无需重新填写

## 前置要求

- Go 1.21+
- GitHub Personal Access Token（免费获取，需要有 `repo` 权限）

## 安装

```bash
cd issue-downloader
go build -o issue-downloader .
```

编译完成后直接运行 `./issue-downloader` 即可。

## 使用方式

### GUI 模式（默认）

```bash
./issue-downloader
# 或者
go run .
```

程序启动后自动打开浏览器访问 `http://localhost:8877`，在页面上：

1. 填写 GitHub 仓库地址，例如 `https://github.com/gin-gonic/gin`
2. 填写 Personal Access Token
3. （可选）指定本地保存路径，默认为 `./issues`
4. 点击 **拉取 Issue 列表** 查看该仓库的全部 Issue
5. 勾选需要的 Issue，点击 **下载选中** 或 **下载全部**
6. 下载完成后在保存路径查看 markdown 文件

> 填写的仓库地址和 Token 会自动保存到 `config.json`，下次启动时自动回填。

### CLI 模式

```bash
./issue-downloader cli
# 或者
go run . cli
```

通过环境变量配置，下载整个仓库的全部 Issue：

```bash
export GITHUB_REPO_URL="https://github.com/owner/repo"
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
export ISSUE_OUT_DIR="./issues"          # 可选，默认 ./issues

./issue-downloader cli
```

运行效果：

```
共找到 42 个 Issue
  #1 欢迎页面 [open]
  #2 修复登录页样式 [closed]
  ...
已保存到 ./issues
```

## 获取 GitHub Token

1. 登录 GitHub → 右上角头像 → **Settings**
2. 左侧菜单 → **Developer settings**
3. **Personal access tokens** → **Tokens (classic)**
4. 点击 **Generate new token (classic)**
5. 勾选 `repo` 权限（私有仓库需要，公开仓库勾选 `public_repo` 即可）
6. 生成后复制 Token（只显示一次，请妥善保存）

## 配置文件

### GUI 配置文件

GUI 模式支持本地配置文件，优先级从高到低：

1. **`config.json`**（当前工作目录）
2. **`~/.issue-downloader.json`**（用户主目录）

配置内容示例：

```json
{
  "repo_url": "https://github.com/owner/repo",
  "token": "ghp_xxxxxxxxxxxx",
  "out_dir": "./issues"
}
```

配置文件适合个人使用，请勿将含 Token 的配置文件提交到 Git。仓库已提供 `config.example.json` 作为模板。

### CLI 环境变量

CLI 模式不支持配置文件，完全通过环境变量控制：

| 环境变量 | 必填 | 说明 |
|----------|------|------|
| `GITHUB_REPO_URL` | 是 | GitHub 仓库完整地址 |
| `GITHUB_TOKEN` | 是 | Personal Access Token |
| `ISSUE_OUT_DIR` | 否 | 输出目录，默认 `./issues` |

## 输出格式

每个 Issue 保存为一个独立的 `.md` 文件，文件名为 `#编号-标题.md`。

文件内容示例：

```markdown
# #1 欢迎页面

> 状态：open | 创建：2024-01-15T10:30:00Z | 更新：2024-01-15T10:30:00Z

标签：`documentation`、`good first issue`

原文链接：https://github.com/owner/repo/issues/1

---

（Issue 正文内容）
```

包含的元信息：
- Issue 编号和标题（作为一级标题）
- 状态（open / closed）
- 创建和更新时间
- 标签列表
- 原文链接（方便回溯）
- 完整正文内容

## 项目结构

```
issue-downloader/
├── main.go              # CLI 入口、Issue 拉取与保存逻辑
├── gui.go               # Web 服务、API 路由
├── static/
│   └── index.html       # GUI 前端页面
├── config.example.json  # 配置模板
├── config.json          # 本地配置（Git 忽略）
├── go.mod / go.sum      # Go 模块依赖
└── README.md
```

## API 接口

GUI 模式启动后在 `localhost:8877` 提供以下接口：

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/` | GUI 页面 |
| `GET/POST` | `/api/config` | 读取或保存配置 |
| `POST` | `/api/list` | 拉取 Issue 列表 |
| `POST` | `/api/fetch` | 下载指定 Issue |
