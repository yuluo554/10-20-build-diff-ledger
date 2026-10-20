// Package artifact 负责把输入路径装载成内存中的条目流。
//
// 支持的形态（plan/03 §二）：目录、tar、tar.gz、zip、whl、jar、OCI layer tar。
// 纪律：解到内存、不落临时目录、不写输入目录、不做大小写折叠。
//
// M0 状态：类型与错误契约已定，装载实现在 M1 落地。
package artifact

import "errors"

// ErrNotImplemented 表示该能力尚未在对应里程碑落地。
// CLI 层把它映射为退出码 1（用法/输入不可解析），并在 stderr 指名里程碑。
var ErrNotImplemented = errors.New("尚未实现")

// Kind 是容器的形态。
type Kind string

const (
	KindDir   Kind = "dir"
	KindTar   Kind = "tar"
	KindTarGz Kind = "tar.gz"
	KindZip   Kind = "zip"
	KindOCI   Kind = "oci-layer-tar"
)

// Raw 是装载后的一个条目。Path 已规范化为 `/` 分隔、无前导 `./`。
type Raw struct {
	Path string
	Kind string // "file" | "dir" | "symlink"
	Size int64
	Data []byte // 仅 file 有内容；dir 为 nil
}

// Load 装载一个产物。M1 实现。
func Load(path string, layerKind string, maxEntryBytes int64) ([]Raw, Kind, error) {
	return nil, "", ErrNotImplemented
}
