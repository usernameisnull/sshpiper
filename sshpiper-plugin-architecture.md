# sshpiper 主程序与 Plugin 通信机制详解

## 概述

sshpiper 采用 **gRPC** 作为通信协议，在设计上支持多种传输方式，包括 stdio 管道、TCP 网络、Unix Socket 等。

## 1. 默认通信方式：gRPC over Stdio

当前默认实现使用 **stdin/stdout** 管道进行进程间通信（IPC），无需网络端口：

```
┌─────────────────┐         stdin/stdout          ┌─────────────────┐
│   sshpiperd     │  ═══════════════════════════► │  Plugin Process │
│   (gRPC Client) │  ◄═══════════════════════════ │  (gRPC Server)  │
└─────────────────┘         (双向管道)              └─────────────────┘
```

### 为什么使用 stdio？
- 简单直接：启动子进程自动建立通信
- 无需配置：不需要端口、网络设置
- 进程隔离：插件崩溃不影响主程序
- 生命周期绑定：插件随主程序启动/停止

## 2. 核心组件

| 文件 | 作用 |
|------|------|
| `libplugin/plugin.proto` | 定义 gRPC 接口和消息结构 |
| `libplugin/pluginbase.go` | Plugin 侧：实现 gRPC 服务端 |
| `cmd/sshpiperd/internal/plugin/grpc.go` | 主程序侧：实现 gRPC 客户端 |
| `libplugin/ioconn/` | 将 stdin/stdout 包装成 `net.Conn` |

## 3. 启动与通信流程

### 3.1 主程序启动插件

`cmd/sshpiperd/main.go`:
```go
func createCmdPlugin(args []string) (*plugin.CmdPlugin, error) {
    cmd := exec.Command(exe)  // 启动插件子进程
    cmd.Args = args
    
    // 通过 stdin/stdout 建立 gRPC 连接
    p, err := plugin.DialCmd(cmd)
    return p, err
}
```

### 3.2 建立 stdio 连接

`libplugin/ioconn/cmd.go`:
```go
func DialCmd(cmd *exec.Cmd) (net.Conn, io.ReadCloser, error) {
    in, err := cmd.StdoutPipe()   // 读取插件输出
    out, err := cmd.StdinPipe()   // 写入插件输入
    stderr, err := cmd.StderrPipe() // 错误输出
    
    cmd.Start()
    
    // 包装成 net.Conn 接口
    return &cmdconn{conn: *dial(in, out), cmd: cmd}, stderr, nil
}
```

### 3.3 gRPC 连接建立

`cmd/sshpiperd/internal/plugin/grpc.go`:
```go
func DialGrpc(conn *grpc.ClientConn) (*GrpcPlugin, error) {
    p := &GrpcPlugin{
        grpcconn: conn,
        client:   libplugin.NewSshPiperPluginClient(conn),
    }
    return p, nil
}
```

## 4. gRPC 服务定义

`libplugin/plugin.proto`:
```protobuf
service SshPiperPlugin {
  rpc ListCallbacks(ListCallbackRequest) returns (ListCallbackResponse) {}
  rpc NewConnection(NewConnectionRequest) returns (NewConnectionResponse) {}
  rpc NextAuthMethods(NextAuthMethodsRequest) returns (NextAuthMethodsResponse) {}
  rpc NoneAuth(NoneAuthRequest) returns (NoneAuthResponse) {}
  rpc PasswordAuth(PasswordAuthRequest) returns (PasswordAuthResponse) {}
  rpc PublicKeyAuth(PublicKeyAuthRequest) returns (PublicKeyAuthResponse) {}
  rpc KeyboardInteractiveAuth(stream KeyboardInteractiveAuthMessage) returns (stream KeyboardInteractiveAuthMessage) {}
  rpc UpstreamAuthFailureNotice(UpstreamAuthFailureNoticeRequest) returns (UpstreamAuthFailureNoticeResponse) {}
  rpc Banner(BannerRequest) returns (BannerResponse) {}
  rpc VerifyHostKey(VerifyHostKeyRequest) returns (VerifyHostKeyResponse) {}
  rpc PipeCreateErrorNotice(PipeCreateErrorNoticeRequest) returns (PipeCreateErrorNoticeResponse) {}
  rpc PipeStartNotice(PipeStartNoticeRequest) returns (PipeStartNoticeResponse) {}
  rpc PipeErrorNotice(PipeErrorNoticeRequest) returns (PipeErrorNoticeResponse) {}
}
```

## 5. 插件开发模式

### 5.1 Plugin 侧（服务端实现）

`plugin/fixed/main.go`:
```go
func main() {
    libplugin.CreateAndRunPluginTemplate(&libplugin.PluginTemplate{
        Name:  "fixed",
        Usage: "sshpiperd fixed plugin",
        Flags: []cli.Flag{
            &cli.StringFlag{
                Name:     "target",
                Required: true,
            },
        },
        CreateConfig: func(c *cli.Context) (*libplugin.SshPiperPluginConfig, error) {
            target := c.String("target")
            host, port, _ := libplugin.SplitHostPortForSSH(target)
            
            return &libplugin.SshPiperPluginConfig{
                // 注册密码认证回调
                PasswordCallback: func(conn libplugin.ConnMetadata, password []byte) (*libplugin.Upstream, error) {
                    return &libplugin.Upstream{
                        Host:          host,
                        Port:          int32(port),
                        IgnoreHostKey: true,
                        Auth:          libplugin.CreatePasswordAuth(password),
                    }, nil
                },
            }, nil
        },
    })
}
```

