package downloader

import (
	"context"
	"os" // 需要添加这个导入

	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram/downloader"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/iyear/tdl/core/dcpool"
	"github.com/iyear/tdl/core/logctx"
	"github.com/iyear/tdl/core/util/tutil"
)

// MaxPartSize refer to https://core.telegram.org/api/files#downloading-files
const MaxPartSize = 1024 * 1024

type Downloader struct {
	opts Options
}

type Options struct {
	Pool     dcpool.Pool
	Threads  int
	Iter     Iter
	Progress Progress
	// ===== 新增字段 =====
	Stdout   bool // 是否输出到 stdout
}

func New(opts Options) *Downloader {
	return &Downloader{
		opts: opts,
	}
}

func (d *Downloader) Download(ctx context.Context, limit int) error {
	wg, wgctx := errgroup.WithContext(ctx)
	wg.SetLimit(limit)

	for d.opts.Iter.Next(wgctx) {
		elem := d.opts.Iter.Value()

		wg.Go(func() (rerr error) {
			d.opts.Progress.OnAdd(elem)
			defer func() { d.opts.Progress.OnDone(elem, rerr) }()

			if err := d.download(wgctx, elem); err != nil {
				// canceled by user, so we directly return error to stop all
				if errors.Is(err, context.Canceled) {
					return errors.Wrap(err, "download")
				}

				// don't return error, just log it
				logctx.
					From(ctx).
					Error("Download error",
						zap.Any("element", elem),
						zap.Error(err),
					)
			}

			return nil
		})
	}

	if err := d.opts.Iter.Err(); err != nil {
		return errors.Wrap(err, "iter")
	}

	return wg.Wait()
}

func (d *Downloader) download(ctx context.Context, elem Elem) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	logctx.From(ctx).Debug("Start download elem",
		zap.Any("elem", elem))

	client := d.opts.Pool.Client(ctx, elem.File().DC())
	if elem.AsTakeout() {
		client = d.opts.Pool.Takeout(ctx, elem.File().DC())
	}

	// ===== 关键修改：根据 Stdout 选项选择不同的 writer =====
	var writer downloader.WriteAtFunc
	if d.opts.Stdout {
		writer = newStdoutWriteAt(elem, d.opts.Progress, MaxPartSize)
	} else {
		writer = newWriteAt(elem, d.opts.Progress, MaxPartSize)
	}

	_, err := downloader.NewDownloader().WithPartSize(MaxPartSize).
		Download(client, elem.File().Location()).
		WithThreads(tutil.BestThreads(elem.File().Size(), d.opts.Threads)).
		Parallel(ctx, writer)
	if err != nil {
		return errors.Wrap(err, "download")
	}

	return nil
}

// ===== 新增：stdout writer 函数 =====
// newStdoutWriteAt 创建一个写入标准输出的 WriteAt 函数
func newStdoutWriteAt(elem Elem, prog Progress, partSize int) downloader.WriteAtFunc {
	return func(p []byte, offset int64) (n int, err error) {
		// 直接写入标准输出
		n, err = os.Stdout.Write(p)
		if err == nil {
			// 更新进度
			prog.OnWrite(elem, partSize, n)
		}
		return n, err
	}
}
