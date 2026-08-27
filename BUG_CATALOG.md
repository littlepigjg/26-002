# UBAAS 缺陷候选清单 (BUG_CATALOG)

> 用户行为路径分析系统缺陷候选清单，共30个缺陷，覆盖并发、context、错误传播、运行时崩溃、状态污染、跨层数据流等多个维度。

---

## BUG-001: 事件批量处理缓冲区无容量限制导致内存溢出
- **严重程度**: 高
- **涉及文件**: `internal/service/event_service.go`, `cmd/server/main.go`
- **问题描述**: 事件批量缓冲处理时，channel缓冲区容量固定但无背压机制，高并发下事件堆积导致内存持续增长，最终OOM崩溃。
- **影响范围**: 所有依赖事件处理的API，包括埋点上报、统计查询等
- **复现条件**: 持续以高频率（>1000 QPS）发送POST /api/events请求，观察进程内存占用持续增长

## BUG-002: 会话过期检查与事件处理竞态条件
- **严重程度**: 高
- **涉及文件**: `internal/service/session_service.go`, `internal/store/session_store.go`
- **问题描述**: 会话过期检查（IsExpired）和事件处理（ProcessEvent）之间没有原子性保证，导致事件可能被添加到已过期的会话中，或者活跃会话被错误地标记为过期。
- **影响范围**: 会话分析准确性，可能导致会话边界错误
- **复现条件**: 在会话即将过期时发送事件，检查事件是否被正确归属于新会话或旧会话

## BUG-003: 维度筛选查询中context超时未正确传播
- **严重程度**: 高
- **涉及文件**: `internal/service/dimension_service.go`, `internal/store/memory_store.go`
- **问题描述**: GetDimensionBreakdown方法接收的context未传递给底层的ListEvents调用，导致客户端超时后服务器仍继续处理，造成资源浪费。
- **影响范围**: 维度分析API的超时行为
- **复现条件**: 设置较短的客户端超时（如100ms），在大量数据上执行维度查询，观察服务器是否提前终止

## BUG-004: 路径序列计算中DurationMs可能溢出
- **严重程度**: 中
- **涉及文件**: `internal/model/path.go`, `internal/service/path_service.go`
- **问题描述**: PathSequence.ComputeDuration()累计各事件DurationMs时未检查int64溢出，当用户产生大量事件时可能溢出为负数。
- **影响范围**: 路径分析中的停留时长统计
- **复现条件**: 为同一用户添加超过9,223,372,036,854,775,807纳秒的累计DurationMs

## BUG-005: 优雅关闭时事件处理goroutine泄漏
- **严重程度**: 高
- **涉及文件**: `cmd/server/main.go`, `internal/service/event_service.go`, `internal/service/scheduler.go`
- **问题描述**: 服务器关闭时，EventService的flush通道和Scheduler的goroutine没有正确等待完成，导致goroutine泄漏或数据丢失。
- **影响范围**: 服务重启时的数据完整性
- **复现条件**: 在高负载下触发优雅关闭，检查是否有未处理的事件丢失

## BUG-006: 过滤器in-memory过滤与数据库过滤不一致
- **严重程度**: 中
- **涉及文件**: `internal/service/dimension_service.go`, `internal/service/query_service.go`, `internal/store/memory_store.go`
- **问题描述**: 部分过滤条件（如OS、Browser、Country）在ListEvents中已经过滤，但维度服务又进行了一轮in-memory过滤，导致逻辑重复且可能出现不一致。
- **影响范围**: 过滤结果的准确性
- **复现条件**: 设置OS过滤条件，比较ListEvents直接结果与维度服务结果的差异

## BUG-007: 指标收集器Snapshot并发读不一致
- **严重程度**: 中
- **涉及文件**: `internal/model/metrics.go`, `internal/middleware/middleware.go`
- **问题描述**: MetricsCollector.Snapshot()在读取counter时只使用RLock，但读取activeRequests用的是atomic，两种同步机制混用可能导致读到不一致的快照。
- **影响范围**: 健康检查API的指标数据准确性
- **复现条件**: 高并发请求下频繁调用/health接口，检查activeRequests和totalRequests是否合理

