package androidlib

import "fmt"

// Hello returns a greeting string used by the gomobile bind smoke test.
func Hello(name string) string {
	return fmt.Sprintf("Hello, %s! From Go core via gomobile.", name)
}
