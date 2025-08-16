package internal

import "os"

func fib(i int) int {
	if i <= 1 {
		return i
	}
	return fib(i-1) + fib(i-2)
}

func countFilesInDir(dirPath string) int {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return 0
	}
	return len(files)
}
