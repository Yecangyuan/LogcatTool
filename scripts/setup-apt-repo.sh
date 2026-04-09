#!/bin/bash
# APT 仓库设置脚本 - 在服务器上运行

set -e

REPO_DIR="/var/www/apt"
GPG_EMAIL="your-email@example.com"
GPG_NAME="simley"

# 安装依赖
apt-get update
apt-get install -y reprepro gnupg nginx

# 创建仓库目录结构
mkdir -p ${REPO_DIR}/{conf,dists,pool}

# 生成 GPG 密钥（如果不存在）
if ! gpg --list-keys | grep -q "${GPG_EMAIL}"; then
    cat > /tmp/gpg-batch << EOF
%echo Generating GPG key
Key-Type: RSA
Key-Length: 4096
Name-Real: ${GPG_NAME}
Name-Email: ${GPG_EMAIL}
Expire-Date: 0
%no-protection
%commit
%echo done
EOF
    gpg --batch --gen-key /tmp/gpg-batch
    rm /tmp/gpg-batch
fi

GPG_KEY=$(gpg --list-keys --with-colons | grep fpr | head -1 | cut -d: -f10)

# 导出公钥
gpg --armor --export ${GPG_KEY} > ${REPO_DIR}/key.gpg

# 配置仓库
cat > ${REPO_DIR}/conf/distributions << EOF
Origin: simley
Label: Logcatool Repository
Codename: stable
Architectures: amd64 arm64
Components: main
Description: APT repository for logcatool
SignWith: ${GPG_KEY}
EOF

# 配置 Nginx
cat > /etc/nginx/sites-available/apt << EOF
server {
    listen 80;
    server_name apt.yourdomain.com;
    root ${REPO_DIR};
    autoindex on;
    
    location ~ /(.*)/conf {
        deny all;
    }
}
EOF

ln -sf /etc/nginx/sites-available/apt /etc/nginx/sites-enabled/
nginx -t && systemctl restart nginx

echo "APT 仓库已设置完成！"
echo "GPG 公钥: ${REPO_DIR}/key.gpg"
echo "添加包的命令: reprepro -b ${REPO_DIR} includedeb stable <package.deb>"
