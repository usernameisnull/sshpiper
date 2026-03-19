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
deepwiki的理解: https://deepwiki.com/tg123/sshpiper

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
主程序(二进制)sshpiper和plugin(二进制)之间通过grpc(不是传统的ip:port的连接而是/dev/stdin和/dev/stdout)通信, 这种方式被称为`标准流通信`   
- 主程序作为grpc的客户端
```cgo
// func DialCmd(cmd *exec.Cmd) (*CmdPlugin, error)
conn, err := grpc.NewClient("127.0.0.1", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
		return cmdconn, nil
	}))
```
- plugin程序作为服务端
- demo: https://chatgpt.com/share/697afd9d-ec00-800d-96bb-9b1b867572eb
```cgo
// func NewFromGrpc(config SshPiperPluginConfig, grpc *grpc.Server, listener net.Listener) (SshPiperPlugin, error)
RegisterSshPiperPluginServer // 这里应该就是作为服务端
```

## libplugin/plugin.proto
- 生成镜像: 使用[build-proto-gen-image.sh](build-proto-gen-image.sh), 推送到了registry.cn-hangzhou.aliyuncs.com/mabing/sshpiper-ci:v0.1
- 生成go文件: 使用proto-gen.sh

### stream
在libplugin/plugin.proto文件里的service里有些字段为stream, 作用: https://www.qianwen.com/share/chat/4fb9961e4b28448eb66a478336a7d525

## debug
启动:
```cgo
./sshpiperd_linux ./fixed_linux --target 127.0.0.1:5522
- Grim reaper disabled, pid not 1
  INFO[0000] starting sshpiperd version: (devel), 47afcdacb, 2026-03-18T07:52:02Z, go1.26.0
  INFO[0000] found host keys [/etc/ssh/ssh_host_ed25519_key]
  INFO[0000] loading host key /etc/ssh/ssh_host_ed25519_key
  INFO[0000] starting child process plugin: [./fixed_linux --target 127.0.0.1:5522]
  INFO[0000] sshpiperd is listening on: [::]:2222

```
在另一个命令行查看, fixed_linux是作为sshpiperd_linux的子进程启动的
```cgo
➜ ps -ef|grep fixed_linux
root      463152  320973  1 17:32 pts/6    00:00:00 ./sshpiperd_linux ./fixed_linux --target 10.6.178.178:22334
root      463159  463152  1 17:32 pts/6    00:00:00 ./fixed_linux --target 10.6.178.178:22334
root      463180  459785  0 17:32 pts/4    00:00:00 grep --color=auto fixed_linux
```
查看2个进程的stdin, stdout, stderr
```cgo
➜ ls -l /proc/463159/fd
total 0
lr-x------ 1 root root 64 Mar 18 17:32 0 -> 'pipe:[345026]'
l-wx------ 1 root root 64 Mar 18 17:32 1 -> 'pipe:[345025]'
l-wx------ 1 root root 64 Mar 18 17:32 2 -> 'pipe:[345027]'
lr-x------ 1 root root 64 Mar 18 17:32 3 -> /sys/fs/cgroup/init.scope/cpu.max
lrwx------ 1 root root 64 Mar 18 17:32 5 -> 'anon_inode:[eventpoll]'
lrwx------ 1 root root 64 Mar 18 17:32 6 -> 'anon_inode:[eventfd]'

root in 󱃾 a223() ~ via  v24.1.0 via 🐍 v3.10.4
➜ ls -l /proc/463152/fd
total 0
lrwx------ 1 root root 64 Mar 18 17:33 0 -> /dev/pts/6
lrwx------ 1 root root 64 Mar 18 17:33 1 -> /dev/pts/6
l-wx------ 1 root root 64 Mar 18 17:33 10 -> 'pipe:[345026]'
lr-x------ 1 root root 64 Mar 18 17:33 11 -> 'pipe:[345027]'
lrwx------ 1 root root 64 Mar 18 17:33 15 -> 'anon_inode:[pidfd]'
lrwx------ 1 root root 64 Mar 18 17:33 2 -> /dev/pts/6
lr-x------ 1 root root 64 Mar 18 17:33 3 -> /sys/fs/cgroup/init.scope/cpu.max
lrwx------ 1 root root 64 Mar 18 17:33 4 -> 'socket:[345024]'
lrwx------ 1 root root 64 Mar 18 17:33 5 -> 'anon_inode:[eventpoll]'
lrwx------ 1 root root 64 Mar 18 17:33 6 -> 'anon_inode:[eventfd]'
lr-x------ 1 root root 64 Mar 18 17:33 7 -> 'pipe:[345025]'
```
- 通信示意图
```cgo
                  ┌──────────────────────┐
                  │  sshpiperd_linux     │
                  │  (PID 463152)        │
                  └─────────┬────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼

pipe:[345026]        pipe:[345025]        pipe:[345027]
(write fd=10)        (read fd=7)          (read fd=11)
    │                    ▲                    ▲
    │                    │                    │
    ▼                    │                    │

stdin (fd=0)        stdout (fd=1)        stderr (fd=2)
         ┌──────────────────────────────────────┐
         │         fixed_linux (463159)         │
         └──────────────────────────────────────┘
```
- 用strace抓取内容, 内容大多无法阅读
```cgo
strace -f -p 463152 -e trace=read,write -e read=10 -e write=7
```