## BUG-008: 用户维度更新UserID为空时的状态污染
- **严重程度**: 中
- **涉及文件**: `internal/store/user_store.go`, `internal/service/event_service.go`
- **问题描述**: 当事件中UserID为空时，SanitizeUserID会生成一个匿名ID，但此时UserDimension的Update方法可能用空值覆盖已存在的有效字段。
- **影响范围**: 用户画像数据的完整性
- **复现条件**: 发送UserID为空的事件后，再发送带有效UserID的事件，检查UserDimension字段是否被清空

## BUG-009: 转换趋势计算中goal.StartPage/EndPage匹配过于严格
- **严重程度**: 中
- **涉及文件**: `internal/service/conversion_service.go`, `internal/store/conversion_store_ext.go`
- **问题描述**: GetConversionTrends使用严格的字符串匹配来匹配goal.StartPage和event.PageURL，不处理查询参数或URL编码的情况，导致转化率计算偏低。
- **影响范围**: 转化率分析的准确性
- **复现条件**: 创建一个URL带查询参数的转化目标，检查转化事件是否被正确匹配

## BUG-010: SessionStore中活跃会话索引未定期清理
- **严重程度**: 中
- **涉及文件**: `internal/store/session_store.go`, `internal/service/scheduler.go`
- **问题描述**: 活跃会话索引map在会话过期后仅标记为过期但不删除，长时间运行后索引持续膨胀，增加内存使用和查找开销。
- **影响范围**: 系统长期运行的稳定性
- **复现条件**: 运行服务器数天，检查session_store中activeSessions map的大小

## BUG-011: 导出服务中CSV字段顺序与JSON字段不一致
- **严重程度**: 低
- **涉及文件**: `internal/service/export_service.go`, `internal/model/export.go`
- **问题描述**: CSV导出使用固定的字段顺序，而JSON导出使用map结构，两者字段顺序不一致。当用户指定自定义字段时，CSV的列顺序可能与请求的字段顺序不匹配。
- **影响范围**: 数据导出的可用性
- **复现条件**: 指定自定义导出字段顺序，检查CSV列顺序是否与请求一致

## BUG-012: 维度服务中matchesAllConditions的AND/OR逻辑反转
- **严重程度**: 高
- **涉及文件**: `internal/service/dimension_service.go`
- **问题描述**: matchesAllConditions函数在Logic为LogicOR时仍使用AND逻辑判断（所有条件都要满足），导致OR条件被错误地当作AND处理。
- **影响范围**: 复合筛选条件的结果完全错误
- **复现条件**: 设置两个OR连接的过滤条件，检查返回结果是否正确包含满足任一条件的事件

## BUG-013: 事件缓冲处理中panic未被recover
- **严重程度**: 高
- **涉及文件**: `internal/service/event_service.go`, `internal/middleware/middleware.go`
- **问题描述**: 事件缓冲处理goroutine中的ProcessEvent调用如果发生panic，会导致整个EventService停止处理，且只有Recovery中间件能捕获HTTP层面的panic，后台goroutine的panic无法被捕获。
- **影响范围**: 整个事件处理管道停止
- **复现条件**: 向store中注入nil事件或损坏事件，检查事件处理是否停止

## BUG-014: 热门路径统计map并发访问无锁保护
- **严重程度**: 高
- **涉及文件**: `internal/service/path_service.go`, `internal/store/path_store.go`
- **问题描述**: 热门路径统计时，`pathStats`的map在迭代过程中可能被另一个goroutine修改，导致map并发读写的运行时崩溃。
- **影响范围**: 热门路径API的稳定性
- **复现条件**: 高并发下频繁调用GetPopularPaths接口，观察是否触发concurrent map read/write panic

## BUG-015: Config.Get()返回副本导致并发读写问题
- **严重程度**: 中
- **涉及文件**: `internal/config/config.go`, `cmd/server/main.go`
- **问题描述**: Config.Get()返回值类型的副本，但Config.Update()修改的是指针。在Get和Update之间配置可能已被修改，导致获取到过期的配置快照。
- **影响范围**: 配置动态更新的实时性
- **复现条件**: 运行时修改配置（如端口），检查Get()返回的值是否立即反映变更

## BUG-016: 时间窗口解析中时区处理不一致
- **严重程度**: 中
- **涉及文件**: `pkg/timeutil/time.go`, `internal/service/query_service.go`, `internal/handler/helpers.go`
- **问题描述**: ParseTimeWindow使用time.Parse（默认UTC），而resolveTimeRange使用time.Now()（本地时区），导致用户输入的时间范围与实际查询的时间范围可能有偏差。
- **影响范围**: 基于自定义时间范围查询的结果准确性
- **复现条件**: 在非UTC时区下查询指定时间范围的数据，检查返回数据的时间戳是否在预期范围内

