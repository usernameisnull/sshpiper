## 使用
本地启动, 默认监听端口2222
```cgo
sshpiperd /etc/sshpiperd/plugins/fixed --target 10.6.178.178:22334
```
本地ssh
```cgo
ssh 127.0.0.1 -l root -p 2222
The authenticity of host '[127.0.0.1]:2222 ([127.0.0.1]:2222)' can't be established.
ED25519 key fingerprint is SHA256:9yYoAW5FUUUcVIE+LpLJdGUxNeEgpphJz3MOcE6RREU.
This key is not known by any other names.
Are you sure you want to continue connecting (yes/no/[fingerprint])? yes
Warning: Permanently added '[127.0.0.1]:2222' (ED25519) to the list of known hosts.
root@127.0.0.1's password:
```

### 使用yaml插件实现publickey登录
使用yaml这个plugin来实现通过`10.6.178.179`登录`10.6.127.21`
- upstream机器: 10.6.127.21  
- 运行sshpiperd的机器: 10.6.178.179
- 10.6.178.179的公钥需要在10.6.127.21的~/.ssh/authorized_keys中
- 客户机(执行ssh命令的机器)的公钥需要在10.6.178.179的~/.ssh/authorized_keys中

10.6.178.179上的文件目录:  
```cgo
# 在10.6.178.179的文件目录
root@worker01:/etc/sshpiperd# eza -lT
drwxr-xr-x   - root 26 Jan 18:06 .
drwxr-xr-x   - root 23 Jan 16:24 ├── plugins
.rwxr-xr-x 15M root  1 Dec  2025 │   ├── docker
.rwxr-xr-x 11M root  1 Dec  2025 │   ├── failtoban
.rwxr-xr-x 11M root  1 Dec  2025 │   ├── fixed
.rwxr-xr-x 42M root  1 Dec  2025 │   ├── kubernetes
.rwxr-xr-x 11M root  1 Dec  2025 │   ├── username-router
.rwxr-xr-x 12M root  1 Dec  2025 │   ├── workingdir
.rwxr-xr-x 12M root  1 Dec  2025 │   └── yaml
.rw------- 262 root 26 Jan 18:03 └── sshpiperd.yaml
```
使用yaml这个plugin, authorized_keys和private_key是10.6.178.179上的文件
```bash
#cat sshpiperd.yaml
version: "1.0"
pipes:
- from:
    - username: "root"
      authorized_keys:
      - /root/.ssh/authorized_keys
  to:
    host: 10.6.127.21:22
    username: "root"
    ignore_hostkey: true
    private_key: /root/.ssh/id_rsa
```
10.6.178.179上执行如下命令, 默认监听端口: 2222
```cgo
sshpiperd /etc/sshpiperd/plugins/yaml --config /etc/sshpiperd/sshpiperd.yaml
```
在本地登录
```cgo
ssh 10.6.178.179 -p 2222
```

## 代码

### .proto
- 只有一个.proto文件: `libplugin/plugin.proto`
- .proto文件生成go文件: 在`libplugin/doc.go`里有这么一句 `//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative plugin.proto`
- `libplugin/plugin_grpc.pb.go`里的`RegisterSshPiperPluginServer`函数怎么生成的, 应该是自动生成的, 不需要对应的proto文件里有相关内容, 总是会根据.proto文件里的service生成类似的Register函数
- 生成代码: `/usr/local/go/bin/go generate -run protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative plugin.proto`
### 当plugin为二进制时
- `createCmdPlugin`
  - plugin.DialCmd(cmd)里用cmd.Wait()作为退出信号
- (d *daemon) install
- (d *daemon) run

### 当plugin在另一台机器上, 需要网络连接

## 插件协议
主程序(二进制)sshpiper和plugin(二进制)之间通过grpc(不是传统的ip:port的连接而是/dev/stdin和/dev/stdout)通信   
- 主程序作为grpc的客户端
```cgo
// func DialCmd(cmd *exec.Cmd) (*CmdPlugin, error)
conn, err := grpc.NewClient("127.0.0.1", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
		return cmdconn, nil
	}))
```
- plugin程序作为服务端
```cgo
// func NewFromGrpc(config SshPiperPluginConfig, grpc *grpc.Server, listener net.Listener) (SshPiperPlugin, error)
RegisterSshPiperPluginServer // 这里应该就是作为服务端
```

## libplugin/plugin.proto
- 生成镜像: 使用[build-proto-gen-image.sh](build-proto-gen-image.sh)
- 生成go文件: 使用proto-gen.sh