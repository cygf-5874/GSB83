# minreorder

`minreorder` keeps a set of integer nodes together with `before -> after`
dependency relations and reports an order that satisfies every relation while
staying as close as possible to the previous output.

Go 1.24, standard library only.

## 对外契约

1. `NewOrderer(ids []int, deps [][2]int) (*Orderer, error)`：`deps` 的元素是
   `{before, after}`；有环 → `ErrCycle`。
2. `Insert(id int, after []int)` 插入一个新节点。
3. `Order() []int` 返回一个满足全部依赖的顺序，且在所有可行顺序中
   **与「上一次输出的顺序」的相对位移最小**：判据是**最长公共子序列（LCS）长度最大**，
   并列时取**字典序最小**的那个顺序（逐元素比较）。
4. 输出必须**逐字节确定**：同样的操作序列，任意次运行、任意 `map` 迭代顺序下结果相同。
5. 违反依赖（`Insert` 的 `after` 里有已删除的节点）→ `ErrBadDep`。
6. `Order()` 在无解时不得返回部分结果（构造成环的插入直接 `ErrCycle`，状态不变）。
7. `Stats()` 必须如实反映内部工作量：`Compares` 是**节点间依赖关系的比较次数**，
   `Allocs` 是堆分配次数，`Rebuilds` 是**整表重建次数**（这是本次要压的指标）。

## 代价上界

- 规模 20000 节点、10000 次 `Insert`、每次 `Insert` 后调一次 `Order()`：
  `Rebuilds` 必须为 **0**（不许整表重建）、`Compares` 必须 ≤ `6 * (节点数 + 边数)`。
- 基准按本机标定，跑题机慢按比例放宽（固定件里写死的是计数上界，不是时间上界）。
