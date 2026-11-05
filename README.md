# dezip

独立的 Go 解压与解压缩包，使用 [`mholt/archives`](https://github.com/mholt/archives) 识别并读取归档格式。

```go
result, err := dezip.Unpack(ctx, "release.tar.gz", "./unpacked", dezip.Options{})
if err != nil {
	return err
}
fmt.Printf("%s: %d files, %d bytes\n", result.Format, result.Files, result.ExpandedBytes)
```

## 命令行调用

命令会自动识别格式并调用 `dezip.Unpack`。输出目录必须不存在：

```powershell
go run ./cmd/dezip -src .\release.tar.gz -dest .\release
```

也支持位置参数：`go run ./cmd/dezip .\release.zip .\release`。

源码按 API、格式识别、解包流程、路径校验、条目写入、链接处理和流复制职责拆分。

目标目录必须尚不存在。程序会先解到同级临时目录，全部成功后再重命名为目标目录；失败时删除临时内容。

## 格式

归档格式：ZIP、TAR、RAR（只读）、7z（只读），以及库支持的压缩 TAR 组合格式，例如 TAR.GZ。

单流压缩格式：Brotli、BZIP2、Flate/ZIP、GZIP、LZ4、lzip、minlz、Snappy/S2、XZ、zlib 和 Zstandard。压缩流会输出为一个文件；默认从输入文件名去除压缩扩展名，也可通过 `Options.OutputName` 指定文件名。

## 安全限制

- 默认最多处理 100,000 个条目、最多展开 1 GiB；可通过 `Options` 调整。
- 拒绝绝对路径、路径穿越、Windows 保留名称、特殊文件和重复/冲突路径。
- 默认拒绝符号链接。启用 `AllowSymlinks` 后，只接受相对且词法上位于输出目录内的链接；Windows 创建链接仍可能受系统权限限制。
- 支持取消、剥离路径前缀和进度回调。

```go
options := dezip.Options{
	MaxEntries:       20_000,
	MaxExpandedBytes: 512 << 20,
	StripComponents:  1,
	OnProgress: func(p dezip.Progress) {
		fmt.Printf("%s: %d/%d\n", p.Name, p.Files, p.Entries)
	},
}
_, err := dezip.Unpack(ctx, "release.zip", "./stage", options)
```