### 5.2 主程序调用（客户端）

```go
func (g *GrpcPlugin) PasswordCallback(conn ssh.ConnMetadata, password []byte, challengeCtx ssh.ChallengeContext) (*ssh.Upstream, error) {
    meta := toMeta(challengeCtx, conn)
    
    // 调用远程插件的 gRPC 方法
    reply, err := g.client.PasswordAuth(context.Background(), &libplugin.PasswordAuthRequest{
        Meta:     meta,
        Password: password,
    })
    if err != nil {
        return nil, err
    }
    
    // 将返回的 Upstream 转换为 ssh.Upstream
    return g.createUpstream(conn, challengeCtx, reply.Upstream)
}
```

## 6. 插件链式调用

支持多个插件通过 `--` 分隔链式调用：

```bash
sshpiperd simplemath -- ./fixed --target 127.0.0.1:5522
```

链式插件通过 `UpstreamNextPluginAuth` 传递控制流：

```protobuf
message Upstream {
    oneof auth {
        UpstreamNoneAuth none = 100;
        UpstreamPasswordAuth password = 101;
        UpstreamPrivateKeyAuth private_key = 102;
        UpstreamRemoteSignerAuth remote_signer = 103;
        UpstreamNextPluginAuth next_plugin = 200;        // 链式调用下一个插件
        UpstreamRetryCurrentPluginAuth retry_current_plugin = 201;  // 重试当前插件
    }
}
```

## 7. 网络通信支持

### 7.1 架构设计支持

代码设计上**完全支持网络通信**，核心接口都接受 `net.Listener` 和 `*grpc.ClientConn`：

**Plugin 侧** (`libplugin/pluginbase.go`):
```go
func NewFromGrpc(config SshPiperPluginConfig, grpc *grpc.Server, listener net.Listener) (SshPiperPlugin, error)
```

**主程序侧** (`cmd/sshpiperd/internal/plugin/grpc.go`):
```go
func DialGrpc(conn *grpc.ClientConn) (*GrpcPlugin, error)
```

### 7.2 改造成网络版 Plugin

将 stdio 插件改为网络版只需修改 `main()`：

```go
func main() {
    addr := os.Getenv("SSHPIPER_PLUGIN_ADDR")
    if addr == "" {
        // 默认使用 stdio 方式
        libplugin.CreateAndRunPluginTemplate(&libplugin.PluginTemplate{...})
        return
    }
    
    // 网络模式
    lis, err := net.Listen("tcp", addr)
    if err != nil {
        log.Fatal(err)
    }
    
    s := grpc.NewServer()
    config := createConfig() // 你的插件配置
    plugin, _ := libplugin.NewFromGrpc(config, s, lis)
    plugin.Serve()
}
```

### 7.3 主程序连接远程 Plugin

```go
import "google.golang.org/grpc"
import "google.golang.org/grpc/credentials"

// 创建到远程插件的 gRPC 连接
conn, err := grpc.Dial("plugin-server:50051",
    grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), // TLS 加密
)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

// 使用 DialGrpc 创建插件实例
p, err := plugin.DialGrpc(conn)
```

## 8. 通信方式对比

| 方式 | 优点 | 缺点 | 适用场景 |
|------|------|------|----------|
| **Stdio** (当前默认) | 简单、自动启动、进程隔离 | 只能本机、插件必须可执行文件 | 单机部署、简单场景 |
| **TCP** | 跨机器、插件可远程部署 | 需要网络配置、安全考虑 | 分布式、插件集群 |
| **Unix Socket** | 本机高性能、安全 | 只能本机 | 本机高性能场景 |

## 9. 远程插件集群架构

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  sshpiperd  │◄───►│ Plugin LB   │◄───►│ Plugin Node │
│  (主程序)    │     │ (可选)       │     │ (gRPC Server)│
└─────────────┘     └─────────────┘     └─────────────┘
                                               │
                                          ┌────┴────┐
                                          ▼         ▼
                                    ┌─────────┐ ┌─────────┐
                                    │Plugin-1 │ │Plugin-2 │
                                    └─────────┘ └─────────┘
```

## 10. 总结

- **协议**: gRPC
- **默认传输**: stdin/stdout 管道（无网络端口）
- **角色**: 主程序 = Client，插件 = Server
- **发现**: `ListCallbacks` 查询插件支持的功能
- **认证**: Password/PublicKey/KeyboardInteractive 等回调
- **路由**: 返回 `Upstream` 指定目标服务器
- **链式**: 支持 `--` 分隔的多插件链
- **扩展**: 原生支持网络通信（TCP/Unix Socket）
