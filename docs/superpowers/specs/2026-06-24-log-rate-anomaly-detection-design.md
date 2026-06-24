# 设计文档：多维度日志速率异常检测

**日期**：2026-06-24  
**主题**：为 LogcatTool 增加日志速率异常检测（Log Rate Anomaly Detection）  
**状态**：待实现评审

---

## 1. 目标

为 LogcatTool 增加实时、多维度的日志速率异常检测能力：当某个维度（全局、级别、Tag、PID、包名、进程名）的日志流量相对于历史基线出现显著突增/突降时，立即在状态栏、独立面板和日志行内给出提示，并允许用户一键应用对应过滤，快速定位异常来源。

## 2. 非目标

- 不做复杂的根因推断或 AI 摘要（属于后续“分析 & 智能”方向的其他特性）。
- 不做跨文件/跨设备对比。
- 第一版不实现 Z-score 策略，只预留策略接口。

## 3. 功能范围

### 3.1 监控维度

| 维度 | 键示例 | 说明 |
|------|--------|------|
| `global` | `global` | 全部日志聚合 |
| `level` | `E`, `W`, `I` … | 按日志级别聚合 |
| `tag` | `NetworkManager` | 按 Tag 聚合 |
| `pid` | `1234` | 按 PID 聚合 |
| `package` | `com.example.app` | 通过 PID→包名映射聚合 |
| `process` | `com.example.app:remote` | 按进程名聚合 |

### 3.2 异常判定

第一版实现 **移动平均倍数策略**，同时支持突增和突降：

```
ratio = recentRate / baselineRate

spike 触发条件：
  recentRate > minBaseline
  AND baselineRate > 0
  AND ratio >= multiplier

drop 触发条件（dropMultiplier > 0 时）：
  baselineRate > minBaseline
  AND recentRate > 0
  AND ratio <= dropMultiplier
```

- `recentRate`：最近 `recentWindowSec` 秒的平均每秒日志数。
- `baselineRate`：过去 `baselineWindowSec` 秒（不含最近窗口）的平均每秒日志数。
- `minBaseline`：基线最小阈值，防止零星日志导致误报。
- `multiplier`：突增判定倍数（默认 3.0）。
- `dropMultiplier`：突降判定倍数（默认 0.0，表示不检测突降）。

同一维度在 `cooldownSec` 内重复触发只更新最后时间，不新增异常条目。

预留 `DetectionStrategy` 接口，后续可接入 Z-score、EWMA 等策略。

### 3.3 展示方式

1. **状态栏**：检测到异常时显示 🔺 及最严重项，例如 `🔺 Tag=Network 12x @ 14:32:05`。
2. **异常面板**：按 `Y` 打开/关闭，列出所有活跃异常：维度、当前速率、基线速率、倍数、触发时间。
3. **行内高亮**：异常触发时间前后 5 秒内的日志行使用 theme-aware 浅色背景，行前缀显示 `⚠`。
4. **自动提示**：首次检测到新异常时状态栏闪烁 2 秒，不强制弹窗。

### 3.4 交互

| 场景 | 操作 |
|------|------|
| 打开/关闭异常面板 | `Y` |
| 面板内移动 | `j` / `k` |
| 应用选中异常对应的过滤 | `Enter` |
| 清空历史异常 | `c` |
| 查看面板帮助 | `?` |
| 关闭面板 | `Esc` |

## 4. 架构

新增 `internal/anomaly` 包负责纯检测逻辑，现有 `internal/model` 负责把检测事件接入 UI。

```
internal/anomaly/
├── types.go      # Dimension, AnomalyEvent, AnomalyConfig
├── series.go     # TimeSeries 固定长度秒级桶
├── strategy.go   # DetectionStrategy 接口 + MovingAverageStrategy
├── detector.go   # 维护各维度时间序列，周期性评估
internal/model/
├── anomaly.go    # 模型中的异常状态（列表、面板开关、高亮窗口）
internal/ui/
├── anomaly_panel.go  # 异常列表渲染
internal/config/
└── config.go     # 扩展持久化配置
```

## 5. 数据流

1. 新日志进入 `model.Update`。
2. `Update` 调用 `detector.Record(entry)`，传入日志的时间戳和维度属性。
3. `detector` 更新全局及各维度对应的时间序列桶。
4. 每秒 `detector.Evaluate()` 调用策略，产出零个或多个 `AnomalyEvent`。
5. model 收到事件后更新异常列表，触发 UI 重绘。
6. 用户在面板中按 `Enter` 时，model 把维度转换为现有 filter（Tag/PID/Package/Level 等），复用已有过滤逻辑。

