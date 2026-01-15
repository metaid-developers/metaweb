**中文** | [English](README.md)

# Meta App Service

MetaApp 索引服务 - 基于 MetaID 协议的去中心化应用索引与查询服务

## 功能特性

- 🔍 **区块链扫描** - 自动扫描 BTC/MVC 链并解析 MetaID 协议数据
- 📦 **应用索引** - 索引和存储 MetaApp 应用信息
- 🔎 **查询服务** - 提供 RESTful API 查询 MetaApp
- 📥 **应用部署** - 支持 MetaApp 的部署和下载
- 🌐 **Web 界面** - 提供友好的 Web 管理界面
- 📚 **API 文档** - 集成 Swagger 文档

## 快速开始

### 环境要求

- Go 1.24+
- PebbleDB (内置)
- BTC/MVC 节点 (RPC 访问)

### 安装

```bash
# 克隆仓库
git clone https://github.com/metaid-developers/meta-app-service.git
cd meta-app-service

# 安装依赖
go mod download

# 配置
cp conf/conf_example.yaml conf/conf_loc.yaml
# 编辑配置文件
vim conf/conf_loc.yaml

# 运行
go run cmd/indexer/main.go -env loc
```

### Docker 部署

```bash
docker-compose -f deploy/docker-compose.indexer.yml up -d
```

## Web 界面

启动服务后，访问 `http://localhost:7333` 即可查看和管理 MetaApp。

![Web 界面](static/image.png)

## API 文档

启动服务后访问：`/swagger/index.html`

## 配置说明

主要配置项：

- `indexer.port`: 服务端口
- `indexer.scan_interval`: 扫描间隔（秒）
- `database.data_dir`: 数据库目录
- `chain.rpc_url`: 区块链节点 RPC 地址

详细配置请参考 `conf/conf_example.yaml`

## 技术栈

- **语言**: Go 1.24+
- **框架**: Gin
- **数据库**: PebbleDB
- **协议**: MetaID Protocol
- **区块链**: BTC/MVC

## 许可证

Apache 2.0

