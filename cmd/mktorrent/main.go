// mktorrent 从本地文件生成 .torrent 种子，并打印 magnet 链接。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gotorrent/internal/magnet"
	"gotorrent/internal/metainfo"
	"gotorrent/internal/util"
)

func main() {
	announce := flag.String("announce", "http://localhost:8080/announce", "Tracker 地址")
	comment := flag.String("comment", "", "备注")
	pieceLen := flag.Int("piece-length", metainfo.DefaultPieceLength, "分片大小（字节）")
	out := flag.String("o", "", "输出 .torrent 路径（默认 <文件名>.torrent）")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: mktorrent [选项] <文件>\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	src := flag.Arg(0)
	tf, err := metainfo.Create(src, *announce, *comment, *pieceLen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "生成种子失败: %v\n", err)
		os.Exit(1)
	}
	dst := *out
	if dst == "" {
		dst = src + ".torrent"
	}
	if err := tf.Save(dst); err != nil {
		fmt.Fprintf(os.Stderr, "写入种子失败: %v\n", err)
		os.Exit(1)
	}
	abs, _ := filepath.Abs(dst)
	m := magnet.Encode(tf.InfoHash, tf.Name, []string{tf.Announce}, tf.Length)
	fmt.Printf("文件:   %s (%s)\n", tf.Name, util.FormatBytes(tf.Length))
	fmt.Printf("分片:   %d × %s\n", tf.NumPieces(), util.FormatBytes(int64(tf.PieceLength)))
	fmt.Printf("Hash:   %s\n", util.EncodeHex20(tf.InfoHash))
	fmt.Printf("种子:   %s\n", abs)
	fmt.Printf("Magnet: %s\n", m)
}
