# dezip

独立的 Go 解压包。默认构建只支持 ZIP，并使用 Go 标准库读取归档；`dezip_full` 构建 tag 启用 [`mholt/archives`](https://github.com/mholt/archives) 支持的完整格式。`Unpack` 用于明确要求解压；`Process` 兼容 DASM 风格的 `extract_as` 配置，可选择保留下载文件或解压。

```go
result, err := dezip.Unpack(ctx, "release.zip", "./unpacked", dezip.Options{})
if err != nil {
	return err
}
fmt.Printf("%s: %d files, %d bytes\n", result.Format, result.Files, result.ExpandedBytes)
```

## 下载文件处理模式

`Process` 接收已下载的文件，不负责网络下载。它支持 `auto`、`zip`、`tar`、`gzip`、`skip` 这些 API 模式。默认构建只实际解压 ZIP；其他格式需要 `dezip_full` tag。空值表示保留原文件；完整格式构建中，`tar.gz`、`tar.xz` 等 TAR 压缩值映射到 TAR 模式。`gzip` 解出一个文件，归档模式解出完整目录。`exe` 不属于支持值，会返回无效模式错误。

```go
mode, err := dezip.ParseExtractAs(extractAs) // 空值、skip -> 不解压
if err != nil {
	return err
}
result, err := dezip.Process(ctx, downloadedPath, outputDir, mode, dezip.Options{})
if err != nil {
	return err
}
if result.Skipped {
	useDownloadedFile(result.SourcePath)
} else {
	useExtractedDirectory(result.OutputDir)
	if result.OutputFile != "" {
		usePrimaryFile(result.OutputFile)
	}
}
```

Set `Options.ExpectedFile` to require a relative regular file before the staged output directory is published. In full format builds, it must match the single output file name for raw compressed streams.

- `skip` 保留下载文件原样，不需要输出目录。
- 默认构建的 `auto` 只识别 ZIP；普通非归档文件保留原样。
- 默认构建支持 `zip`。带 `dezip_full` tag 时，`auto`、`tar` 和 `gzip` 也可解压其支持的格式。
- 归档解到一个新目录，保留目录层级，可包含一个或多个文件；完整格式构建中的单流压缩格式会在该目录内生成一个文件。
- 解压失败时目标目录不会发布，下载文件保留供重试。

## 命令行调用

命令会自动识别格式并调用 `dezip.Process`。默认只识别 ZIP。输出目录必须不存在；`skip` 不需要输出目录：

```powershell
go run ./cmd/dezip -src .\release.zip -dest .\release
```

启用完整格式支持：

```powershell
go run -tags=dezip_full ./cmd/dezip -src .\release.tar.gz -dest .\release
```

使用 dezip 的应用可以同样通过 `go build -tags=dezip_full` 启用完整格式。默认构建的二进制不链接完整格式实现；完整格式依赖仍保留在模块依赖图中。

保留单个下载文件：

```powershell
go run ./cmd/dezip -src .\plugin.exe -as skip
```

也支持位置参数：`go run ./cmd/dezip .\release.zip .\release`。

源码按 API、格式识别、解包流程、路径校验、条目写入、链接处理和流复制职责拆分。

目标目录必须尚不存在。程序会先解到同级临时目录，全部成功后再重命名为目标目录；失败时删除临时内容。

## 格式

默认构建：ZIP。`dezip_full` 构建：ZIP、TAR、RAR（只读）、7z（只读），以及库支持的压缩 TAR 组合格式，例如 TAR.GZ。

`dezip_full` 单流压缩格式：Brotli、BZIP2、Flate/ZIP、GZIP、LZ4、lzip、minlz、Snappy/S2、XZ、zlib 和 Zstandard。压缩流会输出为一个文件；默认从输入文件名去除压缩扩展名，也可通过 `Options.OutputName` 指定文件名。

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
