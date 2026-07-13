# md-downloader

从 GitHub 下载 Markdown 文档到本地，支持仓库文件（blob）和 Wiki 页面。

## 功能

- 解析 GitHub blob URL，通过 API 下载仓库文件
- 解析 GitHub Wiki URL，通过浅克隆 Wiki 仓库下载页面
- 支持一次下载多个文档
- Wiki 仓库自带本地缓存（24 小时），避免重复克隆
- 公开仓库无需 Token，私有仓库设置 `GITHUB_TOKEN` 即可

## 安装

```bash
cd md-downloader
go build -o md-downloader .
```

## 使用

### 下载仓库文件

```bash
# 下载单个文件
./md-downloader https://github.com/owner/repo/blob/main/docs/readme.md

# 指定输出目录
./md-downloader -o ./my-docs https://github.com/owner/repo/blob/main/docs/readme.md

# 一次下载多个文件
./md-downloader \
  https://github.com/owner/repo/blob/main/docs/a.md \
  https://github.com/owner/repo/blob/main/docs/b.md \
  https://github.com/owner/repo/blob/main/README.md
```

文件会保持仓库原始的目录结构保存：

```
.
├── docs/
│   └── product-interaction-design.md
└── README.md
```

### 下载 Wiki 页面

```bash
# 下载单个 Wiki 页面（中文标题直接在 URL 里）
./md-downloader https://github.com/owner/repo/wiki/产品定义

# 下载到指定目录
./md-downloader -o ./my-docs https://github.com/owner/repo/wiki/产品定义

# 一次下载多个 Wiki 页面
./md-downloader \
  https://github.com/owner/repo/wiki/mvp产品设计文档-初稿 \
  https://github.com/owner/repo/wiki/技术架构说明
```

Wiki 文件保存为 `{页面名}.md`，例如 `产品定义.md`、`mvp产品设计文档-初稿.md`。

Wiki 仓库会克隆到 `~/.md-downloader/wiki-cache/{owner}/{repo}/`，24 小时内重复下载会直接读缓存。

### 私有仓库

```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
./md-downloader https://github.com/owner/private-repo/blob/main/docs/readme.md
```

### 永久设置 Token（推荐）

把下面这行加到 `~/.zshrc`（macOS 默认）或 `~/.bashrc`：

```bash
echo 'export GITHUB_TOKEN=ghp_xxxxxxxxxxxx' >> ~/.zshrc
source ~/.zshrc
```

之后运行 md-downloader 就不用每次手动加前缀了。

## 限流说明

| 场景 | 限制 |
|------|------|
| 未认证 API 请求 | 每小时 60 次 |
| 认证后（设置 `GITHUB_TOKEN`） | 每小时 5000 次 |

如果下载大量公开文档或遇到 `触发 GitHub 限流` 错误，设置 `GITHUB_TOKEN` 即可解决。

## 生成 Token

1. 打开 https://github.com/settings/tokens
2. 点击 **Generate new token (classic)**
3. 公开仓库无需勾选任何权限，私有仓库勾选 `repo`
4. 生成后复制 Token（只显示一次）

## 支持的 URL 类型

| 类型 | 格式 |
|------|------|
| 仓库文件 | `https://github.com/owner/repo/blob/branch/path/to/file.md` |
| Wiki 页面 | `https://github.com/owner/repo/wiki/页面标题` |

## 常见错误

| 错误信息 | 原因 | 解决 |
|----------|------|------|
| `触发 GitHub 限流，请设置 GITHUB_TOKEN` | 未认证请求超限 | 设置 `GITHUB_TOKEN` 后重试 |
| `Wiki 仓库不存在或未开启` | 仓库没有 Wiki 功能 | 联系仓库作者开启 Wiki |
| `Wiki 页面不存在` | Wiki 中没有这个页面 | 检查页面标题拼写 |
