package androidlib

import "fmt"

// Hello 返回一个问候字符串，用于验证 gomobile bind 能成功生成 .aar
// gomobile bind 要求：package 名不能是 main，导出函数首字母大写
func Hello(name string) string {
	return fmt.Sprintf("Hello, %s! From Go core via gomobile.", name)
}
