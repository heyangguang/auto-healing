# iTop 适配器联调说明

这套环境的边界是：

- `iTop` 负责保存真实工单和资产
- `iTop` 站点内的页面负责造测试数据
- `外部适配器` 负责把 iTop REST 翻译成 AHS 标准 JSON
- `AHS` 只注册通用 `itsm` / `cmdb` 插件，不内置任何 iTop 特判

## 文件

- Compose: [deployments/docker/docker-compose.itop.yml](/root/auto-healing/deployments/docker/docker-compose.itop.yml)
- 环境变量示例: [deployments/docker/itop.env.example](/root/auto-healing/deployments/docker/itop.env.example)
- 适配器脚本: [tools/itop_adapter.py](/root/auto-healing/tools/itop_adapter.py)
- iTop 内部工单生成页: [deployments/docker/itop-ticket-lab.html](/root/auto-healing/deployments/docker/itop-ticket-lab.html)

## 当前本地实验环境

- iTop UI: `http://127.0.0.1:18084`
- iTop 生成页: `http://127.0.0.1:18084/extensions/itop-ticket-lab.html`
- iTop CMDB 页: `http://127.0.0.1:18084/extensions/itop-cmdb-lab.html`
- 适配器: `http://127.0.0.1:18085`
- iTop 管理员: `admin / admin`

这些本地凭据只保存在 `deployments/docker/.env.itop`，不会提交到仓库。

## 启动

```bash
cd /root/auto-healing/deployments/docker
docker compose --env-file .env.itop -f docker-compose.itop.yml up -d
```

查看状态：

```bash
docker compose --env-file .env.itop -f docker-compose.itop.yml ps
```

查看日志：

```bash
docker compose --env-file .env.itop -f docker-compose.itop.yml logs -f itop
docker compose --env-file .env.itop -f docker-compose.itop.yml logs -f itop-adapter
```

## 验证 iTop REST

```bash
curl -sS -u admin:admin \
  -X POST 'http://127.0.0.1:18084/webservices/rest.php?version=1.3' \
  --data-urlencode 'json_data={"operation":"list_operations"}' | jq
```

## 验证适配器

工单：

```bash
curl -sS http://127.0.0.1:18085/api/incidents | jq '.[0]'
```

资产：

```bash
curl -sS http://127.0.0.1:18085/api/cmdb-items | jq '.[0]'
```

深度健康检查：

```bash
curl -sS http://127.0.0.1:18085/health/deep | jq
```

## 在 AHS 中注册正确的插件

工单插件配置：

```json
{
  "name": "iTop Adapter ITSM",
  "type": "itsm",
  "config": {
    "url": "http://127.0.0.1:18085/api/incidents",
    "auth_type": "none",
    "close_incident_url": "http://127.0.0.1:18085/api/incidents/{external_id}/close",
    "close_incident_method": "POST"
  },
  "field_mapping": {},
  "sync_enabled": false,
  "sync_interval_minutes": 5
}
```

资产插件配置：

```json
{
  "name": "iTop Adapter CMDB",
  "type": "cmdb",
  "config": {
    "url": "http://127.0.0.1:18085/api/cmdb-items",
    "auth_type": "none"
  },
  "field_mapping": {},
  "sync_enabled": false,
  "sync_interval_minutes": 5
}
```

这两类插件都指向适配器，不应该直连 `webservices/rest.php`。

## iTop 内部工单生成页

浏览器访问：

```text
http://127.0.0.1:18084/extensions/itop-ticket-lab.html
```

这个页面直接在 iTop 站点里运行，提交时调用 iTop 自己的 REST `core/create`。

当前页面会写入这些真实字段：

- `agent_id`
- `service_id`
- `servicesubcategory_id`
- `functionalcis_list`

所以 `指派人 / 影响服务 / 影响 CI` 都由 iTop 自己落字段，再由适配器原样翻译给 AHS。

## iTop 内部 CMDB 主机模拟页

浏览器访问：

```text
http://127.0.0.1:18084/extensions/itop-cmdb-lab.html
```

这个页面会直接在 iTop 里创建 `Server` 资产，核心字段包括：

- `name`
- `managementip`
- `serialnumber`
- `cpu`
- `ram`
- `org_id`
- `location_id`
- `brand_id`
- `model_id`
- `osfamily_id`
- `osversion_id`

这些字段都会被外部适配器翻译成 AHS 资产页能直接显示的标准字段。
