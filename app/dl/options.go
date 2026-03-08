package dl

type Options struct {
    Dir        string
    RewriteExt bool
    SkipSame   bool
    Template   string
    URLs       []string
    Files      []string
    Include    []string
    Exclude    []string
    Desc       bool
    Takeout    bool
    Group      bool

    // resume opts
    Continue, Restart bool

    // serve
    Serve bool
    Port  int

    // ===== 新增字段 =====
    Stdout bool // 是否输出到标准输出
}
