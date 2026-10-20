// 14 组样张矩阵（plan/04 §二）+ align golden 的两对输入。
//
// 每组的"注入项"直接写在 `Injected` 与 `Expect` 里；容器字节由 cases.go 渲染。
// 纪律：期望值只从注入项推出，不由工具跑出来的观测值回填（plan/04 §四）。
package main

func flipOneByte(b []byte) []byte {
	out := append([]byte(nil), b...)
	if len(out) == 0 {
		return out
	}
	// 只翻最后一个字节：长度不变、差异字节数恰为 1（residue_bytes 的手算依据）
	out[len(out)-1] ^= 0x01
	return out
}

// pythonFileData 取逻辑树里某个路径的内容（真改注入用）。
func pythonFileData(files []fileSpec, path string) []byte {
	for _, f := range files {
		if f.Path == path {
			return f.Data
		}
	}
	return nil
}

func caseDefs() []caseDef {
	return []caseDef{
		{
			Name:     "c01-mtime",
			Title:    "两侧 tar 仅 mtime 不同",
			Format:   "tar",
			Files:    pythonFiles(),
			Left:     baseSide(),
			Right:    withMTimes(baseSide(), 1700003600),
			Injected: []string{"T1"},
			Manual:   "tar -tvf pair/left/" + synthName + "-" + synthVersion + ".tar   # 观察 mtime 列",
			Expect:   metaOnly([]string{"T1"}),
		},
		{
			Name:   "c02-owner",
			Title:  "两侧 tar 仅 uid/gid/uname/gname 不同",
			Format: "tar",
			Files:  pythonFiles(),
			Left:   baseSide(),
			Right: func() sideSpec {
				s := baseSide()
				s.UID, s.GID = 0, 0
				s.Uname, s.Gname = "root", "root"
				return s
			}(),
			Injected: []string{"T2"},
			Manual:   "tar -tvf pair/left/" + synthName + "-" + synthVersion + ".tar   # 观察 owner 列",
			Expect:   metaOnly([]string{"T2"}),
		},
		{
			Name:   "c03-mode",
			Title:  "两侧 tar 仅文件权限不同（0644 vs 0755）",
			Format: "tar",
			Files:  pythonFiles(),
			Left:   baseSide(),
			Right: func() sideSpec {
				s := baseSide()
				s.FileMode = 0o755
				return s
			}(),
			Injected: []string{"T3"},
			Manual:   "tar -tvf pair/left/" + synthName + "-" + synthVersion + ".tar   # 观察权限列",
			Expect:   metaOnly([]string{"T3"}),
		},
		{
			Name:   "c04-prefix",
			Title:  "条目根前缀不同（build-a/ vs build-b/）",
			Format: "tar",
			Files:  pythonFiles(),
			Left: func() sideSpec {
				s := baseSide()
				s.Prefix = "build-a/"
				return s
			}(),
			Right: func() sideSpec {
				s := baseSide()
				s.Prefix = "build-b/"
				return s
			}(),
			Injected: []string{"T4"},
			Manual:   "tar -tf pair/left/" + synthName + "-" + synthVersion + ".tar   # 条目都以 build-a/ 开头",
			Expect:   metaOnly([]string{"T4"}),
		},
		{
			Name:   "c05-order",
			Title:  "条目集合相同、tar 内存储顺序不同",
			Format: "tar",
			Files:  pythonFiles(),
			Left:   baseSide(),
			Right: func() sideSpec {
				s := baseSide()
				s.Reverse = true
				return s
			}(),
			Injected: []string{"T5"},
			Manual:   "tar -tf pair/left/" + synthName + "-" + synthVersion + ".tar   # 与右侧的条目顺序相反",
			Expect:   metaOnly([]string{"T5"}),
		},
		{
			Name:   "c06-gzip-opts",
			Title:  "tar.gz 的 gzip 成员头 xfl/os 不同",
			Format: "tar.gz",
			Files:  pythonFiles(),
			Left:   baseSide(),
			Right: func() sideSpec {
				s := baseSide()
				s.GzipXFL = 4 // 最快
				s.GzipOS = 0  // FAT
				return s
			}(),
			Injected: []string{"T6"},
			Manual:   "od -An -tx1 -N10 pair/left/" + synthName + "-" + synthVersion + ".tar.gz   # 第 9/10 字节 = xfl/os",
			Expect:   metaOnly([]string{"T6"}),
		},
		{
			Name:   "c07-archive-hdr",
			Title:  "zip 的 create_system（归档级字段）不同",
			Format: "zip",
			Files:  pythonFiles(),
			Left:   baseSide(),
			Right: func() sideSpec {
				s := baseSide()
				s.Zip.CreateSystem = 0 // FAT / MS-DOS
				return s
			}(),
			Injected: []string{"T7"},
			Manual:   "unzip -l pair/left/" + synthName + "-" + synthVersion + ".zip   # 条目一致，差异在中央目录的 version-made-by",
			Expect:   metaOnly([]string{"T7"}),
		},
		{
			Name:   "c08-toolchain",
			Title:  "ELF 的 .comment 工具链串不同",
			Format: "tar",
			Files:  elfFiles(),
			Left:   baseSide(),
			Right: func() sideSpec {
				s := baseSide()
				s.ELF.Comment = "GCC: (GNU) 13.3.0"
				return s
			}(),
			Injected: []string{"T8"},
			Manual:   "strings -a pair/left/" + synthName + "-" + synthVersion + ".tar | grep GCC",
			Expect:   metaOnly([]string{"T8"}),
		},
		{
			Name:   "c09-mtime-owner",
			Title:  "mtime + 属主 同时不同",
			Format: "tar",
			Files:  pythonFiles(),
			Left:   baseSide(),
			Right: func() sideSpec {
				s := withMTimes(baseSide(), 1700003600)
				s.UID, s.GID = 0, 0
				s.Uname, s.Gname = "root", "root"
				return s
			}(),
			Injected: []string{"T1", "T2"},
			Manual:   "tar -tvf pair/left/" + synthName + "-" + synthVersion + ".tar   # 同时看 mtime 与 owner 两列",
			Expect:   metaOnly([]string{"T1", "T2"}),
		},
		{
			Name:   "c10-toolchain-path",
			Title:  "ELF 的 .comment（T8）与 DWARF comp_dir（T11）不同",
			Format: "tar",
			Files:  elfFiles(),
			Left:   baseSide(),
			Right: func() sideSpec {
				s := baseSide()
				s.ELF.Comment = "GCC: (GNU) 13.3.0"
				s.ELF.CompDir = "/build/right"
				return s
			}(),
			Injected: []string{"T8", "T11"},
			Manual:   "strings -a pair/right/" + synthName + "-" + synthVersion + ".tar | grep /build/",
			Expect:   metaOnly([]string{"T8", "T11"}),
		},
		{
			Name:   "c11-real-change",
			Title:  "一处内容字节真改（T12，无最小原因集）",
			Format: "tar.gz",
			Files:  pythonFiles(),
			Left:   baseSide(),
			Right: func() sideSpec {
				s := baseSide()
				s.Overrides = map[string][]byte{
					"synthpkg/data/table.csv": flipOneByte(pythonFileData(pythonFiles(), "synthpkg/data/table.csv")),
				}
				return s
			}(),
			Injected: []string{"T12"},
			Manual: "rm -rf /tmp/bdiff-c11-l /tmp/bdiff-c11-r && mkdir -p /tmp/bdiff-c11-l /tmp/bdiff-c11-r " +
				"&& tar -xzf pair/left/" + synthName + "-" + synthVersion + ".tar.gz -C /tmp/bdiff-c11-l " +
				"&& tar -xzf pair/right/" + synthName + "-" + synthVersion + ".tar.gz -C /tmp/bdiff-c11-r " +
				"&& cmp -l /tmp/bdiff-c11-l/synthpkg/data/table.csv /tmp/bdiff-c11-r/synthpkg/data/table.csv | wc -l   # = 1",
			Expect: expectBody{
				CountsPositive:  []string{"T12"},
				CountsAllowed:   []string{"T12"},
				MinimalCauseSet: nil, // 无解：真改不可归一化
				ResidueBytes:    1,
				ExitCode:        3,
			},
		},
		{
			Name:   "c12-mixed",
			Title:  "T1 + T3 + T9 + 一处真改",
			Format: "tar",
			Files:  elfFiles(),
			Left:   baseSide(),
			Right: func() sideSpec {
				s := withMTimes(baseSide(), 1700003600)
				s.FileMode = 0o755
				s.ELF.BuildID = filler("build-id-right", 20)
				s.Overrides = map[string][]byte{
					"synthpkg/data/table.csv": flipOneByte(pythonFileData(pythonFiles(), "synthpkg/data/table.csv")),
				}
				return s
			}(),
			Injected: []string{"T1", "T3", "T9", "T12"},
			Manual: "tar -xOf pair/right/" + synthName + "-" + synthVersion + ".tar synthpkg/bin/synthpkg | strings -a | grep -c build " +
				"|| true   # build-id 段存在，但两侧内容不同",
			// §4.5：有真改 ⇒ 任何 S 都无法让树摘要相等 ⇒ 最小集必为 null。
			// counts_allowed 限定"除 T12 外只出现可解释类别"（plan/04 §二 c12 列）。
			Expect: expectBody{
				CountsPositive:  []string{"T1", "T3", "T9", "T12"},
				CountsAllowed:   []string{"T1", "T3", "T9", "T12"},
				MinimalCauseSet: nil,
				ResidueBytes:    1,
				ExitCode:        3,
			},
		},
		{
			Name:   "c13-signature",
			Title:  "ELF 签名段存在性差异（T10）",
			Format: "tar",
			Files:  elfFiles(),
			Left: func() sideSpec {
				s := baseSide()
				s.ELF.Signature = filler("signature", 64)
				return s
			}(),
			Right:    baseSide(),
			Injected: []string{"T10"},
			Manual:   "tar -xOf pair/left/" + synthName + "-" + synthVersion + ".tar synthpkg/bin/synthpkg | grep -c '\\.sig'",
			Expect:   metaOnly([]string{"T10"}),
		},
		{
			// M2：raw_header_sha256 的兜底反例——左注入一个**未知 PAX keyword**
			// （SCHILY.xattr.*），解析器不解释它，但必须把它计入 raw_header_sha256。
			Name:   "c14-pax-unknown",
			Title:  "PAX 未知扩展头 keyword（raw_header_sha256 兜底反例，T7）",
			Format: "tar",
			Files:  pythonFiles(),
			Left: func() sideSpec {
				s := baseSide()
				s.PAX = map[string][]string{
					"synthpkg/METADATA": {"SCHILY.xattr.user.marker=fixture-unknown"},
				}
				return s
			}(),
			Right:    baseSide(),
			Injected: []string{"T7"},
			Manual: "tar --xattrs -tvf pair/left/" + synthName + "-" + synthVersion + ".tar   # 左含 PAX 扩展头\n" +
				"strings -a pair/left/" + synthName + "-" + synthVersion + ".tar | grep SCHILY.xattr.user.marker   # = fixture-unknown",
			Expect: metaOnly([]string{"T7"}),
		},
	}
}

