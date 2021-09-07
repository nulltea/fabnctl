package util

import "io"

type SizedReader interface {
	io.Reader
	Len() int
}
