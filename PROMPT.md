一次插入要重排整张图，三万个节点的时候已经肉眼可见地卡 —— minreorder 是 Go 1.24 的
拓扑重排库，只用标准库，自检走 `scripts/check.sh`（`check/` 是固定验收程序，别改）。

现在的实现是对的：`go test ./...` 16 个用例全绿，输出也确定。
但 `go run ./bench` 一跑就能看到代价不对，README「代价上界」那一节写了要压到多少。

任务：把实现重构成增量维护，让「代价上界」达标，同时保证输出**逐元素完全不变**。

验收：
- go build ./... 退出码 0；
- go test ./... 退出码 0（16 个用例必须一直全绿）；
- go run ./check 退出码 0，10 个场景全过（semantics 4 + determinism 2 + budget 3 + equivalence 1）。

约束：
1. 不改 `check/`、不改 `bench/`、不改 `reorder_test.go`；`stats.go` 的字段名与含义不许改。
2. 不引入第三方依赖，不许用 `time` 做判据。
3. 公开的 API 签名不变（可新增内部结构与函数）。
4. 不许通过少算、跳过校验或伪造计数器来达标。
