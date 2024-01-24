# Nodehub

Nodehub是为社交类游戏、棋牌类游戏设计的服务器端框架。

Nodehub的通讯方式建立于[gRPC](https://grpc.io/)基础之上，在使用Nodehub开发之前，需要对[gRPC开发方式](https://grpc.io/docs/languages/go/)有所了解。

## 特性

- 服务注册与发现（使用[etcd](https://etcd.io/)）
- 服务节点负载均衡（允许自定义）
- 有状态服务节点路由
- 集群内事件广播（允许注册自定义事件）
- 所有的节点均内置gRPC管理服务

## 架构

- 整体架构由客户端、网关节点、服务节点以及基础服务组成
- 由etcd实现服务注册与发现，[nats](https://nats.io/)（推荐）或[redis](https://redis.io/)实现服务间消息总线
- 客户端通过websocket或者raw tcp socket方式与网关连接，客户端只会通过网关与服务节点联系，不会直接请求服务节点
- 内部服务节点通过gRPC方式提供接口
- 网关把收到的客户端消息转换为gRPC请求转发到相应的内部节点，然后再把收到的gRPC响应结果返回给客户端

## 消息格式

由于客户端并非直接向内部节点发送gRPC请求而是通过网关转发，因此客户端的上下行消息需要经过一定的包装才能实现gRPC over websocket的效果。

所有的客户端请求一律使用`nodehub.Request`类型，服务器端返回的消息类型一律使用`nodehub.Reply`类型。

在采用raw tcp socket方式与网关通讯时，采用了简单的`length + data`的方式实现数据包（0长度的包为心跳包）。

[client.proto](./api/protobuf/nodehub/client.proto)文件内包含了客户端上下行消息的protobuf定义。

[client.go](./component/gateway/client.go)内提供了websocket client和tcp client实现供参考和测试。

## 服务配置

每个节点在启动之后，都会向etcd注册自身配置信息，配置信息结构如下：

```json
{
	"id": "",	// ulid，每次启动后自动生成
	"name": "",	// 节点名称
	"state": "ok",	// 节点状态
	"entrace": "ws://host:port/grpc",	// 网关入口地址
	"grpc": {
		"endpoint": "ip:port",	// grpc服务监听地址
		"services": [
			{
				"code": 0,	// grpc服务枚举值
				"path": "/helloworld.Greeter",	// 服务路径
				"public": true,	// 是否允许客户端访问的公开服务
				"balancer": "random",	// 负载均衡策略
				"stateful": false,	// 是否有状态服务
				"allocation": "auto",	// 有状态节点分配方式
				"pipeline": "",	// 管道名称
			}
		]
	}
}
```

`state`（节点状态）有如下枚举值：

- ok 表示节点正常提供服务
- lazy 用于有状态节点服务，表示不再接受新的客户端请求，但之前建立了路由状态的请求仍然可以处理
- down 表示节点下线，不再接受任何请求

每个节点允许注册多个gRPC服务，考虑到方便滚动更新，建议不要把有状态节点和无状态节点注册到一起。

集群内的每个gRPC服务，都要有单独的服务代码`grpc.services.code`，这样网关才能够根据服务代码把客户端请求转发到相应的服务节点上。

`grpc.services.path` 是gRPC服务的http2注册路径，会在调用gRPC注册函数时(`rpc.GRPCServer.RegisterService()`方法)自动填写

`grpc.services.public`声明此服务属于公开服务还是私有服务，客户端只能向公开服务发起请求。游戏节点之间的调用逻辑可以放到私有服务上，与客户端接口隔离开。

`grpc.services.balancer`控制此服务的负载均衡方式，目前内置了四种策略：

- random 随机
- roundRobin 加权轮询
- ipHash 根据客户端IP地址哈希
- idHash 根据账号ID哈希

除了内置的负载均衡策略外，也支持注册自定义的其它负载均衡策略。

`grpc.services.stateful`声明此服务属于有状态服务还是无状态服务，有状态服务需要建立了路由关系才能接受客户端请求。

`grpc.services.allocation`控制有状态节点的分配方式，对无状态节点没有影响，允许的配置方式有：

- auto 网关根据负载均衡策略，在可用节点中自动选择一个节点建立路由关系
- server 服务器端自行分配，网关不会参与分配
- client 客户端请求时，通过`nodehub.Request.node_id`指定，网关会把客户端与指定node_id的节点关系记录到路由表内，后续即使不指定node_id，对同一个服务的请求也会继续路由到之前指定的节点，直到客户端指定其它的node_id

`auto`分配方式适用于玩家个人信息类型的服务，玩家登录后访问个人信息时自动分配，节点和客户端的路由关系在整个游戏过程中持续存在。

`server`和`client`适用于房间服务类型的节点请求，在房间创建之后才建立路由关系，玩家在游戏过程中可能会访问多个不同的房间节点。无论使用`server`还是`client`类型的分配策略，都能达到类似的效果，具体开发时可根据实际情况酌情使用。

`grpc.services.pipeline`用于控制单个客户端的消息时序性保证，`pipeline`的名字允许自行指定任意字符串，设置后会按照消息的接收顺序，处理完一条才会继续处理下一条。如果多个服务设置了相同的`pipeline`值，那么对这些服务的请求均会保证时序性。如果没有指定，消息会被并发处理，有可能出现后发先至的结果。

## gRPC约束

- 面向客户端的gRPC方法的返回结果，只能是`nodehub.Reply`和[google.protobuf.Empty](https://github.com/protocolbuffers/protobuf/blob/main/src/google/protobuf/empty.proto)两种类型，否则会导致客户端无法解析
- `MSG`会被encode到`nodehub.Reply.data`字段内，客户端通过`nodehub.Reply.message_type`才能知道具体的消息类型，所以`MSG`需要额外定义一个数据类型枚举值
- 所有会被下行到客户端的消息类型枚举值，都定义到`enum Protocol`枚举值表内，也可以使用`Protocol`之外的名字，这里并不强制

## 示例 (echo server)

[./example/echo/](./example/echo/) 实现了把客户端发送的消息内容原封不动返回的简单服务。
[./example/chat/](./example/chat/) 实现了一个非常非常简单的聊天室。