### screen-recording-format
```cgo
./sshpiperd_linux --screen-recording-format typescript --screen-recording-dir ./recording ./fixed_linux --target 10.6.178.178:22334
```
在recording目录下的内容, 在.typescript后缀名的文件里居然展示了所有的内容
```cgo
root in 󱃾 a223(default) /tmp/sshpiper/recording
➜ ll
total 12K
drwxr-xr-x 3 root root 4.0K Mar 18 18:06 ./
drwxr-xr-x 3 root root 4.0K Mar 18 18:06 ../
drwx------ 2 root root 4.0K Mar 18 18:06 f7dbd5ed-3095-4a2c-afc5-a19edf4237ba/

root in 󱃾 a223(default) /tmp/sshpiper/recording
➜ cd f7dbd5ed-3095-4a2c-afc5-a19edf4237ba

root in 󱃾 a223(default) /tmp/sshpiper/recording/f7dbd5ed-3095-4a2c-afc5-a19edf4237ba
➜ ll
total 16K
drwx------ 2 root root 4.0K Mar 18 18:06 ./
drwxr-xr-x 3 root root 4.0K Mar 18 18:06 ../
-rw------- 1 root root   96 Mar 18 18:06 1773828396.timing
-rw------- 1 root root 2.2K Mar 18 18:06 1773828396.typescript

root in 󱃾 a223(default) /tmp/sshpiper/recording/f7dbd5ed-3095-4a2c-afc5-a19edf4237ba
➜ cat 1773828396.timing
0.764492 1098
0.042217 46
5.810277 1
0.160627 1
0.164734 11
0.002977 957
0.000384 8
0.000367 38

root in 󱃾 a223(default) /tmp/sshpiper/recording/f7dbd5ed-3095-4a2c-afc5-a19edf4237ba
➜ cat 1773828396.typescript
Script started on Wed Mar 18 18:06:36 2026
Welcome to Ubuntu 22.04.5 LTS (GNU/Linux 5.15.0-168-generic x86_64)

 * Documentation:  https://help.ubuntu.com
 * Management:     https://landscape.canonical.com
 * Support:        https://ubuntu.com/pro

 System information as of Wed Mar 18 06:06:36 PM CST 2026

  System load:  1.25                Processes:               328
  Usage of /:   27.7% of 196.55GB   Users logged in:         0
  Memory usage: 13%                 IPv4 address for ens160: 10.6.178.178
  Swap usage:   0%

 * Strictly confined Kubernetes makes edge and IoT secure. Learn how MicroK8s
   just raised the bar for easy, resilient and secure K8s cluster deployment.

   https://ubuntu.com/engage/secure-kubernetes-at-the-edge

Expanded Security Maintenance for Applications is not enabled.

31 updates can be applied immediately.
To see these additional updates run: apt list --upgradable

Enable ESM Apps to receive additional future security updates.
See https://ubuntu.com/esm or run: sudo pro status


*** System restart required ***
Last login: Wed Mar 18 18:04:48 2026 from 10.70.4.23
root@master01:~# ls
0324.log                    clean-charts                 cron_check_and_update_sts.sh  installer_dist         kuboard-data             renew-k8s-certs-100y.sh
all_tables_data.sql         create_kpanda_mgr.sh         datase-sample.tar.gz          job_set_read_only.sh   ls                       sample.yaml
all_tables_data.sql.tar.gz  create_mysql.sh              dump_2000.sql                 kpanda-mgr.log         mcamel                   snap
check_and_update_sts.sh     cri-dockerd                  es_kibana_install_deb         kpanda-mgr-monitor.sh  pytorch                  test
check_point                 cri-dockerd-0.3.7.amd64.tgz  gvm.sh                        kubekey                remove-networkpolicy.sh
root@master01:~#
```

## grpc调用
cat start_fixed.sh
```cgo
#!/bin/bash
./fixed_linux --target 10.6.178.178:22334
```
启动plugin
```cgo
socat TCP-LISTEN:12345,reuseaddr,fork EXEC:"./start_fixed.sh"
```
```cgo
root in 󱃾 a223() /mnt/d/github_repositories/sshpiper on  notes [!] via 🐹 v1.26.0 
➜ grpcurl -plaintext \
    -proto ./libplugin/plugin.proto \
    -import-path . \
    localhost:12345 \
    libplugin.SshPiperPlugin/ListCallbacks
{
  "callbacks": [
    "PasswordAuth"
  ]
}
```