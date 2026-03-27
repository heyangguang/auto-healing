package database

import (
	"time"

	engagementmodel "github.com/company/auto-healing/internal/modules/engagement/model"
)

func buildSeedSiteMessages(expiresAt time.Time) []engagementmodel.SiteMessage {
	return []engagementmodel.SiteMessage{
		{Category: engagementmodel.SiteMessageCategorySystemUpdate, Title: "平台 v3.2.0 版本发布公告", Content: "<h3>🚀 v3.2.0 版本更新</h3><p>本次更新包含以下内容：</p><ul><li>新增站内信功能，支持消息分类和已读管理</li><li>优化工作台性能，加载速度提升 40%</li><li>修复自愈流程在并发场景下的稳定性问题</li></ul><p>详情请查看 <b>更新日志</b>。</p>", CreatedAt: time.Now().Add(-2 * time.Hour), ExpiresAt: &expiresAt},
		{Category: engagementmodel.SiteMessageCategorySystemUpdate, Title: "系统维护通知：数据库升级", Content: "<p>尊敬的用户，我们将于 <b>2026年2月20日 02:00-04:00</b> 进行数据库升级维护。</p><p>维护期间系统将<span style='color:red'>暂时不可用</span>，请提前做好准备。</p><p>预计维护时长：2小时。感谢您的理解与支持。</p>", CreatedAt: time.Now().Add(-24 * time.Hour), ExpiresAt: &expiresAt},
		{Category: engagementmodel.SiteMessageCategorySystemUpdate, Title: "API 接口变更通知", Content: "<p>以下接口将在 v3.3.0 中废弃：</p><ul><li><code>GET /api/v1/workflows</code> → 请迁移至 <code>GET /api/v1/healing/flows</code></li><li><code>POST /api/v1/execute</code> → 请迁移至 <code>POST /api/v1/execution-tasks/:id/execute</code></li></ul><p>请及时调整您的集成脚本。</p>", CreatedAt: time.Now().Add(-72 * time.Hour), ExpiresAt: &expiresAt},
		{Category: engagementmodel.SiteMessageCategoryFaultAlert, Title: "生产环境数据库连接异常告警", Content: "<p>⚠️ <b>告警级别：严重</b></p><p>检测到生产环境数据库连接池使用率达到 <b>95%</b>，可能导致请求超时。</p><p>建议操作：</p><ol><li>检查慢查询日志</li><li>考虑扩容连接池配置</li><li>排查是否有未释放的连接</li></ol>", CreatedAt: time.Now().Add(-30 * time.Minute), ExpiresAt: &expiresAt},
		{Category: engagementmodel.SiteMessageCategoryFaultAlert, Title: "节点 worker-03 离线通知", Content: "<p>工作节点 <code>worker-03</code> 于 <b>2026-02-17 23:45:00</b> 失去心跳。</p><p>影响范围：该节点上运行的 3 个定时任务已自动迁移至其他节点。</p><p>请尽快排查节点状态。</p>", CreatedAt: time.Now().Add(-6 * time.Hour), ExpiresAt: &expiresAt},
		{Category: engagementmodel.SiteMessageCategoryServiceNotice, Title: "您的定时任务执行报告", Content: "<h3>📊 每日执行报告</h3><p>2026-02-17 执行统计：</p><table border='1' cellpadding='6'><tr><th>指标</th><th>数值</th></tr><tr><td>总执行次数</td><td>128</td></tr><tr><td>成功率</td><td>97.6%</td></tr><tr><td>平均耗时</td><td>45s</td></tr><tr><td>失败任务</td><td>3</td></tr></table>", CreatedAt: time.Now().Add(-12 * time.Hour), ExpiresAt: &expiresAt},
		{Category: engagementmodel.SiteMessageCategoryServiceNotice, Title: "自愈流程触发通知", Content: "<p>自愈规则 <b>「磁盘空间不足自动清理」</b> 已触发执行。</p><p>触发工单：<code>INC-2026021700042</code></p><p>执行结果：✅ 成功清理 12.3 GB 临时文件</p>", CreatedAt: time.Now().Add(-4 * time.Hour), ExpiresAt: &expiresAt},
		{Category: engagementmodel.SiteMessageCategoryServiceNotice, Title: "密钥即将过期提醒", Content: "<p>以下凭据将在 <b>7天内</b> 过期，请及时更新：</p><ul><li>SSH Key: <code>prod-deploy-key</code> — 过期时间：2026-02-25</li><li>API Token: <code>monitoring-token</code> — 过期时间：2026-02-24</li></ul>", CreatedAt: time.Now().Add(-48 * time.Hour), ExpiresAt: &expiresAt},
		{Category: engagementmodel.SiteMessageCategoryProductNews, Title: "新功能上线：可视化流程编辑器", Content: "<h3>🎉 全新可视化流程编辑器</h3><p>现在您可以通过<b>拖拽方式</b>构建自愈流程，无需编写复杂配置。</p><p>主要特性：</p><ul><li>支持条件分支和并行节点</li><li>实时预览和模拟执行</li><li>一键导入/导出流程模板</li></ul><p>前往 <b>自愈流程</b> 页面体验 →</p>", CreatedAt: time.Now().Add(-36 * time.Hour), ExpiresAt: &expiresAt},
		{Category: engagementmodel.SiteMessageCategoryProductNews, Title: "产品路线图更新 - 2026 Q1", Content: "<p>2026年第一季度产品规划：</p><ol><li><b>智能告警聚合</b> — 自动合并相似告警，减少噪音</li><li><b>变更风险评估</b> — 基于历史数据评估变更风险等级</li><li><b>多云管理</b> — 支持 AWS/阿里云/腾讯云统一管理</li></ol><p>欢迎在产品反馈群提出建议！</p>", CreatedAt: time.Now().Add(-120 * time.Hour), ExpiresAt: &expiresAt},
		{Category: engagementmodel.SiteMessageCategoryActivity, Title: "运维技术分享会邀请", Content: "<h3>📅 技术分享会</h3><p><b>主题</b>：「从告警到自愈 — 智能运维实践分享」</p><p><b>时间</b>：2026年2月28日 14:00-16:00</p><p><b>地点</b>：线上腾讯会议</p><p><b>讲师</b>：DevOps 团队 张工</p><p>现场将抽取<b>3名幸运观众</b>赠送技术书籍！</p>", CreatedAt: time.Now().Add(-8 * time.Hour), ExpiresAt: &expiresAt},
		{Category: engagementmodel.SiteMessageCategoryActivity, Title: "平台满意度调研", Content: "<p>尊敬的用户，为了提升平台体验，诚邀您参与<b>2分钟快速调研</b>。</p><p>您的反馈将直接影响我们的产品改进方向。</p><p>参与即有机会获得平台<b>高级功能30天试用权益</b>。</p>", CreatedAt: time.Now().Add(-96 * time.Hour), ExpiresAt: &expiresAt},
		{Category: engagementmodel.SiteMessageCategorySecurity, Title: "安全公告：请立即更新访问密钥", Content: "<p>🔒 <b>安全等级：高</b></p><p>我们检测到部分用户的 API 访问密钥可能存在泄露风险。</p><p><b>建议操作</b>：</p><ol><li>立即前往「密钥管理」页面轮换您的 API Key</li><li>检查最近 7 天的异常 API 调用记录</li><li>启用双因子认证（2FA）保护账号安全</li></ol>", CreatedAt: time.Now().Add(-1 * time.Hour), ExpiresAt: &expiresAt},
		{Category: engagementmodel.SiteMessageCategorySecurity, Title: "安全策略更新：密码复杂度要求提升", Content: "<p>为提升平台安全性，自 <b>2026年3月1日</b> 起，密码策略将更新为：</p><ul><li>最少 12 位字符</li><li>必须包含大小写字母、数字和特殊符号</li><li>禁止使用最近 5 次历史密码</li></ul><p>不符合新策略的账户将在下次登录时被要求修改密码。</p>", CreatedAt: time.Now().Add(-168 * time.Hour), ExpiresAt: &expiresAt},
		{Category: engagementmodel.SiteMessageCategorySecurity, Title: "异常登录提醒", Content: "<p>您的账号在 <b>2026-02-17 22:15:00</b> 从新设备登录：</p><ul><li>IP 地址：<code>203.0.113.42</code></li><li>位置：上海市</li><li>设备：Chrome 120 / macOS</li></ul><p>如非本人操作，请立即修改密码并联系管理员。</p>", CreatedAt: time.Now().Add(-3 * time.Hour), ExpiresAt: &expiresAt},
	}
}
