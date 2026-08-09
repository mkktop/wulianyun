package gateway

import (
	"bufio"
	"errors"
	"strings"
	"testing"
)

// TestReadRegLine 正常注册行：去空白、含换行。
func TestReadRegLine(t *testing.T) {
	r := bufio.NewReaderSize(strings.NewReader("GY7638063548,mkk_S3,secret123\n"), 1024)
	line, err := readRegLine(r)
	if err != nil {
		t.Fatalf("readRegLine err: %v", err)
	}
	if line != "GY7638063548,mkk_S3,secret123" {
		t.Errorf("got %q", line)
	}
}

// TestReadRegLineNoNewline 单缓冲内无换行也返回（数据留在缓冲供后续读取）。
func TestReadRegLineNoNewline(t *testing.T) {
	// 无换行的注册包（随后紧跟数据帧）
	r := bufio.NewReaderSize(strings.NewReader("GY76,mkk,secret"), 1024)
	line, err := readRegLine(r)
	if err == nil {
		t.Errorf("无换行应返回 EOF，got line=%q err=nil", line)
	}
}

// TestReadRegLineTooLong 连续无换行字节流不得超过单缓冲：拒绝且不膨胀内存。
func TestReadRegLineTooLong(t *testing.T) {
	// 2KB 无换行数据，超过 1KB 缓冲 → ErrBufferFull → errRegLineTooLong
	r := bufio.NewReaderSize(strings.NewReader(strings.Repeat("A", 2048)), 1024)
	_, err := readRegLine(r)
	if !errors.Is(err, errRegLineTooLong) {
		t.Fatalf("期望 errRegLineTooLong，得到 %v", err)
	}
}

// TestReadRegLinePreservesBuffered 超长拒绝后，未超限的合法注册包仍可被读取
// （ReadSlice 不破坏 reader 状态，注册后的数据循环依赖此行为）。
func TestReadRegLinePreservesBuffered(t *testing.T) {
	input := "pk,dn,secret\nPING\n"
	r := bufio.NewReaderSize(strings.NewReader(input), 1024)
	line, err := readRegLine(r)
	if err != nil {
		t.Fatalf("readRegLine err: %v", err)
	}
	if line != "pk,dn,secret" {
		t.Errorf("got %q", line)
	}
	// 后续数据仍可继续从同一 reader 读取
	rest, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("后续读取 err: %v", err)
	}
	if rest != "PING\n" {
		t.Errorf("缓冲数据未保留，got %q", rest)
	}
}