## BUG-017: 批量事件创建部分失败后无回滚机制
- **严重程度**: 中
- **涉及文件**: `internal/store/memory_store.go`, `internal/service/event_service.go`
- **问题描述**: CreateEvents批量创建事件时，如果中间某个事件处理失败，之前的事件已经被添加到store中，没有原子性保证。
- **影响范围**: 批量数据导入的一致性
- **复现条件**: 发送一个包含有效和无效事件的批量请求，检查是否部分事件被持久化

## BUG-018: 日志缓冲区写入时无长度限制
- **严重程度**: 中
- **涉及文件**: `pkg/logger/logger.go`, `pkg/logger/format.go`
- **问题描述**: JSONFormatter的Format方法使用bytes.Buffer进行字符串拼接，但没有对单个日志条目的最大长度进行限制，极端情况下可能生成非常大的日志条目。
- **影响范围**: 日志系统稳定性和磁盘使用
- **复现条件**: 记录包含大字段（如100MB的props map）的日志，检查日志条目是否被截断

## BUG-019: 转化漏斗分析步骤计算未考虑时间窗口重叠
- **严重程度**: 中
- **涉及文件**: `internal/service/conversion_service.go`
- **问题描述**: BuildFunnelAnalysis计算漏斗步骤时，没有检查步骤之间的时间窗口是否合理（如前一步的end时间大于后一步的start时间），可能产生不合理的漏斗步骤。
- **影响范围**: 转化漏斗分析的合理性
- **复现条件**: 创建一个转化目标，其步骤时间窗口重叠，检查漏斗步骤是否正确

## BUG-020: 速率限制器状态在服务重启后丢失
- **严重程度**: 低
- **涉及文件**: `internal/middleware/middleware.go`, `internal/config/config.go`
- **问题描述**: 速率限制器使用内存存储状态，服务重启后所有速率限制计数被重置，攻击者可以通过触发服务重启绕过速率限制。
- **影响范围**: 速率限制的安全性
- **复现条件**: 触发服务重启后立即发送大量请求，检查速率限制是否生效

## BUG-021: 路径序列中PageURL字段可能被截断
- **严重程度**: 低
- **涉及文件**: `internal/model/path.go`, `pkg/validator/sanitize.go`
- **问题描述**: SanitizePageURL限制URL最大长度为4096，但PathSequence存储的PageNode.URL字段没有同样的限制，导致长URL被静默截断或导致不一致。
- **影响范围**: 特定长URL页面的路径分析
- **复现条件**: 提交超过4096字符的URL，检查SanitizePageURL和PathSequence的行为是否一致

## BUG-022: EventStore与SessionStore数据不同步
- **严重程度**: 高
- **涉及文件**: `internal/service/event_service.go`, `internal/service/session_service.go`
- **问题描述**: 事件处理时，EventStore和SessionStore的操作不是在同一个事务中进行的。如果SessionStore的更新失败，EventStore中的事件仍然被保存，导致数据不一致。
- **影响范围**: 事件与会话数据的一致性
- **复现条件**: 模拟SessionStore更新失败，检查已保存的事件是否有对应的会话

## BUG-023: 导出服务中LargeQuery超时后仍占用资源
- **严重程度**: 中
- **涉及文件**: `internal/service/export_service.go`, `pkg/timeutil/time.go`
- **问题描述**: 导出服务使用context.WithTimeout控制超时，但超时后goroutine仍在执行，查询结果可能被写入channel但无人消费，导致资源泄漏。
- **影响范围**: 大规模数据导出的资源效率
- **复现条件**: 执行超过超时时间的导出查询，检查goroutine是否在超时后正确退出

## BUG-024: 中间件Recovery无法捕获所有类型的panic
- **严重程度**: 中
- **涉及文件**: `internal/middleware/middleware.go`
- **问题描述**: Recovery中间件使用defer recover()捕获panic，但对于runtime错误（如concurrent map read/write、nil pointer dereference发生在非HTTP handler goroutine中）无法被捕获。
- **影响范围**: 系统整体稳定性
- **复现条件**: 在后台goroutine中触发nil pointer dereference，检查服务器是否崩溃

