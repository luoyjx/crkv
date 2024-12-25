# 基于 CRDT 的 Redis Proxy Server

## 目标

开发一个基于 CRDT 技术的 Redis Proxy Server，实现 Active-Active 架构，提供高可用和低延迟的 Redis 服务。

## 核心功能

- **Redis 协议兼容:** 完全兼容 Redis 客户端协议，允许现有 Redis 客户端无缝连接。
- **就近接入:** 客户端能够连接到地理位置最近的 Proxy Server，降低访问延迟。
- **Active-Active 架构:** 多个 Proxy Server 实例同时提供服务，数据在不同实例间通过 CRDT 进行同步。
- **CRDT 数据复制:** 使用 CRDT 算法解决不同节点对相同 Key 的并发修改冲突。
- **多 Master 复制:** 所有 Proxy Server 实例都是可写的，任何实例上的修改都会同步到其他实例。

## 模块划分

1. **网络层 (Network Layer):**
    - 负责接收客户端连接。
    - 负责 Proxy Server 之间的网络通信。
    - 可以考虑使用 `net` 包处理 TCP 连接，并为 Proxy Server 间的通信选择合适的协议。

2. **Redis 协议处理层 (Redis Protocol Layer):**
    - 解析客户端发送的 Redis 命令。
    - 将命令转发到 CRDT 引擎。
    - 将 CRDT 引擎的响应序列化为 Redis 协议格式返回给客户端。
    - 可以使用 `github.com/gomodule/redcon` 库来简化 Redis 协议的处理。

3. **CRDT 引擎层 (CRDT Engine Layer):**
    - 维护不同 Redis 数据类型的 CRDT 状态。
    - 实现各种 CRDT 算法，用于处理不同数据类型的并发修改。
    - 提供 API 供协议处理层调用，执行 CRDT 操作。
    - 需要考虑如何存储 CRDT 的状态，可以使用内存存储或持久化存储。

4. **复制层 (Replication Layer):**
    - 负责 Proxy Server 之间的 CRDT 数据同步。
    - 选择合适的网络协议进行通信，例如 gRPC 或自定义的基于 Protocol Buffers 的协议。
    - 实现数据的广播或 Gossip 协议进行同步。

5. **路由层 (Routing Layer):**
    - 负责将客户端请求路由到合适的本地 Proxy Server。
    - 可以基于客户端 IP 地址或配置的地理位置信息进行路由。
    - 需要考虑服务发现机制，以便客户端能够找到可用的 Proxy Server。

## 迭代计划

### 第一阶段：基础架构与核心命令实现 (Set/Get - LWW)

- **目标:** 搭建基本 Proxy Server 框架，实现 SET 和 GET 命令，使用 Last Write Wins (LWW) CRDT 策略。
- **任务:**
    - 搭建网络层，能够接收客户端连接。
    - 集成 `redcon` 库，实现 Redis 协议的解析和序列化。
    - 实现简单的 CRDT 引擎，使用内存存储，支持 LWW 策略的 SET 和 GET 操作。
    - 实现基本的 Proxy Server 间的连接和数据同步机制（例如，简单的全量广播）。
    - 实现简单的路由层，客户端连接到指定的 Proxy Server。
- **技术选型:**
    - `net` 包处理网络连接。
    - `github.com/gomodule/redcon` 处理 Redis 协议。
    - `map[string]string` 作为 LWW 的简单 CRDT 存储。
    - `net/rpc` 或 `net/http` 用于 Proxy Server 间的基础通信。

### 第二阶段：扩展命令支持与 CRDT 算法实现

- **目标:** 扩展支持更多的 Redis 命令，并为不同的数据类型实现更合适的 CRDT 算法。
- **任务:**
    - 支持例如 INCR、DEL 等常用命令。
    - 为不同的 Redis 数据类型（例如 List、Set、Hash）实现相应的 CRDT 算法（例如，Grow-only Counter, Add-wins Set, Observed-Remove Map）。
    - 优化 CRDT 引擎，使其能够处理更复杂的数据结构和操作。
    - 改进 Proxy Server 间的数据同步机制，例如使用基于操作的复制或状态向量。
- **技术选型:**
    - 研究并实现各种 CRDT 算法。
    - 可以考虑使用更高效的序列化库，例如 `gogo/protobuf`。

### 第三阶段：就近接入与服务发现

- **目标:** 实现客户端就近连接到 Proxy Server 的功能，并引入服务发现机制。
- **任务:**
    - 实现基于客户端 IP 或配置的地理位置信息进行路由的逻辑。
    - 引入服务发现机制，例如使用 Consul、Etcd 或 ZooKeeper，让客户端能够动态发现可用的 Proxy Server。
    - 考虑使用负载均衡器来分发客户端请求。
- **技术选型:**
    - Consul, Etcd, ZooKeeper 等服务发现工具。
    - 负载均衡器（例如，Nginx, HAProxy）。

### 第四阶段：稳定性和性能优化

- **目标:** 提高系统的稳定性和性能。
- **任务:**
    - 进行全面的单元测试和集成测试。
    - 进行性能测试和瓶颈分析。
    - 优化 CRDT 算法和数据同步机制。
    - 增加监控和日志功能。
    - 考虑持久化 CRDT 数据，例如使用 BoltDB 或 RocksDB。
- **技术选型:**
    - `go test` 进行单元测试。
    - `pprof` 进行性能分析。
    - Prometheus, Grafana 等监控工具。
    - BoltDB, RocksDB 等嵌入式数据库。

## 未来扩展

- 支持更多的 Redis 命令和数据类型。
- 实现更高级的冲突解决策略。
- 改进网络分区容错能力。
- 提供管理和监控接口。