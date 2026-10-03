package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var (
	target_folder = "images"
)

func main() {
	current_dir, _ := os.Getwd()
	target_dir, _ := filepath.Abs(target_folder)
	fmt.Printf("Current working directory: %s\n", current_dir)
	fmt.Printf("Target folder: %s\n", target_dir)

	entries, err := os.ReadDir(target_dir)
	if err != nil {
		log.Fatalf("Can't read target folder: %v\n", err)
	}
	file_count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			file_count++
		}
	}
	fmt.Printf("Number of images in target folder: %d\n", file_count)
}