## BUG-025: 用户维度统计中NewUser定义不一致
- **严重程度**: 中
- **涉及文件**: `internal/model/user.go`, `internal/service/session_service.go`
- **问题描述**: UserDimension中的UserType在首次创建时设为UserNew，但SessionService中的new/returning用户判断逻辑使用Session的创建时间来判断，两者标准不一致。
- **影响范围**: 新用户/老用户统计的准确性
- **复现条件**: 先创建一个会话（标记为新用户），然后创建第二个会话，检查用户类型是否被正确更新

## BUG-026: ListEvents分页参数PageSize为0时导致全表扫描
- **严重程度**: 高
- **涉及文件**: `internal/model/event.go`, `internal/store/memory_store.go`
- **问题描述**: EventQuery.PageSize默认为0，当PageSize<=0时ListEvents使用默认值10，但如果PageSize被显式设置为0，会导致返回所有事件，可能造成内存溢出。
- **影响范围**: 系统在异常输入下的稳定性
- **复现条件**: 发送PageSize为0的查询请求，检查是否返回全量数据

## BUG-027: 会话滑动窗口超时计算偏差
- **严重程度**: 中
- **涉及文件**: `internal/service/session_service.go`, `internal/model/session.go`
- **问题描述**: 会话超时检查使用事件的Timestamp与会话的LastActivity比较，但如果事件的Timestamp早于会话创建时间（客户端时钟不同步），可能导致会话被错误地标记为已过期。
- **影响范围**: 会话识别的准确性
- **复现条件**: 使用时钟回拨的客户端发送事件，检查会话是否被错误地终止

## BUG-028: 缓存LRU淘汰策略未考虑TTL过期
- **严重程度**: 低
- **涉及文件**: `pkg/cache/lru.go`, `pkg/cache/cache.go`
- **问题描述**: LRU缓存的淘汰逻辑先于TTL过期执行。当缓存满时，如果最旧的条目仍然有效（TTL未过期），会被淘汰，而某些已过期的条目却因为访问频繁而保留。
- **影响范围**: 缓存命中率和内存使用效率
- **复现条件**: 填满缓存后访问一个即将过期的条目，检查它是否被正确淘汰

## BUG-029: 跨层数据流：handler到service错误信息丢失
- **严重程度**: 中
- **涉及文件**: `internal/handler/event_handler.go`, `internal/service/event_service.go`, `pkg/response/response.go`
- **问题描述**: handler层调用service后，如果service返回的error包含详细信息（如验证失败的具体字段名），handler仅返回通用错误信息，导致客户端无法定位具体问题。
- **影响范围**: API的可调试性
- **复现条件**: 发送缺少必填字段的事件请求，检查错误响应是否包含具体缺失的字段名

## BUG-030: Context生命周期：后台任务未正确继承请求上下文
- **严重程度**: 中
- **涉及文件**: `cmd/server/main.go`, `internal/service/scheduler.go`, `internal/service/export_service.go`
- **问题描述**: Scheduler和ExportService的后台任务使用context.Background()作为根context，没有继承服务器的关闭信号。当服务器收到SIGTERM时，后台任务无法感知并继续运行。
- **影响范围**: 服务优雅关闭的完整性
- **复现条件**: 启动长时间运行的导出任务后立即发送SIGTERM信号，检查导出任务是否被正确取消

---

## 缺陷统计

| 严重程度 | 数量 |
|---------|------|
| 高       | 10   |
| 中       | 14   |
| 低       | 6    |
| **总计** | **30** |

## 缺陷分布

| 缺陷类型 | 数量 |
|---------|------|
| 并发/竞态 | 6 |
| Context传播 | 4 |
| 错误传播/丢失 | 4 |
| 运行时崩溃 | 5 |
| 状态污染 | 5 |
| 跨层数据流 | 3 |
| 内存泄漏/溢出 | 3 |

## 涉及文件分布

主要涉及的文件包括：
- `internal/service/` 下的所有服务文件
- `internal/store/memory_store.go`
- `internal/middleware/middleware.go`
- `internal/config/config.go`
- `internal/model/` 下的多个模型文件
- `cmd/server/main.go`
- `pkg/cache/`, `pkg/logger/`, `pkg/timeutil/`
