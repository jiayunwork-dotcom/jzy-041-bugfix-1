# beziersvc — 平面三次 Bézier 曲线几何内核

一个只啃**平面三次 Bézier 曲线**的小服务：上游丢进来四个控制点，服务返回三样东西——

1. **弧长**：对速度模长 `|r'(t)|` 在 `[0,1]` 上做**自适应数值积分**（不用控制多边形周长凑数），按请求容差逐段加密直到收敛，并如实返回最终分段数与收敛残差；
2. **曲率**：闭式公式 `κ(t) = |x'y'' − y'x''| / |r'(t)|³`（平面叉积是标量，分母是速度模长的三次方）；
3. **法向等距偏移折线**：`r(t) + d·n(t)`，单位法向 `n` 由单位切向逆时针转 90° 得到。

唯一的调用入口是 HTTP。矢量画板、凸轮升程、齿轮啮合线一概不做。

## 构建与运行

```bash
# 本地（Go 1.22）
go build ./... && go test -race ./...
go run ./cmd/server            # 监听 :8080

# Docker（一键构建启动，构建基于 golang:1.22-alpine）
docker build -t beziersvc .
docker run --rm -p 8080:8080 beziersvc
```

环境变量：`PORT`（默认 8080）、`BEZIER_DEFAULT_TOLERANCE`（默认 1e-9）、
`BEZIER_MAX_REFINEMENT_DEPTH`（默认 20，即加密上限）。

## 数学约定

曲线用 Bernstein 基表示，参数 `t ∈ [0,1]`：

```
r(t) = (1−t)³ P0 + 3(1−t)²t P1 + 3(1−t)t² P2 + t³ P3
```

- **弧长** `L = ∫₀¹ |r'(t)| dt`：自适应 Simpson 积分。每个子区间二分一次后，
  若积分值的变化不超过该区间分到的容差份额则接受（并做 Richardson 外推），
  否则继续二分；容差份额随二分逐层减半，因此收敛时**残差总和 ≤ 请求容差**。
  响应里的 `segments` 是 `[0,1]` 最终被切成的段数，`residual` 是各段最后一档
  加密带来的变化量之和，`converged=false` 只会在触及加密上限时出现。
- **曲率** `κ(t) = |r'(t) × r''(t)| / |r'(t)|³`，平面叉积即标量 `x'y'' − y'x''`。
- **偏移** `q(t) = r(t) + d·n(t)`，`n = (−t_y, t_x)` 为单位切向旋转 +90°；
  `d < 0` 偏移到另一侧。折线在参数域均匀采样 `segments+1` 个点。

## API（前缀 `/api/v1`）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/arc-length` | 单条弧长：`{points, tolerance?}` |
| POST | `/curvature` | 单条曲率：`{points, t}` 或 `{points, ts:[...]}` |
| POST | `/offset` | 单条偏移折线：`{points, distance, segments?}` |
| POST | `/evaluate` | 三件套：`{points, tolerance?, curvature_t?, offset?}` |
| POST | `/batch` | 批量：`{curves:[{id?, points, tolerance?, curvature_t?, offset?}]}`，单条失败不拖累其他 |
| GET | `/config` | 只读：默认容差、加密上限等配置回显 + 运行状态 |
| GET | `/demo` | 预置弓形算例（弧长恰为 2，弦长恰为 1） |

`points` 为 4 个 `{"x":…,"y":…}`。示例：

```bash
curl -X POST localhost:8080/api/v1/evaluate -H 'Content-Type: application/json' -d '{
  "points": [{"x":0,"y":0},{"x":0,"y":1},{"x":1,"y":1},{"x":1,"y":0}],
  "tolerance": 1e-12,
  "curvature_t": [0.25, 0.5, 0.75],
  "offset": {"distance": 0.1, "segments": 32}
}'
```

弧长响应：

```json
{"length": 2, "segments": 8, "residual": 0, "converged": true, "tolerance": 1e-12}
```

## 错误模型

所有失败都是**带类型的结构化 JSON**，不抛异常、不给空响应：

```json
{"error": {"type": "invalid_input", "message": "…", "field": "points[2].y"}}
```

| type | HTTP | 含义 |
|---|---|---|
| `invalid_input` | 400 | 控制点不是 4 个、坐标缺失/非数值/超界、容差非正、t 越出 [0,1] 等 |
| `singular` | 422 | 输入合法但所求量无定义：尖点（速度为零）处求曲率或法向偏移 |
| `not_found` / `method_not_allowed` | 404 / 405 | 路由/方法错误 |
| `internal` | 500 | 兜底（正常不会触发） |

注意区分：**共线不是错误**（曲率恒为零是合法结果，偏移是平行直线）；
只有速度为零的尖点求曲率/法向才报 `singular`，且绝不返回 NaN/Inf。

批量接口中每条曲线独立走与 `/evaluate` **完全相同的**计算管线，
某条非法只让那一条返回 `error`，其余照常。

## 代码结构

```
cmd/server/          进程入口（仅装配 HTTP 服务）
internal/bernstein/  Bernstein 求值与一阶/二阶导数
internal/arclength/  自适应弧长积分（与 HTTP 无关的纯数值包）
internal/geometry/   曲率、单位切向/法向、等距偏移
internal/validate/   输入校验与类型化错误
internal/api/        Gin 路由、请求/响应 DTO、批量与单条共用管线
```

## 测试

`go test -race ./...` 覆盖（对应验收清单）：

- 控制点平移后弧长与曲率不变；缩放 k 使弧长 ×|k|、曲率 ×1/|k|
- d=0 时偏移折线与原曲线逐点重合
- 共线时曲率恒为零、偏移为平行直线且距离恰为 |d|
- 弓形曲线：粗弦多边形系统性偏短、随加密单调逼近；收敛弧长 > 弦长且 < 控制多边形周长
- 抛物线 r(t)=(t,t²) 弧长对闭式解 `√5/2 + asinh(2)/4`、曲率对 `2/(1+4t²)^{3/2}`
- 控制点个数错误、坐标缺失/非数值、容差非正 → 400 `invalid_input`
- 尖点求曲率/偏移 → 422 `singular`，响应不含 NaN/Inf
- 批量部分失败隔离；批量与单条结果逐位一致
- 16 并发 worker 交错请求不同缩放曲线，结果与串行基准逐一相等（`-race` 下无数据竞争）
