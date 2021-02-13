# 2021 辛丑年『解谜红包』 - Victor

祝神仙们牛年牛气冲天、 offer 拿满、顶会 n 篇~

## 题面
![challenge-screenshot](challenge-screenshot.png)

> 互联网（“Internet”）由词“互联”（Inter）和“网络”（Network）组成。而扮演“互联”作用的也就是“路由器”（Router）。
>
> 根据合理推测，你的请求经过了 11 个路由器到达我，且剩余可被传播的距离为 53 。如果可以让剩余的距离超过 200 ，我就将红包密码告诉你！
>

题目信息还包含网页的一小段 JavaScript 逻辑。

## 解法 & 思路

背景：在 IP 网络中，数据的发送方会在 IP 报文头部设置 TTL （Time-To-Live）字段。

IP 报文在客户端发出后，经过每一个 IP 路由节点[^ip_router]时，路由设备会将该报文中的 TTL 字段减去 1，接着：
- 如果减去 1 后 TTL 大于 0，则按既定规则将报文转发至下一台连接的路由器
- 如果减去 1 后 TTL 等于 0，则丢弃该包



参考 这篇文章 [^default_ttl_values] ，Windows 7 ~ Windows 10 的默认 TTL 为 128，常见 Linux/Unix-like (incl. Android, macOS) 发行版本为 64。仅有少数网络设备的默认 TTL 为 256。

假定客户端的请求需要经过 10 个路由节点到达网站服务器。

- Windows 下请求的 IP 报文到达网站服务器时 TTL 剩余 `(64-10)=54`
- Linux 下请求的 IP 保温到达网站服务器时 TTL 剩余 `(128-10)=118` 

因此，考虑一般只会通过 PC 或 macOS 桌面浏览器打开网页，故到达网站服务器时 IP 报文的 TTL 通常不会大于 200。



Linux 下标准解法：

```shell
~# curl http://101.32.26.208:8080
{"visited_routers":11,"remaining_routers":55,"rpCode":null}
~# echo 255 > /proc/sys/net/ipv4/ip_default_ttl
~# curl http://101.32.26.208:8080
{"visited_routers":11,"remaining_routers":245,"rpCode":"祝你牛气冲天0209"}
```

返回的红包码字段 `rpCode` 会更新在页面上。支付宝输入口令红包“Victor祝你牛气冲天0209”即可领取。



# 服务端设计

Source Code: [main.go](main.go)

由于需要在应用层（L7）感知 IP 数据面，在 Linux 上服务设计思路如下：

- 为了能响应客户端的请求，需要向内核申请 IP socket (创建`0.0.0.0:8089`套接字)，运行网页服务器

- 为了能得到请求对应的 IP 报文 TTL，需要捕获系统的 IP 报文并筛选发送到上述套接字的报文

  - 设置监听 `ip4:tcp` 
  - 过滤 TCP:Destination 为 8089 （避免发送到其他端口的数据也被识别，占用内存空间）
  - 过滤 ip4:Destination 为服务器网卡 IP （进一步筛选，避免其他无关数据被识别，占用 buffer 内存）
  - 过滤后，将 ip4:Source 和 TCP:Source 及对应的 TTL 做 hash mapping （例如: `123.210.5.9:34921 -> TTL: 100` ) 
  - 压入 buffer 和 Go Channel

- 网页服务器获得客户端的 ip4:Source + TCP:Source 二元组后，同时在 buffer 和 Go Channel 中查找对应的 TTL。当任一方式查找到后，返回 JSON 请求。

  ```json
  {"visited_routers":11,"remaining_routers":55,"rpCode":null}
  ```



[^default_ttl_values]: https://subinsb.com/default-device-ttl-values/
[^ip_router]: 当经过 MPLS 转发面 P 路由器时，IP 报文的 TTL 可能不会降低；因此强调为“IP路由节点”

