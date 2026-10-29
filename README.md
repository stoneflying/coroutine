# Coroutine

用 Go 和 **Plan 9 amd64 汇编**演示有栈协程及协作式 FIFO 调度器。
`Resume` / `Yield` 直接切换 SP、BP，没有用 channel 或 goroutine 握手模拟切换。
这是教学实验，不是与 Go runtime 集成的生产级调度器。

## 运行

已验证的平台为 **Linux amd64、Go 1.20.14**；其他 Go 版本和平台尚未验证。
本云环境需要先启用实际的 Go 工具链：

```bash
export PATH=/workspace/toolchains/go1.20.14/bin:$PATH
export GOPATH=/workspace/go
export GOCACHE=/workspace/.cache/go-build
cd /workspace/coroutine
```

普通开发环境使用自己的 Go 1.20.14 安装即可。执行实验时关闭自动 GC 和异步抢占：

```bash
GOGC=off GODEBUG=asyncpreemptoff=1 go test -count=1 ./...
GOGC=off GODEBUG=asyncpreemptoff=1 go run ./examples/roundrobin
go vet ./...
```

示例输出：`[1 2 3 1 2 3 1 2 3]`。

## 调度器

```go
s := coroutine.NewScheduler() // 零值 Scheduler 也可用
order := make([]int, 0, 6)
for id := 1; id <= 2; id++ {
    id := id
    s.Spawn(func() {
        for i := 0; i < 3; i++ {
            order = append(order, id)
            coroutine.Yield()
        }
    })
}
s.Run() // order == []int{1, 2, 1, 2, 1, 2}
```

- `Spawn(fn)`：将新协程加入队尾，返回 `*Coroutine`，暂不执行函数。
- `Run()`：取队首运行；让出后放回队尾，结束后移除，直到队列为空。
- 回调可以 `Spawn` 新任务；新任务按加入顺序排队。`Run` 结束后可再次添加任务并运行。
- 不能递归运行同一个调度器，也不能手动 `Resume` 调度器拥有的协程。
- 调度是协作式的：回调不 `Yield` 就会独占调度器。没有时间片、I/O 事件循环或抢占调度。
- 队列定期压缩，清空已消费的指针，避免队列无限增长或保留已结束的栈。

也可直接使用 `NewCoroutine(fn)`、`Resume(co)`、`Yield()`。
嵌套 `Resume(child)` 后，child 的 `Yield` 或结束会返回直接调用者，
而不是无条件返回主协程。`Yield()` 在主执行流中是空操作。

生命周期为 `Ready → Running → Suspended → Running → … → Finished`。
`State()` 和 `Finished()` 可查询状态；祖先协程等待嵌套调用时仍视为 `Running`，
因此不能从子协程重新进入祖先。已结束协程再次 `Resume` 是空操作。
函数为 nil、无效协程、重复进入运行中的协程、违反调度器归属会 panic；
必须遵守这些前置条件，不能依赖在自建栈上 panic/recover 来处理误用。
不要复制使用中的 `Coroutine` 或 `Scheduler`，也不要手工构造 `Coroutine` 零值。

## 汇编切换原理

`swap(old, next)` 是 Go ABI0 汇编函数。这个调用边界由 Go 调用者负责保存
仍然存活的通用寄存器值，切换代码只需保存 **SP 和 BP**。返回 PC 已经在栈上，
恢复目标 SP/BP 后用 `RET` 进入目标位置，不必再独立保存 RIP 或一组 C ABI
callee-save 寄存器。`R14` 是 Go 的 `g` 指针，不能用未初始化的协程字段覆盖。

`NOSPLIT|NOFRAME` 保证汇编不额外建立栈帧。字段偏移使用编译器生成的
`Context_sp` / `Context_bp`，不在汇编里硬编码 Go 结构体偏移。

首次恢复时才分配 1 MiB 固定栈。`initContext` 直接取得汇编入口 `coroEntry`
的地址并写入初始返回槽，避免通过 `reflect` 取得 ABIInternal 包装入口。
首次 `RET` 进入 `coroEntry`，再调用 Go 包装函数执行用户回调。
回调返回后标记结束并切回调用者；在调用者栈上才释放协程栈引用。
栈内存的实际回收由之后的 GC 决定。结束入口不会再次被调度。

## 必须了解的实验边界

**自建 `[]byte` 栈没有注册到 Go runtime。关闭两个开关只用于运行受限实验，
并不让它成为 GC 安全或完整兼容 Go runtime 的实现。**

- 整个包（包括多个调度器）只能由同一个 Go goroutine 使用，不能并行访问。
  根 `Resume` 会锁定 OS 线程直到返回，但这并不提供 GC、栈扫描或 goroutine 隔离。
- 从创建到全部协程结束期间必须关闭自动 GC；不要调用 `runtime.GC`、
  开启内存限制导致的 GC，或让其他 goroutine 触发 GC。
  挂起栈上的指针不会被扫描，运行中栈也不在 runtime 记录的栈范围内。
- 不支持栈增长，没有保护页或可靠的栈溢出检测。保持回调及其调用链短小、浅层，
  不做深递归、大栈帧或依赖栈增长的标准库调用。栈溢出可能破坏内存。
- 不支持在自建栈上 panic/recover、Goexit、defer 链跨协程切换、阻塞系统调用、
  cgo、锁竞争、启动其他 goroutine 或 runtime 调度操作。输出等复杂操作放到
  `Run` 返回后的真实 Go 栈上；示例在协程内只记录顺序。
- `asyncpreemptoff=1` 只关闭异步抢占，**不会关闭同步抢占**。长时间运行、
  profiler、信号栈回溯和其他 runtime 干预仍可能导致崩溃；实验应短小并频繁返回主栈。
- 不支持 race detector、coverage 或其他改变栈布局/插入 runtime 调用的插桩。
  现有测试通过只验证受限条件下的切换和队列逻辑，不证明上述运行时功能安全。
- 库不会擅自修改进程全局 GC/抢占设置；调用方负责在进程启动时设置实验环境。

若需要任意 Go 回调、GC、动态栈和阻塞 I/O，应另行采用 runtime 支持的协程机制，
不能只通过保存更多寄存器让这个自建栈方案获得这些能力。

## 验证范围

测试覆盖：生命周期、结束后恢复、懒分配及结束后释放引用、无效调用、嵌套返回、
多层栈帧及整数/浮点局部值在 1000 次切换后保持不变、FIFO 轮转、动态加入任务、
空队列及复用，以及 129 个任务下的队列压缩。

可选微基准（一轮包含 Resume/Yield 往返，约两次汇编切换及 Go 层维护开销）：

```bash
GOGC=off GODEBUG=asyncpreemptoff=1 go test -run '^$' -bench BenchmarkResumeYield -benchtime=10000x
```

这不是与旧实现的受控性能对比，不能据此推断相对加速幅度。