## 6. 关键设计决策

- **时间基准**：按日志自身时间戳分桶，而不是墙钟。实时模式、暂停恢复、文件回放行为一致。
- **聚合粒度**：1 秒 1 个桶。
- **时间序列长度**：默认保存 `baselineWindowSec + recentWindowSec` 秒，即 330 个桶。
- **维度数量控制**：Tag/PID/Package/Process 维度使用 LRU 淘汰，每个维度最多保留 1000 个活跃键。
- **Package 动态映射**：通过现有 PID→Package 映射实时解析；PID 重新映射后，后续日志归属到新 Package，旧键按 LRU 淘汰。
- **暂停模式**：暂停时 detector 不接收新日志，恢复后按日志时间戳正常分桶，不会产生误报。
- **检测器生命周期**：`model.New` 创建 detector 并启动后台 goroutine，每秒评估一次；程序退出时通过 `tea.Program` 生命周期或 `model` 析构调用 `detector.Stop()` 停止 goroutine。

## 7. 配置

默认配置：

```json
{
  "anomaly": {
    "enabled": true,
    "recentWindowSec": 30,
    "baselineWindowSec": 300,
    "multiplier": 3.0,
    "dropMultiplier": 0.0,
    "minBaseline": 5,
    "highlightWindowSec": 5,
    "maxKeysPerDimension": 1000,
    "cooldownSec": 30,
    "strategy": "moving_average",
    "dimensions": {
      "global": { "enabled": true },
      "level": { "enabled": true, "multiplier": 2.0 },
      "tag": { "enabled": true },
      "pid": { "enabled": true },
      "package": { "enabled": true },
      "process": { "enabled": false }
    }
  }
}
```

- 全局字段可被各维度覆盖。
- 禁用的维度不维护时间序列。
- 配置持久化到 `~/.config/logcatool/config.json`，与现有配置合并。

## 8. 错误处理

- **策略 panic 隔离**：`detector.Evaluate` 用 `recover` 捕获策略 panic，只记录 debug 日志，不中断日志流。
- **零基线保护**：基线为 0 或低于 `minBaseline` 时不触发异常。
- **配置降级**：解析 `anomaly` 字段失败时回退到默认配置，并打印一次警告。
- **无效维度**：遇到未识别维度键时忽略该覆盖配置。

## 9. 测试策略

### 9.1 单元测试

- `MovingAverageStrategy`：正常突增、正常突降、平稳流量、基线为 0、recent 为 0、recent > baseline、dropMultiplier 关闭等边界。
- `TimeSeries`：桶覆盖、秒级聚合、固定长度淘汰。
- 配置解析：默认值、维度覆盖、无效字段降级。

### 9.2 集成测试

- 模拟 1000 条/秒日志流，注入 10 倍突增，验证 detector 在 1-2 秒内产出 `AnomalyEvent`。
- 验证 Pause/Resume 不触发误报。
- 验证 Package 维度在 PID 映射变化后正确归属。

### 9.3 模型/UI 测试

- 异常事件更新 model 状态后，面板渲染和过滤映射正确。
- 行内高亮时间窗口计算正确。

### 9.4 基准测试

- 在 10 万行/秒输入下评估 detector CPU 占用，目标为不显著影响主循环帧率。

## 10. 实现阶段建议

虽然本设计按“完整版一步到位”定义，但实现时可按以下顺序降低风险：

1. `internal/anomaly` 核心包（types/series/strategy/detector）+ 单元测试。
2. model 接入 + 状态栏提示。
3. 异常面板 UI。
4. 行内高亮。
5. 配置持久化 + 端到端测试。

## 11. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 维度过多导致内存上涨 | LRU 限制每个维度 1000 键；禁用维度不分配 |
| 检测计算影响主循环 | detector 在独立 goroutine 中运行，通过 channel 向 model 发事件 |
| 暂停恢复误报 | 按日志时间戳分桶 |
| 配置升级冲突 | 新增 `anomaly` 顶层字段，与旧配置兼容 |

## 12. 验收标准

- [ ] 全局/Level/Tag/PID/Package/Process 任一维度触发异常时，状态栏显示 🔺。
- [ ] 按 `Y` 可打开面板，列出异常详情。
- [ ] 面板中按 `Enter` 能正确应用对应过滤。
- [ ] 行内高亮在异常时间窗口内生效。
- [ ] 配置修改后重启自动恢复。
- [ ] 单元测试覆盖率 ≥ 80%，核心检测路径 100%。
- [ ] 10 万行/秒下 UI 不卡顿。
