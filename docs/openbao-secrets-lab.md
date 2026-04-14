# OpenBao 密钥联调说明

这套环境用于给 AHS 的 `vault` 类型密钥源提供一个本地可跑、可验证的品牌化开源后端。

## 文件

- Compose: [deployments/docker/docker-compose.openbao.yml](/root/auto-healing/deployments/docker/docker-compose.openbao.yml)
- 环境变量示例: [deployments/docker/openbao.env.example](/root/auto-healing/deployments/docker/openbao.env.example)
- 种子脚本: [tools/seed_openbao_lab.py](/root/auto-healing/tools/seed_openbao_lab.py)

## 启动

```bash
cd /root/auto-healing/deployments/docker
cp openbao.env.example .env.openbao
docker compose --env-file .env.openbao -f docker-compose.openbao.yml up -d
```

访问地址：

- OpenBao API: `http://127.0.0.1:18200`
- Root Token: `root`

## 预置实验数据

```bash
cd /root/auto-healing
OPENBAO_ADDR=http://127.0.0.1:18200 \
OPENBAO_ROOT_TOKEN=root \
python3 tools/seed_openbao_lab.py
```

脚本会把下面这些主机写到 `secret/data/hosts/<hostname>`：

- `Server1`
- `Server2`
- `Server3`
- `Server4`
- `cmdb-page-232605`

返回结构是：

```json
{
  "data": {
    "username": "ops",
    "password": "Server4Pass!2026"
  }
}
```

## 在 AHS 中创建密钥源

`vault` 类型密钥源配置示例：

```json
{
  "name": "OpenBao Password Lab",
  "type": "vault",
  "auth_type": "password",
  "config": {
    "address": "http://127.0.0.1:18200",
    "secret_path": "secret/data/hosts",
    "query_key": "hostname",
    "auth": {
      "type": "token",
      "token": "root"
    },
    "field_mapping": {
      "username": "username",
      "password": "password"
    }
  },
  "is_default": true,
  "priority": 1
}
```

## 验证

测试连接：

```bash
curl -sS http://127.0.0.1:18200/v1/sys/health | jq
```

读取某台主机的密钥：

```bash
curl -sS \
  -H 'X-Vault-Token: root' \
  http://127.0.0.1:18200/v1/secret/data/hosts/Server4 | jq
```
