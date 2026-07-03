# 发布指南

本文档说明如何将 logcatool 发布到 Homebrew 和 APT 包管理器。

## 目录

- [Homebrew 发布](#homebrew-发布)
- [APT 发布](#apt-发布)
- [自动发布流程](#自动发布流程)

---

## Homebrew 发布

### 1. 创建 Homebrew Tap 仓库

1. 在 GitHub 创建名为 `homebrew-tap` 的仓库
2. 保持仓库为空（不要添加 README）

### 2. 配置 GitHub Secrets

在 `logcatool` 仓库设置中添加：

- `HOMEBREW_TAP_TOKEN`: GitHub Personal Access Token
  - 需要 `repo` 权限
  - 需要能访问 `homebrew-tap` 仓库

创建 PAT 步骤：
1. GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Generate new token
3. 勾选 `repo` 权限
4. 复制 token 添加到 Secrets

### 3. 发布新版本

```bash
# 1. 更新版本号（修改 main.go 中的 version 变量）
vim main.go

# 2. 提交更改
git add -A
git commit -m "bump version to 0.1.1"
git push

# 3. 打标签并推送（触发自动发布）
git tag v0.1.1
git push origin v0.1.1
```

### 4. 用户安装方式

```bash
# 添加 tap
brew tap simley/tap

# 安装
brew install logcatool

# 更新
brew update && brew upgrade logcatool
```

---

## APT 发布

### 方式一：自建 APT 仓库（推荐）

#### 1. 准备服务器

需要一台运行 Ubuntu/Debian 的服务器，并配置好 Nginx。

#### 2. 运行仓库初始化脚本

```bash
# 在服务器上运行
chmod +x scripts/setup-apt-repo.sh
sudo ./scripts/setup-apt-repo.sh
```

#### 3. 添加包到仓库

```bash
# 下载发布的 .deb 包
wget https://github.com/Yecangyuan/LogcatTool/releases/download/v0.1.0/logcatool_0.1.0_amd64.deb
wget https://github.com/Yecangyuan/LogcatTool/releases/download/v0.1.0/logcatool_0.1.0_arm64.deb

# 添加到仓库
reprepro -b /var/www/apt includedeb stable logcatool_0.1.0_amd64.deb
reprepro -b /var/www/apt includedeb stable logcatool_0.1.0_arm64.deb
```

#### 4. 用户安装方式

```bash
# 添加 GPG 密钥
wget -qO - https://apt.yourdomain.com/key.gpg | sudo apt-key add -

# 添加仓库
echo "deb https://apt.yourdomain.com stable main" | sudo tee /etc/apt/sources.list.d/logcatool.list

# 安装
sudo apt update
sudo apt install logcatool
```

### 方式二：Ubuntu PPA（Launchpad）

这种方式需要将源码包上传到 Launchpad，由 Launchpad 自动构建。

#### 1. 注册 Launchpad 账号并配置 GPG

```bash
# 安装必要工具
sudo apt install devscripts dput

# 生成 GPG 密钥（如果还没有）
gpg --gen-key

# 上传公钥到 Ubuntu 密钥服务器
gpg --send-keys --keyserver keyserver.ubuntu.com YOUR_KEY_ID
```

#### 2. 创建 Debian 源码包

```bash
# 创建必要的文件
debmake -p logcatool -u 0.1.0 -r 1

# 或者手动创建 debian/ 目录结构
mkdir -p debian
```

#### 3. 配置 debian/control

```
Source: logcatool
Section: utils
Priority: optional
Maintainer: simley <your-email@example.com>
Build-Depends: debhelper (>= 11), golang-go (>= 1.25)
Standards-Version: 4.1.4
Homepage: https://github.com/Yecangyuan/LogcatTool

Package: logcatool
Architecture: any
Depends: ${shlibs:Depends}, ${misc:Depends}
Description: 终端版 Android Logcat 查看器
 基于 Bubble Tea 构建的终端版 Android Logcat 查看器，
 支持实时日志流、日志文件读取、搜索过滤等功能。
```

#### 4. 创建 debian/rules

```makefile
#!/usr/bin/make -f

export DH_GOPKG := github.com/Yecangyuan/LogcatTool
export GO111MODULE := on

%:
	dh $@ --buildsystem=golang --builddirectory=_build

override_dh_auto_build:
	go build -ldflags "-s -w" -o logcatool

override_dh_auto_install:
	install -D -m 0755 logcatool debian/logcatool/usr/bin/logcatool
```

#### 5. 构建并上传

```bash
# 构建源码包
dpkg-buildpackage -S -sa

# 签名
debsign -k YOUR_KEY_ID ../logcatool_0.1.0-1_source.changes

# 上传到 PPA
dput ppa:simley/ppa ../logcatool_0.1.0-1_source.changes
```

#### 6. 用户安装方式

```bash
sudo add-apt-repository ppa:simley/ppa
sudo apt update
sudo apt install logcatool
```

---

## 自动发布流程

### GitHub Actions 工作流说明

项目已配置两个 GitHub Actions 工作流：

#### 1. `.github/workflows/release.yml`

触发条件：推送 `v*` 标签

功能：
- 编译多平台二进制文件（macOS/Linux，AMD64/ARM64）
- 打包为 tar.gz
- 创建 GitHub Release
- 自动更新 Homebrew Formula

#### 2. `.github/workflows/build-deb.yml`

触发条件：推送 `v*` 标签

功能：
- 构建 Debian 包（amd64/arm64）
- 上传到 GitHub Release

### 发布流程

```bash
# 1. 确保代码已提交
git status

# 2. 更新版本号（main.go 中的 version 变量）
# 编辑 main.go

# 3. 提交
git add main.go
git commit -m "release: bump version to 0.1.1"

# 4. 打标签（这会触发自动发布）
git tag v0.1.1
git push origin v0.1.1
```

### 发布检查清单

- [ ] 版本号已更新（main.go）
- [ ] 代码已提交并推送
- [ ] 标签格式正确（v*）
- [ ] GitHub Secrets 已配置
- [ ] Homebrew Tap 仓库已创建
- [ ] 发布成功后测试安装

---

## Windows 7 专用分支

`go-win7` 分支用于构建能在 Windows 7 SP1 上运行的二进制文件。该分支使用 [XTLS/go-win7](https://github.com/XTLS/go-win7) 打过补丁的 Go SDK 进行交叉编译，产物为 `logcatool_<version>_windows_amd64.zip`。

### 使用条件

- Windows 7 SP1
- 必须安装 Convenience Rollup [KB3125574](https://support.microsoft.com/kb/3125574)

### 发布产物

Windows 7 构建通过 `.github/workflows/release.yml` 中的 `build-windows7` job 生成，并随 Release 一起发布。Homebrew 更新任务保持不变，仅包含 macOS / Linux 平台。

### PowerShell 安装脚本

`go-win7` 分支提供 `install.ps1`，支持一键安装：

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force
Invoke-RestMethod -Uri https://raw.githubusercontent.com/Yecangyuan/LogcatTool/go-win7/install.ps1 | Invoke-Expression
```

### 终端建议

Windows 7 默认 `cmd.exe` 对 ANSI/VT 序列支持有限，TUI 可能出现渲染问题。建议用户配合 **ConEmu**、**Cmder** 或 **Git Bash** 使用。

---

## 其他包管理器

### Snap（Ubuntu 官方商店）

创建 `snap/snapcraft.yaml`：

```yaml
name: logcatool
version: '0.1.0'
summary: 终端版 Android Logcat 查看器
description: |
  基于 Bubble Tea 构建的终端版 Android Logcat 查看器

grade: stable
confinement: strict

parts:
  logcatool:
    plugin: go
    source: .
    source-type: git
    build-packages:
      - golang-go

apps:
  logcatool:
    command: bin/logcatool
    plugs:
      - home
      - removable-media
```

发布：
```bash
snapcraft
snapcraft upload --release=stable logcatool_0.1.0_amd64.snap
```

### Scoop（Windows）

`go-win7` 分支发布的 `logcatool_<version>_windows_amd64.zip` 可直接用于 Scoop manifest。创建 `scoop/logcatool.json`：

```json
{
  "version": "0.1.0",
  "description": "终端版 Android Logcat 查看器",
  "homepage": "https://github.com/Yecangyuan/LogcatTool",
  "license": "MIT",
  "architecture": {
    "64bit": {
      "url": "https://github.com/Yecangyuan/LogcatTool/releases/download/v0.1.0/logcatool_v0.1.0_windows_amd64.zip",
      "hash": "sha256_hash_here"
    }
  },
  "bin": "logcatool.exe"
}
```

获取 `sha256_hash_here`：

```bash
curl -sL -o logcatool_v0.1.0_windows_amd64.zip \
  https://github.com/Yecangyuan/LogcatTool/releases/download/v0.1.0/logcatool_v0.1.0_windows_amd64.zip
shasum -a 256 logcatool_v0.1.0_windows_amd64.zip
```

注意：Windows 7 用户需满足 [KB3125574](https://support.microsoft.com/kb/3125574) 前提。Windows 10/11 用户可直接使用标准版本。

---

## 故障排除

### Homebrew 问题

**Q: Formula 更新后用户无法安装？**
A: 检查 SHA256 是否正确，确保 URL 可访问

**Q: 如何测试 Formula？**
```bash
brew install --build-from-source ./Formula/logcatool.rb
brew test logcatool
```

### APT 问题

**Q: 添加仓库时出现 GPG 错误？**
A: 确保 GPG 公钥已正确导出并上传到服务器

**Q: 包依赖问题？**
A: 检查 `debian/control` 中的 `Depends` 字段

---

## 参考文档

- [Homebrew Formula Cookbook](https://docs.brew.sh/Formula-Cookbook)
- [Debian New Maintainers' Guide](https://www.debian.org/doc/manuals/maint-guide/)
- [Ubuntu PPA 文档](https://help.launchpad.net/Packaging/PPA)
