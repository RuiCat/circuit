//go:build unix

// linkUnix.go 是 Link 的 Unix 实现：把已打开的 tty 文件包装成带 poll 期限的字节通道。
//
// 为什么不用 os.File 的 SetReadDeadline：本环境下 pty 描述符注册进运行期 poller 会失败
// （epoll 注册被拒），os.File 会把描述符退回阻塞模式，SetReadDeadline 要么报
// "file type does not support deadline"、要么设了不生效而让 read 永久阻塞（实测挂死 60s）。
// 因此 PtyLink 自己用 unix.Poll 提供期限：读/写都先 poll 再 syscall，超时必然可判定。
package serialbridge

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// pollAttempts 是 EINTR 打断后的有界重试次数：不无限重试，也不把信号当成超时。
const pollAttempts = 4

// PtyLink 把已打开的 tty 文件包装成带超时的字节通道。
type PtyLink struct {
	file        *os.File
	fd          int
	timeout     time.Duration
	nonblockErr error // Fd() 之后显式置回非阻塞失败的原因（非 nil 时所有操作先报错）
}

// NewPtyLink 用已打开的 tty 文件构造通道；timeout ≤ 0 时取 DefaultReadTimeout。
// 构造时缓存描述符并置为非阻塞：os.File.Fd() 会把描述符改回阻塞模式，之后一律不经 os.File 读写。
func NewPtyLink(file *os.File, timeout time.Duration) *PtyLink {
	if timeout <= 0 {
		timeout = DefaultReadTimeout
	}
	link := &PtyLink{file: file, fd: -1, timeout: timeout}
	if file != nil {
		link.fd = int(file.Fd())
		link.nonblockErr = unix.SetNonblock(link.fd, true)
	}
	return link
}

// ready 返回可用的描述符；已关闭、无效或非阻塞设置失败时返回可判定错误。
func (l *PtyLink) ready() (int, error) {
	if l.file == nil || l.fd < 0 {
		return -1, ErrLinkClosed
	}
	if l.nonblockErr != nil {
		return -1, fmt.Errorf("serialbridge: 置非阻塞失败: %w", l.nonblockErr)
	}
	return l.fd, nil
}

// timeoutMs 把超时换算成 poll 的毫秒参数（向上取整，至少 1ms）。
func (l *PtyLink) timeoutMs() int {
	ms := int((l.timeout + time.Millisecond - 1) / time.Millisecond)
	if ms <= 0 {
		ms = 1
	}
	return ms
}

// ReadByte 读 1 字节：期限内无数据 → ErrTimeout；对端关闭（POLLHUP/EIO/EOF）→ ErrLinkClosed。
func (l *PtyLink) ReadByte() (byte, error) {
	fd, err := l.ready()
	if err != nil {
		return 0, err
	}
	pfds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	ready := 0
	for attempt := 0; attempt < pollAttempts; attempt++ {
		n, perr := unix.Poll(pfds, l.timeoutMs())
		if perr == unix.EINTR {
			continue // 被信号打断不是故障，重试有界次数
		}
		if perr != nil {
			return 0, fmt.Errorf("serialbridge: poll 读等待失败: %w", perr)
		}
		ready = n
		break
	}
	if ready == 0 {
		return 0, ErrTimeout
	}
	if pfds[0].Revents&unix.POLLIN == 0 {
		if pfds[0].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			return 0, ErrLinkClosed
		}
		return 0, ErrTimeout
	}
	var buf [1]byte
	n, rerr := unix.Read(fd, buf[:])
	if rerr != nil {
		if rerr == unix.EAGAIN {
			return 0, ErrTimeout
		}
		return 0, fmt.Errorf("%w: %v", ErrLinkClosed, rerr)
	}
	if n == 0 {
		return 0, ErrLinkClosed
	}
	return buf[0], nil
}

// WriteBytes 全量写出 p：期限内写不出去 → ErrTimeout；写失败 → ErrLinkClosed。
func (l *PtyLink) WriteBytes(p []byte) (int, error) {
	fd, err := l.ready()
	if err != nil {
		return 0, err
	}
	written := 0
	for written < len(p) {
		n, werr := unix.Write(fd, p[written:])
		if werr == unix.EAGAIN {
			if perr := l.waitWritable(fd); perr != nil {
				return written, perr
			}
			continue
		}
		if werr == unix.EINTR {
			continue
		}
		if werr != nil {
			return written, fmt.Errorf("%w: %v", ErrLinkClosed, werr)
		}
		written += n
	}
	return written, nil
}

// waitWritable 在有期限内等待描述符可写；超时 → ErrTimeout，挂断 → ErrLinkClosed。
func (l *PtyLink) waitWritable(fd int) error {
	pfds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLOUT}}
	for attempt := 0; attempt < pollAttempts; attempt++ {
		n, perr := unix.Poll(pfds, l.timeoutMs())
		if perr == unix.EINTR {
			continue
		}
		if perr != nil {
			return fmt.Errorf("serialbridge: poll 写等待失败: %w", perr)
		}
		if n == 0 {
			return ErrTimeout
		}
		if pfds[0].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			return ErrLinkClosed
		}
		return nil
	}
	return ErrTimeout
}

// Close 关闭底层 tty 文件；重复关闭返回 ErrLinkClosed（不 panic）。
func (l *PtyLink) Close() error {
	if l.file == nil {
		return ErrLinkClosed
	}
	err := l.file.Close()
	l.file = nil
	l.fd = -1
	if err != nil && errors.Is(err, os.ErrClosed) {
		return ErrLinkClosed
	}
	return err
}
