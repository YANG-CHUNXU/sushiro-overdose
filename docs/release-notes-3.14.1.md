# v3.14.1

## fix: macOS 隔离标记提示的命令路径未加引号

隔离标记提示卡里生成的 `xattr` 命令直接拼了可执行文件全路径，而 macOS 上路径含空格（`Sushiro Overdose.app`）且没加引号，粘到终端会被空格拆断：

```
xattr: No such file: /Applications/Sushiro
xattr: No such file: Overdose.app/Contents/MacOS/sushiro
```

修复：

- 路径用双引号包住，空格不再被当作参数分隔符。
- 改为对 **`.app` 包**移除隔离标记（标记在整个包上），而不是包内的可执行文件——和 README 里的写法一致。

> 仅前端 `web_static.go` 一行改动。README/docs 里的命令本来就是带引号的，不受影响。
