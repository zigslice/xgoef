//go:build !unix

package log

func noFollowFlag() int { return 0 }