func withMTimes(s sideSpec, mtime int64) sideSpec {
	s.MTime = mtime
	return s
}

// metaOnly 是"仅元数据差异"样张的期望模板：exit 2、无残留、最小集 = 注入项。
func metaOnly(rules []string) expectBody {
	return expectBody{
		CountsPositive:  rules,
		CountsAllowed:   rules,
		MinimalCauseSet: rules,
		ResidueBytes:    0,
		ExitCode:        2,
	}
}

// goldenPair 是 align golden 的输入对（目录形态，便于手工复算）。
type goldenPair struct {
	Name  string
	Left  []fileSpec
	Right []fileSpec
}

func goldenPairs() []goldenPair {
	return []goldenPair{
		{
			Name:  "rename-add",
			Left:  alignGoldenFiles(),
			Right: alignGoldenRight(),
		},
		{
			Name: "dup-hash",
			Left: []fileSpec{
				{Path: "pkg/a.txt", Data: []byte("same on both sides\n")},
				{Path: "pkg/copy1.bin", Data: []byte("DUPLICATED-PAYLOAD\n")},
				{Path: "pkg/copy2.bin", Data: []byte("DUPLICATED-PAYLOAD\n")},
			},
			Right: []fileSpec{
				{Path: "pkg/a.txt", Data: []byte("same on both sides\n")},
				{Path: "pkg/moved.bin", Data: []byte("DUPLICATED-PAYLOAD\n")},
			},
		},
	}
}
