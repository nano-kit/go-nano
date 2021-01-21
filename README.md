# Go Nano

Nano is a lightweight and efficient framework in Golang for real-time interactive systems.
It provides a core network architecture and a series of tools and libraries that
can help developers eliminate boring duplicate work for common underlying logic.
The goal of nano is to improve development efficiency by eliminating the need to
spend time on repetitious network related programming.

Nano was designed for server-side applications like real-time games, social games,
mobile games, etc of all sizes. Nano also contains a simple JavaScript library to help developing web games.

## How to build a system with `Nano`

### What does a `Nano` application look like?

The simplest "nano" application as shown in the following figure, you can make powerful applications by combining different components.

```
+-------------+  Response
| Nano Client <-----------+
|(Web Browser)|           |
+-------------+           | Request   +-------------+
                          +----------->             |
+-------------+                       |             |
| Nano Client <-Persistent Connection-> Nano Server |
|(Mobile App) |                       | (Components)|
+-------------+           +----------->             |
                          | Notify    |             |
+-------------+           |           +-------------+
| Nano Client <-----------+
|(Desktop App)|   Push
+-------------+
```

In fact, the `nano` application server is a collection of [Component](./docs/get_started.md#component), and a component is a bundle of [Handler](./docs/get_started.md#handler). once you register a component to nano, nano will register all methods that can be converted to `Handler` to nano application server. The handler will be called while client request. The handler will receive two parameters while handling a message:
  - `*session.Session`: corresponding a client that apply this request or notify.
  - `*protocol.FooBar`: the payload of the request.

While you had processed your logic, you can response or push message to the client by `session.Response(payload)` and `session.Push('eventName', payload)`, or returns error when some unexpected data received.

See [Get Started](./docs/get_started.md) for more informations.

### How to build distributed system with `Nano`

Nano contains built-in distributed system solution, and make you creating a distributed game server easily.

See [The distributed chat demo](./examples/cluster)

The Nano will remain simple, but you can perform any operations in the component and get the desired goals. You can startup a group of `Nano` application as agent to dispatch message to backend servers.

### How to execute the asynchronous task

```go
func (manager *PlayerManager) Login(s *session.Session, msg *ReqPlayerLogin) error {
    var onDBResult = func(player *Player) {
        manager.players = append(manager.players, player)
        s.Push("PlayerSystem.LoginSuccess", &ResPlayerLogin)
    }

    // run slow task in new gorontine
    go func() {
        player, err := db.QueryPlayer(msg.PlayerId) // ignore error in demo
        // handle result in main logical gorontine
        scheduler.Run(func(){ onDBResult(player) })
    }
    return nil
}
```

## Documents

- English
    + [How to build your first nano application](./docs/get_started.md)
    + [Communication protocol](./docs/communication_protocol.md)
    + [Route compression](./docs/route_compression.md)

- 简体中文
    + [如何构建你的第一个nano应用](./docs/get_started_zh_CN.md)
    + [通信协议](./docs/communication_protocol_zh_CN.md)
    + [路由压缩](./docs/route_compression_zh_CN.md)

## Resources

- Demo
  + [Implement a chat room in 100 lines with nano and WebSocket](./examples/demo/chat)
  + [Tadpole demo](./examples/demo/tadpole)

## Go version

`>= go1.14`

## Installation

```shell
go get github.com/aclisp/go-nano

# dependencies
go get -u github.com/golang/protobuf
go get -u github.com/gorilla/websocket
```

## Benchmark

```shell
# Case:   PingPong
# OS:     Windows 10
# Device: i5-6500 3.2GHz 4 Core/1000-Concurrent   => IOPS 11W(Average)
# Other:  ...

cd ./benchmark/io
go test -v -tags "benchmark"
```

## License

[MIT License](./LICENSE)

## Fork

This project is a refined version of the [original repo](https://github.com/lonng/nano) made by Lonng.
It has following critical improvements:

* a new scheduler
* tidy logging messages
* various bug fixing
* [writev](https://github.com/golang/go/issues/13451) for agent.write
* renamed some APIs
* fixed broken demos and docs
* remove stale sessions
* shrink rpc client
* cluster: smarter startup: unregister then retry
